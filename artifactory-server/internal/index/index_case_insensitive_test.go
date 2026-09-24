// Copyright 2026 The MathWorks, Inc.

package index

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
)

// QueryByName matches regardless of input casing.
func TestIndex_QueryByName_CaseInsensitive(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Math", Version: "1.0.0"},
		{ID: "uuid-1", Name: "Math", Version: "2.0.0"},
	})

	cases := []string{"Math", "math", "MATH", "mAtH"}
	for _, query := range cases {
		results, err := idx.QueryByName(query)
		if err != nil {
			t.Fatalf("QueryByName(%q) error = %v", query, err)
		}
		if len(results) != 2 {
			t.Errorf("QueryByName(%q) returned %d results, want 2", query, len(results))
		}
	}
}

// QueryByName preserves original casing in results.
func TestIndex_QueryByName_PreservesOriginalCasing(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "MyToolbox", Version: "1.0.0"},
	})

	results, err := idx.QueryByName("mytoolbox")
	if err != nil {
		t.Fatalf("QueryByName() error = %v", err)
	}
	if results[0].Name != "MyToolbox" {
		t.Errorf("Name = %q, want %q (original casing preserved)", results[0].Name, "MyToolbox")
	}
}

// NameConflict detects different UUID with case-insensitive name match.
func TestIndex_NameConflict_DifferentUUID(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
	})

	if got := idx.NameConflict("battery", "uuid-2"); got == "" {
		t.Error("NameConflict() = empty, want existing name for different UUID with case-insensitive match")
	} else if got != "Battery" {
		t.Errorf("NameConflict() = %q, want %q", got, "Battery")
	}
	if got := idx.NameConflict("BATTERY", "uuid-3"); got == "" {
		t.Error("NameConflict() = empty, want existing name for different UUID with uppercase match")
	}
}

// NameConflict allows same UUID with different casing (no conflict).
func TestIndex_NameConflict_SameUUID(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
	})

	if got := idx.NameConflict("battery", "uuid-1"); got != "" {
		t.Errorf("NameConflict() = %q, want empty for same UUID (new version allowed)", got)
	}
	if got := idx.NameConflict("BATTERY", "uuid-1"); got != "" {
		t.Errorf("NameConflict() = %q, want empty for same UUID with different casing", got)
	}
}

// Exists detects duplicate UUID+version regardless of name casing.
func TestIndex_Exists_CatchesDuplicateVersion(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
	})

	if !idx.Exists("uuid-1", "1.0.0") {
		t.Error("Exists() = false, want true for same UUID+version")
	}
	if idx.Exists("uuid-1", "2.0.0") {
		t.Error("Exists() = true, want false for new version")
	}
}

// NameConflict returns empty when no package with that name exists.
func TestIndex_NameConflict_NoExisting(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
	})

	if got := idx.NameConflict("Unrelated", "uuid-2"); got != "" {
		t.Errorf("NameConflict() = %q, want empty for unrelated name", got)
	}
}

// Add() indexes packages for case-insensitive lookup.
func TestIndex_Add_CaseInsensitiveLookup(t *testing.T) {
	idx := New()
	idx.Build(nil)

	idx.Add(domain.PackageManifest{ID: "uuid-1", Name: "MyToolbox", Version: "1.0.0"})

	results, err := idx.QueryByName("mytoolbox")
	if err != nil {
		t.Fatalf("QueryByName() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("QueryByName() len = %d, want 1", len(results))
	}
	if results[0].Name != "MyToolbox" {
		t.Errorf("Name = %q, want %q", results[0].Name, "MyToolbox")
	}
}

// Build logs a warning when different UUIDs share a case-folded name.
func TestIndex_Build_LogsWarningOnCaseCollision(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	idx := NewWithLogger(logger)
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
		{ID: "uuid-2", Name: "battery", Version: "1.0.0"},
	})

	logOutput := buf.String()
	if !strings.Contains(logOutput, "case-insensitive name collision") {
		t.Errorf("expected warning log about case collision, got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "Battery") {
		t.Errorf("expected log to mention existing name 'Battery', got: %q", logOutput)
	}
	if !strings.Contains(logOutput, "battery") {
		t.Errorf("expected log to mention conflicting name 'battery', got: %q", logOutput)
	}
}

// Build logs exactly one warning per collision, not spurious duplicates when
// the same UUID appears after a different UUID in the manifest list.
func TestIndex_Build_NoSpuriousWarningForEstablishedOwner(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	idx := NewWithLogger(logger)
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-2", Name: "battery", Version: "1.0.0"},
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
		{ID: "uuid-1", Name: "Battery", Version: "2.0.0"},
	})

	logOutput := buf.String()
	count := strings.Count(logOutput, "case-insensitive name collision")
	if count != 1 {
		t.Errorf("expected exactly 1 collision warning, got %d; log: %q", count, logOutput)
	}
}

// Build() collision detection for manual-upload + refresh scenarios.
// Starting state: "Battery" UUID c1438758 v1.1.0 already exists in the manifest list.
func TestIndex_Build_CollisionCases(t *testing.T) {
	const (
		batteryUUID = "c1438758-4eaf-438f-b04f-6a7279735de5"
		otherUUID   = "99999999-0000-0000-0000-000000000000"
	)

	existing := domain.PackageManifest{
		ID: batteryUUID, Name: "Battery", Version: "1.1.0",
	}

	tests := []struct {
		name        string
		added       domain.PackageManifest
		wantWarning bool
		wantCount   int
		wantNames   []string
	}{
		{
			name:        "same UUID same version no warning",
			added:       domain.PackageManifest{ID: batteryUUID, Name: "battery", Version: "1.1.0"},
			wantWarning: false,
			wantCount:   2,
			wantNames:   []string{"Battery", "battery"},
		},
		{
			name:        "different UUID same version logs collision",
			added:       domain.PackageManifest{ID: otherUUID, Name: "battery", Version: "1.1.0"},
			wantWarning: true,
			wantCount:   2,
			wantNames:   []string{"Battery", "battery"},
		},
		{
			name:        "different UUID different version logs collision",
			added:       domain.PackageManifest{ID: otherUUID, Name: "battery", Version: "2.0.0"},
			wantWarning: true,
			wantCount:   2,
			wantNames:   []string{"Battery", "battery"},
		},
		{
			name:        "same UUID different version no warning",
			added:       domain.PackageManifest{ID: batteryUUID, Name: "battery", Version: "2.0.0"},
			wantWarning: false,
			wantCount:   2,
			wantNames:   []string{"Battery", "battery"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))

			idx := NewWithLogger(logger)
			idx.Build([]domain.PackageManifest{existing, tt.added})

			logOutput := buf.String()
			hasWarning := strings.Contains(logOutput, "case-insensitive name collision")
			if tt.wantWarning && !hasWarning {
				t.Errorf("expected collision warning, got log: %q", logOutput)
			}
			if !tt.wantWarning && hasWarning {
				t.Errorf("unexpected collision warning in log: %q", logOutput)
			}

			results, err := idx.QueryByName("battery")
			if err != nil {
				t.Fatalf("QueryByName(\"battery\") error = %v", err)
			}
			if len(results) != tt.wantCount {
				t.Fatalf("QueryByName(\"battery\") returned %d results, want %d", len(results), tt.wantCount)
			}

			gotNames := make(map[string]bool)
			for _, r := range results {
				gotNames[r.Name] = true
			}
			for _, wantName := range tt.wantNames {
				if !gotNames[wantName] {
					t.Errorf("expected name %q in results, got names: %v", wantName, gotNames)
				}
			}
		})
	}
}

// NameConflict allows established owners to publish when a pre-existing
// collision was loaded at Build() time (e.g., from manual Artifactory upload).
func TestIndex_NameConflict_AllowsEstablishedOwnerDespiteCollision(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "Battery", Version: "1.0.0"},
		{ID: "uuid-2", Name: "battery", Version: "1.0.0"}, // pre-existing collision from manual upload
	})

	if got := idx.NameConflict("Battery", "uuid-1"); got != "" {
		t.Errorf("NameConflict(Battery, uuid-1) = %q, want empty — uuid-1 is an established owner", got)
	}
	if got := idx.NameConflict("battery", "uuid-2"); got != "" {
		t.Errorf("NameConflict(battery, uuid-2) = %q, want empty — uuid-2 is an established owner", got)
	}
	if got := idx.NameConflict("BATTERY", "uuid-3"); got == "" {
		t.Error("NameConflict(BATTERY, uuid-3) = empty, want conflict for new UUID not in index")
	}
}

// Build() must never panic, error, or skip manifests — even with collisions.
func TestIndex_Build_NeverDropsManifests(t *testing.T) {
	const (
		batteryUUID = "c1438758-4eaf-438f-b04f-6a7279735de5"
		otherUUID   = "99999999-0000-0000-0000-000000000000"
	)

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	idx := NewWithLogger(logger)
	manifests := []domain.PackageManifest{
		{ID: batteryUUID, Name: "Battery", Version: "1.1.0"},
		{ID: otherUUID, Name: "battery", Version: "1.1.0"},
		{ID: otherUUID, Name: "battery", Version: "2.0.0"},
		{ID: batteryUUID, Name: "Battery", Version: "2.0.0"},
	}
	idx.Build(manifests)

	all := idx.QueryAll()
	if len(all) != 4 {
		t.Errorf("QueryAll() returned %d manifests, want 4", len(all))
	}

	results, err := idx.QueryByName("BATTERY")
	if err != nil {
		t.Fatalf("QueryByName(\"BATTERY\") error = %v", err)
	}
	if len(results) != 4 {
		t.Errorf("QueryByName(\"BATTERY\") returned %d results, want 4", len(results))
	}

	byBatteryUUID, err := idx.QueryByUUID(batteryUUID)
	if err != nil {
		t.Fatalf("QueryByUUID(%q) error = %v", batteryUUID, err)
	}
	if len(byBatteryUUID) != 2 {
		t.Errorf("QueryByUUID(%q) returned %d, want 2", batteryUUID, len(byBatteryUUID))
	}

	byOtherUUID, err := idx.QueryByUUID(otherUUID)
	if err != nil {
		t.Fatalf("QueryByUUID(%q) error = %v", otherUUID, err)
	}
	if len(byOtherUUID) != 2 {
		t.Errorf("QueryByUUID(%q) returned %d, want 2", otherUUID, len(byOtherUUID))
	}
}
