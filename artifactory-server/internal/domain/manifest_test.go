// Copyright 2026 The MathWorks, Inc.

package domain

import (
	"encoding/json"
	"testing"
)

func TestPackageManifest_Valid(t *testing.T) {
	m := PackageManifest{
		ID:      "220e47fe-0b34-4abe-991c-f8f121984346",
		Name:    "math",
		Version: "1.0.0",
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}
}

func TestPackageManifest_MissingID(t *testing.T) {
	m := PackageManifest{
		Name:    "math",
		Version: "1.0.0",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestPackageManifest_InvalidID(t *testing.T) {
	m := PackageManifest{
		ID:      "not-a-uuid",
		Name:    "math",
		Version: "1.0.0",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for invalid ID")
	}
}

func TestPackageManifest_MissingName(t *testing.T) {
	m := PackageManifest{
		ID:      "220e47fe-0b34-4abe-991c-f8f121984346",
		Version: "1.0.0",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestPackageManifest_InvalidName_PathTraversal(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"path traversal", "../../etc/evil"},
		{"slashes", "my/package"},
		{"dots", "my..pkg"},
		{"starts with number", "1badname"},
		{"starts with dash", "-badname"},
		{"spaces", "my package"},
		{"control chars", "pkg\x00name"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := PackageManifest{
				ID:      "220e47fe-0b34-4abe-991c-f8f121984346",
				Name:    tc.val,
				Version: "1.0.0",
			}
			if err := m.Validate(); err == nil {
				t.Fatalf("expected error for invalid name %q", tc.val)
			}
		})
	}
}

func TestPackageManifest_ValidNames(t *testing.T) {
	cases := []string{"math", "My-Package", "signal_processing", "A", "toolbox-v2"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			m := PackageManifest{
				ID:      "220e47fe-0b34-4abe-991c-f8f121984346",
				Name:    name,
				Version: "1.0.0",
			}
			if err := m.Validate(); err != nil {
				t.Fatalf("expected valid name %q, got error: %v", name, err)
			}
		})
	}
}

func TestPackageManifest_MissingVersion(t *testing.T) {
	m := PackageManifest{
		ID:   "220e47fe-0b34-4abe-991c-f8f121984346",
		Name: "math",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for missing version")
	}
}

func TestPackageManifest_InvalidVersion(t *testing.T) {
	m := PackageManifest{
		ID:      "220e47fe-0b34-4abe-991c-f8f121984346",
		Name:    "math",
		Version: "not.semver",
	}
	if err := m.Validate(); err == nil {
		t.Fatal("expected error for invalid version")
	}
}

func TestPackageManifest_FullJSON(t *testing.T) {
	raw := `{
		"version": "1.0.0",
		"name": "science",
		"id": "118409b9-6a71-4754-ab24-31a4f3871e15",
		"displayName": "science",
		"summary": "",
		"provider": {"name": "", "organization": ""},
		"releaseCompatibility": "",
		"dependencies": [
			{"name": "math", "id": "220e47fe-0b34-4abe-991c-f8f121984346", "compatibleVersions": "1.0.0"}
		],
		"supportedPlatforms": ["agnostic"],
		"formerNames": [],
		"archives": [
			{
				"platforms": ["agnostic"],
				"url": "https://example.com/science-1.0.0.mltbx",
				"size": 27286,
				"digests": [{"alg": "sha512", "digest": "abc123"}]
			}
		],
		"requiredAdditionalSoftware": []
	}`

	var m PackageManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if m.ID != "118409b9-6a71-4754-ab24-31a4f3871e15" {
		t.Fatalf("expected id, got %s", m.ID)
	}
	if m.Name != "science" {
		t.Fatalf("expected science, got %s", m.Name)
	}
	if m.DisplayName != "science" {
		t.Fatalf("expected displayName science, got %s", m.DisplayName)
	}
	if len(m.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(m.Dependencies))
	}
	if m.Dependencies[0].Name != "math" {
		t.Fatalf("expected dep name math, got %s", m.Dependencies[0].Name)
	}
	if len(m.Archives) != 1 {
		t.Fatalf("expected 1 archive, got %d", len(m.Archives))
	}
	if m.Archives[0].Size != 27286 {
		t.Fatalf("expected size 27286, got %d", m.Archives[0].Size)
	}
	if len(m.Archives[0].Digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(m.Archives[0].Digests))
	}
	if m.SupportedPlatforms == nil {
		t.Fatal("expected supportedPlatforms to be set")
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("expected valid manifest, got error: %v", err)
	}
}

func TestPackageManifest_SupportedPlatforms_StringArray(t *testing.T) {
	raw := `{
		"version": "1.0.0", "name": "pkg", "id": "220e47fe-0b34-4abe-991c-f8f121984346",
		"supportedPlatforms": ["agnostic"]
	}`
	var m PackageManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to unmarshal string array platforms: %v", err)
	}
}

func TestPackageManifest_SupportedPlatforms_ObjectArray(t *testing.T) {
	raw := `{
		"version": "1.0.0", "name": "pkg", "id": "220e47fe-0b34-4abe-991c-f8f121984346",
		"supportedPlatforms": [
			{"platform": "windows", "architectures": ["x86_64"]},
			{"platform": "linux", "architectures": ["x86_64"]}
		]
	}`
	var m PackageManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to unmarshal object array platforms: %v", err)
	}
}

func TestPackageManifest_Dependencies_ObjectFormat(t *testing.T) {
	raw := `{
		"version": "1.0.0", "name": "Wheel", "id": "0c0608e1-15e4-4977-9593-be5f735dff8b",
		"dependencies": {
			"Tire": {"id": "ac3b280e-da03-407e-bc39-676456f629df", "compatibleVersions": ""},
			"Rim":  {"id": "b1234567-0000-1111-2222-333344445555", "compatibleVersions": "2.0.0"}
		}
	}`
	var m PackageManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to unmarshal object dependencies: %v", err)
	}
	if len(m.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(m.Dependencies))
	}
	byName := make(map[string]Dependency)
	for _, d := range m.Dependencies {
		byName[d.Name] = d
	}
	tire, ok := byName["Tire"]
	if !ok {
		t.Fatal("expected dependency named Tire")
	}
	if tire.ID != "ac3b280e-da03-407e-bc39-676456f629df" {
		t.Fatalf("expected Tire id, got %s", tire.ID)
	}
	rim, ok := byName["Rim"]
	if !ok {
		t.Fatal("expected dependency named Rim")
	}
	if rim.CompatibleVersions != "2.0.0" {
		t.Fatalf("expected Rim compatibleVersions 2.0.0, got %s", rim.CompatibleVersions)
	}
}

func TestPackageManifest_Dependencies_EmptyObject(t *testing.T) {
	raw := `{
		"version": "1.0.0", "name": "pkg", "id": "220e47fe-0b34-4abe-991c-f8f121984346",
		"dependencies": {}
	}`
	var m PackageManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to unmarshal empty object dependencies: %v", err)
	}
	if len(m.Dependencies) != 0 {
		t.Fatalf("expected 0 dependencies, got %d", len(m.Dependencies))
	}
}

func TestPackageManifest_Dependencies_SingleObject(t *testing.T) {
	raw := `{
		"version": "1.0.0", "name": "Wheel", "id": "0c0608e1-15e4-4977-9593-be5f735dff8b",
		"dependencies": {
			"name": "Tire",
			"compatibleVersions": "",
			"id": "ac3b280e-da03-407e-bc39-676456f629df"
		}
	}`
	var m PackageManifest
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("failed to unmarshal single object dependencies: %v", err)
	}
	if len(m.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(m.Dependencies))
	}
	if m.Dependencies[0].Name != "Tire" {
		t.Fatalf("expected dep name Tire, got %s", m.Dependencies[0].Name)
	}
	if m.Dependencies[0].ID != "ac3b280e-da03-407e-bc39-676456f629df" {
		t.Fatalf("expected dep id, got %s", m.Dependencies[0].ID)
	}
}

func TestPackageManifest_RoundTrip_PreservesFormat(t *testing.T) {
	raw := `{"version":"1.0.0","name":"pkg","id":"220e47fe-0b34-4abe-991c-f8f121984346","supportedPlatforms":[{"platform":"windows","architectures":["x86_64"]}]}`
	var m PackageManifest
	json.Unmarshal([]byte(raw), &m)
	out, _ := json.Marshal(m)
	var parsed map[string]json.RawMessage
	json.Unmarshal(out, &parsed)
	// supportedPlatforms should still be an array of objects, not mangled
	if string(parsed["supportedPlatforms"]) == "null" {
		t.Fatal("supportedPlatforms should not be null after round-trip")
	}
}
