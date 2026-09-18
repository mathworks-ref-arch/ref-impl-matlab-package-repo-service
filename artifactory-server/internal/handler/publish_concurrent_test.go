// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestPublishHandler_ConcurrentDuplicate(t *testing.T) {
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	})

	mltbx := createTestMLTBXBytes(t, validMPackageJSON())

	const numPublishers = 2
	results := make([]int, numPublishers)

	var wg sync.WaitGroup
	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := createPublishRequest(t, mltbx, "math-1.1.0.mltbx")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			results[idx] = rr.Code
		}(i)
	}
	wg.Wait()

	created := 0
	conflict := 0
	for _, code := range results {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if created != 1 {
		t.Errorf("expected exactly 1 success (201), got %d", created)
	}
	if conflict != 1 {
		t.Errorf("expected exactly 1 conflict (409), got %d", conflict)
	}
}

func TestPublishHandler_ConcurrentDifferentPackages(t *testing.T) {
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	packages := []struct {
		name    string
		version string
		uuid    string
	}{
		{"alpha", "1.0.0", "aaaaaaaa-0000-0000-0000-000000000001"},
		{"beta", "1.0.0", "bbbbbbbb-0000-0000-0000-000000000002"},
	}

	var wg sync.WaitGroup
	results := make([]int, len(packages))

	for i, pkg := range packages {
		wg.Add(1)
		go func(idx int, name, version, uuid string) {
			defer wg.Done()
			mpJSON := mpackageJSON(name, version, uuid)
			mltbx := createTestMLTBXBytes(t, mpJSON)
			req := createPublishRequest(t, mltbx, name+"-"+version+".mltbx")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			results[idx] = rr.Code
		}(i, pkg.name, pkg.version, pkg.uuid)
	}
	wg.Wait()

	for i, code := range results {
		if code != http.StatusCreated {
			t.Errorf("package[%d] status = %d, want %d", i, code, http.StatusCreated)
		}
	}
}

func TestPublishHandler_ConcurrentNameSquatting(t *testing.T) {
	_, router := setupPublishTest(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	})

	packages := []struct {
		name string
		uuid string
	}{
		{"Battery", "aaaaaaaa-0000-0000-0000-000000000001"},
		{"battery", "bbbbbbbb-0000-0000-0000-000000000002"},
	}

	var wg sync.WaitGroup
	results := make([]int, len(packages))

	for i, pkg := range packages {
		wg.Add(1)
		go func(idx int, name, uuid string) {
			defer wg.Done()
			mpJSON := mpackageJSON(name, "1.0.0", uuid)
			mltbx := createTestMLTBXBytes(t, mpJSON)
			req := createPublishRequest(t, mltbx, name+"-1.0.0.mltbx")
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
			results[idx] = rr.Code
		}(i, pkg.name, pkg.uuid)
	}
	wg.Wait()

	created := 0
	conflict := 0
	for _, code := range results {
		switch code {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			conflict++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	if created != 1 {
		t.Errorf("expected exactly 1 success (201), got %d", created)
	}
	if conflict != 1 {
		t.Errorf("expected exactly 1 conflict (409), got %d", conflict)
	}
}

func mpackageJSON(name, version, uuid string) string {
	data := map[string]interface{}{
		"name":                 name,
		"version":              version,
		"id":                   uuid,
		"formerNames":          []string{},
		"displayName":          name,
		"summary":              "",
		"tags":                 []string{},
		"readme":               "",
		"provider":             map[string]string{"name": "", "organization": "", "email": "", "url": ""},
		"folders":              []map[string]interface{}{{"path": "src", "languages": []string{"matlab"}}},
		"dependencies":         []string{},
		"releaseCompatibility": ">=R2024b",
		"supportedPlatforms":   []map[string]interface{}{{"platform": "any", "architectures": []string{"any"}}},
		"schemaVersion":        "1.2.0",
	}
	b, _ := json.Marshal(data)
	return string(b)
}
