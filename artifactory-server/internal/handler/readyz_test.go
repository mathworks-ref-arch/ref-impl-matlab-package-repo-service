// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestHealthCheck_NotReady(t *testing.T) {
	var ready atomic.Bool
	h := HandleHealthCheck(&ready)

	req := httptest.NewRequest("GET", "/health-check", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rr.Code)
	}
}

func TestHealthCheck_Ready(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	h := HandleHealthCheck(&ready)

	req := httptest.NewRequest("GET", "/health-check", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	expected := `{"status":"ready"}`
	if rr.Body.String() != expected {
		t.Fatalf("expected %q, got %q", expected, rr.Body.String())
	}
}
