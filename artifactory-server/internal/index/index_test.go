// Copyright 2026 The MathWorks, Inc.

package index

import (
	"testing"
	"time"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
)

func sampleManifests() []domain.PackageManifest {
	return []domain.PackageManifest{
		{ID: "uuid-1", Name: "math", Version: "1.0.0"},
		{ID: "uuid-1", Name: "math", Version: "2.0.0"},
		{ID: "uuid-2", Name: "science", Version: "1.0.0"},
	}
}

func TestIndex_Build_QueryAll(t *testing.T) {
	idx := New()
	idx.Build(sampleManifests())

	all := idx.QueryAll()
	if len(all) != 3 {
		t.Fatalf("expected 3 manifests, got %d", len(all))
	}
}

func TestIndex_QueryByUUID(t *testing.T) {
	idx := New()
	idx.Build(sampleManifests())

	results, err := idx.QueryByUUID("uuid-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 manifests for uuid-1, got %d", len(results))
	}
}

func TestIndex_QueryByUUID_NotFound(t *testing.T) {
	idx := New()
	idx.Build(sampleManifests())

	_, err := idx.QueryByUUID("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent UUID")
	}
}

func TestIndex_QueryByName(t *testing.T) {
	idx := New()
	idx.Build(sampleManifests())

	results, err := idx.QueryByName("math")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 manifests for math, got %d", len(results))
	}
}

func TestIndex_QueryByName_NotFound(t *testing.T) {
	idx := New()
	idx.Build(sampleManifests())

	_, err := idx.QueryByName("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent name")
	}
}

func TestIndex_Build_SetsBuiltAt(t *testing.T) {
	idx := New()
	before := time.Now()
	idx.Build(sampleManifests())
	after := time.Now()

	builtAt := idx.BuiltAt()
	if builtAt.Before(before) || builtAt.After(after) {
		t.Fatalf("expected builtAt between %v and %v, got %v", before, after, builtAt)
	}
}

func TestIndex_Build_ReturnsDefensiveCopies(t *testing.T) {
	idx := New()
	idx.Build(sampleManifests())

	all1 := idx.QueryAll()
	all2 := idx.QueryAll()
	all1[0].Name = "mutated"
	if all2[0].Name == "mutated" {
		t.Fatal("QueryAll should return a copy, not a reference to internal state")
	}
}
