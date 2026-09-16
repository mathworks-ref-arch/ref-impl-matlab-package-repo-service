// Copyright 2026 The MathWorks, Inc.

package publish

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
)

const mpackagePath = "fsroot/resources/mpackage.json"
const maxMPackageSize = 1 << 20 // 1 MB

type MPackage struct {
	Name                 string          `json:"name"`
	Version              string          `json:"version"`
	ID                   string          `json:"id"`
	FormerNames          []string        `json:"formerNames"`
	DisplayName          string          `json:"displayName"`
	Summary              string          `json:"summary"`
	Provider             MPackageProvider `json:"provider"`
	Dependencies         json.RawMessage `json:"dependencies"`
	ReleaseCompatibility string          `json:"releaseCompatibility"`
	SupportedPlatforms   json.RawMessage `json:"supportedPlatforms"`
}

type MPackageProvider struct {
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Email        string `json:"email"`
	URL          string `json:"url"`
}

func ExtractMPackage(mltbxPath string) (*MPackage, error) {
	zr, err := zip.OpenReader(mltbxPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open .mltbx: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != mpackagePath {
			continue
		}
		if f.UncompressedSize64 > maxMPackageSize {
			return nil, fmt.Errorf("mpackage.json exceeds maximum size of %d bytes", maxMPackageSize)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("failed to open mpackage.json in archive: %w", err)
		}
		defer rc.Close()

		data, err := io.ReadAll(io.LimitReader(rc, maxMPackageSize))
		if err != nil {
			return nil, fmt.Errorf("failed to read mpackage.json: %w", err)
		}

		var mp MPackage
		if err := json.Unmarshal(data, &mp); err != nil {
			return nil, fmt.Errorf("failed to parse mpackage.json: %w", err)
		}
		return &mp, nil
	}

	return nil, fmt.Errorf("fsroot/resources/mpackage.json not found in .mltbx archive")
}
