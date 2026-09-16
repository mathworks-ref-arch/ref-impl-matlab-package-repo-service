// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/domain"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/publish"
)

type UploaderConfig struct {
	BaseURL string
	RepoKey string
	Token   string
	Timeout time.Duration
}

// Uploader uploads packages to Artifactory via HTTP PUT.
// It implements publish.PackageUploader.
type Uploader struct {
	cfg    UploaderConfig
	client *http.Client
}

func NewUploader(cfg UploaderConfig) *Uploader {
	return &Uploader{cfg: cfg, client: &http.Client{Timeout: cfg.Timeout}}
}

// Publish uploads the .mltbx archive and manifest JSON to Artifactory in a
// single atomic request using X-Explode-Archive-Atomic. Both files are bundled
// into a wrapper ZIP and uploaded to the version directory; Artifactory extracts
// them in place and removes the bundle.
func (u *Uploader) Publish(archivePath string, manifest *domain.PackageManifest, callerToken string) error {
	bundlePath, err := u.createBundle(archivePath, manifest)
	if err != nil {
		return fmt.Errorf("failed to create publish bundle: %w", err)
	}
	defer os.Remove(bundlePath)

	targetDir := u.versionDir(manifest)
	bundleName := fmt.Sprintf("%s-%s-bundle.zip", manifest.Name, manifest.Version)
	url := fmt.Sprintf("%s/%s/%s/%s", u.cfg.BaseURL, u.cfg.RepoKey, targetDir, bundleName)

	f, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to open bundle for upload: %w", err)
	}
	defer f.Close()

	req, err := http.NewRequest(http.MethodPut, url, f)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Authorization", "Bearer "+callerToken)
	req.Header.Set("X-Explode-Archive-Atomic", "true")

	resp, err := u.client.Do(req)
	if err != nil {
		return fmt.Errorf("publish upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		return publish.ErrConflict
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("publish upload returned status %d", resp.StatusCode)
	}
	return nil
}

// createBundle builds a temporary ZIP containing the .mltbx and manifest JSON
// with filenames matching the repository layout convention.
func (u *Uploader) createBundle(archivePath string, manifest *domain.PackageManifest) (string, error) {
	tmpFile, err := os.CreateTemp("", "publish-bundle-*.zip")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	zw := zip.NewWriter(tmpFile)

	mltbxName := fmt.Sprintf("%s-%s.mltbx", manifest.Name, manifest.Version)
	mltbxWriter, err := zw.Create(mltbxName)
	if err != nil {
		return "", err
	}
	mltbxFile, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(mltbxWriter, mltbxFile); err != nil {
		mltbxFile.Close()
		return "", err
	}
	mltbxFile.Close()

	manifestName := fmt.Sprintf("%s-%s.manifest.json", manifest.Name, manifest.Version)
	manifestWriter, err := zw.Create(manifestName)
	if err != nil {
		return "", err
	}
	manifestData, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return "", err
	}
	if _, err := manifestWriter.Write(manifestData); err != nil {
		return "", err
	}

	if err := zw.Close(); err != nil {
		return "", err
	}

	return tmpFile.Name(), nil
}

func (u *Uploader) ArchiveURL(manifest *domain.PackageManifest) string {
	return fmt.Sprintf("%s/%s/%s/%s-%s.mltbx",
		u.cfg.BaseURL, u.cfg.RepoKey, u.versionDir(manifest), manifest.Name, manifest.Version)
}

func (u *Uploader) versionDir(m *domain.PackageManifest) string {
	return fmt.Sprintf("%s/%s/%s", m.Name, m.ID, m.Version)
}
