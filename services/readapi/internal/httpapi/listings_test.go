package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListingsCountHandler(t *testing.T) {
	store := &fakeStore{count: 42}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/count", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var payload countPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Count != 42 {
		t.Fatalf("expected count 42, got %d", payload.Count)
	}
}

func TestListingsCountHandler_DBError(t *testing.T) {
	store := &fakeStore{countErr: errors.New("db down")}
	handler := NewHandler(store, true, func(ctx context.Context) error { return nil })
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings/count", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	var payload errorPayload
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Error != "db_unavailable" {
		t.Fatalf("expected error db_unavailable, got %q", payload.Error)
	}
}
