package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/sylvain/realtoragent/services/readapi/internal/db"
	"github.com/sylvain/realtoragent/services/readapi/internal/listings"
)

type ListingsRepository struct {
	db *db.DB
}

func NewListingsRepository(db *db.DB) *ListingsRepository {
	return &ListingsRepository{db: db}
}

func (r *ListingsRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, "SELECT COUNT(*) FROM listings").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *ListingsRepository) List(ctx context.Context, limit, offset int) ([]listings.Listing, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			property_key,
			address,
			postal_code,
			current_price::double precision,
			beds,
			baths::double precision,
			sqft,
			url,
			last_seen_at
		FROM listings
		ORDER BY last_seen_at DESC, property_key ASC
		LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []listings.Listing
	for rows.Next() {
		var (
			propertyKey string
			address     string
			postalCode  string
			priceVal    pgtype.Float8
			bedsVal     pgtype.Int4
			bathsVal    pgtype.Float8
			sqftVal     pgtype.Int4
			urlVal      pgtype.Text
			scrapedAt   time.Time
		)
		if err := rows.Scan(
			&propertyKey,
			&address,
			&postalCode,
			&priceVal,
			&bedsVal,
			&bathsVal,
			&sqftVal,
			&urlVal,
			&scrapedAt,
		); err != nil {
			return nil, err
		}

		results = append(results, listings.Listing{
			PropertyKey: propertyKey,
			Address:     address,
			PostalCode:  postalCode,
			Price:       floatPtr(priceVal),
			Beds:        intPtr(bedsVal),
			Baths:       floatPtr(bathsVal),
			Sqft:        intPtr(sqftVal),
			URL:         textPtr(urlVal),
			ScrapedAt:   scrapedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func floatPtr(value pgtype.Float8) *float64 {
	if !value.Valid {
		return nil
	}
	val := value.Float64
	return &val
}

func intPtr(value pgtype.Int4) *int {
	if !value.Valid {
		return nil
	}
	val := int(value.Int32)
	return &val
}

func textPtr(value pgtype.Text) *string {
	if !value.Valid {
		return nil
	}
	val := value.String
	return &val
}
