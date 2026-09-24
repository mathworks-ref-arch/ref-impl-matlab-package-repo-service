// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/index"
)

// Query by-name is case-insensitive: different casings return the same results.
func TestQueryByName_CaseInsensitive_HTTP(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "MyToolbox", Version: "1.0.0"},
		{ID: "uuid-1", Name: "MyToolbox", Version: "2.0.0"},
	})
	h := NewQueryHandler(idx)

	queries := []string{"MyToolbox.json", "mytoolbox.json", "MYTOOLBOX.json", "myToolbox.json"}
	for _, q := range queries {
		req := httptest.NewRequest("GET", "/v1/packages/by-name/"+q, nil)
		req.SetPathValue("name", q)
		rr := httptest.NewRecorder()

		h.HandleByName().ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("QueryByName(%q) status = %d, want 200", q, rr.Code)
		}

		var manifests []domain.PackageManifest
		json.Unmarshal(rr.Body.Bytes(), &manifests)
		if len(manifests) != 2 {
			t.Errorf("QueryByName(%q) returned %d results, want 2", q, len(manifests))
		}
	}
}

// Response preserves original casing from the index through the HTTP layer.
func TestQueryByName_PreservesOriginalCasing_HTTP(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "FSDAToolbox", Version: "1.0.0"},
	})
	h := NewQueryHandler(idx)

	req := httptest.NewRequest("GET", "/v1/packages/by-name/fsdatoolbox.json", nil)
	req.SetPathValue("name", "fsdatoolbox.json")
	rr := httptest.NewRecorder()

	h.HandleByName().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var manifests []domain.PackageManifest
	json.Unmarshal(rr.Body.Bytes(), &manifests)
	if len(manifests) != 1 {
		t.Fatalf("expected 1 result, got %d", len(manifests))
	}
	if manifests[0].Name != "FSDAToolbox" {
		t.Errorf("Name = %q, want %q (original casing preserved)", manifests[0].Name, "FSDAToolbox")
	}
}
