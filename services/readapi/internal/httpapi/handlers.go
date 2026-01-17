package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/sylvain/realtoragent/services/readapi/internal/model"
	"github.com/sylvain/realtoragent/services/readapi/internal/store"
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
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int           `json:"total"`
	Items    []listingItem `json:"items"`
}

type listingItem struct {
	PropertyKey    string   `json:"property_key"`
	PropertyType   string   `json:"property_type"`
	Address        string   `json:"address"`
	Unit           *string  `json:"unit"`
	City           *string  `json:"city"`
	Province       *string  `json:"province"`
	PostalCode     string   `json:"postal_code"`
	Lat            *float64 `json:"lat"`
	Lon            *float64 `json:"lon"`
	Beds           *int     `json:"beds"`
	Baths          *float64 `json:"baths"`
	Sqft           *int     `json:"sqft"`
	CurrentPrice   *float64 `json:"current_price"`
	PPSF           *float64 `json:"ppsf"`
	PPSFPercentile *float64 `json:"ppsf_percentile"`
	ValueScore     *float64 `json:"value_score"`
	CompsCount     int      `json:"comps_count"`
	URL            *string  `json:"url"`
	Source         string   `json:"source"`
	FirstSeenAt    string   `json:"first_seen_at"`
	LastSeenAt     string   `json:"last_seen_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type priceHistoryResponse struct {
	PropertyKey string              `json:"property_key"`
	Series      []priceHistoryPoint `json:"series"`
	Stats       priceHistoryStats   `json:"stats"`
}

type priceHistoryPoint struct {
	ObservedAt string  `json:"observed_at"`
	Price      float64 `json:"price"`
}

type priceHistoryStats struct {
	FirstObservedAt     string  `json:"first_observed_at"`
	LastObservedAt      string  `json:"last_observed_at"`
	FirstPrice          float64 `json:"first_price"`
	LastPrice           float64 `json:"last_price"`
	AbsChange           float64 `json:"abs_change"`
	PctChange           float64 `json:"pct_change"`
	MinPrice            float64 `json:"min_price"`
	MaxPrice            float64 `json:"max_price"`
	NumObservations     int     `json:"num_observations"`
	NumPriceChanges     int     `json:"num_price_changes"`
	DaysSinceLastChange int     `json:"days_since_last_change"`
	MaxDrawdownPct      float64 `json:"max_drawdown_pct"`
}

type changesResponse struct {
	Since string       `json:"since"`
	Items []changeItem `json:"items"`
}

type changeItem struct {
	PropertyKey string  `json:"property_key"`
	ObservedAt  string  `json:"observed_at"`
	OldPrice    float64 `json:"old_price"`
	NewPrice    float64 `json:"new_price"`
	AbsChange   float64 `json:"abs_change"`
	PctChange   float64 `json:"pct_change"`
}

type Handler struct {
	store     store.Store
	dbEnabled bool
	dbPing    func(ctx context.Context) error
}

func NewHandler(store store.Store, dbEnabled bool, dbPing func(ctx context.Context) error) *Handler {
	return &Handler{
		store:     store,
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
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	count, err := h.store.CountListings(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, countResponse{Count: count})
}

func (h *Handler) ListingsBrowse(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	query, errCode := parseListQuery(r)
	if errCode != "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: errCode})
		return
	}

	items, total, err := h.store.ListListings(r.Context(), query.Filter, query.PageSize, query.Offset)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	respItems := make([]listingItem, 0, len(items))
	for _, item := range items {
		respItems = append(respItems, toListingItem(item))
	}

	writeJSON(w, http.StatusOK, listingsResponse{
		Page:     query.Page,
		PageSize: query.PageSize,
		Total:    total,
		Items:    respItems,
	})
}

func (h *Handler) ListingsByKey(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/v1/listings/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	propertyKey := parts[0]
	if propertyKey == "" {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 1 {
		h.listingDetail(w, r, propertyKey)
		return
	}

	if len(parts) == 2 && parts[1] == "price-history" {
		h.listingPriceHistory(w, r, propertyKey)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) Changes(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	changeQuery, errCode := parseChangesQuery(r)
	if errCode != "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: errCode})
		return
	}

	items, err := h.store.ListPriceChanges(r.Context(), changeQuery.Since)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	respItems := make([]changeItem, 0, len(items))
	for _, item := range items {
		abs := item.NewPrice - item.OldPrice
		pct := 0.0
		if item.OldPrice != 0 {
			pct = (abs / item.OldPrice) * 100
		}
		respItems = append(respItems, changeItem{
			PropertyKey: item.PropertyKey,
			ObservedAt:  item.ObservedAt.Format(timeFormatRFC3339),
			OldPrice:    item.OldPrice,
			NewPrice:    item.NewPrice,
			AbsChange:   abs,
			PctChange:   pct,
		})
	}

	writeJSON(w, http.StatusOK, changesResponse{
		Since: changeQuery.Since.Format(timeFormatRFC3339),
		Items: respItems,
	})
}

func (h *Handler) listingDetail(w http.ResponseWriter, r *http.Request, propertyKey string) {
	item, err := h.store.GetListing(r.Context(), propertyKey)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "not_found"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, toListingItem(item))
}

func (h *Handler) listingPriceHistory(w http.ResponseWriter, r *http.Request, propertyKey string) {
	if _, err := h.store.GetListing(r.Context(), propertyKey); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "not_found"})
			return
		}
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	series, err := h.store.GetPriceHistory(r.Context(), propertyKey)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, errorResponse{Error: "db_unavailable"})
		return
	}

	respSeries := make([]priceHistoryPoint, 0, len(series))
	for _, point := range series {
		respSeries = append(respSeries, priceHistoryPoint{
			ObservedAt: point.ObservedAt.Format(timeFormatRFC3339),
			Price:      point.Price,
		})
	}

	stats := computePriceHistoryStats(series)
	writeJSON(w, http.StatusOK, priceHistoryResponse{
		PropertyKey: propertyKey,
		Series:      respSeries,
		Stats:       stats,
	})
}

func toListingItem(item model.Listing) listingItem {
	ppsf := item.PPSF
	if ppsf == nil {
		ppsf = calcPPSF(item.CurrentPrice, item.Sqft)
	} else {
		rounded := math.Round((*ppsf)*100) / 100
		ppsf = &rounded
	}
	return listingItem{
		PropertyKey:    item.PropertyKey,
		PropertyType:   item.PropertyType,
		Address:        item.Address,
		Unit:           item.Unit,
		City:           item.City,
		Province:       item.Province,
		PostalCode:     item.PostalCode,
		Lat:            item.Lat,
		Lon:            item.Lon,
		Beds:           item.Beds,
		Baths:          item.Baths,
		Sqft:           item.Sqft,
		CurrentPrice:   item.CurrentPrice,
		PPSF:           ppsf,
		PPSFPercentile: item.PPSFPercentile,
		ValueScore:     item.ValueScore,
		CompsCount:     item.CompsCount,
		URL:            item.URL,
		Source:         item.Source,
		FirstSeenAt:    item.FirstSeenAt.Format(timeFormatRFC3339),
		LastSeenAt:     item.LastSeenAt.Format(timeFormatRFC3339),
		UpdatedAt:      item.UpdatedAt.Format(timeFormatRFC3339),
	}
}

func calcPPSF(price *float64, sqft *int) *float64 {
	if price == nil || sqft == nil || *sqft <= 0 {
		return nil
	}
	value := *price / float64(*sqft)
	rounded := math.Round(value*100) / 100
	return &rounded
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

const timeFormatRFC3339 = "2006-01-02T15:04:05Z07:00"
