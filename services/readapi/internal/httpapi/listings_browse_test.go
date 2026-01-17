package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/model"
)

func TestListingsBrowse_Defaults(t *testing.T) {
	store := &fakeStore{
		listItems: []model.Listing{
			{
				PropertyKey:  "key-1",
				PropertyType: "HOUSE",
				Address:      "123 Main St",
				PostalCode:   "T2P1A1",
				CurrentPrice: floatPtr(500000),
				Beds:         intPtr(2),
				Baths:        floatPtr(1.5),
				Sqft:         intPtr(1000),
				URL:          stringPtr("http://example.com/1"),
				Source:       "REALTOR_CA",
				FirstSeenAt:  time.Date(2026, 1, 3, 8, 0, 0, 0, time.UTC),
				LastSeenAt:   time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC),
			},
		},
		listTotal: 1,
	}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if store.lastLimit != defaultPageSize || store.lastOffset != 0 {
		t.Fatalf("expected limit %d offset 0, got %d/%d", defaultPageSize, store.lastLimit, store.lastOffset)
	}

	var payload listingsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Page != defaultPage {
		t.Fatalf("expected page %d, got %d", defaultPage, payload.Page)
	}
	if payload.PageSize != defaultPageSize {
		t.Fatalf("expected page_size %d, got %d", defaultPageSize, payload.PageSize)
	}
	if payload.Total != 1 {
		t.Fatalf("expected total 1, got %d", payload.Total)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(payload.Items))
	}
	if payload.Items[0].PropertyKey != "key-1" {
		t.Fatalf("expected property_key key-1, got %q", payload.Items[0].PropertyKey)
	}
	if payload.Items[0].LastSeenAt != "2026-01-03T10:00:00Z" {
		t.Fatalf("expected last_seen_at 2026-01-03T10:00:00Z, got %q", payload.Items[0].LastSeenAt)
	}
	if payload.Items[0].PPSF == nil || *payload.Items[0].PPSF != 500 {
		t.Fatalf("expected ppsf 500, got %+v", payload.Items[0].PPSF)
	}
}

func TestListingsBrowse_InvalidPageSize(t *testing.T) {
	store := &fakeStore{}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?page_size=0", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	var payload errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Error != "invalid_page_size" {
		t.Fatalf("expected error invalid_page_size, got %q", payload.Error)
	}
}

func TestListingsBrowse_InvalidPage(t *testing.T) {
	store := &fakeStore{}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?page=0", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}
	var payload errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Error != "invalid_page" {
		t.Fatalf("expected error invalid_page, got %q", payload.Error)
	}
}

func TestListingsBrowse_HappyPath(t *testing.T) {
	store := &fakeStore{
		listItems: []model.Listing{
			{
				PropertyKey:  "key-2",
				PropertyType: "CONDO",
				Address:      "456 Elm St",
				PostalCode:   "T2P2B2",
				CurrentPrice: floatPtr(750000),
				Beds:         intPtr(3),
				Baths:        floatPtr(2),
				Sqft:         intPtr(1400),
				URL:          stringPtr("http://example.com/2"),
				Source:       "REALTOR_CA",
				FirstSeenAt:  time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC),
				LastSeenAt:   time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 1, 2, 9, 0, 0, 0, time.UTC),
			},
			{
				PropertyKey:  "key-3",
				PropertyType: "HOUSE",
				Address:      "789 Pine St",
				PostalCode:   "T2P3C3",
				CurrentPrice: floatPtr(600000),
				Beds:         intPtr(4),
				Baths:        floatPtr(2),
				Sqft:         intPtr(1600),
				URL:          stringPtr("http://example.com/3"),
				Source:       "REALTOR_CA",
				FirstSeenAt:  time.Date(2026, 1, 1, 7, 0, 0, 0, time.UTC),
				LastSeenAt:   time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC),
				UpdatedAt:    time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC),
			},
		},
		listTotal: 2,
	}
	handler := NewHandler(store, false, nil)
	router := NewRouter(handler, "*")

	req := httptest.NewRequest(http.MethodGet, "/v1/listings?page=2&page_size=2", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if store.lastLimit != 2 || store.lastOffset != 2 {
		t.Fatalf("expected limit 2 offset 2, got %d/%d", store.lastLimit, store.lastOffset)
	}

	var payload listingsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if payload.Page != 2 || payload.PageSize != 2 {
		t.Fatalf("expected page 2 page_size 2, got %d/%d", payload.Page, payload.PageSize)
	}
	if payload.Total != 2 {
		t.Fatalf("expected total 2, got %d", payload.Total)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
}
