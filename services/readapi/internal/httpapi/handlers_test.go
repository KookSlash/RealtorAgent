package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type statusPayload struct {
	Status string `json:"status"`
}

type readyzPayload struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

func TestHealthzHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	handler := NewHandler(nil, false, nil)
	http.HandlerFunc(handler.Healthz).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}
	contentType := recorder.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("expected Content-Type application/json, got %q", contentType)
	}

	var payload statusPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Status != "ok" {
		t.Fatalf("expected status ok, got %q", payload.Status)
	}
}

func TestReadyzHandler(t *testing.T) {
	t.Run("db disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		recorder := httptest.NewRecorder()

		handler := NewHandler(nil, false, nil)
		http.HandlerFunc(handler.Readyz).ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
		}
		contentType := recorder.Header().Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			t.Fatalf("expected Content-Type application/json, got %q", contentType)
		}

		var payload readyzPayload
		if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
			t.Fatalf("decode JSON: %v", err)
		}
		if payload.Status != "ready" {
			t.Fatalf("expected status ready, got %q", payload.Status)
		}
		if payload.DB != "disabled" {
			t.Fatalf("expected db disabled, got %q", payload.DB)
		}
	})

	t.Run("db enabled and ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		recorder := httptest.NewRecorder()

		handler := NewHandler(nil, true, func(ctx context.Context) error { return nil })
		http.HandlerFunc(handler.Readyz).ServeHTTP(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
		}

		var payload readyzPayload
		if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
			t.Fatalf("decode JSON: %v", err)
		}
		if payload.Status != "ready" {
			t.Fatalf("expected status ready, got %q", payload.Status)
		}
		if payload.DB != "ok" {
			t.Fatalf("expected db ok, got %q", payload.DB)
		}
	})

	t.Run("db enabled and down", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
		recorder := httptest.NewRecorder()

		handler := NewHandler(nil, true, func(ctx context.Context) error { return errors.New("down") })
		http.HandlerFunc(handler.Readyz).ServeHTTP(recorder, req)

		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
		}

		var payload readyzPayload
		if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
			t.Fatalf("decode JSON: %v", err)
		}
		if payload.Status != "not_ready" {
			t.Fatalf("expected status not_ready, got %q", payload.Status)
		}
		if payload.DB != "down" {
			t.Fatalf("expected db down, got %q", payload.DB)
		}
	})
}
