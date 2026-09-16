// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

func benchManifest(i int) domain.PackageManifest {
	return domain.PackageManifest{
		Version:              fmt.Sprintf("%d.%d.%d", i/100+1, (i/10)%10, i%10),
		Name:                 fmt.Sprintf("package-%04d", i),
		ID:                   fmt.Sprintf("%08d-0000-0000-0000-000000000000", i),
		DisplayName:          fmt.Sprintf("Package %d", i),
		Summary:              fmt.Sprintf("Summary for package %d with some realistic text content", i),
		Provider:             domain.Provider{Name: "author", Organization: "MathWorks"},
		ReleaseCompatibility: "R2023a-R2025a",
		SupportedPlatforms:   json.RawMessage(`["win64","maci64","glnxa64"]`),
		Archives: []domain.Archive{
			{
				Platforms: []string{"win64", "maci64", "glnxa64"},
				URL:       fmt.Sprintf("https://artifactory.example.com/repo/package-%04d/%d.%d.%d/package.mltbx", i, i/100+1, (i/10)%10, i%10),
				Size:      1024 * 1024 * (int64(i%50) + 1),
				Digests:   []domain.Digest{{Alg: "SHA-512", Digest: fmt.Sprintf("abcdef1234567890%040d", i)}},
			},
		},
	}
}

func startFakeArtifactory(n int) *httptest.Server {
	manifests := make([]domain.PackageManifest, n)
	aqlResults := make([]map[string]any, n)

	for i := range n {
		manifests[i] = benchManifest(i)
		aqlResults[i] = map[string]any{
			"repo": "mpm-packages",
			"path": fmt.Sprintf("%s/%s/%s", manifests[i].Name, manifests[i].ID, manifests[i].Version),
			"name": fmt.Sprintf("%s-%s-manifest.json", manifests[i].Name, manifests[i].Version),
		}
	}

	aqlBody, _ := json.Marshal(map[string]any{"results": aqlResults})

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/search/aql", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(aqlBody)
	})

	mux.HandleFunc("GET /mpm-packages/", func(w http.ResponseWriter, r *http.Request) {
		for i := range manifests {
			expected := fmt.Sprintf("/mpm-packages/%s/%s/%s/%s-%s-manifest.json",
				manifests[i].Name, manifests[i].ID, manifests[i].Version,
				manifests[i].Name, manifests[i].Version)
			if r.URL.Path == expected {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(manifests[i])
				return
			}
		}
		http.NotFound(w, r)
	})

	return httptest.NewServer(mux)
}

func benchmarkFullStartup(b *testing.B, n int) {
	srv := startFakeArtifactory(n)
	defer srv.Close()

	actions := ActionsConfig{
		BaseURL: srv.URL,
		RepoKey: "mpm-packages",
		Auth: AuthConfig{
			Strategy: "bearer",
			TokenEnv: "BENCH_TOKEN",
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

	b.Setenv("BENCH_TOKEN", "bench-token")
	b.ResetTimer()

	for range b.N {
		src := NewSource(actions, defaultTestLimits)
		manifests, err := src.ListManifests()
		if err != nil {
			b.Fatalf("ListManifests failed: %v", err)
		}

		idx := index.New()
		idx.Build(manifests)

		if len(idx.QueryAll()) != n {
			b.Fatalf("expected %d manifests in index, got %d", n, len(idx.QueryAll()))
		}
	}
}

func BenchmarkFullStartup_100(b *testing.B)   { benchmarkFullStartup(b, 100) }
func BenchmarkFullStartup_500(b *testing.B)   { benchmarkFullStartup(b, 500) }
func BenchmarkFullStartup_1000(b *testing.B)  { benchmarkFullStartup(b, 1000) }
func BenchmarkFullStartup_2000(b *testing.B)  { benchmarkFullStartup(b, 2000) }
func BenchmarkFullStartup_5000(b *testing.B)  { benchmarkFullStartup(b, 5000) }
func BenchmarkFullStartup_10000(b *testing.B) { benchmarkFullStartup(b, 10000) }
