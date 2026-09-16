// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"net/http"
	"sync/atomic"

	"github.com/mathworks/matlab-package-repository-services/artifactory-server/internal/index"
)

type RouterConfig struct {
	RequiredHeaders []string
	EnablePublish   bool
	PublishHandler  *PublishHandler
	RefreshHandler  *RefreshHandler
}

func NewRouter(idx *index.Index, ready *atomic.Bool, cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	qh := NewQueryHandler(idx)

	// Health check is outside auth middleware
	mux.Handle("GET /health-check", HandleHealthCheck(ready))

	// API routes — protected by auth
	auth := AuthMiddleware(cfg.RequiredHeaders)
	mux.Handle("GET /v1/packages/index.json", auth(qh.HandleIndex()))
	mux.Handle("GET /v1/packages/by-uuid/{uuid}", auth(qh.HandleByUUID()))
	mux.Handle("GET /v1/packages/by-name/{name}", auth(qh.HandleByName()))

	if cfg.EnablePublish && cfg.PublishHandler != nil {
		mux.Handle("POST /v1/packages/publish", auth(cfg.PublishHandler.HandlePublish()))
	}

	if cfg.RefreshHandler != nil {
		mux.Handle("POST /v1/admin/refresh", auth(cfg.RefreshHandler.HandleRefresh()))
	}

	// Wrap everything with request-id, logging, recovery
	var h http.Handler = mux
	h = LoggingMiddleware(h)
	h = RequestIDMiddleware(h)
	h = RecoveryMiddleware(h)

	return h
}
