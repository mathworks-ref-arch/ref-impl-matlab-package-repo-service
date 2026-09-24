// Copyright 2026 The MathWorks, Inc.

package backend

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/domain"
	"github.com/mathworks-ref-arch/ref-impl-matlab-package-repo-service/artifactory-server/internal/publish"
)

type UploaderConfig struct {
	BaseURL string
	RepoKey string
	Token   string
	Timeout time.Duration
	// AtomicPublish selects the publish strategy. When true, Publish uses a
	// single atomic explode-archive request (Artifactory Pro/Enterprise). When
	// false, it uses a two-step upload compatible with OSS Artifactory.
	AtomicPublish bool
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

// Publish uploads the .mltbx archive and manifest JSON to Artifactory. When
// AtomicPublish is true it uses a single atomic request (see publishAtomic);
// otherwise it falls back to a two-step upload for OSS Artifactory (see
// publishTwoStep).
func (u *Uploader) Publish(archivePath string, manifest *domain.PackageManifest, callerToken string) error {
	if !u.cfg.AtomicPublish {
		return u.publishTwoStep(archivePath, manifest, callerToken)
	}
	return u.publishAtomic(archivePath, manifest, callerToken)
}

// publishAtomic uploads the .mltbx archive and manifest JSON to Artifactory in a
// single atomic request using X-Explode-Archive-Atomic. Both files are bundled
// into a wrapper ZIP and uploaded to the version directory; Artifactory extracts
// them in place and removes the bundle. Requires Artifactory Pro/Enterprise.
func (u *Uploader) publishAtomic(archivePath string, manifest *domain.PackageManifest, callerToken string) error {
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

// publishTwoStep uploads the .mltbx archive and manifest JSON as two separate
// PUT requests to their final paths. This mode targets OSS Artifactory, which
// does not support the atomic explode-archive feature.
//
// It is NOT atomic and NOT idempotent at the storage layer: a plain PUT
// overwrites any existing object, so conflict detection relies entirely on the
// handler's pre-upload checks (index + in-flight reservations). If the manifest
// upload fails after the archive upload succeeds, the archive is left in place
// and an administrator must remove it — the returned error names the orphaned
// artifact. No rollback is attempted, so the service token needs only write
// (not delete) permission.
func (u *Uploader) publishTwoStep(archivePath string, manifest *domain.PackageManifest, callerToken string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive for upload: %w", err)
	}
	defer f.Close()

	archiveURL := u.ArchiveURL(manifest)
	if err := u.putFile(archiveURL, "application/octet-stream", f, callerToken); err != nil {
		return fmt.Errorf("archive upload failed: %w", err)
	}

	manifestData, err := json.MarshalIndent(manifest, "", "    ")
	if err != nil {
		return err
	}
	if err := u.putFile(u.manifestURL(manifest), "application/json",
		bytes.NewReader(manifestData), callerToken); err != nil {
		return fmt.Errorf("manifest upload failed after archive was uploaded; "+
			"orphaned archive %q must be removed manually: %w", archiveURL, err)
	}
	return nil
}

// putFile performs a single authenticated PUT and maps non-2xx responses to an
// error.
func (u *Uploader) putFile(url, contentType string, body io.Reader, callerToken string) error {
	req, err := http.NewRequest(http.MethodPut, url, body)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+callerToken)

	resp, err := u.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload returned status %d", resp.StatusCode)
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

func (u *Uploader) manifestURL(manifest *domain.PackageManifest) string {
	return fmt.Sprintf("%s/%s/%s/%s-%s.manifest.json",
		u.cfg.BaseURL, u.cfg.RepoKey, u.versionDir(manifest), manifest.Name, manifest.Version)
}

func (u *Uploader) versionDir(m *domain.PackageManifest) string {
	return fmt.Sprintf("%s/%s/%s", m.Name, m.ID, m.Version)
}
