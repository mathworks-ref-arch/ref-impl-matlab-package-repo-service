// Copyright 2026 The MathWorks, Inc.

package index

import (
	"encoding/json"
	"testing"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
)

func TestIndex_Add(t *testing.T) {
	idx := New()
	idx.Build(nil)

	m := domain.PackageManifest{
		Name:               "math",
		Version:            "1.0.0",
		ID:                 "220e47fe-0b34-4abe-991c-f8f121984346",
		SupportedPlatforms: json.RawMessage(`[]`),
	}

	idx.Add(m)

	all := idx.QueryAll()
	if len(all) != 1 {
		t.Fatalf("QueryAll() len = %d, want 1", len(all))
	}
	if all[0].Name != "math" {
		t.Errorf("Name = %q, want %q", all[0].Name, "math")
	}

	byUUID, err := idx.QueryByUUID("220e47fe-0b34-4abe-991c-f8f121984346")
	if err != nil {
		t.Fatalf("QueryByUUID() error = %v", err)
	}
	if len(byUUID) != 1 {
		t.Errorf("QueryByUUID() len = %d, want 1", len(byUUID))
	}

	byName, err := idx.QueryByName("math")
	if err != nil {
		t.Fatalf("QueryByName() error = %v", err)
	}
	if len(byName) != 1 {
		t.Errorf("QueryByName() len = %d, want 1", len(byName))
	}
}

func TestIndex_Add_MultipleVersions(t *testing.T) {
	idx := New()
	idx.Build(nil)

	m1 := domain.PackageManifest{
		Name:               "math",
		Version:            "1.0.0",
		ID:                 "220e47fe-0b34-4abe-991c-f8f121984346",
		SupportedPlatforms: json.RawMessage(`[]`),
	}
	m2 := domain.PackageManifest{
		Name:               "math",
		Version:            "2.0.0",
		ID:                 "220e47fe-0b34-4abe-991c-f8f121984346",
		SupportedPlatforms: json.RawMessage(`[]`),
	}

	idx.Add(m1)
	idx.Add(m2)

	all := idx.QueryAll()
	if len(all) != 2 {
		t.Fatalf("QueryAll() len = %d, want 2", len(all))
	}

	byUUID, _ := idx.QueryByUUID("220e47fe-0b34-4abe-991c-f8f121984346")
	if len(byUUID) != 2 {
		t.Errorf("QueryByUUID() len = %d, want 2", len(byUUID))
	}
}

func TestIndex_Exists(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{
			Name:               "math",
			Version:            "1.0.0",
			ID:                 "220e47fe-0b34-4abe-991c-f8f121984346",
			SupportedPlatforms: json.RawMessage(`[]`),
		},
	})

	if !idx.Exists("220e47fe-0b34-4abe-991c-f8f121984346", "1.0.0") {
		t.Error("Exists() = false, want true for existing package")
	}
	if idx.Exists("220e47fe-0b34-4abe-991c-f8f121984346", "2.0.0") {
		t.Error("Exists() = true, want false for non-existing version")
	}
	if idx.Exists("00000000-0000-0000-0000-000000000000", "1.0.0") {
		t.Error("Exists() = true, want false for non-existing UUID")
	}
}
