// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/index"
)

type SourceFactory func(token string) index.ManifestSource

type RefreshHandler struct {
	idx           *index.Index
	sourceFactory SourceFactory
}

func NewRefreshHandler(idx *index.Index, sourceFactory SourceFactory) *RefreshHandler {
	return &RefreshHandler{idx: idx, sourceFactory: sourceFactory}
}

func (h *RefreshHandler) HandleRefresh() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("index refresh requested")

		token := extractBearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token for backend authentication")
			return
		}

		source := h.sourceFactory(token)
		manifests, err := source.ListManifests()
		if err != nil {
			slog.Error("index refresh failed", "error", err)
			writeError(w, http.StatusBadGateway, "failed to fetch manifests from backend")
			return
		}

		h.idx.Build(manifests)

		slog.Info("index refreshed", "packages", len(manifests))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"packages":     len(manifests),
			"lastModified": h.idx.BuiltAt().Format(time.RFC3339),
		})
	})
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}
