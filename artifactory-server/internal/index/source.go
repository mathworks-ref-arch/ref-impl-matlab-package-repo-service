// Copyright 2026 The MathWorks, Inc.

package index

import "github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"

// ManifestSource provides package manifests from a backend.
type ManifestSource interface {
	ListManifests() ([]domain.PackageManifest, error)
}
