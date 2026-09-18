//go:build perftest

// Copyright 2026 The MathWorks, Inc.

package perftest

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestPerfQueryByUUID measures latency and throughput for GET /v1/packages/by-uuid/{uuid}.json.
func TestPerfQueryByUUID(t *testing.T) {
	serverURL := getServerURL()
	token := getServiceToken()
	packageCounts := getPackageCounts()
	concurrencyLevels := getConcurrencyLevels()
	duration := getDuration()

	printHeader("TestPerfQueryByUUID")

	for _, pkgCount := range packageCounts {
		uuids := fetchUUIDs(t, serverURL, token, pkgCount)
		if len(uuids) == 0 {
			t.Fatalf("no UUIDs available for %d packages", pkgCount)
		}

		printSubHeader(fmt.Sprintf("%d packages", pkgCount))
		printStatsHeader()

		for _, conc := range concurrencyLevels {
			stats := runLoadTest(t, duration, conc, func() (time.Duration, error) {
				// Non-deterministic selection is intentional -- load tests should exercise varied access patterns across runs.
				uuid := uuids[rand.Intn(len(uuids))]
				return doGet(serverURL+"/v1/packages/by-uuid/"+uuid+".json", token)
			})
			printStatsRow(conc, stats)
			addResult(testResult{
				Test:         "TestPerfQueryByUUID",
				Packages:     pkgCount,
				Concurrency:  conc,
				DurationSecs: duration.Seconds(),
				TotalReqs:    stats.Count,
				Throughput:   stats.Throughput,
				LatencyMs:    statsToLatMs(stats),
				Errors:       stats.Errors,
				ErrorRate:    stats.ErrorRate,
			})
		}
	}
}

// TestPerfQueryByName measures latency and throughput for GET /v1/packages/by-name/{name}.json.
func TestPerfQueryByName(t *testing.T) {
	serverURL := getServerURL()
	token := getServiceToken()
	packageCounts := getPackageCounts()
	concurrencyLevels := getConcurrencyLevels()
	duration := getDuration()

	printHeader("TestPerfQueryByName")

	for _, pkgCount := range packageCounts {
		names := fetchNames(t, serverURL, token, pkgCount)
		if len(names) == 0 {
			t.Fatalf("no names available for %d packages", pkgCount)
		}

		printSubHeader(fmt.Sprintf("%d packages", pkgCount))
		printStatsHeader()

		for _, conc := range concurrencyLevels {
			stats := runLoadTest(t, duration, conc, func() (time.Duration, error) {
				// Non-deterministic selection is intentional -- load tests should exercise varied access patterns across runs.
				name := names[rand.Intn(len(names))]
				return doGet(serverURL+"/v1/packages/by-name/"+name+".json", token)
			})
			printStatsRow(conc, stats)
			addResult(testResult{
				Test:         "TestPerfQueryByName",
				Packages:     pkgCount,
				Concurrency:  conc,
				DurationSecs: duration.Seconds(),
				TotalReqs:    stats.Count,
				Throughput:   stats.Throughput,
				LatencyMs:    statsToLatMs(stats),
				Errors:       stats.Errors,
				ErrorRate:    stats.ErrorRate,
			})
		}
	}
}

// TestPerfQueryIndex measures latency, throughput, and response size for GET /v1/packages/index.json.
func TestPerfQueryIndex(t *testing.T) {
	serverURL := getServerURL()
	token := getServiceToken()
	packageCounts := getPackageCounts()
	concurrencyLevels := getConcurrencyLevels()
	duration := getDuration()

	printHeader("TestPerfQueryIndex")

	for _, pkgCount := range packageCounts {
		// Measure response size once
		size := measureResponseSize(t, serverURL+"/v1/packages/index.json", token)
		printSubHeader(fmt.Sprintf("%d packages (response: %s)", pkgCount, formatBytes(size)))
		printStatsHeader()

		for _, conc := range concurrencyLevels {
			stats := runLoadTest(t, duration, conc, func() (time.Duration, error) {
				return doGet(serverURL+"/v1/packages/index.json", token)
			})
			printStatsRow(conc, stats)
			addResult(testResult{
				Test:         "TestPerfQueryIndex",
				Packages:     pkgCount,
				Concurrency:  conc,
				DurationSecs: duration.Seconds(),
				TotalReqs:    stats.Count,
				Throughput:   stats.Throughput,
				LatencyMs:    statsToLatMs(stats),
				Errors:       stats.Errors,
				ErrorRate:    stats.ErrorRate,
				ResponseSize: size,
			})
		}
	}
}

// TestPerfAdminRefresh measures the duration of POST /v1/admin/refresh and its impact on concurrent reader latency.
func TestPerfAdminRefresh(t *testing.T) {
	serverURL := getServerURL()
	token := getServiceToken()
	packageCounts := getPackageCounts()
	duration := getDuration()

	printHeader("TestPerfAdminRefresh")

	for _, pkgCount := range packageCounts {
		printSubHeader(fmt.Sprintf("%d packages", pkgCount))

		// Start concurrent readers
		var readerLatencies []time.Duration
		var readerMu sync.Mutex
		var readerErrors atomic.Int64
		stop := make(chan struct{})

		var wg sync.WaitGroup
		for range 25 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
						lat, err := doGet(serverURL+"/v1/packages/index.json", token)
						if err != nil {
							readerErrors.Add(1)
						} else {
							readerMu.Lock()
							readerLatencies = append(readerLatencies, lat)
							readerMu.Unlock()
						}
					}
				}
			}()
		}

		// Fire refresh (may take minutes at large scale — AQL + manifest download)
		time.Sleep(time.Second)
		refreshStart := time.Now()
		_, err := doPostWithTimeout(serverURL+"/v1/admin/refresh", token, 30*time.Minute)
		refreshDuration := time.Since(refreshStart)
		if err != nil {
			t.Fatalf("refresh failed: %v", err)
		}

		time.Sleep(duration)
		close(stop)
		wg.Wait()

		readerStats := computeStats(readerLatencies, duration, readerErrors.Load())
		fmt.Printf("  Refresh duration: %s\n", formatDuration(refreshDuration))
		fmt.Printf("  Reader latency during refresh: p50=%s  p95=%s  p99=%s\n",
			formatDuration(readerStats.P50), formatDuration(readerStats.P95), formatDuration(readerStats.P99))

		addResult(testResult{
			Test:         "TestPerfAdminRefresh",
			Packages:     pkgCount,
			DurationSecs: refreshDuration.Seconds(),
			TotalReqs:    readerStats.Count,
			LatencyMs:    statsToLatMs(readerStats),
			Errors:       readerStats.Errors,
			ErrorRate:    readerStats.ErrorRate,
		})
	}
}

// TestPerfStartupIndexing measures wall-clock time for the server to start and build its index via AQL.
func TestPerfStartupIndexing(t *testing.T) {
	serverURL := getServerURL()
	packageCounts := getPackageCounts()

	printHeader("TestPerfStartupIndexing")

	for _, pkgCount := range packageCounts {
		srv, err := startServer()
		if err != nil {
			t.Fatalf("failed to start server: %v", err)
		}

		startTime := time.Now()
		waitForHealthy(t, serverURL, 5*time.Minute)
		startupDuration := time.Since(startTime)

		fmt.Printf("  %d packages: %s startup\n",
			pkgCount, formatDuration(startupDuration))

		addResult(testResult{
			Test:        "TestPerfStartupIndexing",
			Packages:    pkgCount,
			StartupSecs: startupDuration.Seconds(),
		})

		srv.Stop()
		time.Sleep(2 * time.Second)
	}
}

// TestPerfPublishThroughput measures latency and throughput for POST /v1/packages/publish at each concurrency level.
func TestPerfPublishThroughput(t *testing.T) {
	serverURL := getServerURL()
	token := getServiceToken()
	concurrencyLevels := getConcurrencyLevels()
	duration := getDuration()

	printHeader("TestPerfPublishThroughput")
	printStatsHeader()

	var concIdx atomic.Int64
	concIdx.Store(int64(time.Now().UnixNano() % 100000))

	for _, conc := range concurrencyLevels {
		stats := runLoadTest(t, duration, conc, func() (time.Duration, error) {
			idx := int(concIdx.Add(1))
			return publishOne(serverURL, token, idx)
		})
		printStatsRow(conc, stats)
		addResult(testResult{
			Test:         "TestPerfPublishThroughput",
			Concurrency:  conc,
			DurationSecs: duration.Seconds(),
			TotalReqs:    stats.Count,
			Throughput:   stats.Throughput,
			LatencyMs:    statsToLatMs(stats),
			Errors:       stats.Errors,
			ErrorRate:    stats.ErrorRate,
		})
	}
}

// TestPerfSoak runs sustained mixed read/write load and reports aggregate latency, throughput, and error rate.
func TestPerfSoak(t *testing.T) {
	serverURL := getServerURL()
	token := getServiceToken()
	soakDuration := getSoakDuration()
	packageCounts := getPackageCounts()
	maxPackages := packageCounts[len(packageCounts)-1]

	printHeader(fmt.Sprintf("TestPerfSoak (%d packages, %s)", maxPackages, soakDuration))

	var latencies []time.Duration
	var latMu sync.Mutex
	var errors atomic.Int64
	stop := make(chan struct{})

	// 5 reader goroutines
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					lat, err := doGet(serverURL+"/v1/packages/index.json", token)
					if err != nil {
						errors.Add(1)
					} else {
						latMu.Lock()
						latencies = append(latencies, lat)
						latMu.Unlock()
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	// 1 writer goroutine (publish every 30s)
	var publishIdx atomic.Int64
	publishIdx.Store(int64(time.Now().UnixNano()%100000) + 50000)
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				idx := int(publishIdx.Add(1))
				if _, err := publishOne(serverURL, token, idx); err != nil {
					errors.Add(1)
				}
			}
		}
	}()

	time.Sleep(soakDuration)
	close(stop)
	wg.Wait()

	stats := computeStats(latencies, soakDuration, errors.Load())

	fmt.Printf("  Latency p50: %s  p95: %s  p99: %s\n",
		formatDuration(stats.P50), formatDuration(stats.P95), formatDuration(stats.P99))
	fmt.Printf("  Throughput: %.0f req/s\n", stats.Throughput)
	fmt.Printf("  Errors: %d (%.2f%%)\n", errors.Load(), stats.ErrorRate)

	if stats.ErrorRate > 1.0 {
		t.Errorf("error rate %.2f%% exceeds 1%% threshold", stats.ErrorRate)
	}

	addResult(testResult{
		Test:         "TestPerfSoak",
		Packages:     maxPackages,
		DurationSecs: soakDuration.Seconds(),
		TotalReqs:    stats.Count,
		Throughput:   stats.Throughput,
		LatencyMs:    statsToLatMs(stats),
		Errors:       errors.Load(),
		ErrorRate:    stats.ErrorRate,
	})
}

// TestPerfWriteResults assembles partial results from all prior tests into the final results.json file.
func TestPerfWriteResults(t *testing.T) {
	if err := writeResults(); err != nil {
		t.Fatalf("failed to write results: %v", err)
	}
	path := getOutputJSON()
	if path != "" {
		fmt.Printf("\nResults written to: %s\n", path)
	}
}

// --- Internal helpers ---

// runLoadTest launches N concurrent goroutines that call fn in a tight loop for the given duration,
// then returns aggregated latency percentiles, throughput, and error counts.
func runLoadTest(t *testing.T, duration time.Duration, concurrency int, fn func() (time.Duration, error)) latencyStats {
	t.Helper()
	var latencies []time.Duration
	var mu sync.Mutex
	var errors atomic.Int64
	stop := make(chan struct{})

	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					lat, err := fn()
					if err != nil {
						errors.Add(1)
					} else {
						mu.Lock()
						latencies = append(latencies, lat)
						mu.Unlock()
					}
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	return computeStats(latencies, duration, errors.Load())
}

// perfClient is a shared HTTP client with connection pooling to prevent socket exhaustion under high concurrency.
var perfClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     100,
		IdleConnTimeout:     90 * time.Second,
	},
	Timeout: 30 * time.Second,
}

// doGet performs an authenticated GET request and returns the elapsed time.
func doGet(url, token string) (time.Duration, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	start := time.Now()
	resp, err := perfClient.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	return elapsed, nil
}

// doPostWithTimeout performs an authenticated POST with a custom timeout (used for admin refresh which can take minutes).
func doPostWithTimeout(url, token string, timeout time.Duration) (time.Duration, error) {
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return 0, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := perfClient
	if timeout > 0 {
		client = &http.Client{Timeout: timeout}
	}

	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status %d", resp.StatusCode)
	}
	return elapsed, nil
}

// publishOne builds a minimal .mltbx in memory and publishes it via POST /v1/packages/publish.
func publishOne(serverURL, token string, idx int) (time.Duration, error) {
	mltbx := generatePublishMltbx(idx)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", fmt.Sprintf("perf-publish-%d-1.0.%d.mltbx", idx, idx))
	if err != nil {
		return 0, err
	}
	part.Write(mltbx)
	writer.Close()

	req, err := http.NewRequest("POST", serverURL+"/v1/packages/publish", &body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	start := time.Now()
	resp, err := perfClient.Do(req)
	if err != nil {
		return 0, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("publish status %d", resp.StatusCode)
	}
	return elapsed, nil
}

// generatePublishMltbx creates a minimal .mltbx ZIP with unique metadata for publish throughput testing.
func generatePublishMltbx(i int) []byte {
	pkg := struct {
		Name        string `json:"name"`
		ID          string `json:"id"`
		Version     string `json:"version"`
		DisplayName string `json:"displayName"`
		Summary     string `json:"summary"`
		Provider    struct {
			Name         string `json:"name"`
			Organization string `json:"organization"`
		} `json:"provider"`
		ReleaseCompatibility string   `json:"releaseCompatibility"`
		SupportedPlatforms   []string `json:"supportedPlatforms"`
	}{
		Name:                 fmt.Sprintf("perf-publish-%d", i),
		ID:                   fmt.Sprintf("b2c3d4e5-%04d-4000-8000-%012d", i%10000, i),
		Version:              fmt.Sprintf("1.0.%d", i),
		DisplayName:          fmt.Sprintf("Perf Publish Package %d", i),
		Summary:              "Synthetic package for publish throughput testing",
		ReleaseCompatibility: ">=R2024a",
		SupportedPlatforms:   []string{"win64", "maci64", "glnxa64"},
	}
	pkg.Provider.Name = "PerfTest"
	pkg.Provider.Organization = "MathWorks"

	data, err := json.Marshal(pkg)
	if err != nil {
		panic(fmt.Sprintf("marshaling publish package: %v", err))
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("fsroot/resources/mpackage.json")
	if err != nil {
		panic(fmt.Sprintf("creating zip entry: %v", err))
	}
	f.Write(data)
	if err := zw.Close(); err != nil {
		panic(fmt.Sprintf("closing zip writer: %v", err))
	}
	return buf.Bytes()
}

// fetchUUIDs retrieves package UUIDs from the index endpoint for use as query targets in load tests.
func fetchUUIDs(t *testing.T, serverURL, token string, limit int) []string {
	t.Helper()
	return fetchFieldsFromIndex(t, serverURL, token, "id", limit)
}

// fetchNames retrieves package names from the index endpoint for use as query targets in load tests.
func fetchNames(t *testing.T, serverURL, token string, limit int) []string {
	t.Helper()
	return fetchFieldsFromIndex(t, serverURL, token, "name", limit)
}

// fetchFieldsFromIndex fetches the full package index and extracts values for the given field (e.g., "id" or "name").
func fetchFieldsFromIndex(t *testing.T, serverURL, token, field string, limit int) []string {
	t.Helper()
	req, err := http.NewRequest("GET", serverURL+"/v1/packages/index.json", nil)
	if err != nil {
		t.Fatalf("creating index request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := perfClient.Do(req)
	if err != nil {
		t.Fatalf("fetching index: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading index body: %v", err)
	}

	var indexResp struct {
		Packages []map[string]json.RawMessage `json:"packages"`
	}
	if err := json.Unmarshal(body, &indexResp); err != nil {
		t.Fatalf("parsing index: %v", err)
	}

	var values []string
	for _, pkg := range indexResp.Packages {
		raw, ok := pkg[field]
		if !ok {
			continue
		}
		var val string
		if err := json.Unmarshal(raw, &val); err != nil {
			continue
		}
		values = append(values, val)
		if len(values) >= limit {
			break
		}
	}
	return values
}

// measureResponseSize performs a single GET and returns the response body size in bytes.
func measureResponseSize(t *testing.T, url, token string) int64 {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := perfClient.Do(req)
	if err != nil {
		t.Fatalf("measuring response size: %v", err)
	}
	defer resp.Body.Close()

	n, _ := io.Copy(io.Discard, resp.Body)
	return n
}

// waitForHealthy polls the server's /health-check endpoint until it returns 200 OK or the timeout expires.
func waitForHealthy(t *testing.T, serverURL string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverURL + "/health-check")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("server did not become healthy within %s", timeout)
}

// formatBytes returns a human-readable byte size (e.g., "76.6 KB").
func formatBytes(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(b)/1024/1024)
}

// statsToLatMs converts latency stats from time.Duration to milliseconds for JSON output.
func statsToLatMs(s latencyStats) *latMs {
	return &latMs{
		P50: float64(s.P50.Nanoseconds()) / 1e6,
		P95: float64(s.P95.Nanoseconds()) / 1e6,
		P99: float64(s.P99.Nanoseconds()) / 1e6,
		Max: float64(s.Max.Nanoseconds()) / 1e6,
	}
}
