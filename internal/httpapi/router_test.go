package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q", got)
	}
	if got := rec.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestIndex(t *testing.T) {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "LK Advisor") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestApiIndex(t *testing.T) {
	h := New(slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "Эндпоинты") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
