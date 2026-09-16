// Copyright 2026 The MathWorks, Inc.

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Duration.String())
}

type Config struct {
	Server  ServerConfig  `json:"server"`
	Auth    AuthConfig    `json:"auth"`
	Source  SourceConfig  `json:"source"`
	Backend BackendConfig `json:"backend"`
}

type ServerConfig struct {
	ListenAddr        string   `json:"listenAddr"`
	BasePath          string   `json:"basePath"`
	EnablePublish     bool     `json:"enablePublish"`
	ReadHeaderTimeout Duration `json:"readHeaderTimeout"`
	ReadTimeout       Duration `json:"readTimeout"`
	WriteTimeout      Duration `json:"writeTimeout"`
	IdleTimeout       Duration `json:"idleTimeout"`
}

type BackendConfig struct {
	ReadTimeout              Duration `json:"readTimeout"`
	UploadTimeout            Duration `json:"uploadTimeout"`
	MaxAQLResponseBytes      int64    `json:"maxAQLResponseBytes"`
	MaxManifestResponseBytes int64    `json:"maxManifestResponseBytes"`
	MaxUploadBytes           int64    `json:"maxUploadBytes"`
}

type AuthConfig struct {
	RequiredHeaders []string `json:"requiredHeaders"`
}

type SourceConfig struct {
	ActionsFile string `json:"actionsFile"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if port := os.Getenv("PORT"); port != "" {
		cfg.Server.ListenAddr = ":" + port
	} else if cfg.Server.ListenAddr == "" {
		cfg.Server.ListenAddr = ":8080"
	}

	if v := os.Getenv("ENABLE_PUBLISH"); v == "true" || v == "1" {
		cfg.Server.EnablePublish = true
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	for _, check := range []struct {
		name string
		ok   bool
	}{
		{"server.readHeaderTimeout", c.Server.ReadHeaderTimeout.Duration > 0},
		{"server.readTimeout", c.Server.ReadTimeout.Duration > 0},
		{"server.writeTimeout", c.Server.WriteTimeout.Duration > 0},
		{"server.idleTimeout", c.Server.IdleTimeout.Duration > 0},
		{"backend.readTimeout", c.Backend.ReadTimeout.Duration > 0},
		{"backend.uploadTimeout", c.Backend.UploadTimeout.Duration > 0},
		{"backend.maxAQLResponseBytes", c.Backend.MaxAQLResponseBytes > 0},
		{"backend.maxManifestResponseBytes", c.Backend.MaxManifestResponseBytes > 0},
		{"backend.maxUploadBytes", c.Backend.MaxUploadBytes > 0},
	} {
		if !check.ok {
			return fmt.Errorf("%s must be a positive value", check.name)
		}
	}
	return nil
}
