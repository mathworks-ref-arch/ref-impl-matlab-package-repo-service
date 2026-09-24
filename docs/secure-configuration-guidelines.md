# Secure Configuration Guidelines

This section describes the security posture of the service itself. Deploying
organizations are responsible for hardening their infrastructure according to
their own security policies.

## Authentication Model

The service does not authenticate requests. It never verifies identity, roles, or
token validity - Artifactory validates tokens when they are used against the
backend. This means:

- The service does not maintain user accounts or sessions
- Access control decisions are delegated entirely to Artifactory
- Query endpoints are served from the in-memory index and are unauthenticated: anyone who can reach the service can read the package index, including every package name, version, UUID, and download URL
- Publish and refresh requests must carry a bearer token, which is passed through to Artifactory and validated there
- Artifact downloads are authenticated by Artifactory, not by this service

## Secrets

| Secret | How it's used | Exposure if leaked |
|--------|---------------|-------------------|
| `ARTIFACTORY_SERVICE_TOKEN` | Read for indexing | Attacker can read all manifests |
| Developer tokens (passed via header) | Write access for publishing | Attacker can publish or overwrite packages |

Store all tokens in a secrets manager (e.g., HashiCorp Vault, AWS Secrets
Manager, or Kubernetes Secrets) rather than in plaintext environment variables,
shell history, or source control. Scope the service token's Artifactory
permissions to read-only access on the target repository. Developer and CI
tokens should be limited to the minimum required permissions (read + write to
the specific repository) using Artifactory's Role-Based Access Control (RBAC)
or your identity provider's Identity and Access Management (IAM) roles.

## Container Security

The provided Dockerfile:
- Uses a multi-stage build; the final image is based on `alpine:latest`
- Produces a statically compiled binary (`CGO_ENABLED=0`)
- Exposes only port 8080
- Runs as root (no `USER` directive)
- Includes a shell and package manager (`sh`, `apk`)
- Excludes `.env` files and local build artifacts from the build context via
  `artifactory-server/.dockerignore`, so secrets are not sent to the Docker daemon

## Network Assumptions

The service **does not terminate TLS** and assumes it runs behind a
TLS-terminating reverse proxy or load balancer configured with TLS 1.2 or
higher. Because tokens are sent as plaintext `Authorization` headers, deploying
without TLS exposes credentials to interception.

- The service itself listens on one port only (8080 by default)
- Clients must reach both the service and Artifactory: package manifests carry download URLs that point at Artifactory, and clients fetch archives from there directly
- The service requires outbound HTTPS access to Artifactory
- The `/health-check` endpoint is unauthenticated by design - restrict access at the network layer if this is unacceptable

## What the Service Does NOT Provide

| Concern | Status |
|---------|--------|
| Rate limiting / throttling | Not implemented - use an API gateway or reverse proxy |
| IP allowlisting | Not implemented - use network-layer controls |
| Audit logging of access | Service logs include method, path, and status only; detailed access auditing is provided by Artifactory |
| Encryption at rest | Delegated to Artifactory and underlying storage |
| Identity-based access control | Delegated to Artifactory |
| Automatic token rotation | Not supported - rotate manually and restart |

---

*Copyright 2026 The MathWorks, Inc.*
