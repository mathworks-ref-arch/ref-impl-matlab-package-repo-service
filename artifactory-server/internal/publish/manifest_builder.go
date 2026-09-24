// Copyright 2026 The MathWorks, Inc.

package publish

import (
	"encoding/json"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
)

func BuildManifest(mp *MPackage, archive domain.Archive) (*domain.PackageManifest, error) {
	var deps domain.Dependencies
	if mp.Dependencies != nil {
		if err := json.Unmarshal(mp.Dependencies, &deps); err != nil {
			return nil, err
		}
	}

	manifest := &domain.PackageManifest{
		Name:                       mp.Name,
		Version:                    mp.Version,
		ID:                         mp.ID,
		DisplayName:                mp.DisplayName,
		Summary:                    mp.Summary,
		FormerNames:                mp.FormerNames,
		Provider:                   domain.Provider{Name: mp.Provider.Name, Organization: mp.Provider.Organization},
		Dependencies:               deps,
		ReleaseCompatibility:       mp.ReleaseCompatibility,
		SupportedPlatforms:         mp.SupportedPlatforms,
		Archives:                   []domain.Archive{archive},
		RequiredAdditionalSoftware: []string{},
	}

	if err := manifest.Validate(); err != nil {
		return nil, err
	}

	return manifest, nil
}
