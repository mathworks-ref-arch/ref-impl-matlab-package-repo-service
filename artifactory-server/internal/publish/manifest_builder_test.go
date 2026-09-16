// Copyright 2026 The MathWorks, Inc.

package publish

import (
	"encoding/json"
	"testing"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
)

func TestBuildManifest(t *testing.T) {
	mp := &MPackage{
		Name:                 "math",
		Version:              "1.1.0",
		ID:                   "220e47fe-0b34-4abe-991c-f8f121984346",
		FormerNames:          []string{},
		DisplayName:          "math",
		Summary:              "A math toolbox",
		Provider:             MPackageProvider{Name: "Author", Organization: "Org", Email: "a@b.com", URL: "https://example.com"},
		Dependencies:         json.RawMessage(`[]`),
		ReleaseCompatibility: ">=R2024b",
		SupportedPlatforms:   json.RawMessage(`[{"platform":"any","architectures":["any"]}]`),
	}

	archive := domain.Archive{
		Platforms: []string{"agnostic"},
		URL:       "https://artifactory.example.com/repo/math/220e47fe/1.1.0/math-1.1.0.mltbx",
		Size:      7538,
		Digests:   []domain.Digest{{Alg: "sha512", Digest: "hlt7W5uPdOY..."}},
	}

	manifest, err := BuildManifest(mp, archive)
	if err != nil {
		t.Fatalf("BuildManifest() error = %v", err)
	}

	if manifest.Name != "math" {
		t.Errorf("Name = %q, want %q", manifest.Name, "math")
	}
	if manifest.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.1.0")
	}
	if manifest.ID != "220e47fe-0b34-4abe-991c-f8f121984346" {
		t.Errorf("ID = %q, want %q", manifest.ID, "220e47fe-0b34-4abe-991c-f8f121984346")
	}
	if manifest.DisplayName != "math" {
		t.Errorf("DisplayName = %q, want %q", manifest.DisplayName, "math")
	}
	if manifest.Summary != "A math toolbox" {
		t.Errorf("Summary = %q, want %q", manifest.Summary, "A math toolbox")
	}
	if manifest.Provider.Name != "Author" {
		t.Errorf("Provider.Name = %q, want %q", manifest.Provider.Name, "Author")
	}
	if manifest.Provider.Organization != "Org" {
		t.Errorf("Provider.Organization = %q, want %q", manifest.Provider.Organization, "Org")
	}
	if manifest.ReleaseCompatibility != ">=R2024b" {
		t.Errorf("ReleaseCompatibility = %q, want %q", manifest.ReleaseCompatibility, ">=R2024b")
	}
	if len(manifest.Archives) != 1 {
		t.Fatalf("Archives len = %d, want 1", len(manifest.Archives))
	}
	if manifest.Archives[0].Size != 7538 {
		t.Errorf("Archive.Size = %d, want %d", manifest.Archives[0].Size, 7538)
	}
	if manifest.Archives[0].Digests[0].Alg != "sha512" {
		t.Errorf("Archive.Digest.Alg = %q, want %q", manifest.Archives[0].Digests[0].Alg, "sha512")
	}
	if manifest.RequiredAdditionalSoftware == nil {
		t.Error("RequiredAdditionalSoftware should be non-nil empty slice")
	}
	if len(manifest.RequiredAdditionalSoftware) != 0 {
		t.Errorf("RequiredAdditionalSoftware len = %d, want 0", len(manifest.RequiredAdditionalSoftware))
	}
}

func TestBuildManifest_ValidationError(t *testing.T) {
	mp := &MPackage{
		Name:    "math",
		Version: "not-semver",
		ID:      "220e47fe-0b34-4abe-991c-f8f121984346",
	}
	archive := domain.Archive{
		Platforms: []string{"agnostic"},
		URL:       "https://example.com/file.mltbx",
		Size:      100,
		Digests:   []domain.Digest{{Alg: "sha512", Digest: "abc"}},
	}

	_, err := BuildManifest(mp, archive)
	if err == nil {
		t.Fatal("expected validation error for invalid semver")
	}
}
