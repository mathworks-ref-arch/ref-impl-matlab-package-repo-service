// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
)

var defaultTestLimits = SourceLimits{
	Timeout:             30 * time.Second,
	MaxAQLResponseBytes: 50 << 20,
	MaxManifestBytes:    1 << 20,
}

func TestSource_ListManifests(t *testing.T) {
	manifest1 := domain.PackageManifest{ID: "uuid-1", Name: "math", Version: "1.0.0"}
	manifest2 := domain.PackageManifest{ID: "uuid-2", Name: "science", Version: "2.0.0"}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/search/aql", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		resp := map[string]any{
			"results": []map[string]any{
				{"repo": "mpm-packages", "path": "math/uuid-1/1.0.0", "name": "math-1.0.0-manifest.json"},
				{"repo": "mpm-packages", "path": "science/uuid-2/2.0.0", "name": "science-2.0.0-manifest.json"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("GET /mpm-packages/math/uuid-1/1.0.0/math-1.0.0-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(manifest1)
	})
	mux.HandleFunc("GET /mpm-packages/science/uuid-2/2.0.0/science-2.0.0-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(manifest2)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	actions := ActionsConfig{
		BaseURL: srv.URL,
		RepoKey: "mpm-packages",
		Auth: AuthConfig{
			Strategy: "bearer",
			TokenEnv: "TEST_TOKEN",
		},
		Actions: map[string]actionDef{
			"list_manifests": {
				Method:           "POST",
				URL:              "{baseURL}/api/search/aql",
				ContentType:      "text/plain",
				Body:             `items.find({"repo":"{repoKey}","name":{"$match":"*.manifest.json"}})`,
				ResponseStrategy: "artifactory-aql",
				ResponseMapping: responseMapping{
					Items:   "results",
					ItemURL: "{baseURL}/{repoKey}/{item.path}/{item.name}",
				},
			},
		},
	}

	t.Setenv("TEST_TOKEN", "test-token")

	src := NewSource(actions, defaultTestLimits)
	manifests, err := src.ListManifests()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}
	if manifests[0].Name != "math" {
		t.Fatalf("expected math, got %s", manifests[0].Name)
	}
	if manifests[1].Name != "science" {
		t.Fatalf("expected science, got %s", manifests[1].Name)
	}
}

func TestSource_ListManifests_BadToken(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search/aql", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	actions := ActionsConfig{
		BaseURL: srv.URL,
		RepoKey: "mpm-packages",
		Auth: AuthConfig{
			Strategy: "bearer",
			TokenEnv: "TEST_TOKEN_BAD",
		},
		Actions: map[string]actionDef{
			"list_manifests": {
				Method:           "POST",
				URL:              "{baseURL}/api/search/aql",
				ResponseStrategy: "artifactory-aql",
				ResponseMapping:  responseMapping{Items: "results", ItemURL: "{baseURL}/{item.path}"},
			},
		},
	}

	t.Setenv("TEST_TOKEN_BAD", "wrong-token")

	src := NewSource(actions, defaultTestLimits)
	_, err := src.ListManifests()
	if err == nil {
		t.Fatal("expected error for unauthorized response")
	}
}

func TestLoadActions(t *testing.T) {
	data := `{
		"backend": "artifactory-generic",
		"baseURL": "https://art.example.com",
		"repoKey": "mpm-packages",
		"auth": {"strategy": "bearer", "tokenEnv": "MY_TOKEN"},
		"actions": {
			"list_manifests": {
				"method": "POST",
				"url": "{baseURL}/api/search/aql",
				"responseStrategy": "artifactory-aql",
				"responseMapping": {"items": "results", "itemURL": "{baseURL}/{item.path}"}
			}
		}
	}`
	dir := t.TempDir()
	path := dir + "/actions.json"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadActions(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://art.example.com" {
		t.Fatalf("expected https://art.example.com, got %s", cfg.BaseURL)
	}
	if cfg.Auth.Strategy != "bearer" {
		t.Fatalf("expected bearer, got %s", cfg.Auth.Strategy)
	}
	if _, ok := cfg.Actions["list_manifests"]; !ok {
		t.Fatal("expected list_manifests action")
	}
}

func TestLoadActions_ResolvesEnvVars(t *testing.T) {
	data := `{
		"backend": "artifactory-generic",
		"baseURL": "{ARTIFACTORY_URL}",
		"repoKey": "{ARTIFACTORY_REPO_KEY}",
		"auth": {"strategy": "bearer", "tokenEnv": "MY_TOKEN"},
		"actions": {}
	}`
	dir := t.TempDir()
	path := dir + "/actions.json"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ARTIFACTORY_URL", "https://art.resolved.com/artifactory")
	t.Setenv("ARTIFACTORY_REPO_KEY", "my-repo")

	cfg, err := LoadActions(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://art.resolved.com/artifactory" {
		t.Fatalf("expected resolved URL, got %s", cfg.BaseURL)
	}
	if cfg.RepoKey != "my-repo" {
		t.Fatalf("expected my-repo, got %s", cfg.RepoKey)
	}
}

func TestLoadActions_UnsetEnvVar(t *testing.T) {
	data := `{
		"backend": "artifactory-generic",
		"baseURL": "{UNSET_VAR}",
		"repoKey": "mpm-packages",
		"auth": {"strategy": "bearer", "tokenEnv": "MY_TOKEN"},
		"actions": {}
	}`
	dir := t.TempDir()
	path := dir + "/actions.json"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadActions(path)
	if err == nil {
		t.Fatal("expected error for unresolved env var placeholder")
	}
}

func TestParseAQLResponse(t *testing.T) {
	body := []byte(`{
		"results": [
			{"repo": "mpm-packages", "path": "math/uuid-1/1.0.0", "name": "math-1.0.0-manifest.json"},
			{"repo": "mpm-packages", "path": "science/uuid-2/1.0.0", "name": "science-1.0.0-manifest.json"}
		],
		"range": {"start_pos": 0, "end_pos": 2, "total": 2}
	}`)
	mapping := responseMapping{
		Items:   "results",
		ItemURL: "{baseURL}/{repoKey}/{item.path}/{item.name}",
	}
	vars := map[string]string{
		"baseURL": "https://art.example.com",
		"repoKey": "mpm-packages",
	}

	urls, err := parseAQLResponse(body, mapping, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d", len(urls))
	}
	want0 := "https://art.example.com/mpm-packages/math/uuid-1/1.0.0/math-1.0.0-manifest.json"
	if urls[0] != want0 {
		t.Fatalf("urls[0] = %q, want %q", urls[0], want0)
	}
}

func TestParseAQLResponse_EmptyResults(t *testing.T) {
	body := []byte(`{"results": [], "range": {"start_pos": 0, "end_pos": 0, "total": 0}}`)
	mapping := responseMapping{Items: "results", ItemURL: "{baseURL}/{item.path}/{item.name}"}

	urls, err := parseAQLResponse(body, mapping, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 0 {
		t.Fatalf("expected 0 URLs, got %d", len(urls))
	}
}

func TestParseAQLResponse_InvalidJSON(t *testing.T) {
	_, err := parseAQLResponse([]byte(`{invalid`), responseMapping{Items: "results"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSource_ListManifests_UnsupportedResponseStrategy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"results": []}`))
	}))
	defer srv.Close()

	actions := ActionsConfig{
		BaseURL: srv.URL,
		RepoKey: "mpm-packages",
		Auth:    AuthConfig{Strategy: "none"},
		Actions: map[string]actionDef{
			"list_manifests": {
				Method:           "POST",
				URL:              "{baseURL}/api/search/aql",
				ResponseStrategy: "unknown-strategy",
				ResponseMapping:  responseMapping{Items: "results", ItemURL: "{baseURL}/{item.path}"},
			},
		},
	}

	src := NewSource(actions, defaultTestLimits)
	_, err := src.ListManifests()
	if err == nil {
		t.Fatal("expected error for unsupported response strategy")
	}
}

func TestSource_ListManifests_OversizedAQLResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search/aql", func(w http.ResponseWriter, r *http.Request) {
		oversized := strings.Repeat("x", 1025)
		fmt.Fprint(w, oversized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	actions := ActionsConfig{
		BaseURL: srv.URL,
		RepoKey: "mpm-packages",
		Auth:    AuthConfig{Strategy: "none"},
		Actions: map[string]actionDef{
			"list_manifests": {
				Method:           "POST",
				URL:              "{baseURL}/api/search/aql",
				ResponseStrategy: "artifactory-aql",
				ResponseMapping:  responseMapping{Items: "results", ItemURL: "{baseURL}/{item.path}"},
			},
		},
	}

	limits := SourceLimits{
		Timeout:             30 * time.Second,
		MaxAQLResponseBytes: 1024,
		MaxManifestBytes:    512,
	}

	src := NewSource(actions, limits)
	_, err := src.ListManifests()
	if err == nil {
		t.Fatal("expected error for oversized AQL response")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected limit error, got: %v", err)
	}
}

func TestSource_ListManifests_OversizedManifestResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search/aql", func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"results": []map[string]any{
				{"repo": "mpm-packages", "path": "math/uuid-1/1.0.0", "name": "math-1.0.0-manifest.json"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("GET /mpm-packages/math/uuid-1/1.0.0/math-1.0.0-manifest.json", func(w http.ResponseWriter, r *http.Request) {
		oversized := `{"name":"` + strings.Repeat("x", 1025) + `"}`
		fmt.Fprint(w, oversized)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	actions := ActionsConfig{
		BaseURL: srv.URL,
		RepoKey: "mpm-packages",
		Auth:    AuthConfig{Strategy: "none"},
		Actions: map[string]actionDef{
			"list_manifests": {
				Method:           "POST",
				URL:              "{baseURL}/api/search/aql",
				ResponseStrategy: "artifactory-aql",
				ResponseMapping: responseMapping{
					Items:   "results",
					ItemURL: "{baseURL}/{repoKey}/{item.path}/{item.name}",
				},
			},
		},
	}

	limits := SourceLimits{
		Timeout:             30 * time.Second,
		MaxAQLResponseBytes: 50 << 20,
		MaxManifestBytes:    1024,
	}

	src := NewSource(actions, limits)
	_, err := src.ListManifests()
	if err == nil {
		t.Fatal("expected error for oversized manifest response")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("expected limit error, got: %v", err)
	}
}

func TestExpandTemplate_Simple(t *testing.T) {
	vars := map[string]string{
		"baseURL": "https://art.example.com",
		"repoKey": "mpm-packages",
	}
	got := expandTemplate("{baseURL}/api/search/{repoKey}", vars)
	want := "https://art.example.com/api/search/mpm-packages"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandTemplate_NoPlaceholders(t *testing.T) {
	got := expandTemplate("plain string", nil)
	if got != "plain string" {
		t.Fatalf("got %q, want %q", got, "plain string")
	}
}

func TestExpandTemplate_UnknownPlaceholder(t *testing.T) {
	vars := map[string]string{"a": "1"}
	got := expandTemplate("{a}/{b}", vars)
	if got != "1/{b}" {
		t.Fatalf("got %q, want %q", got, "1/{b}")
	}
}
