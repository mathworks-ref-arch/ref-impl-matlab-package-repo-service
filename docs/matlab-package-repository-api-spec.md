```
Spec: 1
Title: MATLAB Package Repository API Specification
Author: MathWorks Package Management Team
Type: Standards Track
Created: 2026-05-18
Requires: RFC 2119, RFC 6750, RFC 7231, RFC 7578, RFC 9110
```

# MATLAB Package Repository API Specification

## Abstract

This specification defines the HTTP API for MATLAB Package Repositories --
services that expose structured, queryable metadata for MATLAB packages to the
MATLAB Package Manager (MPM) client. The API provides deterministic, path-based
endpoints for discovering, querying, and publishing MATLAB packages (`.mltbx`
archives) over HTTP.


## Motivation

The MPM client requires structured, searchable package metadata in a
deterministic REST format to resolve dependencies and install packages. Generic
artifact managers do not natively understand MATLAB package metadata.

Without a dedicated repository service:

1. Clients cannot query packages by name, UUID, or version in a structured
   format -- generic storage only offers file-level access.

2. Organizations must build custom bridging solutions to serve package metadata.

3. Filtering by platform, version, and compatibility is unavailable since MATLAB
   packages are not a known type to generic storage systems.

4. Publishing workflows must understand the internal metadata schema and storage
   layout -- an error-prone and tightly coupled arrangement.

5. Concurrent publishes of the same version can corrupt storage state without
   conflict detection.

This specification defines a standard API that enables any conforming service to
serve as a MATLAB package repository, regardless of the underlying storage
technology.


## Specification

### Conventions and Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

| Term | Definition |
|------|-----------|
| Registry | The HTTP API service defined by this specification; accepts queries and publishes from clients |
| Repository | The underlying storage backend (e.g., Artifactory, Nexus, S3) where package artifacts and metadata are persisted |
| MPM | MATLAB Package Manager -- the client that queries the registry |
| MLTBX | A MATLAB toolbox archive (ZIP format with `.mltbx` extension) |
| Package Metadata | A JSON object describing a single version of a MATLAB package |
| UUID | A package's unique identifier, as declared in its `mpackage.json` |

[1] The term "registry" follows established convention in other package
management ecosystems. npm (registry.npmjs.org), Docker (Registry HTTP API V2),
Cargo (crates.io registry), and NuGet all use "registry" to denote the API
service that clients interact with, distinct from the underlying storage.
See also: PEP 503 (Python Simple Repository API), Docker Registry API V2
specification, npm Registry API documentation.


### Base URL and Versioning

All endpoints defined in this specification are prefixed with `/v1/`. The
version component is a monotonically increasing integer that MUST be incremented
when backwards-incompatible changes are introduced to the API.

All endpoints use deterministic, path-based routing with no query parameters.

```
{scheme}://{host}:{port}/v1/{endpoint}
```


### Content Types

All JSON response endpoints MUST return responses with the header:

```
Content-Type: application/json; charset=utf-8
```

The publish endpoint (`POST /v1/packages/publish`) accepts requests with:

```
Content-Type: multipart/form-data
```

Clients SHOULD send an `Accept: application/json` header on all requests.
Registries MAY reject requests with an unsupported `Accept` header with
`406 Not Acceptable`, but this is not required.


### Authentication

The registry supports optional authentication via HTTP headers:

- When authentication is configured, the registry MUST validate the **presence**
  of required headers (e.g., `Authorization`). If a required authentication
  header is missing, the registry MUST respond with `401 Unauthorized`.

- A conforming implementation MAY perform full token validation (signature
  verification, expiry checks, scope enforcement) using its own authentication
  system. This specification does not constrain how an implementation validates
  tokens -- only that presence is the minimum required check.

  > **Note:** The MathWorks reference implementation delegates token
  > introspection to the client's identity provider and only validates header
  > presence. Enterprise implementations (e.g., JFrog Artifactory, Sonatype
  > Nexus) will typically integrate with their native auth subsystems and
  > perform full validation.

- The `GET /health-check` and `GET /version` endpoints MUST NOT require
  authentication.

A registry MAY be configured to require no authentication, in which case all
endpoints are publicly accessible.

The registry MUST NOT log or persist client authentication tokens.


### Endpoints

#### GET /v1/packages/index.json

Returns metadata for all versions of all packages available in the registry.

**Request:**

```
GET /v1/packages/index.json HTTP/1.1
Host: registry.example.com
Authorization: Bearer <token>
```

No request body. No query parameters.

**Response (200 OK):**

<details>
<summary>Example response (click to expand)</summary>

```json
{
  "packages": [
    {
      "version": "3.1.0",
      "name": "Traffic_Light_Toolbox",
      "id": "27043d83-a1b2-4c3d-9e4f-567890abcdef",
      "displayName": "Traffic Light Toolbox",
      "summary": "Traffic light detection and classification algorithms",
      "provider": {
        "name": "Jane Smith",
        "organization": "MathWorks"
      },
      "releaseCompatibility": ">=R2024b",
      "dependencies": [
        {
          "name": "Computer_Vision_Toolbox",
          "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
          "compatibleVersions": ">=2.0.0"
        }
      ],
      "supportedPlatforms": [
        {"platform": "windows", "architectures": ["any"]},
        {"platform": "linux", "architectures": ["any"]},
        {"platform": "macos", "architectures": ["any"]}
      ],
      "formerNames": ["TrafficLightTbx"],
      "archives": [
        {
          "platforms": ["agnostic"],
          "url": "https://artifacts.example.com/matlab-packages/Traffic_Light_Toolbox/27043d83-a1b2-4c3d-9e4f-567890abcdef/3.1.0/Traffic_Light_Toolbox-3.1.0.mltbx",
          "size": 15728640,
          "digests": [
            {
              "alg": "sha512",
              "digest": "a1b2c3d4e5f67890abcdef..."
            }
          ]
        }
      ],
      "requiredAdditionalSoftware": []
    }
  ],
  "pagination": {
    "nextUrl": "/v1/packages/page-2.json",
    "total": 350
  },
  "lastModified": "2026-05-18T14:30:00Z"
}
```

</details>

| Field | Type | Description |
|-------|------|-------------|
| `packages` | PackageMetadata[] | Package metadata entries for this page |
| `pagination.nextUrl` | string or null | Path to the next page, or `null` if this is the final page |
| `pagination.total` | integer | Total number of packages across all pages |
| `lastModified` | string | RFC 3339 timestamp indicating when the data was last modified |

The `PackageMetadata` object schema is fully defined in the companion
specification: **MATLAB Package Metadata Schema Specification**
(`matlab-package-metadata-spec.md`). Subsequent endpoints in this document use the
shorthand `<PackageMetadata>` to refer to this object.

The response MUST always include the `pagination` object. When `nextUrl` is
`null`, the client has received the final page. Subsequent pages are fetched
via the path in `nextUrl`:

```
GET /v1/packages/page-2.json HTTP/1.1
```

Page size is determined by the registry. Clients MUST NOT assume a fixed page
size.

Clients MUST ignore unknown fields in `PackageMetadata` objects (forward
compatibility).

---

#### GET /v1/packages/by-uuid/{uuid}.json

Returns metadata for all versions of a specific package, identified by UUID.

**Request:**

```
GET /v1/packages/by-uuid/27043d83-a1b2-4c3d-9e4f-567890abcdef.json HTTP/1.1
Host: registry.example.com
Authorization: Bearer <token>
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `uuid` | string | The package UUID. MUST be a valid UUID format (RFC 4122). |

**Response (200 OK):**

The response body is a JSON array of package manifest objects (one per version).

<details>
<summary>Example response (click to expand)</summary>

```json
[
  {
    "version": "1.2.0",
    "name": "science",
    "id": "118409b9-6a71-4754-ab24-31a4f3871e15",
    "displayName": "science",
    "summary": "",
    "provider": {
      "name": "",
      "organization": ""
    },
    "releaseCompatibility": ">=R2024b",
    "dependencies": [
      {
        "name": "math",
        "id": "220e47fe-0b34-4abe-991c-f8f121984346",
        "compatibleVersions": ">=1.1.0"
      }
    ],
    "supportedPlatforms": [
      {"platform": "any", "architectures": ["any"]}
    ],
    "formerNames": [],
    "archives": [
      {
        "platforms": ["agnostic"],
        "url": "https://artifacts.example.com/matlab-packages/science/118409b9-6a71-4754-ab24-31a4f3871e15/1.2.0/science-1.2.0.mltbx",
        "size": 8870,
        "digests": [
          {
            "alg": "sha512",
            "digest": "2nBVUBiYVUMvjLv9KrmVmdx85TZaHO4r9oekxMzapwjF+UeLdJtO37bT+z0OEEi8NHaU80VflZamxQFLi/bfrQ=="
          }
        ]
      }
    ],
    "requiredAdditionalSoftware": []
  },
  {
    "version": "1.1.0",
    "name": "science",
    "id": "118409b9-6a71-4754-ab24-31a4f3871e15",
    "displayName": "science",
    "summary": "",
    "provider": {
      "name": "",
      "organization": ""
    },
    "releaseCompatibility": ">=R2024b",
    "dependencies": [
      {
        "name": "math",
        "id": "220e47fe-0b34-4abe-991c-f8f121984346",
        "compatibleVersions": "1.1.0"
      }
    ],
    "supportedPlatforms": [
      {"platform": "any", "architectures": ["any"]}
    ],
    "formerNames": [],
    "archives": [
      {
        "platforms": ["agnostic"],
        "url": "https://artifacts.example.com/matlab-packages/science/118409b9-6a71-4754-ab24-31a4f3871e15/1.1.0/science-1.1.0.mltbx",
        "size": 7918,
        "digests": [
          {
            "alg": "sha512",
            "digest": "LEH8Y40yCXQafZ8BdP8vcwaiVHxqznvRHlpEodZOQj4ZiCJz+EOZZJb7r2Qx9reVpGpq/BqZdzzBnXfiDaLEjw=="
          }
        ]
      }
    ],
    "requiredAdditionalSoftware": []
  }
]
```

</details>

**Response (404 Not Found):**

```json
{
  "error": {"code": 404, "message": "package with UUID \"118409b9-...\" not found"}
}
```

---

#### GET /v1/packages/by-name/{name}.json

Returns metadata for all versions of a specific package, identified by name.

**Request:**

```
GET /v1/packages/by-name/Car.json HTTP/1.1
Host: registry.example.com
Authorization: Bearer <token>
```

**Path Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | The package name. Case-sensitive. |

**Response (200 OK):**

The response body is a JSON array of package manifest objects (one per version).

<details>
<summary>Example response (click to expand)</summary>

```json
[
  {
    "version": "1.0.0",
    "name": "Car",
    "id": "d8cf5438-5cc1-40e2-b245-2f4ed8e49cb9",
    "displayName": "Car",
    "summary": "",
    "provider": {
      "name": "David Barnes",
      "organization": "MathWorks"
    },
    "releaseCompatibility": "",
    "dependencies": [
      {
        "name": "Battery",
        "id": "c1438758-4eaf-438f-b04f-6a7279735de5",
        "compatibleVersions": "*"
      },
      {
        "name": "Suspension",
        "id": "0c0608e1-15e4-4977-9593-be5f735dff8b",
        "compatibleVersions": "*"
      }
    ],
    "supportedPlatforms": [
      {"platform": "windows", "architectures": ["any"]},
      {"platform": "linux", "architectures": ["any"]},
      {"platform": "macos", "architectures": ["any"]}
    ],
    "formerNames": [],
    "archives": [
      {
        "platforms": ["agnostic"],
        "url": "https://artifacts.example.com/matlab-packages/Car/d8cf5438-5cc1-40e2-b245-2f4ed8e49cb9/1.0.0/Car-1.0.0.mltbx",
        "size": 7022,
        "digests": [
          {
            "alg": "sha512",
            "digest": "+MaVJ+rZ/3gndAQNhKlSLRJAQX8RNONCwX4S2WRH/7C4ATK63NOQcePgkqxoSp5Mpkskb/xEzInqivSPqOE6wA=="
          }
        ]
      }
    ],
    "requiredAdditionalSoftware": []
  }
]
```

</details>

**Response (404 Not Found):**

```json
{
  "error": {"code": 404, "message": "package with name \"Car\" not found"}
}
```

---

#### Search and Filtering

This specification does not define a dedicated search endpoint. The query
endpoints (`index.json`, `by-uuid`, `by-name`) return all versions of the
matched packages, including platform compatibility and MATLAB release
compatibility metadata. Clients perform filtering (by platform, MATLAB release,
version range) on the returned metadata.

This design supports the current client architecture where the MPM client
resolves dependencies and filters results locally. Registries with large
package counts MAY implement server-side search as an extension, but this is
outside the scope of V1.

---

#### POST /v1/packages/publish

Publishes a new version of a MATLAB package to the registry.

**Request:**

```
POST /v1/packages/publish HTTP/1.1
Host: registry.example.com
Authorization: Bearer <token>
Content-Type: multipart/form-data; boundary=----FormBoundary

------FormBoundary
Content-Disposition: form-data; name="file"; filename="Traffic_Light_Toolbox-3.1.0.mltbx"
Content-Type: application/octet-stream

<binary data>
------FormBoundary--
```

The request MUST be a `multipart/form-data` upload with a single file part
named `file`. The file MUST be a valid `.mltbx` archive (ZIP format) containing
a `fsroot/resources/mpackage.json` entry with valid package metadata.

**Size Limit:** The registry MUST enforce a maximum upload size (default:
500 MB). Requests exceeding this limit MUST be rejected with
`413 Content Too Large`.

**Validation Requirements:**

The registry MUST validate the following before accepting a publish:

1. The upload is a valid ZIP archive.
2. The archive contains `fsroot/resources/mpackage.json`.
3. The `mpackage.json` contains a valid UUID (RFC 4122) and semantic version.
4. No package with the same UUID+version already exists in the registry.
5. No package with a different UUID already exists under the same
   case-insensitive name (e.g., publishing "battery" MUST be rejected with
   `409 Conflict` if "Battery" already exists under a different UUID). This
   prevents ambiguity in case-insensitive package lookup.

If validation fails, the registry MUST respond with the appropriate `4xx` error.

**Response (201 Created):**

```json
{
  "id": "27043d83-a1b2-4c3d-9e4f-567890abcdef",
  "name": "Traffic_Light_Toolbox",
  "version": "3.1.0"
}
```

**Error Responses:**

| Status | Condition |
|--------|-----------|
| 400 | Missing `file` field, invalid ZIP, missing `mpackage.json`, invalid UUID/version |
| 401 | Missing required authentication header |
| 409 | Package with same UUID+version already exists |
| 413 | File exceeds maximum upload size |
| 502 | Storage backend failure |

**Behavioral Guarantees:**

- **Atomicity:** A publish either fully succeeds or has no observable effect.
  Clients MUST NOT observe a partially published package.

- **Immediate Visibility:** Upon receiving a `201` response, the published
  package MUST be immediately discoverable via the query endpoints
  (`index.json`, `by-uuid`, `by-name`).

- **Immutability:** Once a UUID+version combination is published, it MUST NOT be
  overwritable. There is no force-publish mechanism in V1 of this specification.

---

#### GET /health-check

Readiness probe. Returns `200 OK` when the registry is ready to serve
requests. This endpoint is outside the `/v1/` namespace and MUST NOT require
authentication.

**Request:**

```
GET /health-check HTTP/1.1
Host: registry.example.com
```

**Response (200 OK):**

```json
{
  "status": "ready"
}
```

While the registry is not yet ready to serve requests, it SHOULD return
`503 Service Unavailable`.

---

#### GET /version

Returns the API version supported by the registry. This endpoint is outside the
`/v1/` namespace and MUST NOT require authentication.

**Request:**

```
GET /version HTTP/1.1
Host: registry.example.com
```

**Response (200 OK):**

```json
{
  "apiVersion": "v1"
}
```


### Error Envelope

All error responses MUST use the following JSON structure:

```json
{
  "error": {
    "code": <integer: HTTP status code>,
    "message": "<string: human-readable description>"
  }
}
```

The `code` field MUST match the HTTP response status code. The `message` field
SHOULD provide enough context for the caller to understand and correct the
issue.


### Status Codes

| HTTP Status | Condition |
|-------------|-----------|
| 200 | Success (query, health-check, version) |
| 201 | Success (publish) |
| 400 | Invalid request (malformed upload, missing metadata, invalid UUID/version) |
| 401 | Missing required authentication header |
| 404 | Package not found (by UUID or name) |
| 409 | Package UUID+version already exists, or case-insensitive name conflict with different UUID |
| 413 | Upload exceeds maximum size |
| 502 | Storage backend failure |
| 503 | Service not ready |

All `4xx` and `5xx` responses MUST include the error envelope body.


### Repository File Layout

This section defines the canonical storage layout for MATLAB package artifacts
and metadata within a repository. Conforming implementations MUST organize
stored files according to this layout. This enables consistent tooling,
migration between providers, and static-hosting implementations.

```
{repoRoot}/
└── {packageName}/
    └── {uuid}/
        └── {version}/
            ├── {packageName}-{version}.manifest.json
            └── {packageName}-{version}.mltbx
```

**Path components:**

| Component | Description |
|-----------|-------------|
| `{repoRoot}` | Repository root path (e.g., Artifactory repository key, S3 bucket prefix) |
| `{packageName}` | Package name as declared in `mpackage.json`. SHOULD be lowercase (see note below). |
| `{uuid}` | Package UUID (RFC 4122 format) as declared in `mpackage.json` |
| `{version}` | Semantic version string (e.g., `2.1.0`) |

**Example:**

```
matlab-packages/
└── Robot_Path_Planner/
    └── 27043d83-a1b2-4c3d-9e4f-567890abcdef/
        └── 2.1.0/
            ├── Robot_Path_Planner-2.1.0.manifest.json
            └── Robot_Path_Planner-2.1.0.mltbx
```

**Requirements:**

- The manifest file MUST contain the full `PackageMetadata` JSON object for
  this version, as defined in the companion metadata schema specification.

- The `.mltbx` file is the original uploaded archive, stored unmodified.

- File names MUST follow the pattern `{packageName}-{version}.{ext}` to
  ensure uniqueness within the version directory.

- Implementations MUST NOT alter the directory structure or file naming when
  storing packages. This guarantees that package artifacts can be migrated
  between conforming implementations by copying the file tree.

- The `{packageName}` directory SHOULD use lowercase (e.g.,
  `traffic_light_toolbox/` rather than `Traffic_Light_Toolbox/`). This avoids
  ambiguity on case-insensitive file systems and aligns with the registry's
  case-insensitive package lookup behavior. When manually uploading packages
  to a generic repository (bypassing the publish API), administrators SHOULD
  ensure folder names are lowercase to prevent name collisions during index
  refresh.


## Backwards Compatibility

This is the initial version of this API specification. Future revisions
MUST maintain backwards compatibility within the same major version (`/v1/`).

Backwards-incompatible changes require incrementing the version prefix (e.g.,
`/v2/`). A registry MAY support multiple API versions simultaneously.

Clients SHOULD gracefully handle the presence of unknown fields in response
bodies (forward compatibility).


## Known Limitations and Future Work

The following capabilities are intentionally excluded from V1 of this
specification. They are documented here to set expectations and provide a
forward path for future revisions.

### Deletion and Deprecation

V1 is a publish-and-query-only specification. There is no mechanism to delete,
yank, or deprecate a published package version via this API. Package lifecycle
management (removal of old versions, marking packages as deprecated) is
expected to be performed by repository administrators using their platform's
native tooling (e.g., JFrog Artifactory's REST API, S3 bucket policies).

A future version of this spec MAY introduce lifecycle endpoints such as
`DELETE /v1/packages/by-uuid/{uuid}/versions/{version}` or a deprecation flag
in the publish workflow.

### Server-side Search

V1 relies on client-side filtering of query results. For registries with
thousands of packages, this may become a performance concern. A future version
MAY introduce a search endpoint (e.g., `GET /v1/packages/search?platform=...`)
for server-side filtering.

### Pagination of Per-Package Version Lists

The `by-uuid` and `by-name` endpoints return all versions of a single package
in a single response. Package version counts are expected to be in the range of
10-20 at any given time. If version counts grow significantly, a future revision
MAY add pagination to these endpoints.

### Caching and Conditional Requests

V1 responses include a `lastModified` field in the response body for the
`index.json` endpoint. This specification does not currently require `ETag`,
`If-None-Match`, or `Cache-Control` response headers. Implementations MAY add
these headers as an optimization. A future version MAY promote caching headers
to a MUST requirement, particularly for registries that proxy remote
repositories.

### Download Endpoint

V1 does not define a standard download endpoint. Package artifacts are
referenced via the `archives[].url` field in metadata, which contains an
absolute URL to the storage backend. A future version MAY define a download
path (e.g., `GET /v1/packages/by-uuid/{uuid}/versions/{version}/download`) for
registries that wish to proxy artifact downloads through the API.


## References

- Spec 2 -- MATLAB Package Metadata Schema Specification (`matlab-package-metadata-spec.md`)
- [RFC 2119](https://www.rfc-editor.org/rfc/rfc2119) -- Key words for use in RFCs to Indicate Requirement Levels
- [RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) -- Date and Time on the Internet: Timestamps
- [RFC 4122](https://www.rfc-editor.org/rfc/rfc4122) -- A Universally Unique IDentifier (UUID) URN Namespace
- [RFC 6750](https://www.rfc-editor.org/rfc/rfc6750) -- The OAuth 2.0 Authorization Framework: Bearer Token Usage
- [RFC 7231](https://www.rfc-editor.org/rfc/rfc7231) -- Hypertext Transfer Protocol (HTTP/1.1): Semantics and Content
- [RFC 7578](https://www.rfc-editor.org/rfc/rfc7578) -- Returning Values from Forms: multipart/form-data
- [RFC 9110](https://www.rfc-editor.org/rfc/rfc9110) -- HTTP Semantics
- [Semantic Versioning 2.0.0](https://semver.org/)


---

*Copyright 2026 The MathWorks, Inc.*
