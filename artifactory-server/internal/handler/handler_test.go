// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

func TestRouter_FullStack(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "math", Version: "1.0.0"},
	})
	var ready atomic.Bool
	ready.Store(true)

	router := NewRouter(idx, &ready, RouterConfig{RequiredHeaders: []string{"Authorization"}})
	srv := httptest.NewServer(router)
	defer srv.Close()

	tests := []struct {
		name   string
		path   string
		auth   string
		status int
	}{
		{"index with auth", "/v1/packages/index.json", "Bearer tok", http.StatusOK},
		{"index without auth", "/v1/packages/index.json", "", http.StatusUnauthorized},
		{"by-uuid", "/v1/packages/by-uuid/uuid-1.json", "Bearer tok", http.StatusOK},
		{"by-uuid not found", "/v1/packages/by-uuid/nope.json", "Bearer tok", http.StatusNotFound},
		{"by-name", "/v1/packages/by-name/math.json", "Bearer tok", http.StatusOK},
		{"health-check", "/health-check", "", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", srv.URL+tc.path, nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.status {
				t.Fatalf("expected %d, got %d", tc.status, resp.StatusCode)
			}
		})
	}
}

func TestRouter_PublishDisabled_Returns404(t *testing.T) {
	idx := index.New()
	idx.Build(nil)
	var ready atomic.Bool
	ready.Store(true)

	router := NewRouter(idx, &ready, RouterConfig{
		RequiredHeaders: []string{"Authorization"},
		EnablePublish:   false,
	})
	srv := httptest.NewServer(router)
	defer srv.Close()

	req, _ := http.NewRequest("POST", srv.URL+"/v1/packages/publish", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when publish disabled, got %d", resp.StatusCode)
	}
}

func TestRouter_IndexReturnsJSON(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "math", Version: "1.0.0"},
		{ID: "uuid-2", Name: "science", Version: "2.0.0"},
	})
	var ready atomic.Bool
	ready.Store(true)

	router := NewRouter(idx, &ready, RouterConfig{RequiredHeaders: []string{"Authorization"}})
	srv := httptest.NewServer(router)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/v1/packages/index.json", nil)
	req.Header.Set("Authorization", "Bearer tok")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var result struct {
		Packages     []domain.PackageManifest `json:"packages"`
		LastModified string                   `json:"lastModified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result.Packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(result.Packages))
	}
	if result.LastModified == "" {
		t.Fatal("expected lastModified to be set")
	}
}
