// Copyright 2026 The MathWorks, Inc.

package publish

import (
	"errors"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
)

var ErrConflict = errors.New("package version already exists")

// PackageUploader abstracts the storage backend for publishing packages.
// Upload methods accept a caller token for authenticating as the requesting user.
type PackageUploader interface {
	Publish(archivePath string, manifest *domain.PackageManifest, callerToken string) error
	ArchiveURL(manifest *domain.PackageManifest) string
}
