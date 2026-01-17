package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORSHeadersOnGet(t *testing.T) {
	store := &fakeStore{count: 1}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "http://localhost:3000")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/count", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("expected Access-Control-Allow-Origin to be set")
	}
}

func TestCORSPreflightOptions(t *testing.T) {
	store := &fakeStore{count: 1}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodOptions, "/v1/listings/count", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin to be set")
	}
	if recorder.Header().Get("Access-Control-Allow-Methods") != corsAllowMethods {
		t.Fatalf("expected Access-Control-Allow-Methods to be %q", corsAllowMethods)
	}
	if recorder.Header().Get("Access-Control-Allow-Headers") != corsAllowHeaders {
		t.Fatalf("expected Access-Control-Allow-Headers to be %q", corsAllowHeaders)
	}
}
