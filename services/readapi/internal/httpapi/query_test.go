package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sylvain/realtoragent/services/readapi/internal/store"
)

func TestParseListQuery_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings", nil)

	query, errCode := parseListQuery(req)
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if query.Page != defaultPage || query.PageSize != defaultPageSize || query.Offset != 0 {
		t.Fatalf("unexpected pagination: page=%d size=%d offset=%d", query.Page, query.PageSize, query.Offset)
	}
	if query.Filter.Sort != store.SortLastSeenDesc {
		t.Fatalf("expected default sort last_seen_desc, got %s", query.Filter.Sort)
	}
}

func TestParseListQuery_PageParams(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings?page=2&page_size=25", nil)

	query, errCode := parseListQuery(req)
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if query.Page != 2 || query.PageSize != 25 || query.Offset != 25 {
		t.Fatalf("unexpected pagination: page=%d size=%d offset=%d", query.Page, query.PageSize, query.Offset)
	}
}

func TestParseListQuery_LimitOffsetFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings?limit=10&offset=20", nil)

	query, errCode := parseListQuery(req)
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if query.Page != 3 || query.PageSize != 10 || query.Offset != 20 {
		t.Fatalf("unexpected pagination: page=%d size=%d offset=%d", query.Page, query.PageSize, query.Offset)
	}
}

func TestParseListQuery_InvalidSort(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings?sort=bad", nil)

	_, errCode := parseListQuery(req)
	if errCode != "invalid_sort" {
		t.Fatalf("expected invalid_sort, got %q", errCode)
	}
}

func TestParseListQuery_SortMapping(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings?sort=ppsf_desc", nil)

	query, errCode := parseListQuery(req)
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if query.Filter.Sort != store.SortPPSFDesc {
		t.Fatalf("expected sort ppsf_desc, got %s", query.Filter.Sort)
	}
}

func TestParseListQuery_SortValueDesc(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings?sort=value_desc", nil)

	query, errCode := parseListQuery(req)
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if query.Filter.Sort != store.SortValueDesc {
		t.Fatalf("expected sort value_desc, got %s", query.Filter.Sort)
	}
}

func TestParseListQuery_PropertyTypeNormalization(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/listings?property_type=house", nil)

	query, errCode := parseListQuery(req)
	if errCode != "" {
		t.Fatalf("unexpected error: %s", errCode)
	}
	if query.Filter.PropertyType == nil || *query.Filter.PropertyType != "HOUSE" {
		t.Fatalf("expected property_type HOUSE, got %+v", query.Filter.PropertyType)
	}
}
