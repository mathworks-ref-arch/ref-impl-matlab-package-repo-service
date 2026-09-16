// Copyright 2026 The MathWorks, Inc.

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/backend"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/config"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/handler"
	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

func main() {
	configPath := flag.String("config", "configs/server-artifactory.json", "path to server config file")
	flag.Parse()

	if err := run(*configPath); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	// 1. Load config
	slog.Info("loading config", "file", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Load actions and build index from backend
	slog.Info("loading actions", "file", cfg.Source.ActionsFile)
	actions, err := backend.LoadActions(cfg.Source.ActionsFile)
	if err != nil {
		return fmt.Errorf("loading actions: %w", err)
	}

	sourceLimits := backend.SourceLimits{
		Timeout:             cfg.Backend.ReadTimeout.Duration,
		MaxAQLResponseBytes: cfg.Backend.MaxAQLResponseBytes,
		MaxManifestBytes:    cfg.Backend.MaxManifestResponseBytes,
	}
	source := backend.NewSource(*actions, sourceLimits)

	slog.Info("building index from backend...")
	manifests, err := source.ListManifests()
	if err != nil {
		return fmt.Errorf("building index: %w", err)
	}

	idx := index.New()
	idx.Build(manifests)
	slog.Info("index built", "packages", len(manifests))

	// 3. Set up router and start server
	var ready atomic.Bool
	ready.Store(true)

	refreshFactory := func(token string) index.ManifestSource {
		return backend.NewSourceWithToken(*actions, sourceLimits, token)
	}

	routerCfg := handler.RouterConfig{
		RequiredHeaders: cfg.Auth.RequiredHeaders,
		EnablePublish:   cfg.Server.EnablePublish,
		RefreshHandler:  handler.NewRefreshHandler(idx, refreshFactory),
	}

	if cfg.Server.EnablePublish {
		uploader := backend.NewUploader(backend.UploaderConfig{
			BaseURL: actions.BaseURL,
			RepoKey: actions.RepoKey,
			Token:   os.Getenv(actions.Auth.TokenEnv),
			Timeout: cfg.Backend.UploadTimeout.Duration,
		})
		routerCfg.PublishHandler = handler.NewPublishHandler(idx, uploader, cfg.Backend.MaxUploadBytes)
		slog.Info("publish endpoint enabled")
	}

	router := handler.NewRouter(idx, &ready, routerCfg)

	srv := &http.Server{
		Addr:              cfg.Server.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout.Duration,
		ReadTimeout:       cfg.Server.ReadTimeout.Duration,
		WriteTimeout:      cfg.Server.WriteTimeout.Duration,
		IdleTimeout:       cfg.Server.IdleTimeout.Duration,
	}
	slog.Info("server listening", "addr", cfg.Server.ListenAddr)
	return srv.ListenAndServe()
}
