// Copyright 2026 The MathWorks, Inc.

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func validConfigJSON(t *testing.T, overrides ...string) string {
	t.Helper()
	base := map[string]any{
		"source": map[string]any{"actionsFile": "actions/artifactory.json"},
		"server": map[string]any{
			"readHeaderTimeout": "10s",
			"readTimeout":       "60s",
			"writeTimeout":      "120s",
			"idleTimeout":       "120s",
		},
		"backend": map[string]any{
			"readTimeout":              "30s",
			"uploadTimeout":            "120s",
			"maxAQLResponseBytes":      52428800,
			"maxManifestResponseBytes": 1048576,
			"maxUploadBytes":           524288000,
		},
	}
	for _, o := range overrides {
		var patch map[string]any
		if err := json.Unmarshal([]byte(o), &patch); err != nil {
			t.Fatalf("bad override JSON: %v", err)
		}
		for k, v := range patch {
			if sub, ok := v.(map[string]any); ok {
				existing, _ := base[k].(map[string]any)
				if existing == nil {
					existing = map[string]any{}
				}
				for sk, sv := range sub {
					existing[sk] = sv
				}
				base[k] = existing
			} else {
				base[k] = v
			}
		}
	}
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}
	return string(data)
}

func TestLoad_ValidConfig(t *testing.T) {
	data := validConfigJSON(t,
		`{"server": {"listenAddr": ":9090", "basePath": "/v1",
			"readHeaderTimeout": "5s", "readTimeout": "30s",
			"writeTimeout": "60s", "idleTimeout": "90s"}}`,
		`{"auth": {"requiredHeaders": ["Authorization"]}}`,
		`{"backend": {"readTimeout": "15s", "uploadTimeout": "60s",
			"maxAQLResponseBytes": 10485760, "maxManifestResponseBytes": 524288,
			"maxUploadBytes": 268435456}}`,
	)
	path := writeTemp(t, data)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %s, want :9090", cfg.Server.ListenAddr)
	}
	if cfg.Server.BasePath != "/v1" {
		t.Errorf("BasePath = %s, want /v1", cfg.Server.BasePath)
	}
	if len(cfg.Auth.RequiredHeaders) != 1 || cfg.Auth.RequiredHeaders[0] != "Authorization" {
		t.Errorf("RequiredHeaders = %v, want [Authorization]", cfg.Auth.RequiredHeaders)
	}
	if cfg.Source.ActionsFile != "actions/artifactory.json" {
		t.Errorf("ActionsFile = %s, want actions/artifactory.json", cfg.Source.ActionsFile)
	}
	if cfg.Server.ReadHeaderTimeout.Duration != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want 5s", cfg.Server.ReadHeaderTimeout.Duration)
	}
	if cfg.Server.ReadTimeout.Duration != 30*time.Second {
		t.Errorf("ReadTimeout = %v, want 30s", cfg.Server.ReadTimeout.Duration)
	}
	if cfg.Server.WriteTimeout.Duration != 60*time.Second {
		t.Errorf("WriteTimeout = %v, want 60s", cfg.Server.WriteTimeout.Duration)
	}
	if cfg.Server.IdleTimeout.Duration != 90*time.Second {
		t.Errorf("IdleTimeout = %v, want 90s", cfg.Server.IdleTimeout.Duration)
	}
	if cfg.Backend.ReadTimeout.Duration != 15*time.Second {
		t.Errorf("Backend.ReadTimeout = %v, want 15s", cfg.Backend.ReadTimeout.Duration)
	}
	if cfg.Backend.UploadTimeout.Duration != 60*time.Second {
		t.Errorf("Backend.UploadTimeout = %v, want 60s", cfg.Backend.UploadTimeout.Duration)
	}
	if cfg.Backend.MaxAQLResponseBytes != 10485760 {
		t.Errorf("MaxAQLResponseBytes = %d, want 10485760", cfg.Backend.MaxAQLResponseBytes)
	}
	if cfg.Backend.MaxManifestResponseBytes != 524288 {
		t.Errorf("MaxManifestResponseBytes = %d, want 524288", cfg.Backend.MaxManifestResponseBytes)
	}
	if cfg.Backend.MaxUploadBytes != 268435456 {
		t.Errorf("MaxUploadBytes = %d, want 268435456", cfg.Backend.MaxUploadBytes)
	}
}

func TestLoad_ListenAddr(t *testing.T) {
	for _, tt := range []struct {
		name       string
		overrides  []string
		portEnv    string
		wantAddr   string
	}{
		{
			name:     "default",
			wantAddr: ":8080",
		},
		{
			name:      "from config",
			overrides: []string{`{"server": {"listenAddr": ":9090"}}`},
			wantAddr:  ":9090",
		},
		{
			name:     "PORT env overrides default",
			portEnv:  "4000",
			wantAddr: ":4000",
		},
		{
			name:      "PORT env overrides config",
			overrides: []string{`{"server": {"listenAddr": ":9090"}}`},
			portEnv:   "3000",
			wantAddr:  ":3000",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			data := validConfigJSON(t, tt.overrides...)
			path := writeTemp(t, data)

			if tt.portEnv != "" {
				t.Setenv("PORT", tt.portEnv)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Server.ListenAddr != tt.wantAddr {
				t.Fatalf("ListenAddr = %s, want %s", cfg.Server.ListenAddr, tt.wantAddr)
			}
		})
	}
}

func TestLoad_InvalidConfig(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
	}{
		{
			name: "file not found",
		},
		{
			name: "invalid JSON",
			data: `{invalid`,
		},
		{
			name: "missing timeouts",
			data: `{"source": {"actionsFile": "actions/artifactory.json"}}`,
		},
		{
			name: "negative maxAQLResponseBytes",
			data: validConfigJSON(t, `{"backend": {"maxAQLResponseBytes": -1}}`),
		},
		{
			name: "zero readHeaderTimeout",
			data: validConfigJSON(t, `{"server": {"readHeaderTimeout": "0s"}}`),
		},
		{
			name: "zero maxUploadBytes",
			data: validConfigJSON(t, `{"backend": {"maxUploadBytes": 0}}`),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.data == "" {
				path = "/nonexistent/path.json"
			} else {
				path = writeTemp(t, tt.data)
			}

			_, err := Load(path)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
