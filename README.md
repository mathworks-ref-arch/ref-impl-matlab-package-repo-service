# Reference Implementation for MATLAB Package Repository Service for Artifactory

This repository provides a reference implementation of the [MATLAB Package Repository REST API](./docs/matlab-package-repository-api-spec.md) for JFrog® Artifactory®, showing how the MATLAB® Package Manager (MPM) can integrate with your organization's existing artifact storage. Once integrated, your developers can develop and deploy MATLAB packages (.mltbx files) to Artifactory, and your end users can use MPM to install packages from Artifactory into MATLAB.

## Get Started

**[Artifactory](artifactory-server/):** How to set up and run a MATLAB package repository service that enables the MATLAB Package Manager to discover, install, and publish MATLAB packages from your organization's JFrog Artifactory instance.

**Limitations:**
* The MATLAB Package Repository API is only supported by MPM in MATLAB **R2026b** and later
* The reference implementation supports only `.mltbx` files packaged in MATLAB **R2026b** and later


## Roles and Personas

```
┌──────────┐           ┌────────────────────────────────┐         ┌──────────────┐
│  Admin   │──deploy──►│    Reference Implementation    │◄───────►│    Storage   │
└──────────┘           └────────────────────────────────┘         └──────────────┘
                                ▲                ▲ 
                        publish │                │ query + install
                                │                │ 
                         ┌──────┴────┐    ┌──────┴─────────┐
                         │ Developer │    │ Consumer (MPM) │
                         └───────────┘    └────────────────┘
```

| Role | What they do |
|------|-------------|
| **Admin** | Administers the storage, deploys / operates the reference implementation, and distributes configuration to the team |
| **Developer** | Authors MATLAB packages and publishes .mltbx files via CI or manually |
| **Consumer** | Installs packages in MATLAB using MPM |

## MPM API Specification

The reference implementation supports the [MATLAB Package Repository API](docs/matlab-package-repository-api-spec.md). The table below lists the API endpoints; see the implementation's README for its support status.

| Capability | Required by MPM |
|------------|----------------|
| `GET /v1/packages/by-uuid/{uuid}.json` | Required |
| `GET /v1/packages/by-name/{name}.json` | Required |
| `POST /v1/packages/publish` | Optional |
| `GET /health-check` | Optional |
| Authentication (header presence check) | Optional |
| Pagination (`nextUrl`) | Optional |

## Support and Extensions

For general support of the MATLAB Package Manager, contact [MathWorks Support](https://www.mathworks.com/support.html).

For production use cases or extended capabilities (including the below), contact [MathWorks Consulting](https://www.mathworks.com/services/consulting.html):
- Support for similar workflows in R2026a and earlier
- User-managed access control and integration with enterprise identity providers (SSO)
- Support for alternative artifact management systems and storage
- A web-based store UI to aid user navigation of packages

---

*Copyright 2026 The MathWorks, Inc.*
