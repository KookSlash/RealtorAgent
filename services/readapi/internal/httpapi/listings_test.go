package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sylvain/realtoragent/services/readapi/internal/listings"
)

type fakeListingsRepo struct {
	count      int64
	err        error
	listings   []listings.Listing
	listErr    error
	lastLimit  int
	lastOffset int
}

func (f *fakeListingsRepo) Count(ctx context.Context) (int64, error) {
	return f.count, f.err
}

func (f *fakeListingsRepo) List(ctx context.Context, limit, offset int) ([]listings.Listing, error) {
	f.lastLimit = limit
	f.lastOffset = offset
	return f.listings, f.listErr
}

type countPayload struct {
	Count int64 `json:"count"`
}

type errorPayload struct {
	Error string `json:"error"`
}

func TestListingsCountHandler(t *testing.T) {
	repo := &fakeListingsRepo{count: 42}
	service := listings.NewService(repo)
	handler := NewHandler(service, false, nil)
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
	repo := &fakeListingsRepo{err: errors.New("db down")}
	service := listings.NewService(repo)
	handler := NewHandler(service, true, func(ctx context.Context) error { return nil })
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
