# Artifactory Reference Implementation

This document shows you how to set up and run a MATLAB® package repository service. The service enables the MATLAB Package Manager (MPM) to discover, install, and publish MATLAB packages (`.mltbx` files) from your organization's Artifactory instance.

JFrog® Artifactory® can store MATLAB package files but does not understand MATLAB package metadata — it cannot query packages by name or UUID, resolve dependencies, or filter by platform and MATLAB release compatibility. This service bridges that gap by indexing MATLAB package metadata stored in Artifactory and serving the data in the structured format that MPM expects.

## Requirements

- JFrog Artifactory instance (self-hosted or cloud) with a **Generic** repository
- Docker® (recommended) or Go 1.26+ for building from source
- MATLAB R2026b or later (for package consumers)
- Artifactory access tokens for the service and for users

> **Note:** For MATLAB releases before R2026b or for advanced use cases, contact [MathWorks Consulting](https://www.mathworks.com/services/consulting.html).

## Quick Start

Clone the repository, then build and run the service from the `artifactory-server` directory:

```bash
git clone <repository-url>
cd <repository-directory>/artifactory-server

docker build -t mpm-repo-server .

docker run -p 8080:8080 \
  -e ARTIFACTORY_URL=https://artifactory.corp.com/artifactory \
  -e ARTIFACTORY_REPO_KEY=mpm-packages \
  -e ARTIFACTORY_SERVICE_TOKEN=your-service-token \
  -e PUBLISH_MODE=atomic \
  mpm-repo-server
```

Verify the service is running:

```bash
curl http://localhost:8080/health-check
# Returns 200 OK when the service is ready
```

## Configuration

| Variable | Required | Description |
|----------|----------|-------------|
| `ARTIFACTORY_URL` | Yes | Artifactory base URL, including the `http://` or `https://` scheme |
| `ARTIFACTORY_REPO_KEY` | Yes | Name of the Generic repository |
| `ARTIFACTORY_SERVICE_TOKEN` | Yes | Token with read access (indexing at startup) |
| `PORT` | No | Server port (default: `8080`) |
| `PUBLISH_MODE` | No | Publish strategy: `off` (default), `atomic`, or `two-step`. See [Publish Modes](#publish-modes) |

### Publish Modes

`PUBLISH_MODE` (or the `server.publishMode` config setting) selects how the publish endpoint behaves. The publish endpoint is served only when the mode is `atomic` or `two-step`.

| Mode | Artifactory edition | Behavior |
|------|---------------------|----------|
| `off` (default) | — | Publish endpoint is disabled. |
| `atomic` | **Pro / Enterprise** | Archive and manifest are bundled and uploaded in a single request using the `X-Explode-Archive-Atomic` header. The publish is all-or-nothing. |
| `two-step` | **OSS** (also works on Pro/Enterprise) | Archive and manifest are uploaded as two separate requests. Required for OSS Artifactory, which does not support atomic explode-archive. |

> **Note on two-step mode:** Because the archive and manifest are uploaded separately, a publish is **not atomic**. If the manifest upload fails after the archive has been uploaded, the archive is left in place and the publish returns an error naming the orphaned file. An administrator must remove it manually. The service token still requires only write access — no rollback (and therefore no delete permission) is performed.

### Server Configuration File

The service reads its configuration from `configs/server-artifactory.json`. This file controls HTTP timeouts, backend communication limits, and upload size bounds. The default values are tuned for security hardening:

```json
{
  "server": {
    "listenAddr": ":8080",
    "readHeaderTimeout": "10s",
    "readTimeout": "60s",
    "writeTimeout": "120s",
    "idleTimeout": "120s"
  },
  "backend": {
    "readTimeout": "30s",
    "uploadTimeout": "120s",
    "maxAQLResponseBytes": 52428800,
    "maxManifestResponseBytes": 1048576,
    "maxUploadBytes": 524288000
  }
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `server.readHeaderTimeout` | `10s` | Max time to read request headers (slowloris defense) |
| `server.readTimeout` | `60s` | Max time to read the full request including body |
| `server.writeTimeout` | `120s` | Max time to write the response |
| `server.idleTimeout` | `120s` | Max time to keep idle keep-alive connections open |
| `backend.readTimeout` | `30s` | Timeout for AQL queries and manifest fetches from Artifactory |
| `backend.uploadTimeout` | `120s` | Timeout for uploading archives and manifests to Artifactory |
| `backend.maxAQLResponseBytes` | `52428800` (50 MB) | Max size of AQL response body from Artifactory |
| `backend.maxManifestResponseBytes` | `1048576` (1 MB) | Max size of a single manifest response from Artifactory |
| `backend.maxUploadBytes` | `524288000` (500 MB) | Max size of an uploaded MLTBX file |

---

## Admin Setup

An admin deploys the service once for the entire team. Individual users do not need to run it.

### Create a Generic Repository in Artifactory

Create a **Generic** repository type in Artifactory. When you publish packages through the service, the service creates the following layout automatically:

```
<repo-key>/
  <package-name>/
    <uuid>/
      <version>/
        <name>-<version>.manifest.json
        <name>-<version>.mltbx
```

### Configure Access Tokens

| Token | Scope | Used by |
|-------|-------|---------|
| Service token | Read-only | The service (indexing at startup) |
| Developer tokens | Read + write | Developers and CI pipelines that publish packages |
| Consumer tokens | Read | MATLAB users installing packages |

The service itself never writes to Artifactory. Publishes are performed using the developer's own token, passed via the `Authorization` header.

### Deploy the Service

**Option A: Docker Compose (recommended)**

Create a `.env` file:

```
ARTIFACTORY_URL=https://artifactory.corp.com/artifactory
ARTIFACTORY_REPO_KEY=mpm-packages
ARTIFACTORY_SERVICE_TOKEN=your-service-token
PUBLISH_MODE=atomic
```

Then run:

```bash
docker compose up -d
```

**Option B: Build from source**

```bash
cd artifactory-server
go build -o mpm-repo-server ./cmd/mpm-repo-server

export ARTIFACTORY_URL=https://artifactory.corp.com/artifactory
export ARTIFACTORY_REPO_KEY=mpm-packages
export ARTIFACTORY_SERVICE_TOKEN=your-service-token
export PUBLISH_MODE=atomic

./mpm-repo-server --config configs/server-artifactory.json
```

Run tests:

```bash
go test -v ./...                        # All tests
go test -v ./internal/handler/...       # Single package
```

### Deployment Options

| Option | Setup effort | Best for |
|--------|-------------|----------|
| **VM / internal server** | Low — run Docker on any Linux host, open the port | Small teams, simple environments |
| **Behind a reverse proxy** (nginx, ALB) | Moderate — adds TLS, clean hostname | Production deployments |
| **Kubernetes / container orchestration** | Standard — pod spec + Service + Ingress | Organizations already using K8s |

### Distribute Configuration to Users

Create a `repo-config.json` and distribute it to your team:

```json
{
  "schema_version": "1.0.0",
  "repository_configuration": [
    {
      "name": "corp-repo",
      "url": "https://mprs.corp.com:8080/",
      "type": "custom",
      "auth": {
        "type": "token",
        "access_token": "${ARTIFACTORY_TOKEN}"
      }
    }
  ]
}
```

The `${ARTIFACTORY_TOKEN}` placeholder is resolved from the user's environment variable at runtime.

---

## Developer Workflow

### Publish a Package

Publish an MLTBX to the repository service. The publish endpoint uses your Artifactory token (not the service token) to upload:

```bash
curl -X POST https://mprs.corp.com:8080/v1/packages/publish \
  -H "Authorization: Bearer ${ARTIFACTORY_TOKEN}" \
  -F "file=@MyToolbox-1.2.0.mltbx"
```

Returns `201 Created` on success, or `409 Conflict` if the same package version already exists.

### CI Pipeline Example

```yaml
- name: Publish package
  run: |
    curl -X POST https://mprs.corp.com:8080/v1/packages/publish \
      -H "Authorization: Bearer ${{ secrets.ARTIFACTORY_TOKEN }}" \
      -F "file=@build/MyToolbox.mltbx"
```

---

## Consumer Workflow

MATLAB queries the service for package metadata. The manifests the service returns carry download URLs that point at Artifactory, so MATLAB fetches the `.mltbx` files from Artifactory directly, using the consumer's token. Consumers therefore need network access to both the service and Artifactory.

Consumers run the following steps in MATLAB.

### Set Your Access Token

```bash
# Linux/macOS
export ARTIFACTORY_TOKEN=<your-read-token>

# Windows
set ARTIFACTORY_TOKEN=<your-read-token>
```

### Add the Repository in MATLAB

```matlab
jsonFile = "/path/to/repo-config.json";
cfg = matlab.mpm.RepositoryConfiguration(jsonFile);
mpmAddRepository(cfg);
```

### Install Packages

```matlab
mpminstall MyToolbox

% Install a specific version
mpminstall MyToolbox@1.2.0
```

---

## Index Refresh

If you add or delete packages directly in Artifactory, rebuild the in-memory index:

```bash
curl -X POST https://mprs.corp.com:8080/v1/admin/refresh \
  -H "Authorization: Bearer $YOUR_TOKEN"
```

The refresh endpoint uses the caller's Bearer token to authenticate with the backend (not the server service token).

---

## Security Considerations

For details on the service's authentication model, secrets handling, container hardening, and network assumptions, see the [Secure Configuration Guidelines](../docs/secure-configuration-guidelines.md).

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Service fails to start | Cannot reach Artifactory | Verify `ARTIFACTORY_URL` is correct and the service token is valid |
| `connection refused` at startup, e.g. `dial tcp [::1]:8082` | `ARTIFACTORY_URL` uses `localhost`, which inside the container means the container itself, not the host | Point `ARTIFACTORY_URL` at the host, for example `http://host.docker.internal:8082/artifactory` (`docker-compose.yml` already maps that name to the host gateway). Set this before publishing: published manifests record download URLs derived from `ARTIFACTORY_URL`, and re-indexing does not rewrite them |
| `401 Unauthorized` on publish | The request carried no `Authorization: Bearer <token>` header | Send a bearer token. Publish and refresh require one because the service passes it through to Artifactory |
| `409 Conflict` on publish | Package version already exists | Bump the version before re-publishing |
| Empty index after startup | Wrong repo key or no manifests | Verify `ARTIFACTORY_REPO_KEY` matches your repository and that it contains `*.manifest.json` files |
| MATLAB cannot connect | Wrong URL in repo-config.json | Ensure the `url` field points to the service (not directly to Artifactory) |

---

## MPM API Specification

This service implements the [MATLAB Package Repository API](../docs/matlab-package-repository-api-spec.md). The following table summarizes what this reference implementation covers:

| Capability | Status | Needs Authentication |
|------------|--------|---------------------|
| `GET /v1/packages/by-uuid/{uuid}.json` | Implemented | No |
| `GET /v1/packages/by-name/{name}.json` | Implemented | No |
| `GET /v1/packages/index.json` | Implemented (single page, no pagination) | No |
| `POST /v1/packages/publish` | Implemented | Yes |
| `GET /health-check` | Implemented | No |
| Pagination (`nextUrl`) | Not yet implemented | N/A |
| `GET /version` | Not yet implemented | N/A |
| Download endpoint | Not yet implemented | N/A |

---


## Performance Testing

The repository includes a self-contained performance test suite that benchmarks the server against a real Artifactory OSS instance running locally in Docker®.

### Prerequisites

- Docker (with Docker Compose)
- Go 1.26+

### Quick Start

```bash
cd perf-test && go run ./cmd/orchestrator
```

This will:
1. Generate cryptographic credentials for Artifactory OSS
2. Start Artifactory OSS + PostgreSQL in Docker (~60s first boot)
3. Initialize Artifactory (set password, generate token)
4. Build the Go server from source (including any local modifications)
5. Seed packages incrementally (default: 100, 1,000, 10,000)
6. Run all performance benchmarks at each scale x concurrency combination
7. Print results, tear down containers, and delete all credentials

### Configuration

Override defaults with environment variables:

```bash
cd perf-test
PERF_PACKAGE_COUNTS="100,500,1000,5000" \
PERF_CONCURRENCY_LEVELS="5,10,25,50" \
PERF_DURATION="10s" \
PERF_SOAK_DURATION="1m" \
go run ./cmd/orchestrator
```

| Variable | Default | Description |
|----------|---------|-------------|
| `PERF_PACKAGE_COUNTS` | `100,1000,10000` | Comma-separated package counts (tested incrementally) |
| `PERF_CONCURRENCY_LEVELS` | `5,25,50` | Comma-separated concurrency levels |
| `PERF_DURATION` | `30s` | Duration per load test scenario |
| `PERF_SOAK_DURATION` | `15m` | Duration for soak test (set to `1m` for a quick check) |

### Notes

- Performance tests are NOT run by `go test ./...`
- They require Docker and take several minutes to complete
- All credentials are auto-generated and destroyed after each run
- Results vary by machine — use for relative comparisons, not absolute numbers

See [`perf-test/README.md`](perf-test/README.md) for full details.

---

*Copyright 2026 The MathWorks, Inc.*
