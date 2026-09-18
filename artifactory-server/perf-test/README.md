# Performance Testing

This document shows you how to run performance tests against the MATLAB® Package Repository server. The test suite exercises the server against a real JFrog® Artifactory® OSS instance running locally in Docker®, measuring latency, throughput, and stability across configurable scales.

Performance tests are completely self-contained — they start their own infrastructure, generate their own credentials, and clean up after themselves. You do not need an existing Artifactory instance or any manual configuration.

## Requirements

- Docker (with Docker Compose)
- Go 1.26+
- Approximately 8 GB of free RAM (Artifactory requires 4 GB)

> **Note:** The first run downloads Docker images from JFrog's registry (~1 GB for Artifactory OSS). Subsequent runs reuse the cached images and start significantly faster.

## Quick Start

From the `artifactory-server/perf-test/` directory:

```bash
go run ./cmd/orchestrator
```

No setup is required. All credentials are auto-generated at the start of the run and deleted on completion. The orchestrator is pure Go — it works on Windows (cmd, PowerShell), Linux, and macOS without bash or any shell scripts.

---

## What It Tests

The suite measures the server's performance across seven scenarios:

| ID | Test | What It Measures |
|----|------|-----------------|
| P-1 | Startup Indexing | Wall-clock time for the server to discover and index all packages via AQL on cold start |
| P-2 | Query by UUID | Latency (p50/p95/p99) and throughput for `GET /v1/packages/by-uuid/{uuid}.json` |
| P-3 | Query by Name | Latency and throughput for `GET /v1/packages/by-name/{name}.json` |
| P-4 | Query Full Index | Latency and throughput for `GET /v1/packages/index.json`, plus response size |
| P-5 | Publish Throughput | Latency and throughput for `POST /v1/packages/publish` at each concurrency level |
| P-6 | Admin Refresh | Duration of `POST /v1/admin/refresh` and its impact on concurrent reader latency |
| P-7 | Soak | Aggregate latency, throughput, and error rate under sustained mixed read (`GET index.json`) and write (`POST publish`) load |

---

## Configuration

Override any default with environment variables before running:

```bash
PERF_PACKAGE_COUNTS="100,500,1000,5000" PERF_CONCURRENCY_LEVELS="5,10,25,50" go run ./cmd/orchestrator
```

| Variable | Default | Description |
|----------|---------|-------------|
| `PERF_PACKAGE_COUNTS` | `100,1000,10000` | Comma-separated package counts to test at (seeded incrementally) |
| `PERF_CONCURRENCY_LEVELS` | `5,25,50` | Comma-separated concurrency levels for load tests |
| `PERF_DURATION` | `30s` | How long each load test scenario runs |
| `PERF_SOAK_DURATION` | `15m` | Duration for the soak test (set to `1m` for a quick check) |
| `PERF_OUTPUT_JSON` | `results.json` | Path for machine-readable JSON results (set to `""` to disable) |

---

## How It Works

The Go orchestrator (`cmd/orchestrator/main.go`) performs these steps:

1. Generates cryptographic credentials for Artifactory OSS and PostgreSQL using `crypto/rand`
2. Starts Artifactory OSS and PostgreSQL via Docker Compose (~60 seconds on first boot)
3. Initializes Artifactory via its REST API (sets admin password, generates a service token)
4. Builds the Go server from the parent directory source code (including any local modifications)
5. Seeds packages incrementally at each configured scale by publishing realistic `.mltbx` files (themed names, MATLAB source files, varied metadata) to the server's publish endpoint
6. Runs the Go test benchmarks — the tests send HTTP requests to the server, which in turn queries and writes to the real Artifactory instance
7. Prints results to the console and writes `results.json`
8. Stops the server, destroys all containers and volumes, and deletes all generated credentials (via `defer`)

Any modifications you make to the server source code in `artifactory-server/internal/` are compiled in step 4 and reflected in the benchmark results.

---

## Output

Every run produces two forms of output:

**Console (stdout):** Human-readable tables showing latency percentiles, throughput, and error rates for each test at each scale and concurrency level.

**JSON (results.json):** Machine-readable results suitable for CI pipelines. Use this to track regressions over time, set pass/fail thresholds, or post results as PR comments.

The JSON file includes metadata (git commit, Go version, OS, configuration) alongside the raw metrics. Each result entry contains the following fields:

| Field | Description |
|-------|-------------|
| `test` | Test function name (e.g., `TestPerfQueryByUUID`) |
| `packages` | Number of packages in the index during this test |
| `concurrency` | Number of concurrent goroutines sending requests |
| `duration_seconds` | How long the load test ran |
| `total_requests` | Total successful requests completed |
| `throughput_rps` | Requests per second sustained over the test duration |
| `latency_ms.p50` | Median request latency — 50% of requests were faster than this |
| `latency_ms.p95` | 95th percentile — only 5% of requests were slower |
| `latency_ms.p99` | 99th percentile — only 1% of requests were slower |
| `latency_ms.max` | Worst-case single request latency observed |
| `errors` | Total number of failed requests |
| `error_rate` | Percentage of requests that failed |
| `startup_seconds` | (P-1 only) Wall-clock time for the server to start and build its index |
| `response_size_bytes` | (P-4 only) Size of the full index JSON response |

---

## Running Locally

From the `artifactory-server/perf-test/` directory:

```bash
go run ./cmd/orchestrator
```

To run a quick smoke test with smaller scale and shorter durations:

```bash
cd perf-test
PERF_PACKAGE_COUNTS="100" PERF_DURATION="10s" PERF_SOAK_DURATION="1m" go run ./cmd/orchestrator
```

No setup is required. The orchestrator handles all infrastructure, credentials, seeding, benchmarking, and cleanup automatically.

---

## Using Results in CI

The `results.json` file is designed for CI integration. The orchestrator produces it at the end of every successful run.

### GitHub Actions Example

```yaml
perf-test:
  runs-on: ubuntu-latest  # must have Docker and 8GB+ RAM
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version: '1.26'
    - run: cd artifactory-server/perf-test && go run ./cmd/orchestrator
    - uses: actions/upload-artifact@v3
      with:
        name: perf-results
        path: artifactory-server/perf-test/results.json
```

### What You Can Do With the Results

- **Regression detection:** Compare `latency_ms.p99` against previous runs and alert if latency increases by more than a defined threshold
- **Build artifacts:** Upload `results.json` as a build artifact for historical tracking
- **PR gating:** Parse with `jq` and fail the pipeline if any metric exceeds a limit
- **Dashboards:** Feed results into a time-series store to visualize performance trends

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Port 8082, 8081, or 8080 already in use | Artifactory OSS uses ports 8082 and 8081 by default; the repository server uses 8080 | Stop the conflicting service, or stop other Artifactory containers with `docker stop perf-test-artifactory` |
| "Artifactory did not become healthy within 180s" | Slow first-time startup or insufficient resources | Ensure Docker has at least 4 GB of memory allocated. Check `docker logs perf-test-artifactory` for startup errors. |
| Token generation fails | Artifactory did not fully initialize | Check `docker logs perf-test-artifactory` for errors |
| Socket errors at high concurrency on Windows | Windows TCP connection limits | Reduce `PERF_CONCURRENCY_LEVELS` (50 or below is generally safe) |
| Out of memory | Artifactory requires 4 GB | Ensure at least 8 GB of total free RAM on your system |
| Stale containers from a previous run | Previous run was interrupted before cleanup | Run `docker compose down -v` in the `perf-test/` directory |

---

## Notes

- Performance tests are gated by the `//go:build perftest` build tag and are never compiled or run by `go test ./...`
- All credentials are generated fresh for each run and destroyed on completion — nothing sensitive persists
- The Docker Compose configuration pins Artifactory OSS to v7.133.10 (tested, AQL confirmed working)
- Results vary by machine — use them for relative comparisons and trend detection, not absolute benchmarks

---

*Copyright 2026 The MathWorks, Inc.*
