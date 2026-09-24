// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/index"
)

type indexResponse struct {
	Packages     []domain.PackageManifest `json:"packages"`
	LastModified string                   `json:"lastModified"`
}

type QueryHandler struct {
	idx *index.Index
}

func NewQueryHandler(idx *index.Index) *QueryHandler {
	return &QueryHandler{idx: idx}
}

func (h *QueryHandler) HandleIndex() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		manifests := h.idx.QueryAll()
		resp := indexResponse{
			Packages:     manifests,
			LastModified: h.idx.BuiltAt().Format(time.RFC3339),
		}
		writeJSON(w, http.StatusOK, resp)
	})
}

func (h *QueryHandler) HandleByUUID() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uuid := strings.TrimSuffix(r.PathValue("uuid"), ".json")
		manifests, err := h.idx.QueryByUUID(uuid)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, manifests)
	})
}

func (h *QueryHandler) HandleByName() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(r.PathValue("name"), ".json")
		manifests, err := h.idx.QueryByName(name)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, manifests)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
