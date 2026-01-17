package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sylvain/realtoragent/services/readapi/internal/listings"
)

type statusResponse struct {
	Status string `json:"status"`
}

type readyzResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
}

type countResponse struct {
	Count int64 `json:"count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type listingsResponse struct {
	Items    []listingItem `json:"items"`
	Limit    int           `json:"limit"`
	Offset   int           `json:"offset"`
	Returned int           `json:"returned"`
}

type listingItem struct {
	PropertyKey string   `json:"property_key"`
	Address     string   `json:"address"`
	PostalCode  string   `json:"postal_code"`
	Price       *float64 `json:"price"`
	Beds        *int     `json:"beds"`
	Baths       *float64 `json:"baths"`
	Sqft        *int     `json:"sqft"`
	URL         *string  `json:"url"`
	ScrapedAt   string   `json:"scraped_at"`
}

type Handler struct {
	listings  *listings.Service
	dbEnabled bool
	dbPing    func(ctx context.Context) error
}

func NewHandler(listings *listings.Service, dbEnabled bool, dbPing func(ctx context.Context) error) *Handler {
	return &Handler{
		listings:  listings,
		dbEnabled: dbEnabled,
		dbPing:    dbPing,
	}
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, statusResponse{Status: "ok"})
}

func (h *Handler) Readyz(w http.ResponseWriter, r *http.Request) {
	if !h.dbEnabled {
		writeJSON(w, http.StatusOK, readyzResponse{Status: "ready", DB: "disabled"})
		return
	}

	if h.dbPing == nil {
		writeJSON(w, http.StatusServiceUnavailable, readyzResponse{Status: "not_ready", DB: "down"})
		return
	}

	if err := h.dbPing(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, readyzResponse{Status: "not_ready", DB: "down"})
		return
	}

	writeJSON(w, http.StatusOK, readyzResponse{Status: "ready", DB: "ok"})
}

func (h *Handler) ListingsCount(w http.ResponseWriter, r *http.Request) {
	if h.listings == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	count, err := h.listings.Count(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, countResponse{Count: count})
}

func (h *Handler) ListingsBrowse(w http.ResponseWriter, r *http.Request) {
	if h.listings == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	limit, offset, errCode := parsePagination(r)
	if errCode != "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: errCode})
		return
	}

	items, err := h.listings.List(r.Context(), limit, offset)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	respItems := make([]listingItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, listingItem{
			PropertyKey: item.PropertyKey,
			Address:     item.Address,
			PostalCode:  item.PostalCode,
			Price:       item.Price,
			Beds:        item.Beds,
			Baths:       item.Baths,
			Sqft:        item.Sqft,
			URL:         item.URL,
			ScrapedAt:   item.ScrapedAt.Format(timeFormatRFC3339),
		})
	}

	writeJSON(w, http.StatusOK, listingsResponse{
		Items:    respItems,
		Limit:    limit,
		Offset:   offset,
		Returned: len(respItems),
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

const (
	defaultLimit = 50
	maxLimit     = 100
	minLimit     = 1
)

const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"

func parsePagination(r *http.Request) (int, int, string) {
	query := r.URL.Query()
	limit := defaultLimit
	offset := 0

	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, "invalid_limit"
		}
		if parsed < minLimit || parsed > maxLimit {
			return 0, 0, "invalid_limit"
		}
		limit = parsed
	}

	if raw := query.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return 0, 0, "invalid_offset"
		}
		if parsed < 0 {
			return 0, 0, "invalid_offset"
		}
		offset = parsed
	}

	return limit, offset, ""
}
