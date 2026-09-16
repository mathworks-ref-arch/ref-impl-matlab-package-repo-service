// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"net/http"
	"sync/atomic"
)

func HandleHealthCheck(ready *atomic.Bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ready.Load() {
			writeError(w, http.StatusServiceUnavailable, "not ready")
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})
}
