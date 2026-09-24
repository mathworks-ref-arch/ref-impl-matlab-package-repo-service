// Copyright 2026 The MathWorks, Inc.

package index

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
)

type Index struct {
	mu      sync.RWMutex
	all     []domain.PackageManifest
	byUUID  map[string][]domain.PackageManifest
	byName  map[string][]domain.PackageManifest // keyed by strings.ToLower(name)
	builtAt time.Time
	logger  *slog.Logger
}

func New() *Index {
	return NewWithLogger(slog.Default())
}

func NewWithLogger(logger *slog.Logger) *Index {
	return &Index{
		byUUID: make(map[string][]domain.PackageManifest),
		byName: make(map[string][]domain.PackageManifest),
		logger: logger,
	}
}

func (idx *Index) Build(manifests []domain.PackageManifest) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.builtAt = time.Now().UTC()
	idx.all = make([]domain.PackageManifest, len(manifests))
	copy(idx.all, manifests)

	idx.byUUID = make(map[string][]domain.PackageManifest)
	idx.byName = make(map[string][]domain.PackageManifest)

	seenUUIDs := make(map[string]map[string]bool) // lowercase(name) → set of UUIDs
	for _, m := range manifests {
		idx.byUUID[m.ID] = append(idx.byUUID[m.ID], m)
		key := strings.ToLower(m.Name)

		if uuids := seenUUIDs[key]; len(uuids) > 0 && !uuids[m.ID] {
			first := idx.byName[key][0]
			idx.logger.Warn("case-insensitive name collision",
				"existingName", first.Name,
				"existingUUID", first.ID,
				"conflictingName", m.Name,
				"conflictingUUID", m.ID,
			)
		}
		if seenUUIDs[key] == nil {
			seenUUIDs[key] = make(map[string]bool)
		}
		seenUUIDs[key][m.ID] = true

		idx.byName[key] = append(idx.byName[key], m)
	}
}

func (idx *Index) BuiltAt() time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.builtAt
}

func (idx *Index) QueryAll() []domain.PackageManifest {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]domain.PackageManifest, len(idx.all))
	copy(result, idx.all)
	return result
}

func (idx *Index) QueryByUUID(uuid string) ([]domain.PackageManifest, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results, ok := idx.byUUID[uuid]
	if !ok {
		return nil, fmt.Errorf("%w: package with UUID %q", domain.ErrNotFound, uuid)
	}
	out := make([]domain.PackageManifest, len(results))
	copy(out, results)
	return out, nil
}

func (idx *Index) QueryByName(name string) ([]domain.PackageManifest, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results, ok := idx.byName[strings.ToLower(name)]
	if !ok {
		return nil, fmt.Errorf("%w: package with name %q", domain.ErrNotFound, name)
	}
	out := make([]domain.PackageManifest, len(results))
	copy(out, results)
	return out, nil
}

func (idx *Index) Add(m domain.PackageManifest) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.all = append(idx.all, m)
	idx.byUUID[m.ID] = append(idx.byUUID[m.ID], m)
	key := strings.ToLower(m.Name)
	idx.byName[key] = append(idx.byName[key], m)
	idx.builtAt = time.Now().UTC()
}

// NameConflict checks whether a new publish would introduce a name-squatting
// collision. Returns the conflicting package's original name, or empty string
// if the publish is safe. A UUID that already has entries under this
// case-folded name is an established owner and is always allowed.
func (idx *Index) NameConflict(name, uuid string) string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	existing, ok := idx.byName[strings.ToLower(name)]
	if !ok {
		return ""
	}

	conflictName := ""
	for _, m := range existing {
		if m.ID == uuid {
			return ""
		}
		if conflictName == "" {
			conflictName = m.Name
		}
	}
	return conflictName
}

func (idx *Index) Exists(uuid, version string) bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	for _, m := range idx.byUUID[uuid] {
		if m.Version == version {
			return true
		}
	}
	return false
}
