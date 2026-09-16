// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	storagebackend "github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/backend"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

func mpackageJSONWithNameAndUUID(name, uuid, version string) string {
	return fmt.Sprintf(`{
		"name": %q,
		"version": %q,
		"id": %q,
		"formerNames": [],
		"displayName": %q,
		"summary": "",
		"tags": [],
		"readme": "",
		"provider": {"name": "", "organization": "", "email": "", "url": ""},
		"folders": [{"path": "src", "languages": ["matlab"]}],
		"dependencies": [],
		"releaseCompatibility": ">=R2024b",
		"supportedPlatforms": [{"platform": "any", "architectures": ["any"]}],
		"schemaVersion": "1.2.0"
	}`, name, version, uuid, name)
}

func setupPublishTestWithIndex(t *testing.T, idx *index.Index, backendHandler http.HandlerFunc) (*httptest.Server, http.Handler) {
	t.Helper()
	backend := httptest.NewServer(backendHandler)
	t.Cleanup(backend.Close)

	uploader := storagebackend.NewUploader(storagebackend.UploaderConfig{
		BaseURL: backend.URL,
		RepoKey: "mpm-packages",
		Token:   "service-token",
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

const (
	existingUUID = "c1438758-4eaf-438f-b04f-6a7279735de5"
	diffUUID     = "99999999-9999-9999-9999-999999999999"
)

// Starting state: "Battery" / existingUUID / v1.1.0 in the index.
func TestPublishHandler_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name         string
		inputName    string
		inputUUID    string
		inputVersion string
		wantStatus   int
		wantErrMsg   string
	}{
		{
			name:         "same UUID, same version, different casing returns 409 version exists",
			inputName:    "battery",
			inputUUID:    existingUUID,
			inputVersion: "1.1.0",
			wantStatus:   http.StatusConflict,
			wantErrMsg:   "already exists",
		},
		{
			name:         "different UUID, different version, different casing returns 409 name conflict",
			inputName:    "battery",
			inputUUID:    diffUUID,
			inputVersion: "2.0.0",
			wantStatus:   http.StatusConflict,
			wantErrMsg:   "conflicts with existing package",
		},
		{
			name:         "different UUID, same version, different casing returns 409 name conflict",
			inputName:    "battery",
			inputUUID:    diffUUID,
			inputVersion: "1.1.0",
			wantStatus:   http.StatusConflict,
			wantErrMsg:   "conflicts with existing package",
		},
		{
			name:         "same UUID, new version, original casing returns 201 success",
			inputName:    "Battery",
			inputUUID:    existingUUID,
			inputVersion: "2.0.0",
			wantStatus:   http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := index.New()
			idx.Build([]domain.PackageManifest{
				{ID: existingUUID, Name: "Battery", Version: "1.1.0"},
			})

			_, router := setupPublishTestWithIndex(t, idx, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			})

			mltbx := createTestMLTBXBytes(t, mpackageJSONWithNameAndUUID(
				tt.inputName, tt.inputUUID, tt.inputVersion))
			req := createPublishRequest(t, mltbx, tt.inputName+"-"+tt.inputVersion+".mltbx")

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if tt.wantErrMsg != "" {
				var resp map[string]map[string]any
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode error response: %v", err)
				}
				msg, _ := resp["error"]["message"].(string)
				if !strings.Contains(msg, tt.wantErrMsg) {
					t.Errorf("error message = %q, want it to contain %q", msg, tt.wantErrMsg)
				}
			}
		})
	}
}

func TestPublishHandler_CaseInsensitive_ErrorUsesUserTypedName(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: existingUUID, Name: "Battery", Version: "1.1.0"},
	})

	_, router := setupPublishTestWithIndex(t, idx, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, mpackageJSONWithNameAndUUID(
		"battery", diffUUID, "2.0.0"))
	req := createPublishRequest(t, mltbx, "battery-2.0.0.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusConflict, rr.Body.String())
	}

	var resp map[string]map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	msg, _ := resp["error"]["message"].(string)

	if !strings.Contains(msg, `"battery"`) {
		t.Errorf("error message should contain user-typed name %q, got: %s", "battery", msg)
	}
	if !strings.Contains(msg, `"Battery"`) {
		t.Errorf("error message should contain existing name %q, got: %s", "Battery", msg)
	}
}

func TestPublishHandler_CaseInsensitive_201PreservesOriginalCasing(t *testing.T) {
	idx := index.New()
	idx.Build([]domain.PackageManifest{
		{ID: existingUUID, Name: "Battery", Version: "1.1.0"},
	})

	_, router := setupPublishTestWithIndex(t, idx, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, mpackageJSONWithNameAndUUID(
		"BaTtErY", existingUUID, "3.0.0"))
	req := createPublishRequest(t, mltbx, "BaTtErY-3.0.0.mltbx")

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", rr.Code, http.StatusCreated, rr.Body.String())
	}

	var manifest domain.PackageManifest
	if err := json.NewDecoder(rr.Body).Decode(&manifest); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if manifest.Name != "BaTtErY" {
		t.Errorf("Name = %q, want %q -- 201 response must preserve original casing from mpackage.json", manifest.Name, "BaTtErY")
	}
	if manifest.Version != "3.0.0" {
		t.Errorf("Version = %q, want %q", manifest.Version, "3.0.0")
	}
	if manifest.ID != existingUUID {
		t.Errorf("ID = %q, want %q", manifest.ID, existingUUID)
	}
}
