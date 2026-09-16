# Artifactory Reference Implementation

This document shows you how to set up and run a MATLAB® package repository service that enables the MATLAB Package Manager (MPM) to discover, install, and publish MATLAB packages (`.mltbx` files) from your organization's Artifactory instance.

JFrog® Artifactory® can store MATLAB package files but does not understand MATLAB package metadata — it cannot query packages by name or UUID, resolve dependencies, or filter by platform and MATLAB release compatibility. This service bridges that gap by indexing MATLAB package metadata stored in Artifactory and serving the data in the structured format that MPM expects.

## Requirements

- JFrog Artifactory instance (self-hosted or cloud) with a **Generic** repository
- Docker® (recommended) or Go 1.26+ for building from source
- MATLAB R2026b or later (for package consumers)
- Artifactory access tokens for the service and for users

> **Note:** For MATLAB releases prior to R2026b or for advanced use cases, contact [MathWorks Consulting](https://www.mathworks.com/services/consulting.html).

## Quick Start

```bash
docker build -t mpm-repo-server .

docker run -p 8080:8080 \
  -e ARTIFACTORY_URL=https://artifactory.corp.com/artifactory \
  -e ARTIFACTORY_REPO_KEY=mw-packages \
  -e ARTIFACTORY_SERVICE_TOKEN=your-service-token \
  -e ENABLE_PUBLISH=true \
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
| `ARTIFACTORY_URL` | Yes | Artifactory base URL |
| `ARTIFACTORY_REPO_KEY` | Yes | Name of the Generic repository |
| `ARTIFACTORY_SERVICE_TOKEN` | Yes | Token with read access (indexing at startup) |
| `PORT` | No | Server port (default: `8080`) |
| `ENABLE_PUBLISH` | No | Enable the publish endpoint (`true` or `1`) |

### Server Configuration File

The service reads its configuration from `configs/server-artifactory.json`. This file controls HTTP timeouts, backend communication limits, and upload size bounds. The default values are tuned for security hardening:

```json
{
  "server": {
    "listenAddr": ":8080",
    "basePath": "/v1",
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

The service is deployed once by an admin for the entire team. Individual users do not need to run it.

### Create a Generic Repository in Artifactory

Create a **Generic** repository type in Artifactory. When packages are published via the service, the following layout is created automatically:

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
ARTIFACTORY_REPO_KEY=mw-packages
ARTIFACTORY_SERVICE_TOKEN=your-service-token
ENABLE_PUBLISH=true
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
export ARTIFACTORY_REPO_KEY=mw-packages
export ARTIFACTORY_SERVICE_TOKEN=your-service-token
export ENABLE_PUBLISH=true

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

Consumers never interact with Artifactory directly. MATLAB talks to the service, which handles all backend communication.

---

## Index Refresh

When packages are added or deleted directly in Artifactory, rebuild the in-memory index:

```bash
curl -X POST https://mprs.corp.com:8080/v1/admin/refresh \
  -H "Authorization: Bearer $YOUR_TOKEN"
```

The refresh endpoint uses the caller's Bearer token to authenticate with the backend (not the server's service token).

---

## Security Considerations

For details on the service's authentication model, secrets handling, container hardening, and network assumptions, see the [Secure Configuration Guidelines](../docs/secure-configuration-guidelines.md).

---

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Service fails to start | Cannot reach Artifactory | Verify `ARTIFACTORY_URL` is correct and the service token is valid |
| `401 Unauthorized` on queries | Missing or invalid token | Ensure `ARTIFACTORY_TOKEN` env var is set and the token has read access |
| `409 Conflict` on publish | Package version already exists | Bump the version before re-publishing |
| Empty index after startup | Wrong repo key or no manifests | Verify `ARTIFACTORY_REPO_KEY` matches your repository and that it contains `*.manifest.json` files |
| MATLAB cannot connect | Wrong URL in repo-config.json | Ensure the `url` field points to the service (not directly to Artifactory) |

---

## MPM API Specification

This service implements the [MATLAB Package Repository API](../docs/matlab-package-repository-api-spec.md). The following table summarizes what is covered in this reference implementation:

| Capability | Status | Needs Authentication |
|------------|--------|---------------------|
| `GET /v1/packages/by-uuid/{uuid}.json` | Implemented | No |
| `GET /v1/packages/by-name/{name}.json` | Implemented | No |
| `GET /v1/packages/index.json` | Implemented (single page, no pagination) | No |
| `POST /v1/packages/publish` | Implemented | Yes |
| `GET /health-check` | Implemented | No |
| Authentication (header presence check) | Implemented | N/A |
| Pagination (`nextUrl`) | Not yet implemented | N/A |
| `GET /version` | Not yet implemented | N/A |
| Download endpoint | Not yet implemented | N/A |

---


*Copyright 2026 The MathWorks, Inc.*
