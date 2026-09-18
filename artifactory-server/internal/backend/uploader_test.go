// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/publish"
)

func testManifest() *domain.PackageManifest {
	return &domain.PackageManifest{
		Name:               "math",
		Version:            "1.1.0",
		ID:                 "220e47fe-0b34-4abe-991c-f8f121984346",
		SupportedPlatforms: json.RawMessage(`[]`),
		Archives: []domain.Archive{{
			Platforms: []string{"agnostic"},
			URL:       "placeholder",
			Size:      100,
			Digests:   []domain.Digest{{Alg: "sha512", Digest: "abc123"}},
		}},
		RequiredAdditionalSoftware: []string{},
	}
}

func TestUploader_Publish_Success(t *testing.T) {
	var receivedPath string
	var receivedAuth string
	var receivedExplode string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		receivedExplode = r.Header.Get("X-Explode-Archive-Atomic")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{
		BaseURL:       server.URL,
		RepoKey:       "mpm-packages",
		Token:         "service-token",
		AtomicPublish: true,
	})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("fake archive content"), 0644)

	m := testManifest()
	err := u.Publish(tmp, m, "user-token")
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	expectedPath := "/mpm-packages/math/220e47fe-0b34-4abe-991c-f8f121984346/1.1.0/math-1.1.0-bundle.zip"
	if receivedPath != expectedPath {
		t.Errorf("path = %q, want %q", receivedPath, expectedPath)
	}
	if receivedAuth != "Bearer user-token" {
		t.Errorf("auth = %q, want %q", receivedAuth, "Bearer user-token")
	}
	if receivedExplode != "true" {
		t.Errorf("X-Explode-Archive-Atomic = %q, want %q", receivedExplode, "true")
	}

	zr, err := zip.NewReader(bytes.NewReader(receivedBody), int64(len(receivedBody)))
	if err != nil {
		t.Fatalf("received body is not a valid ZIP: %v", err)
	}

	fileNames := make(map[string]bool)
	for _, f := range zr.File {
		fileNames[f.Name] = true
	}

	if !fileNames["math-1.1.0.mltbx"] {
		t.Error("bundle ZIP missing math-1.1.0.mltbx")
	}
	if !fileNames["math-1.1.0.manifest.json"] {
		t.Error("bundle ZIP missing math-1.1.0.manifest.json")
	}
	if len(zr.File) != 2 {
		t.Errorf("bundle ZIP has %d files, want 2", len(zr.File))
	}
}

func TestUploader_Publish_BundleContainsManifestJSON(t *testing.T) {
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{BaseURL: server.URL, RepoKey: "repo", Token: "t", AtomicPublish: true})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("archive"), 0644)

	m := testManifest()
	if err := u.Publish(tmp, m, "tok"); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(receivedBody), int64(len(receivedBody)))
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range zr.File {
		if f.Name == "math-1.1.0.manifest.json" {
			rc, _ := f.Open()
			data, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(data), `"name": "math"`) {
				t.Error("manifest JSON does not contain expected content")
			}
			return
		}
	}
	t.Error("math-1.1.0.manifest.json not found in bundle")
}

func TestUploader_Publish_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{BaseURL: server.URL, RepoKey: "repo", Token: "t", AtomicPublish: true})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("content"), 0644)

	err := u.Publish(tmp, testManifest(), "user-token")
	if err != publish.ErrConflict {
		t.Errorf("error = %v, want ErrConflict", err)
	}
}

func TestUploader_Publish_Failure_Returns_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{BaseURL: server.URL, RepoKey: "repo", Token: "t", AtomicPublish: true})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("content"), 0644)

	err := u.Publish(tmp, testManifest(), "user-token")
	if err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestUploader_Publish_UsesCallerToken(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{BaseURL: server.URL, RepoKey: "repo", Token: "service-token", AtomicPublish: true})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("content"), 0644)

	if err := u.Publish(tmp, testManifest(), "caller-token-123"); err != nil {
		t.Fatal(err)
	}

	if receivedAuth != "Bearer caller-token-123" {
		t.Errorf("auth = %q, want caller token", receivedAuth)
	}
	if receivedAuth == "Bearer service-token" {
		t.Error("used service token instead of caller token")
	}
}

type recordedPut struct {
	path        string
	auth        string
	explode     string
	contentType string
	body        []byte
}

func TestUploader_Publish_TwoStep_UploadsArchiveThenManifest(t *testing.T) {
	var puts []recordedPut
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		puts = append(puts, recordedPut{
			path:        r.URL.Path,
			auth:        r.Header.Get("Authorization"),
			explode:     r.Header.Get("X-Explode-Archive-Atomic"),
			contentType: r.Header.Get("Content-Type"),
			body:        body,
		})
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{
		BaseURL:       server.URL,
		RepoKey:       "mpm-packages",
		Token:         "service-token",
		AtomicPublish: false,
	})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("fake archive content"), 0644)

	if err := u.Publish(tmp, testManifest(), "user-token"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if len(puts) != 2 {
		t.Fatalf("got %d PUT requests, want 2", len(puts))
	}

	base := "/mpm-packages/math/220e47fe-0b34-4abe-991c-f8f121984346/1.1.0"
	archivePut, manifestPut := puts[0], puts[1]

	if archivePut.path != base+"/math-1.1.0.mltbx" {
		t.Errorf("archive path = %q, want %q", archivePut.path, base+"/math-1.1.0.mltbx")
	}
	if !bytes.Equal(archivePut.body, []byte("fake archive content")) {
		t.Errorf("archive body = %q, want raw archive content", archivePut.body)
	}
	if archivePut.explode != "" {
		t.Errorf("two-step archive PUT set X-Explode-Archive-Atomic = %q, want unset", archivePut.explode)
	}

	if manifestPut.path != base+"/math-1.1.0.manifest.json" {
		t.Errorf("manifest path = %q, want %q", manifestPut.path, base+"/math-1.1.0.manifest.json")
	}
	if !strings.Contains(string(manifestPut.body), `"name": "math"`) {
		t.Errorf("manifest body does not contain expected content: %s", manifestPut.body)
	}

	for _, p := range puts {
		if p.auth != "Bearer user-token" {
			t.Errorf("auth = %q, want caller token", p.auth)
		}
	}
}

func TestUploader_Publish_TwoStep_ManifestFailure_ReportsOrphanedArchive(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.WriteHeader(http.StatusCreated) // archive PUT succeeds
			return
		}
		w.WriteHeader(http.StatusInternalServerError) // manifest PUT fails
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{BaseURL: server.URL, RepoKey: "repo", Token: "t", AtomicPublish: false})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("content"), 0644)

	err := u.Publish(tmp, testManifest(), "user-token")
	if err == nil {
		t.Fatal("expected error when manifest upload fails")
	}
	if !strings.Contains(err.Error(), "math-1.1.0.mltbx") {
		t.Errorf("error should name the orphaned archive, got: %v", err)
	}
	if !strings.Contains(err.Error(), "manually") {
		t.Errorf("error should indicate manual cleanup, got: %v", err)
	}
}

func TestUploader_Publish_TwoStep_ArchiveFailure(t *testing.T) {
	var count int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.WriteHeader(http.StatusInternalServerError) // archive PUT fails
	}))
	defer server.Close()

	u := NewUploader(UploaderConfig{BaseURL: server.URL, RepoKey: "repo", Token: "t", AtomicPublish: false})

	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	os.WriteFile(tmp, []byte("content"), 0644)

	if err := u.Publish(tmp, testManifest(), "user-token"); err == nil {
		t.Fatal("expected error when archive upload fails")
	}
	if count != 1 {
		t.Errorf("made %d PUT requests, want 1 (manifest should not upload after archive fails)", count)
	}
}
