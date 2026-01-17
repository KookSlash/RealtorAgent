package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/store"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
	minPageSize     = 1
	defaultPage     = 1
	minPage         = 1
)

var allowedPropertyTypes = map[string]struct{}{
	"HOUSE":     {},
	"CONDO":     {},
	"TOWNHOUSE": {},
	"DUPLEX":    {},
	"LAND":      {},
	"OTHER":     {},
}

type listQuery struct {
	Page     int
	PageSize int
	Offset   int
	Filter   store.ListingsFilter
}

type changesQuery struct {
	Since time.Time
}

func parseListQuery(r *http.Request) (listQuery, string) {
	query := r.URL.Query()
	result := listQuery{
		Page:     defaultPage,
		PageSize: defaultPageSize,
		Filter: store.ListingsFilter{
			Sort: store.SortLastSeenDesc,
		},
	}

	pageProvided := query.Get("page") != "" || query.Get("page_size") != ""
	if pageProvided {
		if raw := query.Get("page"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < minPage {
				return listQuery{}, "invalid_page"
			}
			result.Page = parsed
		}
		if raw := query.Get("page_size"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < minPageSize || parsed > maxPageSize {
				return listQuery{}, "invalid_page_size"
			}
			result.PageSize = parsed
		}
		result.Offset = (result.Page - 1) * result.PageSize
	} else {
		if raw := query.Get("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < minPageSize || parsed > maxPageSize {
				return listQuery{}, "invalid_limit"
			}
			result.PageSize = parsed
		}
		if raw := query.Get("offset"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 0 {
				return listQuery{}, "invalid_offset"
			}
			result.Offset = parsed
		}
		result.Page = (result.Offset / result.PageSize) + 1
	}

	if raw := strings.TrimSpace(query.Get("min_price")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 {
			return listQuery{}, "invalid_min_price"
		}
		result.Filter.MinPrice = &value
	}
	if raw := strings.TrimSpace(query.Get("max_price")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 {
			return listQuery{}, "invalid_max_price"
		}
		result.Filter.MaxPrice = &value
	}
	if raw := strings.TrimSpace(query.Get("min_beds")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return listQuery{}, "invalid_min_beds"
		}
		result.Filter.MinBeds = &value
	}
	if raw := strings.TrimSpace(query.Get("min_baths")); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil || value < 0 {
			return listQuery{}, "invalid_min_baths"
		}
		result.Filter.MinBaths = &value
	}
	if raw := strings.TrimSpace(query.Get("min_sqft")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return listQuery{}, "invalid_min_sqft"
		}
		result.Filter.MinSqft = &value
	}
	if raw := strings.TrimSpace(query.Get("max_sqft")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return listQuery{}, "invalid_max_sqft"
		}
		result.Filter.MaxSqft = &value
	}
	if raw := strings.TrimSpace(query.Get("property_type")); raw != "" {
		normalized := strings.ToUpper(strings.ReplaceAll(raw, " ", ""))
		if _, ok := allowedPropertyTypes[normalized]; !ok {
			return listQuery{}, "invalid_property_type"
		}
		result.Filter.PropertyType = &normalized
	}
	if raw := strings.TrimSpace(query.Get("postal_code")); raw != "" {
		normalized := normalizePostalPrefix(raw)
		if normalized == "" {
			return listQuery{}, "invalid_postal_code"
		}
		result.Filter.PostalPrefix = &normalized
	}
	if raw := strings.TrimSpace(query.Get("city")); raw != "" {
		result.Filter.City = &raw
	}
	if raw := strings.TrimSpace(query.Get("q")); raw != "" {
		result.Filter.Query = &raw
	}

	if raw := strings.TrimSpace(query.Get("sort")); raw != "" {
		sortValue, ok := parseSort(strings.ToLower(raw))
		if !ok {
			return listQuery{}, "invalid_sort"
		}
		result.Filter.Sort = sortValue
	}

	return result, ""
}

func parseChangesQuery(r *http.Request) (changesQuery, string) {
	query := r.URL.Query()
	rawSince := strings.TrimSpace(query.Get("since"))
	if rawSince == "" {
		return changesQuery{}, "missing_since"
	}
	since, err := time.Parse(time.RFC3339, rawSince)
	if err != nil {
		return changesQuery{}, "invalid_since"
	}

	changeType := strings.ToLower(strings.TrimSpace(query.Get("type")))
	if changeType == "" {
		return changesQuery{}, "missing_type"
	}
	if changeType != "price_change" {
		return changesQuery{}, "invalid_type"
	}

	return changesQuery{Since: since}, ""
}

func parseSort(value string) (store.ListingSort, bool) {
	switch value {
	case string(store.SortLastSeenDesc):
		return store.SortLastSeenDesc, true
	case string(store.SortPriceAsc):
		return store.SortPriceAsc, true
	case string(store.SortPriceDesc):
		return store.SortPriceDesc, true
	case string(store.SortPPSFAsc):
		return store.SortPPSFAsc, true
	case string(store.SortPPSFDesc):
		return store.SortPPSFDesc, true
	case string(store.SortValueDesc):
		return store.SortValueDesc, true
	default:
		return "", false
	}
}

func normalizePostalPrefix(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	var b strings.Builder
	for _, r := range value {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
