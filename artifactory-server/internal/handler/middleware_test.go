// Copyright 2026 The MathWorks, Inc.

package handler

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthMiddleware_ValidBearer(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := AuthMiddleware([]string{"Authorization"})(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer my-token")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})
	mw := AuthMiddleware([]string{"Authorization"})(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_EmptyHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	})
	mw := AuthMiddleware([]string{"Authorization"})(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
}

func TestAuthMiddleware_NoRequiredHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := AuthMiddleware(nil)(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestRequestIDMiddleware_AddsHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") == "" {
			t.Fatal("expected X-Request-ID to be set")
		}
		w.WriteHeader(http.StatusOK)
	})
	mw := RequestIDMiddleware(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
	if rr.Header().Get("X-Request-ID") == "" {
		t.Fatal("expected X-Request-ID in response")
	}
}

func TestRequestIDMiddleware_PreservesExisting(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") != "existing-id" {
			t.Fatalf("expected existing-id, got %s", r.Header.Get("X-Request-ID"))
		}
		w.WriteHeader(http.StatusOK)
	})
	mw := RequestIDMiddleware(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "existing-id")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
}

func TestLoggingMiddleware_IncludesClientName(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := LoggingMiddleware(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Client-Name", "matlab-desktop")
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "matlab-desktop") {
		t.Fatalf("expected log to contain client name, got: %s", logOutput)
	}
}

func TestLoggingMiddleware_OmitsClientNameWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	slog.SetDefault(logger)

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := LoggingMiddleware(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)

	logOutput := buf.String()
	if strings.Contains(logOutput, "client_name") {
		t.Fatalf("expected no client_name in log when header absent, got: %s", logOutput)
	}
}

func TestRecoveryMiddleware_CatchesPanic(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	mw := RecoveryMiddleware(inner)

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	mw.ServeHTTP(rr, req)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rr.Code)
	}
}
