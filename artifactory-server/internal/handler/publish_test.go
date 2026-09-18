// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	storagebackend "github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/backend"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

func validMPackageJSON() string {
	return `{
		"name": "math",
		"version": "1.1.0",
		"id": "220e47fe-0b34-4abe-991c-f8f121984346",
		"formerNames": [],
		"displayName": "math",
		"summary": "",
		"tags": [],
		"readme": "",
		"provider": {"name": "", "organization": "", "email": "", "url": ""},
		"folders": [{"path": "src", "languages": ["matlab"]}],
		"dependencies": [],
		"releaseCompatibility": ">=R2024b",
		"supportedPlatforms": [{"platform": "any", "architectures": ["any"]}],
		"schemaVersion": "1.2.0"
	}`
}

func createTestMLTBXBytes(t *testing.T, mpackageJSON string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("fsroot/resources/mpackage.json")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte(mpackageJSON))
	zw.Close()
	return buf.Bytes()
}

func createPublishRequest(t *testing.T, mltbxContent []byte, filename string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(mltbxContent)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/packages/publish", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-token")
	return req
}

func setupPublishTest(t *testing.T, backendHandler http.HandlerFunc) (*httptest.Server, http.Handler) {
	t.Helper()
	backend := httptest.NewServer(backendHandler)
	t.Cleanup(backend.Close)

	idx := index.New()
	idx.Build(nil)

	uploader := storagebackend.NewUploader(storagebackend.UploaderConfig{
		BaseURL:       backend.URL,
		RepoKey:       "mpm-packages",
		Token:         "service-token",
		AtomicPublish: true,
	})

	ph := NewPublishHandler(idx, uploader, 500<<20)

	ready := &atomic.Bool{}
	ready.Store(true)
	router := NewRouter(idx, ready, RouterConfig{
		RequiredHeaders: []string{"Authorization"},
		EnablePublish:   true,
		PublishHandler:  ph,
	})

	return backend, router
}

func TestPublishHandler_Success(t *testing.T) {
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())
	req := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var manifest domain.PackageManifest
	if err := json.NewDecoder(rr.Body).Decode(&manifest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
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
	if len(manifest.Archives) != 1 {
		t.Fatalf("Archives len = %d, want 1", len(manifest.Archives))
	}
	if manifest.Archives[0].Digests[0].Alg != "sha512" {
		t.Errorf("Digest alg = %q, want sha512", manifest.Archives[0].Digests[0].Alg)
	}

	location := rr.Header().Get("Location")
	if location != "/v1/packages/by-uuid/220e47fe-0b34-4abe-991c-f8f121984346.json" {
		t.Errorf("Location = %q, want /v1/packages/by-uuid/220e47fe-0b34-4abe-991c-f8f121984346.json", location)
	}
}

func TestPublishHandler_MissingFile(t *testing.T) {
	_, router := setupPublishTest(t, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/packages/publish", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-token")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestPublishHandler_WrongExtension(t *testing.T) {
	_, router := setupPublishTest(t, nil)

	req := createPublishRequest(t, []byte("content"), "file.zip")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
}

func TestPublishHandler_CorruptZip(t *testing.T) {
	_, router := setupPublishTest(t, nil)

	req := createPublishRequest(t, []byte("not a zip"), "test.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestPublishHandler_DuplicateVersion(t *testing.T) {
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())

	// First publish
	req1 := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")
	rr1 := httptest.NewRecorder()
	router.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusCreated {
		t.Fatalf("first publish status = %d, want 201; body = %s", rr1.Code, rr1.Body.String())
	}

	// Second publish — should conflict
	req2 := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusConflict {
		t.Errorf("second publish status = %d, want %d; body = %s", rr2.Code, http.StatusConflict, rr2.Body.String())
	}
}

func TestPublishHandler_BackendFailure_Returns502(t *testing.T) {
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())
	req := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want %d; body = %s", rr.Code, http.StatusBadGateway, rr.Body.String())
	}
}

func TestPublishHandler_SinglePUT_WithExplodeHeader(t *testing.T) {
	var putCount int
	var receivedExplode string
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			putCount++
			receivedExplode = r.Header.Get("X-Explode-Archive-Atomic")
		}
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())
	req := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}
	if putCount != 1 {
		t.Errorf("PUT count = %d, want 1 (single atomic upload)", putCount)
	}
	if receivedExplode != "true" {
		t.Errorf("X-Explode-Archive-Atomic = %q, want %q", receivedExplode, "true")
	}
}

func TestPublishHandler_UploadsUseCallerToken(t *testing.T) {
	var uploadAuths []string
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			uploadAuths = append(uploadAuths, r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())
	req := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	for i, auth := range uploadAuths {
		if auth != "Bearer test-token" {
			t.Errorf("upload[%d] auth = %q, want %q (caller token)", i, auth, "Bearer test-token")
		}
		if auth == "Bearer service-token" {
			t.Errorf("upload[%d] used service token instead of caller token", i)
		}
	}
}

func TestPublishHandler_NoAuth_Returns401(t *testing.T) {
	_, router := setupPublishTest(t, nil)

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", "math-1.1.0.mltbx")
	part.Write(mltbx)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/packages/publish", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	// No Authorization header

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestPublishHandler_RealMLTBX(t *testing.T) {
	path := "/mathworks/devel/sandbox/tsapre/metadata_test_server/v1/artifacts/math/220e47fe-0b34-4abe-991c-f8f121984346/1.1.0/math-1.1.0.mltbx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("real MLTBX not available")
	}

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	idx := index.New()
	idx.Build(nil)

	uploader := storagebackend.NewUploader(storagebackend.UploaderConfig{
		BaseURL:       backend.URL,
		RepoKey:       "mpm-packages",
		Token:         "service-token",
		AtomicPublish: true,
	})

	ph := NewPublishHandler(idx, uploader, 500<<20)

	ready := &atomic.Bool{}
	ready.Store(true)
	router := NewRouter(idx, ready, RouterConfig{
		RequiredHeaders: []string{"Authorization"},
		EnablePublish:   true,
		PublishHandler:  ph,
	})

	mltbxData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, _ := writer.CreateFormFile("file", filepath.Base(path))
	part.Write(mltbxData)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/packages/publish", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer test-token")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var manifest domain.PackageManifest
	json.NewDecoder(rr.Body).Decode(&manifest)
	if manifest.Name != "math" {
		t.Errorf("Name = %q, want %q", manifest.Name, "math")
	}
	if manifest.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "1.1.0")
	}

	// Verify it appears in index
	req2 := httptest.NewRequest(http.MethodGet, "/v1/packages/by-name/math.json", nil)
	req2.Header.Set("Authorization", "Bearer test-token")
	rr2 := httptest.NewRecorder()
	router.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("query after publish status = %d", rr2.Code)
	}

	var results []domain.PackageManifest
	json.NewDecoder(rr2.Body).Decode(&results)
	if len(results) != 1 {
		t.Errorf("expected 1 result after publish, got %d", len(results))
	}
}

// Suppress unused import warning
var _ = io.Discard
