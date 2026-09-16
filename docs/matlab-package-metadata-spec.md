```
MPRS-Spec: 2
Title: MATLAB Package Manifest Schema
Author: MathWorks Package Management Team
Status: Draft
Type: Standards Track
Created: 2026-05-18
Requires: MPRS-Spec 1, RFC 2119, Semantic Versioning 2.0.0
Target-Release: R2026b
```

# MATLAB Package Manifest Schema Specification

## Abstract

This specification defines the Package Manifest -- the canonical JSON schema
used by the MATLAB Package Repository Service (MPRS) to represent package
metadata. The manifest encapsulates all data required by the MATLAB Package
Manager (MPM) to perform install, search, and dependency resolution activities.
This document is a companion to MPRS-Spec 1 (HTTP API Specification) and
normatively defines the `PackageManifest` object referenced therein.


## Motivation

The MPRS HTTP API (MPRS-Spec 1) returns `PackageManifest` objects from all
query endpoints. A precise, versioned schema definition is necessary to ensure:

1. Interoperability between MPRS implementations and MPM client versions.
2. Correct server-side manifest generation during the publish workflow.
3. Unambiguous field semantics for dependency resolution and platform filtering.
4. Forward compatibility as new fields are introduced in future releases.


## Specification

### Conventions and Terminology

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC 2119.

| Term | Definition |
|------|-----------|
| Package Manifest | A JSON object describing a single version of a MATLAB package |
| MATLAB Identifier | A string matching the pattern `[a-zA-Z][a-zA-Z0-9_]*` (valid MATLAB variable/package name) |
| Semver | A version string conforming to Semantic Versioning 2.0.0 (MAJOR.MINOR.PATCH) |
| Semver Range | A node-semver compatible version range expression (e.g., `>=9.14.0 <9.28.0`) |
| Platform | A target operating system: `win64`, `glnxa64`, or `maca64` |
| Architecture | A CPU architecture qualifier for a given platform |
| UUID | A Version 4 UUID as defined by RFC 4122 |


### Schema Overview

A `PackageManifest` is a JSON object with the following top-level structure:

```json
{
  "name": "<string>",
  "displayName": "<string>",
  "formerNames": ["<string>", ...],
  "id": "<string>",
  "version": "<string>",
  "releaseCompatibility": "<string>",
  "summary": "<string>",
  "description": "<string>",
  "provider": { <Provider> },
  "dependencies": [ <Dependency>, ... ],
  "supportedPlatforms": [ <PlatformSpec>, ... ],
  "archives": [ <Archive>, ... ],
  "requiredAdditionalSoftware": [ "<string>", ... ]
}
```


### Field Definitions

#### 1. `name`

| Property | Value |
|----------|-------|
| Type | string |
| Required | MUST |
| Restrictions | MUST be a valid MATLAB identifier |

The canonical package name. Used as the primary human-readable identifier for
install commands (e.g., `mpm install <name>`). MUST conform to MATLAB identifier
rules: begin with a letter, followed by letters, digits, or underscores.

Package names SHOULD be lowercase. Registries enforce case-insensitive
uniqueness: two packages with names that differ only in casing (e.g.,
"Battery" and "battery") MUST NOT coexist under different UUIDs within the
same repository. See MPRS-Spec 1, Publish Validation Requirement 5.

```json
"name": "traffic_light_toolbox"
```

---

#### 2. `displayName`

| Property | Value |
|----------|-------|
| Type | string |
| Required | SHOULD |
| Restrictions | None |

A human-readable display name for UI presentation. MAY contain spaces, special
characters, and mixed case. Not used for programmatic lookup.

```json
"displayName": "Traffic Light Toolbox"
```

---

#### 3. `formerNames`

| Property | Value |
|----------|-------|
| Type | string[] |
| Required | MAY |
| Restrictions | Each element MUST be a valid MATLAB identifier |

A list of previous names this package was known by. Used to support rename
scenarios -- MPM clients MAY use former names for backwards-compatible package
resolution.

```json
"formerNames": ["TrafficLightTbx", "TL_Toolbox"]
```

---

#### 4. `id`

| Property | Value |
|----------|-------|
| Type | string |
| Required | MUST |
| Restrictions | MUST be a valid Version 4 UUID (RFC 4122) |

The globally unique, immutable identifier for the package. All versions of the
same logical package share the same `id`. The `id` is assigned at package
creation time and MUST NOT change across versions or renames.

```json
"id": "27043d83-a1b2-4c3d-9e4f-567890abcdef"
```

---

#### 5. `version`

| Property | Value |
|----------|-------|
| Type | string |
| Required | MUST |
| Restrictions | MUST conform to Semantic Versioning 2.0.0 |

The version of this specific package release. The combination of `id` +
`version` MUST be unique within a repository (enforced by MPRS via 409
Conflict).

```json
"version": "3.1.0"
```

---

#### 6. `releaseCompatibility`

| Property | Value |
|----------|-------|
| Type | string |
| Required | SHOULD |
| Restrictions | MUST be a valid node-semver version range expression |

Specifies the range of MATLAB releases this package version is compatible with.
MPM uses this field to filter packages that are installable on the user's
current MATLAB release.

```json
"releaseCompatibility": ">=9.14.0 <9.28.0"
```

---

#### 7. `summary`

| Property | Value |
|----------|-------|
| Type | string |
| Required | SHOULD |
| Restrictions | MUST be a single line (no newline characters) |

A brief, one-line description of the package suitable for display in search
results and package listings.

```json
"summary": "Traffic light detection and classification algorithms"
```

---

#### 8. `description`

| Property | Value |
|----------|-------|
| Type | string |
| Required | MAY |
| Restrictions | None |

A full-length description of the package. MAY contain multiple lines and
formatting. No length restriction.

```json
"description": "A comprehensive toolbox for detecting, classifying, and simulating traffic light systems..."
```

---

#### 9. `provider`

| Property | Value |
|----------|-------|
| Type | Provider object |
| Required | SHOULD |
| Restrictions | See Provider Object definition |

Identifies the author or organization that provides this package. Required for
search, filtering, and registration workflows.

```json
"provider": {
  "name": "Jane Smith",
  "organization": "MathWorks"
}
```

---

#### 10. `dependencies`

| Property | Value |
|----------|-------|
| Type | Dependency[] |
| Required | MAY |
| Restrictions | See Dependency Object definition |

A list of packages that this package depends on. MPM uses this field for
transitive dependency resolution during install.

```json
"dependencies": [
  {
    "name": "Computer_Vision_Toolbox",
    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    "compatibleVersions": ">=2.0.0"
  }
]
```

---

#### 11. `supportedPlatforms`

| Property | Value |
|----------|-------|
| Type | PlatformSpec[] |
| Required | SHOULD |
| Restrictions | See PlatformSpec Object definition |

Declares which platforms (and optionally architectures) this package supports.
An empty array or omission indicates the package is platform-independent.

```json
"supportedPlatforms": [
  {"platform": "win64", "architectures": ["x86_64"]},
  {"platform": "glnxa64", "architectures": ["x86_64"]},
  {"platform": "maca64", "architectures": ["arm64"]}
]
```

---

#### 12. `archives`

| Property | Value |
|----------|-------|
| Type | Archive[] |
| Required | MUST |
| Restrictions | See Archive Object definition |

The downloadable artifacts for this package version. Archives are keyed by
platform. For `.mltbx` packages and custom add-on packages,
typically a single platform-independent archive is provided.

```json
"archives": [
  {
    "platforms": ["agnostic"],
    "url": "https://artifactory.example.com/repo/Traffic_Light_Toolbox/27043d83/3.1.0/Traffic_Light_Toolbox-3.1.0.mltbx",
    "size": 15728640,
    "digests": [
      {
        "alg": "sha512",
        "digest": "a1b2c3d4e5f6..."
      }
    ]
  }
]
```

---

#### 13. `requiredAdditionalSoftware`

| Property | Value |
|----------|-------|
| Type | string[] |
| Required | MAY |
| Restrictions | None |

Declares third-party (non-MATLAB) software that this package requires.
These are informational -- MPM does not install third-party dependencies but
MAY surface them to the user.

```json
"requiredAdditionalSoftware": ["OpenCV", "CUDA Toolkit"]
```


### Nested Object Definitions

#### Provider Object

```json
{
  "name": "<string>",
  "organization": "<string>"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | MAY | Provider's name. Can be empty. |
| `organization` | string | MAY | Provider's organization. |

At least one of `name` or `organization` SHOULD be present for search and
filtering purposes.

---

#### Dependency Object

```json
{
  "name": "<string>",
  "id": "<string>",
  "compatibleVersions": "<string>"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | MUST | Dependency package name (MATLAB identifier) |
| `id` | string | MUST | Dependency package UUID (Version 4 UUID) |
| `compatibleVersions` | string | MUST | Acceptable version range (node-semver range) |

The combination of `name` and `id` identifies the dependency. The
`compatibleVersions` field constrains which versions satisfy the dependency.

---

#### PlatformSpec Object

```json
{
  "platform": "<string>",
  "architectures": ["<string>", ...]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `platform` | string | MUST | Platform identifier (see Platform Enumeration) |
| `architectures` | string[] | MAY | Architecture qualifiers for this platform |

---

#### Archive Object

```json
{
  "platforms": ["<string>", ...],
  "url": "<string>",
  "size": <integer>,
  "digests": [ <Digest>, ... ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `platforms` | string[] | MUST | Target platforms (e.g., `["agnostic"]`, `["win64", "glnxa64"]`) |
| `url` | string | MUST | Absolute URL to download the `.mltbx` archive |
| `size` | integer | MUST | Archive size in bytes |
| `digests` | Digest[] | MUST | Cryptographic digests of the archive |

The `url` MUST be an absolute IRI pointing to the storage backend
location of the archive file.

---

#### Digest Object

```json
{
  "alg": "<string>",
  "digest": "<string>"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `alg` | string | MUST | Hash algorithm identifier (e.g., `"sha512"`) |
| `digest` | string | MUST | Base64-encoded hash value |

Implementations MUST support `sha512`. Additional algorithms MAY be supported
in future revisions.

---



### Platform Enumeration

The following platform identifiers are defined:

| Identifier | Description |
|------------|-------------|
| `win64` | Microsoft Windows (64-bit) |
| `glnxa64` | GNU/Linux (x86_64) |
| `maca64` | macOS (Apple Silicon / ARM64) |
| `maci64` | macOS (Intel x86_64) -- legacy |
| `any` | Platform-independent (used in archives) |

Implementations MUST recognize all identifiers listed above. Additional
platform identifiers MAY be introduced in future revisions.


### Package Identifier

A package is uniquely identified by the tuple `(id, version)`. The `name`
field provides human-readable lookup and MUST be case-insensitively unique
within a single repository (i.e., no two packages with different UUIDs MAY
share the same case-folded name).

The `id` is the stable identifier across renames -- the `formerNames` field
tracks historical names for backwards compatibility.


### Manifest Generation

During the publish workflow (MPRS-Spec 1, `POST /v1/packages/publish`), the
server generates the `PackageManifest` by:

1. Extracting `fsroot/resources/mpackage.json` from the uploaded `.mltbx`
   archive.
2. Parsing the `mpackage.json` and mapping its fields to the manifest schema.
3. Computing the archive digest (SHA-512) and size.
4. Constructing the `archives` array with the storage backend download URL.

The server is the authoritative source for the `archives` field (including
`url`, `size`, and `digests`). All other fields are derived from the
package's `mpackage.json`.


### Schema Versioning

This schema corresponds to the MPRS V1 API. Future schema changes follow these
rules:

- **Additive changes** (new optional fields): Backward-compatible. Clients MUST
  ignore unknown fields.
- **Restrictive changes** (making optional fields required): Requires a new API
  version.
- **Removal of fields**: Requires a new API version.

Clients SHOULD be written to tolerate the presence of unknown fields in
manifest objects (forward compatibility).


## Provisional Fields (Deferred)

The following fields are under review for inclusion in future releases. They are
NOT normative for R2026b implementations.

### Archive Signature Verification (Target: R2027a)

Additional fields in the Archive object to support cryptographic signature
verification of downloaded packages:

```json
{
  "platforms": ["agnostic"],
  "url": "...",
  "size": 15728640,
  "digests": [{ "alg": "sha512", "digest": "..." }],
  "signature": {
    "algorithm": "<string>",
    "value": "<string>",
    "certificate": "<string>"
  }
}
```

### Dependency Type (Target: TBD)

An optional `type` field on `Dependency` objects to distinguish required vs.
optional dependencies:

```json
{
  "name": "Deep_Learning_Toolbox",
  "id": "...",
  "compatibleVersions": ">=1.0.0",
  "type": "optional"
}
```

Valid values: `"required"` (default), `"optional"`.

### `hasLicenseFile` (Target: R2026b)

A boolean field indicating whether the package archive contains a license file:

```json
"hasLicenseFile": true
```


## Out of Scope

The following data is explicitly NOT part of the Package Manifest schema. It
belongs to marketplace/browse layers built on top of the MPRS:

- Categories and product family classification
- User ratings and reviews
- Download counts
- Last modified timestamps (the index-level `lastModified` in MPRS-Spec 1
  covers index freshness, not per-package modification time)
- Public/private visibility flags (repository-level concern, not per-manifest)


## Complete Example

```json
{
  "name": "Traffic_Light_Toolbox",
  "displayName": "Traffic Light Toolbox",
  "formerNames": ["TrafficLightTbx"],
  "id": "27043d83-a1b2-4c3d-9e4f-567890abcdef",
  "version": "3.1.0",
  "releaseCompatibility": ">=9.14.0 <9.28.0",
  "summary": "Traffic light detection and classification algorithms",
  "description": "A comprehensive toolbox for detecting, classifying, and simulating traffic light systems using computer vision and deep learning techniques.",
  "provider": {
    "name": "Jane Smith",
    "organization": "MathWorks"
  },
  "dependencies": [
    {
      "name": "Computer_Vision_Toolbox",
      "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
      "compatibleVersions": ">=2.0.0"
    }
  ],
  "supportedPlatforms": [
    {"platform": "win64", "architectures": ["x86_64"]},
    {"platform": "glnxa64", "architectures": ["x86_64"]},
    {"platform": "maca64", "architectures": ["arm64"]}
  ],
  "archives": [
    {
      "platforms": ["agnostic"],
      "url": "https://artifactory.example.com/matlab-packages/Traffic_Light_Toolbox/27043d83-a1b2-4c3d-9e4f-567890abcdef/3.1.0/Traffic_Light_Toolbox-3.1.0.mltbx",
      "size": 15728640,
      "digests": [
        {
          "alg": "sha512",
          "digest": "a1b2c3d4e5f67890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678"
        }
      ]
    }
  ],
  "requiredAdditionalSoftware": ["OpenCV"]
}
```


## References

- MPRS-Spec 1: MATLAB Package Repository Service HTTP API Specification
- RFC 2119: Key words for use in RFCs to Indicate Requirement Levels
- RFC 4122: A Universally Unique IDentifier (UUID) URN Namespace
- Semantic Versioning 2.0.0 (semver.org)
- node-semver: The semver parser for Node.js (range syntax reference)
- MATLAB Identifier naming rules (MathWorks documentation)


---

*Copyright 2026 The MathWorks, Inc.*
