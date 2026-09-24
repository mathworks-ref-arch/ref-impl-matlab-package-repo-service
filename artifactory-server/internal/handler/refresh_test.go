// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/index"
)

type fakeSource struct {
	manifests []domain.PackageManifest
	err       error
}

func (f *fakeSource) ListManifests() ([]domain.PackageManifest, error) {
	return f.manifests, f.err
}

func TestRefreshHandler_RebuildsIndex(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "aaa", Name: "old-pkg", Version: "1.0.0"},
	})

	newManifests := []domain.PackageManifest{
		{ID: "bbb", Name: "new-pkg", Version: "2.0.0"},
		{ID: "ccc", Name: "another-pkg", Version: "1.0.0"},
	}
	factory := func(token string) index.ManifestSource {
		return &fakeSource{manifests: newManifests}
	}

	rh := NewRefreshHandler(idx, factory)
	req := httptest.NewRequest("POST", "/v1/admin/refresh", nil)
	req.Header.Set("Authorization", "Bearer caller-token")
	rr := httptest.NewRecorder()

	rh.HandleRefresh().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}
	if int(resp["packages"].(float64)) != 2 {
		t.Fatalf("expected 2 packages, got %v", resp["packages"])
	}
	if resp["lastModified"] == nil || resp["lastModified"] == "" {
		t.Fatal("expected lastModified in response")
	}

	// Verify index was actually rebuilt — old package gone, new ones present
	all := idx.QueryAll()
	if len(all) != 2 {
		t.Fatalf("expected index to have 2 packages after refresh, got %d", len(all))
	}
	if all[0].Name != "new-pkg" && all[1].Name != "new-pkg" {
		t.Fatal("expected new-pkg in refreshed index")
	}
}

func TestRefreshHandler_UsesCallerToken(t *testing.T) {
	idx := index.New()
	idx.Build(nil)

	var receivedToken string
	factory := func(token string) index.ManifestSource {
		receivedToken = token
		return &fakeSource{manifests: nil}
	}

	rh := NewRefreshHandler(idx, factory)
	req := httptest.NewRequest("POST", "/v1/admin/refresh", nil)
	req.Header.Set("Authorization", "Bearer my-personal-token")
	rr := httptest.NewRecorder()

	rh.HandleRefresh().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if receivedToken != "my-personal-token" {
		t.Fatalf("expected factory to receive caller's token, got %q", receivedToken)
	}
}

func TestRefreshHandler_MissingToken(t *testing.T) {
	idx := index.New()
	idx.Build(nil)

	factory := func(token string) index.ManifestSource {
		t.Fatal("factory should not be called without a token")
		return nil
	}

	rh := NewRefreshHandler(idx, factory)
	req := httptest.NewRequest("POST", "/v1/admin/refresh", nil)
	rr := httptest.NewRecorder()

	rh.HandleRefresh().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestRefreshHandler_SourceError(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "aaa", Name: "old-pkg", Version: "1.0.0"},
	})

	factory := func(token string) index.ManifestSource {
		return &fakeSource{err: errors.New("artifactory unreachable")}
	}

	rh := NewRefreshHandler(idx, factory)
	req := httptest.NewRequest("POST", "/v1/admin/refresh", nil)
	req.Header.Set("Authorization", "Bearer some-token")
	rr := httptest.NewRecorder()

	rh.HandleRefresh().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify index was NOT modified on failure
	all := idx.QueryAll()
	if len(all) != 1 {
		t.Fatalf("expected index to remain unchanged with 1 package, got %d", len(all))
	}
}
