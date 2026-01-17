package httpapi

import (
	"context"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/model"
	"github.com/sylvain/realtoragent/services/readapi/internal/store"
)

type fakeStore struct {
	count      int64
	countErr   error
	listItems  []model.Listing
	listTotal  int
	listErr    error
	lastFilter store.ListingsFilter
	lastLimit  int
	lastOffset int
	getItem    model.Listing
	getErr     error
	history    []model.PricePoint
	historyErr error
	changes    []model.PriceChange
	changesErr error
}

func (f *fakeStore) CountListings(ctx context.Context) (int64, error) {
	return f.count, f.countErr
}

func (f *fakeStore) ListListings(ctx context.Context, filter store.ListingsFilter, limit, offset int) ([]model.Listing, int, error) {
	f.lastFilter = filter
	f.lastLimit = limit
	f.lastOffset = offset
	total := f.listTotal
	if total == 0 && len(f.listItems) > 0 {
		total = len(f.listItems)
	}
	return f.listItems, total, f.listErr
}

func (f *fakeStore) GetListing(ctx context.Context, propertyKey string) (model.Listing, error) {
	if f.getErr != nil {
		return model.Listing{}, f.getErr
	}
	return f.getItem, nil
}

func (f *fakeStore) GetPriceHistory(ctx context.Context, propertyKey string) ([]model.PricePoint, error) {
	return f.history, f.historyErr
}

func (f *fakeStore) ListPriceChanges(ctx context.Context, since time.Time) ([]model.PriceChange, error) {
	return f.changes, f.changesErr
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

type countPayload struct {
	Count int64 `json:"count"`
}

type errorPayload struct {
	Error string `json:"error"`
}
