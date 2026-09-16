// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/publish"
)

type PublishHandler struct {
	idx              *index.Index
	uploader         publish.PackageUploader
	maxUploadBytes   int64
	reservedVersions sync.Map // uuid:version → struct{}
	reservedNames    sync.Map // lowercase(name) → uuid
}

func NewPublishHandler(idx *index.Index, uploader publish.PackageUploader, maxUploadBytes int64) *PublishHandler {
	return &PublishHandler{idx: idx, uploader: uploader, maxUploadBytes: maxUploadBytes}
}

func (h *PublishHandler) reserveVersion(uuid, version string) bool {
	key := uuid + ":" + version
	_, loaded := h.reservedVersions.LoadOrStore(key, struct{}{})
	return !loaded
}

func (h *PublishHandler) releaseVersion(uuid, version string) {
	h.reservedVersions.Delete(uuid + ":" + version)
}

func (h *PublishHandler) reserveName(name, uuid string) bool {
	key := strings.ToLower(name)
	val, loaded := h.reservedNames.LoadOrStore(key, uuid)
	if !loaded {
		return true
	}
	return val.(string) == uuid
}

func (h *PublishHandler) releaseName(name string) {
	h.reservedNames.Delete(strings.ToLower(name))
}

func (h *PublishHandler) HandlePublish() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callerToken := extractBearerToken(r)
		if callerToken == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token for publish")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, h.maxUploadBytes)

		if err := r.ParseMultipartForm(32 << 20); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(w, http.StatusRequestEntityTooLarge,
					fmt.Sprintf("upload exceeds maximum size of %d bytes", h.maxUploadBytes))
				return
			}
			writeError(w, http.StatusBadRequest, "failed to parse multipart form: "+err.Error())
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "missing required 'file' field in multipart form")
			return
		}
		defer file.Close()

		if !strings.HasSuffix(strings.ToLower(header.Filename), ".mltbx") {
			writeError(w, http.StatusBadRequest, "file must have .mltbx extension")
			return
		}

		tmpDir := os.TempDir()
		tmpFile, err := os.CreateTemp(tmpDir, "publish-*.mltbx")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create temp file")
			return
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		hasher := sha512.New()
		written, err := io.Copy(tmpFile, io.TeeReader(file, hasher))
		if err != nil {
			tmpFile.Close()
			writeError(w, http.StatusBadRequest, "failed to read uploaded file: "+err.Error())
			return
		}
		tmpFile.Close()

		digest := base64.StdEncoding.EncodeToString(hasher.Sum(nil))

		mp, err := publish.ExtractMPackage(tmpPath)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		archive := domain.Archive{
			Platforms: platformsFromMPackage(mp),
			URL:       h.uploader.ArchiveURL(buildStubManifest(mp)),
			Size:      written,
			Digests:   []domain.Digest{{Alg: "sha512", Digest: digest}},
		}

		manifest, err := publish.BuildManifest(mp, archive)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		if existingName := h.idx.NameConflict(manifest.Name, manifest.ID); existingName != "" {
			writeError(w, http.StatusConflict,
				fmt.Sprintf("package %q conflicts with existing package %q — names must be unique regardless of casing",
					manifest.Name, existingName))
			return
		}

		if !h.reserveName(manifest.Name, manifest.ID) {
			writeError(w, http.StatusConflict,
				fmt.Sprintf("package %q conflicts with an in-flight publish — names must be unique regardless of casing",
					manifest.Name))
			return
		}
		defer h.releaseName(manifest.Name)

		if h.idx.Exists(manifest.ID, manifest.Version) {
			writeError(w, http.StatusConflict,
				"package "+manifest.ID+" version "+manifest.Version+" already exists")
			return
		}

		if !h.reserveVersion(manifest.ID, manifest.Version) {
			writeError(w, http.StatusConflict,
				"package "+manifest.ID+" version "+manifest.Version+" publish already in progress")
			return
		}
		defer h.releaseVersion(manifest.ID, manifest.Version)

		if err := h.uploader.Publish(tmpPath, manifest, callerToken); err != nil {
			if errors.Is(err, publish.ErrConflict) {
				writeError(w, http.StatusConflict,
					"package "+manifest.ID+" version "+manifest.Version+" already exists in storage")
				return
			}
			slog.Error("publish failed", "error", err, "package", manifest.Name, "version", manifest.Version)
			writeError(w, http.StatusBadGateway, "failed to publish package to storage backend")
			return
		}

		h.idx.Add(*manifest)

		w.Header().Set("Location", "/v1/packages/by-uuid/"+manifest.ID+".json")
		writeJSON(w, http.StatusCreated, manifest)
	})
}

func platformsFromMPackage(mp *publish.MPackage) []string {
	return []string{"agnostic"}
}

func buildStubManifest(mp *publish.MPackage) *domain.PackageManifest {
	return &domain.PackageManifest{
		Name:    mp.Name,
		Version: mp.Version,
		ID:      mp.ID,
	}
}
