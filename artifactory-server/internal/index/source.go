// Copyright 2026 The MathWorks, Inc.

package index

import "github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"

// ManifestSource provides package manifests from a backend.
type ManifestSource interface {
	ListManifests() ([]domain.PackageManifest, error)
}
