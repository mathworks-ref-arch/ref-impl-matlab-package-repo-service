// Copyright 2026 The MathWorks, Inc.

package publish

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createTestMLTBX(t *testing.T, mpackageJSON string) string {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "test.mltbx")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	w, err := zw.Create("fsroot/resources/mpackage.json")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(mpackageJSON)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return tmp
}

func TestExtractMPackage_Valid(t *testing.T) {
	input := `{
		"name": "math",
		"version": "1.1.0",
		"id": "220e47fe-0b34-4abe-991c-f8f121984346",
		"formerNames": [],
		"displayName": "math",
		"summary": "A math toolbox",
		"tags": ["math"],
		"readme": "some readme",
		"provider": {
			"name": "Test Author",
			"organization": "TestOrg",
			"email": "test@example.com",
			"url": "https://example.com"
		},
		"folders": [{"path": "src", "languages": ["matlab"]}],
		"dependencies": [],
		"releaseCompatibility": ">=R2024b",
		"supportedPlatforms": [{"platform": "any", "architectures": ["any"]}],
		"schemaVersion": "1.2.0"
	}`
	path := createTestMLTBX(t, input)

	mp, err := ExtractMPackage(path)
	if err != nil {
		t.Fatalf("ExtractMPackage() error = %v", err)
	}

	if mp.Name != "math" {
		t.Errorf("Name = %q, want %q", mp.Name, "math")
	}
	if mp.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", mp.Version, "1.1.0")
	}
	if mp.ID != "220e47fe-0b34-4abe-991c-f8f121984346" {
		t.Errorf("ID = %q, want %q", mp.ID, "220e47fe-0b34-4abe-991c-f8f121984346")
	}
	if mp.DisplayName != "math" {
		t.Errorf("DisplayName = %q, want %q", mp.DisplayName, "math")
	}
	if mp.Summary != "A math toolbox" {
		t.Errorf("Summary = %q, want %q", mp.Summary, "A math toolbox")
	}
	if mp.Provider.Name != "Test Author" {
		t.Errorf("Provider.Name = %q, want %q", mp.Provider.Name, "Test Author")
	}
	if mp.Provider.Organization != "TestOrg" {
		t.Errorf("Provider.Organization = %q, want %q", mp.Provider.Organization, "TestOrg")
	}
	if mp.ReleaseCompatibility != ">=R2024b" {
		t.Errorf("ReleaseCompatibility = %q, want %q", mp.ReleaseCompatibility, ">=R2024b")
	}
	var platforms []json.RawMessage
	if err := json.Unmarshal(mp.SupportedPlatforms, &platforms); err != nil {
		t.Fatalf("failed to unmarshal SupportedPlatforms: %v", err)
	}
	if len(platforms) != 1 {
		t.Errorf("SupportedPlatforms count = %d, want 1", len(platforms))
	}
}

func TestExtractMPackage_MissingFile(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "empty.mltbx")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("other/file.txt")
	w.Write([]byte("hello"))
	zw.Close()
	f.Close()

	_, err = ExtractMPackage(tmp)
	if err == nil {
		t.Fatal("expected error for missing mpackage.json")
	}
}

func TestExtractMPackage_NotAZip(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "corrupt.mltbx")
	os.WriteFile(tmp, []byte("not a zip file"), 0644)

	_, err := ExtractMPackage(tmp)
	if err == nil {
		t.Fatal("expected error for corrupt file")
	}
}

func TestExtractMPackage_InvalidJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.mltbx")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("fsroot/resources/mpackage.json")
	w.Write([]byte("{invalid json"))
	zw.Close()
	f.Close()

	_, err = ExtractMPackage(tmp)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestExtractMPackage_OversizedMPackageJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "oversized.mltbx")
	f, err := os.Create(tmp)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	w, err := zw.Create("fsroot/resources/mpackage.json")
	if err != nil {
		t.Fatal(err)
	}
	oversized := make([]byte, 1<<20+1) // 1 MB + 1 byte
	for i := range oversized {
		oversized[i] = 'x'
	}
	w.Write(oversized)
	zw.Close()
	f.Close()

	_, err = ExtractMPackage(tmp)
	if err == nil {
		t.Fatal("expected error for oversized mpackage.json")
	}
}

func TestExtractMPackage_RealMLTBX(t *testing.T) {
	path := "/mathworks/devel/sandbox/tsapre/metadata_test_server/v1/artifacts/math/220e47fe-0b34-4abe-991c-f8f121984346/1.1.0/math-1.1.0.mltbx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("real MLTBX not available")
	}

	mp, err := ExtractMPackage(path)
	if err != nil {
		t.Fatalf("ExtractMPackage() error = %v", err)
	}

	if mp.Name != "math" {
		t.Errorf("Name = %q, want %q", mp.Name, "math")
	}
	if mp.Version != "1.1.0" {
		t.Errorf("Version = %q, want %q", mp.Version, "1.1.0")
	}
	if mp.ID != "220e47fe-0b34-4abe-991c-f8f121984346" {
		t.Errorf("ID = %q, want %q", mp.ID, "220e47fe-0b34-4abe-991c-f8f121984346")
	}
}
