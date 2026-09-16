// Copyright 2026 The MathWorks, Inc.

package index

import (
	"fmt"
	"sync"
	"testing"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
)

func TestIndex_ConcurrentReadWrite(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "seed-uuid", Name: "seed-pkg", Version: "1.0.0"},
	})

	const numReaders = 10
	const numIterations = 100
	const numAdds = 50

	var wg sync.WaitGroup

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				_ = idx.QueryAll()
				_, _ = idx.QueryByUUID("seed-uuid")
				_, _ = idx.QueryByName("seed-pkg")
				_ = idx.Exists("seed-uuid", "1.0.0")
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < numAdds; j++ {
			idx.Add(domain.PackageManifest{
				ID:      fmt.Sprintf("added-uuid-%d", j),
				Name:    fmt.Sprintf("added-pkg-%d", j),
				Version: "1.0.0",
			})
		}
	}()

	wg.Wait()

	all := idx.QueryAll()
	expected := 1 + numAdds
	if len(all) != expected {
		t.Errorf("QueryAll() len = %d, want %d", len(all), expected)
	}
}

func TestIndex_ConcurrentBuildAndRead(t *testing.T) {
	idx := New()
	idx.Build([]domain.PackageManifest{
		{ID: "uuid-1", Name: "pkg-1", Version: "1.0.0"},
	})

	const numReaders = 10
	const numIterations = 100

	var wg sync.WaitGroup

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numIterations; j++ {
				_ = idx.QueryAll()
				_, _ = idx.QueryByUUID("uuid-1")
				_ = idx.BuiltAt()
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < numIterations; j++ {
			idx.Build([]domain.PackageManifest{
				{ID: "uuid-1", Name: "pkg-1", Version: fmt.Sprintf("%d.0.0", j)},
			})
		}
	}()

	wg.Wait()

	all := idx.QueryAll()
	if len(all) != 1 {
		t.Errorf("QueryAll() len = %d, want 1", len(all))
	}
}
