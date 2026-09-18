// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

func setupIndex() *index.Index {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "220e47fe-0b34-4abe-991c-f8f121984346", Name: "math", Version: "1.0.0"},
		{ID: "220e47fe-0b34-4abe-991c-f8f121984346", Name: "math", Version: "2.0.0"},
		{ID: "118409b9-6a71-4754-ab24-31a4f3871e15", Name: "science", Version: "1.0.0"},
	})
	return idx
}

func TestQueryIndex(t *testing.T) {
	h := NewQueryHandler(setupIndex())

	req := httptest.NewRequest("GET", "/v1/packages/index.json", nil)
	rr := httptest.NewRecorder()

	h.HandleIndex().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp indexResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(resp.Packages) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(resp.Packages))
	}
	if resp.LastModified == "" {
		t.Fatal("expected lastModified to be set")
	}
	if _, err := time.Parse(time.RFC3339, resp.LastModified); err != nil {
		t.Fatalf("expected RFC3339 lastModified, got %q: %v", resp.LastModified, err)
	}
}

func TestQueryIndex_Empty(t *testing.T) {
	idx := index.New()
	idx.Build(nil)
	h := NewQueryHandler(idx)

	req := httptest.NewRequest("GET", "/v1/packages/index.json", nil)
	rr := httptest.NewRecorder()

	h.HandleIndex().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp indexResponse
	json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Packages) != 0 {
		t.Fatalf("expected 0 packages, got %d", len(resp.Packages))
	}
}

func TestQueryByUUID(t *testing.T) {
	h := NewQueryHandler(setupIndex())

	req := httptest.NewRequest("GET", "/v1/packages/by-uuid/220e47fe-0b34-4abe-991c-f8f121984346.json", nil)
	req.SetPathValue("uuid", "220e47fe-0b34-4abe-991c-f8f121984346.json")
	rr := httptest.NewRecorder()

	h.HandleByUUID().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var manifests []domain.PackageManifest
	json.Unmarshal(rr.Body.Bytes(), &manifests)
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}
}

func TestQueryByUUID_NotFound(t *testing.T) {
	h := NewQueryHandler(setupIndex())

	req := httptest.NewRequest("GET", "/v1/packages/by-uuid/nonexistent.json", nil)
	req.SetPathValue("uuid", "nonexistent.json")
	rr := httptest.NewRecorder()

	h.HandleByUUID().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestQueryByName(t *testing.T) {
	h := NewQueryHandler(setupIndex())

	req := httptest.NewRequest("GET", "/v1/packages/by-name/math.json", nil)
	req.SetPathValue("name", "math.json")
	rr := httptest.NewRecorder()

	h.HandleByName().ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var manifests []domain.PackageManifest
	json.Unmarshal(rr.Body.Bytes(), &manifests)
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}
}

func TestQueryByName_NotFound(t *testing.T) {
	h := NewQueryHandler(setupIndex())

	req := httptest.NewRequest("GET", "/v1/packages/by-name/nonexistent.json", nil)
	req.SetPathValue("name", "nonexistent.json")
	rr := httptest.NewRecorder()

	h.HandleByName().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}
