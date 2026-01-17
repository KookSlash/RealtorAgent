package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/listings"
)

func TestListingsBrowse_Defaults(t *testing.T) {
	repo := &fakeListingsRepo{
		listings: []listings.Listing{
			{
				PropertyKey: "key-1",
				Address:     "123 Main St",
				PostalCode:  "T2P1A1",
				Price:       floatPtr(500000),
				Beds:        intPtr(2),
				Baths:       floatPtr(1.5),
				Sqft:        intPtr(900),
				URL:         stringPtr("http://example.com/1"),
				ScrapedAt:   time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
			},
		},
	}
	service := listings.NewService(repo)
	handler := NewHandler(service, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if repo.lastLimit != defaultLimit || repo.lastOffset != 0 {
		t.Fatalf("expected limit %d offset 0, got %d/%d", defaultLimit, repo.lastLimit, repo.lastOffset)
	}

	var payload listingsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Limit != defaultLimit {
		t.Fatalf("expected limit %d, got %d", defaultLimit, payload.Limit)
	}
	if payload.Offset != 0 {
		t.Fatalf("expected offset 0, got %d", payload.Offset)
	}
	if payload.Returned != 1 {
		t.Fatalf("expected returned 1, got %d", payload.Returned)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(payload.Items))
	}
	if payload.Items[0].PropertyKey != "key-1" {
		t.Fatalf("expected property_key key-1, got %q", payload.Items[0].PropertyKey)
	}
	if payload.Items[0].ScrapedAt != "2026-01-03T10:00:00Z" {
		t.Fatalf("expected scraped_at 2026-01-03T10:00:00Z, got %q", payload.Items[0].ScrapedAt)
	}
}

func TestListingsBrowse_InvalidLimit(t *testing.T) {
	repo := &fakeListingsRepo{}
	service := listings.NewService(repo)
	handler := NewHandler(service, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?limit=0", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	var payload errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Error != "invalid_limit" {
		t.Fatalf("expected error invalid_limit, got %q", payload.Error)
	}
}

func TestListingsBrowse_InvalidOffset(t *testing.T) {
	repo := &fakeListingsRepo{}
	service := listings.NewService(repo)
	handler := NewHandler(service, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?offset=-1", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	var payload errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Error != "invalid_offset" {
		t.Fatalf("expected error invalid_offset, got %q", payload.Error)
	}
}

func TestListingsBrowse_HappyPath(t *testing.T) {
	repo := &fakeListingsRepo{
		listings: []listings.Listing{
			{
				PropertyKey: "key-2",
				Address:     "456 Elm St",
				PostalCode:  "T2P2B2",
				Price:       floatPtr(750000),
				Beds:        intPtr(3),
				Baths:       floatPtr(2),
				Sqft:        intPtr(1400),
				URL:         stringPtr("http://example.com/2"),
				ScrapedAt:   time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC),
			},
			{
				PropertyKey: "key-3",
				Address:     "789 Pine St",
				PostalCode:  "T2P3C3",
				Price:       floatPtr(600000),
				Beds:        intPtr(4),
				Baths:       floatPtr(2),
				Sqft:        intPtr(1600),
				URL:         stringPtr("http://example.com/3"),
				ScrapedAt:   time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC),
			},
		},
	}
	service := listings.NewService(repo)
	handler := NewHandler(service, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?limit=2&offset=1", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if repo.lastLimit != 2 || repo.lastOffset != 1 {
		t.Fatalf("expected limit 2 offset 1, got %d/%d", repo.lastLimit, repo.lastOffset)
	}

	var payload listingsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Limit != 2 || payload.Offset != 1 {
		t.Fatalf("expected limit 2 offset 1, got %d/%d", payload.Limit, payload.Offset)
	}
	if payload.Returned != 2 {
		t.Fatalf("expected returned 2, got %d", payload.Returned)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
