//go:build perftest

// Copyright 2026 The MathWorks, Inc.

package perftest

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// --- Configuration helpers ---

// getEnv returns the value of an environment variable, or the fallback if unset.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getServerURL() string {
	return getEnv("PERF_SERVER_URL", "http://localhost:8080")
}

// getServiceToken reads the bearer token from the environment or the .service-token file.
func getServiceToken() string {
	if t := os.Getenv("PERF_SERVICE_TOKEN"); t != "" {
		return t
	}
	data, err := os.ReadFile(".service-token")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func getPackageCounts() []int {
	raw := getEnv("PERF_PACKAGE_COUNTS", "100,1000,10000")
	return parseIntList(raw)
}

func getConcurrencyLevels() []int {
	raw := getEnv("PERF_CONCURRENCY_LEVELS", "5,25,50")
	return parseIntList(raw)
}

func getDuration() time.Duration {
	raw := getEnv("PERF_DURATION", "30s")
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

func getSoakDuration() time.Duration {
	raw := getEnv("PERF_SOAK_DURATION", "15m")
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 15 * time.Minute
	}
	return d
}

func getOutputJSON() string {
	return getEnv("PERF_OUTPUT_JSON", "results.json")
}

// parseIntList splits a comma-separated string into a sorted slice of positive integers.
func parseIntList(s string) []int {
	parts := strings.Split(s, ",")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		n, err := strconv.Atoi(p)
		if err == nil && n > 0 {
			result = append(result, n)
		}
	}
	sort.Ints(result)
	return result
}

// --- Statistics ---

// latencyStats holds aggregated results from a load test run.
type latencyStats struct {
	Count      int64
	P50        time.Duration
	P95        time.Duration
	P99        time.Duration
	Max        time.Duration
	Throughput float64
	Errors     int64
	ErrorRate  float64
}

// computeStats sorts the latency samples and computes percentiles, throughput, and error rate.
func computeStats(latencies []time.Duration, duration time.Duration, errors int64) latencyStats {
	if len(latencies) == 0 {
		return latencyStats{Errors: errors}
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	total := int64(len(latencies)) + errors
	return latencyStats{
		Count:      int64(len(latencies)),
		P50:        percentile(latencies, 0.50),
		P95:        percentile(latencies, 0.95),
		P99:        percentile(latencies, 0.99),
		Max:        latencies[len(latencies)-1],
		Throughput: float64(len(latencies)) / duration.Seconds(),
		Errors:     errors,
		ErrorRate:  float64(errors) / float64(total) * 100,
	}
}

// percentile returns the p-th percentile from a pre-sorted slice of durations.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// --- Output formatting ---

func printHeader(testName string) {
	fmt.Printf("\n=== %s ===\n", testName)
}

func printSubHeader(label string) {
	fmt.Printf("  --- %s ---\n", label)
}

func printStatsRow(concurrency int, stats latencyStats) {
	fmt.Printf("  %-14s p50=%-9s p95=%-9s p99=%-9s | %8.0f req/s | %d (%.2f%%)\n",
		fmt.Sprintf("%d concurrent:", concurrency),
		formatDuration(stats.P50),
		formatDuration(stats.P95),
		formatDuration(stats.P99),
		stats.Throughput,
		stats.Errors,
		stats.ErrorRate,
	)
}

func printStatsHeader() {
	fmt.Printf("  %-14s %-10s %-10s %-10s | %12s | %s\n",
		"CONCURRENCY", "P50", "P95", "P99", "THROUGHPUT", "ERRORS")
}

// formatDuration returns a human-readable duration (e.g., "1.2ms", "0.5us", "2.30s").
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fus", float64(d.Microseconds()))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1e6)
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// --- JSON results ---

// testResult is a single benchmark result entry written to the final results.json file.
type testResult struct {
	Test         string  `json:"test"`
	Packages     int     `json:"packages,omitempty"`
	Concurrency  int     `json:"concurrency,omitempty"`
	DurationSecs float64 `json:"duration_seconds,omitempty"`
	TotalReqs    int64   `json:"total_requests,omitempty"`
	Throughput   float64 `json:"throughput_rps,omitempty"`
	LatencyMs    *latMs  `json:"latency_ms,omitempty"`
	Errors       int64   `json:"errors"`
	ErrorRate    float64 `json:"error_rate"`

	// P-1 specific
	StartupSecs float64 `json:"startup_seconds,omitempty"`

	// P-4 specific
	ResponseSize int64 `json:"response_size_bytes,omitempty"`
}

type latMs struct {
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
}

type resultsFile struct {
	Metadata metadata     `json:"metadata"`
	Results  []testResult `json:"results"`
}

type metadata struct {
	Timestamp           string `json:"timestamp"`
	GitCommit           string `json:"git_commit"`
	GitBranch           string `json:"git_branch"`
	ArtifactoryVersion  string `json:"artifactory_version"`
	GoVersion           string `json:"go_version"`
	OS                  string `json:"os"`
	PackageCounts       []int  `json:"package_counts"`
	ConcurrencyLevels   []int  `json:"concurrency_levels"`
	DurationPerScenario string `json:"duration_per_scenario"`
}

const resultsPartialFile = "results.partial.tmp"

var resultsMu sync.Mutex

// addResult appends a result as a JSON line to the partial results file.
// Results are written to disk because each test group runs in a separate go test process.
func addResult(r testResult) {
	resultsMu.Lock()
	defer resultsMu.Unlock()

	data, err := json.Marshal(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to marshal result: %v\n", err)
		return
	}
	f, err := os.OpenFile(resultsPartialFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to open %s: %v\n", resultsPartialFile, err)
		return
	}
	defer f.Close()
	f.Write(data)
	f.Write([]byte("\n"))
}

// loadPartialResults reads all JSON lines from the partial results file.
func loadPartialResults() []testResult {
	data, err := os.ReadFile(resultsPartialFile)
	if err != nil {
		return nil
	}
	var results []testResult
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var r testResult
		if json.Unmarshal([]byte(line), &r) == nil {
			results = append(results, r)
		}
	}
	return results
}

// writeResults assembles partial results into the final results.json with metadata (git info, config, etc.).
func writeResults() error {
	path := getOutputJSON()
	if path == "" {
		return nil
	}

	results := loadPartialResults()

	commit := gitOutput("rev-parse", "--short", "HEAD")
	branch := gitOutput("rev-parse", "--abbrev-ref", "HEAD")

	rf := resultsFile{
		Metadata: metadata{
			Timestamp:           time.Now().UTC().Format(time.RFC3339),
			GitCommit:           commit,
			GitBranch:           branch,
			ArtifactoryVersion:  getEnv("ARTIFACTORY_VERSION", "unknown"),
			GoVersion:           runtime.Version(),
			OS:                  runtime.GOOS + "/" + runtime.GOARCH,
			PackageCounts:       getPackageCounts(),
			ConcurrencyLevels:   getConcurrencyLevels(),
			DurationPerScenario: getDuration().String(),
		},
		Results: results,
	}

	data, err := json.MarshalIndent(rf, "", "  ")
	if err != nil {
		return err
	}

	os.Remove(resultsPartialFile)
	return os.WriteFile(path, data, 0644)
}

func gitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// --- Server management (for P-1) ---

// serverProcess wraps the server child process so P-1 can start/stop it to measure cold-start time.
type serverProcess struct {
	cmd *exec.Cmd
}

// startServer launches the compiled server binary as a child process.
func startServer() (*serverProcess, error) {
	binary := "./perf-test/mpm-repo-server"
	if runtime.GOOS == "windows" {
		binary = "./perf-test/mpm-repo-server.exe"
	}

	cmd := exec.Command(binary, "--config", "perf-test/configs/server-artifactory.json")
	cmd.Dir = ".."
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting server: %w", err)
	}
	return &serverProcess{cmd: cmd}, nil
}

func (s *serverProcess) Stop() {
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}
}
