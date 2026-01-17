package store

import (
	"context"
	"errors"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/model"
)

var ErrNotFound = errors.New("not_found")

type ListingSort string

const (
	SortLastSeenDesc ListingSort = "last_seen_desc"
	SortPriceAsc     ListingSort = "price_asc"
	SortPriceDesc    ListingSort = "price_desc"
	SortPPSFAsc      ListingSort = "ppsf_asc"
	SortPPSFDesc     ListingSort = "ppsf_desc"
	SortValueDesc    ListingSort = "value_desc"
)

type ListingsFilter struct {
	MinPrice     *float64
	MaxPrice     *float64
	MinBeds      *int
	MinBaths     *float64
	MinSqft      *int
	MaxSqft      *int
	PropertyType *string
	PostalPrefix *string
	City         *string
	Query        *string
	Sort         ListingSort
}

type Store interface {
	CountListings(ctx context.Context) (int64, error)
	ListListings(ctx context.Context, filter ListingsFilter, limit, offset int) ([]model.Listing, int, error)
	GetListing(ctx context.Context, propertyKey string) (model.Listing, error)
	GetPriceHistory(ctx context.Context, propertyKey string) ([]model.PricePoint, error)
	ListPriceChanges(ctx context.Context, since time.Time) ([]model.PriceChange, error)
}
