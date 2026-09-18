// Copyright 2026 The MathWorks, Inc.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	artifactoryURL = "http://localhost:8082"
	serverURL      = "http://localhost:8080"
)

var (
	serverCmd *exec.Cmd
	perfDir   string
)

func main() {
	var err error
	perfDir, err = filepath.Abs(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: getting working directory: %v\n", err)
		os.Exit(1)
	}

	// Ensure we're in the perf-test directory
	if _, err := os.Stat("docker-compose.yaml"); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: must be run from the perf-test/ directory (docker-compose.yaml not found)\n")
		os.Exit(1)
	}

	code := run()
	os.Exit(code)
}

// run executes the full performance test pipeline and returns the exit code.
// It uses panic/recover so that deferred cleanup always runs, even on fatal errors.
func run() (exitCode int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", r)
			exitCode = 1
		}
		cleanup()
	}()

	packageCounts := getEnv("PERF_PACKAGE_COUNTS", "100,1000,10000")
	concurrencyLevels := getEnv("PERF_CONCURRENCY_LEVELS", "5,25,50")
	maxCount := maxFromCSV(packageCounts)

	fmt.Println("=== MATLAB Package Repository Performance Tests ===")
	fmt.Printf("  Package counts:     %s\n", packageCounts)
	fmt.Printf("  Concurrency levels: %s\n", concurrencyLevels)
	fmt.Printf("  Max seed count:     %d\n", maxCount)
	fmt.Println()

	// Clean stale state from any previous run
	os.Remove(filepath.Join(perfDir, "results.partial.tmp"))
	os.WriteFile(filepath.Join(perfDir, "results.json"), []byte{}, 0644)
	cmd := exec.Command("docker", "compose", "down", "-v")
	cmd.Dir = perfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	// Step 1: Generate credentials and start Docker
	step(1, "Generating credentials and starting Artifactory OSS + PostgreSQL...")
	masterKey := generateHex(32)
	joinKey := generateHex(32)
	postgresPassword := generateHex(16)
	adminPassword := "P!" + generateHex(16)
	writeEnvFile(masterKey, joinKey, postgresPassword)
	runCmd("docker", "compose", "up", "-d")
	fmt.Println()

	// Step 2: Initialize Artifactory
	step(2, "Initializing Artifactory...")
	waitForHealthy(artifactoryURL, 180*time.Second)
	changePassword(artifactoryURL, adminPassword)
	setBaseURL(artifactoryURL, adminPassword)
	token := generateToken(artifactoryURL, adminPassword)
	if err := os.WriteFile(filepath.Join(perfDir, ".service-token"), []byte(token), 0600); err != nil {
		fatal("writing .service-token: %v", err)
	}
	fmt.Println("  Init complete.")
	fmt.Println()

	// Step 3: Build server
	step(3, "Building mpm-repo-server...")
	serverBinary := "mpm-repo-server"
	if runtime.GOOS == "windows" {
		serverBinary = "mpm-repo-server.exe"
	}
	parentDir := filepath.Dir(perfDir)
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(perfDir, serverBinary), "./cmd/mpm-repo-server")
	buildCmd.Dir = parentDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		fatal("building server: %v", err)
	}
	fmt.Printf("  Built: ./%s\n", serverBinary)
	fmt.Println()

	// Step 4: Seed incrementally and run benchmarks at each scale level
	step(4, "Seeding and benchmarking at each scale...")
	counts := parseCSV(packageCounts)
	seeded := 0
	for _, count := range counts {
		fmt.Printf("\n  --- Scale: %d packages ---\n", count)

		// Seed to this level
		startServer(serverBinary, token)
		waitForServerHealthy(5 * time.Minute)

		toSeed := count - seeded
		if toSeed > 0 {
			seedCmd := exec.Command("go", "run", "seed.go",
				fmt.Sprintf("-count=%d", toSeed),
				fmt.Sprintf("-offset=%d", seeded),
				fmt.Sprintf("-server=%s", serverURL))
			seedCmd.Dir = perfDir
			seedCmd.Stdout = os.Stdout
			seedCmd.Stderr = os.Stderr
			seedCmd.Env = append(os.Environ(), "PERF_SERVICE_TOKEN="+token)
			if err := seedCmd.Run(); err != nil {
				fatal("seeding packages: %v", err)
			}
			seeded = count
		}
		stopServer()
		time.Sleep(time.Second)

		// P-1: Startup indexing at this scale
		fmt.Printf("  Measuring startup indexing...\n")
		countStr := strconv.Itoa(count)
		runTests(token, countStr, concurrencyLevels, "TestPerfStartupIndexing")

		// P-2, P-3, P-4, P-6: Query and refresh benchmarks at this scale
		fmt.Printf("  Running query and refresh benchmarks...\n")
		startServer(serverBinary, token)
		waitForServerHealthy(5 * time.Minute)
		runTests(token, countStr, concurrencyLevels, "TestPerfQuery|TestPerfAdminRefresh")
		stopServer()
		time.Sleep(time.Second)
	}
	fmt.Println()

	// Step 5: Publish and soak benchmarks (run once at max scale)
	step(5, "Running publish and soak benchmarks...")
	startServer(serverBinary, token)
	waitForServerHealthy(5 * time.Minute)
	runTests(token, packageCounts, concurrencyLevels, "TestPerfPublish|TestPerfSoak")
	fmt.Println()

	// Step 6: Write results
	step(6, "Writing results...")
	runTests(token, packageCounts, concurrencyLevels, "TestPerfWriteResults")
	fmt.Println()

	fmt.Println("=== Performance tests complete ===")
	return 0
}

// cleanup stops the server, tears down Docker containers/volumes, and deletes generated credentials.
func cleanup() {
	fmt.Println()
	fmt.Println("[cleanup] Stopping server...")
	stopServer()

	fmt.Println("[cleanup] Tearing down containers...")
	cmd := exec.Command("docker", "compose", "down", "-v")
	cmd.Dir = perfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()

	fmt.Println("[cleanup] Removing generated files...")
	os.Remove(filepath.Join(perfDir, ".env"))
	os.Remove(filepath.Join(perfDir, ".service-token"))
	os.Remove(filepath.Join(perfDir, "results.partial.tmp"))
	binary := "mpm-repo-server"
	if runtime.GOOS == "windows" {
		binary = "mpm-repo-server.exe"
	}
	os.Remove(filepath.Join(perfDir, binary))
	fmt.Println("[cleanup] Done.")
}

// startServer launches the compiled server binary with environment variables pointing at the local Artifactory.
func startServer(binary, token string) {
	serverBin := filepath.Join(perfDir, binary)
	configPath := filepath.Join(perfDir, "configs", "server-artifactory.json")

	serverCmd = exec.Command(serverBin, "--config", configPath)
	serverCmd.Dir = filepath.Dir(perfDir)
	serverCmd.Env = append(os.Environ(),
		"ARTIFACTORY_URL="+artifactoryURL+"/artifactory",
		"ARTIFACTORY_REPO_KEY=example-repo-local",
		"ARTIFACTORY_SERVICE_TOKEN="+token,
	)
	if err := serverCmd.Start(); err != nil {
		fatal("starting server: %v", err)
	}
}

func stopServer() {
	if serverCmd != nil && serverCmd.Process != nil {
		serverCmd.Process.Kill()
		serverCmd.Wait()
		serverCmd = nil
	}
}

// waitForServerHealthy polls the server's /health-check endpoint until it returns 200 OK.
func waitForServerHealthy(timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/health-check")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	fatal("server did not become healthy within %s", timeout)
}

// runTests invokes `go test` as a subprocess with the perftest build tag, filtering by the given regex pattern.
func runTests(token, packageCounts, concurrencyLevels, pattern string) {
	cmd := exec.Command("go", "test", "-tags", "perftest", "-v", "-timeout", "120m",
		"-run", pattern, ".")
	cmd.Dir = perfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"PERF_SERVER_URL="+serverURL,
		"PERF_SERVICE_TOKEN="+token,
		"PERF_PACKAGE_COUNTS="+packageCounts,
		"PERF_CONCURRENCY_LEVELS="+concurrencyLevels,
		"ARTIFACTORY_URL="+artifactoryURL+"/artifactory",
		"ARTIFACTORY_REPO_KEY=example-repo-local",
		"ARTIFACTORY_SERVICE_TOKEN="+token,
	)
	if err := cmd.Run(); err != nil {
		fatal("running tests (%s): %v", pattern, err)
	}
}

func runCmd(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = perfDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("running %s %s: %v", name, strings.Join(args, " "), err)
	}
}

// --- Artifactory initialization ---

// waitForHealthy polls Artifactory's system health endpoint until it reports HEALTHY.
func waitForHealthy(baseURL string, timeout time.Duration) {
	fmt.Println("  Waiting for Artifactory to be healthy...")
	deadline := time.Now().Add(timeout)
	start := time.Now()
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/router/api/v1/system/health")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(string(body), "HEALTHY") {
				fmt.Printf("  Healthy after %ds\n", int(time.Since(start).Seconds()))
				return
			}
		}
		time.Sleep(5 * time.Second)
	}
	fatal("Artifactory did not become healthy within %s", timeout)
}

// changePassword sets the Artifactory admin password from the default to the given value.
// Retries on transient errors (502, 503) since Artifactory may not be fully ready after reporting healthy.
func changePassword(baseURL, newPassword string) {
	fmt.Println("  Changing admin password...")
	payload := fmt.Sprintf(`{"userName":"admin","oldPassword":"password","newPassword1":"%s","newPassword2":"%s"}`,
		newPassword, newPassword)

	for attempt := range 10 {
		if attempt > 0 {
			fmt.Printf("  Retrying password change (%d/10)...\n", attempt+1)
			time.Sleep(10 * time.Second)
		}

		req, err := http.NewRequest("POST",
			baseURL+"/artifactory/api/security/users/authorization/changePassword",
			strings.NewReader(payload))
		if err != nil {
			fatal("creating password change request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "password")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fatal("changing password: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			fmt.Println("  Password changed")
			return
		}
		if resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusServiceUnavailable {
			continue
		}
		fatal("changing password: HTTP %d: %s", resp.StatusCode, string(body))
	}
	fatal("changing password: still failing after 10 attempts")
}

// setBaseURL configures Artifactory's base URL so generated artifact URLs are correct.
func setBaseURL(baseURL, adminPassword string) {
	fmt.Println("  Setting base URL...")
	req, err := http.NewRequest("PUT",
		baseURL+"/artifactory/api/system/configuration/baseUrl",
		strings.NewReader(baseURL))
	if err != nil {
		fatal("creating base URL request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth("admin", adminPassword)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal("setting base URL: %v", err)
	}
	resp.Body.Close()
	fmt.Println("  Base URL set")
}

// generateToken creates a short-lived Artifactory access token for the test run.
func generateToken(baseURL, password string) string {
	fmt.Println("  Generating service token...")
	data := url.Values{
		"username":   {"admin"},
		"scope":      {"member-of-groups:*"},
		"expires_in": {"7200"},
	}

	req, err := http.NewRequest("POST",
		baseURL+"/artifactory/api/security/token",
		strings.NewReader(data.Encode()))
	if err != nil {
		fatal("creating token request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth("admin", password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fatal("generating token: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("token generation returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		fatal("parsing token response: %v", err)
	}

	token, ok := result["access_token"].(string)
	if !ok || token == "" {
		fatal("no access_token in response: %s", string(body))
	}

	fmt.Println("  Token generated (valid 2h)")
	return token
}

// --- Helpers ---

// generateHex returns n cryptographically random bytes encoded as a hex string.
func generateHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		fatal("generating random bytes: %v", err)
	}
	return hex.EncodeToString(b)
}

// writeEnvFile creates the .env file used by Docker Compose with the generated security keys.
func writeEnvFile(masterKey, joinKey, postgresPassword string) {
	content := fmt.Sprintf("JF_SHARED_SECURITY_MASTERKEY=%s\nJF_SHARED_SECURITY_JOINKEY=%s\nPOSTGRES_USER=artifactory\nPOSTGRES_PASSWORD=%s\n",
		masterKey, joinKey, postgresPassword)
	if err := os.WriteFile(filepath.Join(perfDir, ".env"), []byte(content), 0600); err != nil {
		fatal("writing .env: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCSV(csv string) []int {
	parts := strings.Split(csv, ",")
	nums := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil {
			nums = append(nums, n)
		}
	}
	sort.Ints(nums)
	return nums
}

func maxFromCSV(csv string) int {
	nums := parseCSV(csv)
	if len(nums) == 0 {
		return 1000
	}
	return nums[len(nums)-1]
}

func step(n int, msg string) {
	fmt.Printf("[%d/6] %s\n", n, msg)
}

// fatal triggers a panic that is caught by run()'s deferred recover, ensuring cleanup always executes.
func fatal(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}
