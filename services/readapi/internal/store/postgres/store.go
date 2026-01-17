package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/sylvain/realtoragent/services/readapi/internal/db"
	"github.com/sylvain/realtoragent/services/readapi/internal/model"
	"github.com/sylvain/realtoragent/services/readapi/internal/store"
)

type Store struct {
	db *db.DB
}

func New(dbClient *db.DB) *Store {
	return &Store{db: dbClient}
}

func (s *Store) CountListings(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.QueryRow(ctx, "SELECT COUNT(*) FROM listings").Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Store) ListListings(ctx context.Context, filter store.ListingsFilter, limit, offset int) ([]model.Listing, int, error) {
	builder := newListingFilterBuilder(filter)
	whereSQL := builder.whereClause()

	var total int
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM listings %s", whereSQL)
	if err := s.db.QueryRow(ctx, countSQL, builder.args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listArgs := append([]any{}, builder.args...)
	limitPos := len(listArgs) + 1
	listArgs = append(listArgs, limit)
	offsetPos := len(listArgs) + 1
	listArgs = append(listArgs, offset)

	query := fmt.Sprintf(`
		WITH filtered AS (
			SELECT
				property_key,
				property_type,
				address,
				unit,
				city,
				province,
				postal_code,
				lat,
				lon,
				beds,
				baths,
				sqft,
				current_price,
				url,
				source,
				first_seen_at,
				last_seen_at,
				updated_at,
				source_listing_id
			FROM listings
			%s
		),
		enriched AS (
			SELECT
				filtered.*,
				CASE
					WHEN beds IS NULL THEN 'unknown'
					WHEN beds <= 1 THEN '0-1'
					WHEN beds = 2 THEN '2'
					WHEN beds = 3 THEN '3'
					ELSE '4+'
				END AS beds_bucket,
				CASE
					WHEN current_price IS NULL OR sqft IS NULL OR sqft = 0 THEN NULL
					ELSE (current_price::double precision / sqft)
				END AS ppsf
			FROM filtered
		),
		ranked AS (
			SELECT
				property_key,
				percent_rank() OVER (PARTITION BY property_type, beds_bucket ORDER BY ppsf ASC) AS ppsf_percent_rank
			FROM enriched
			WHERE ppsf IS NOT NULL
		),
		scored AS (
			SELECT
				enriched.*,
				ranked.ppsf_percent_rank,
				COUNT(ppsf) OVER (PARTITION BY property_type, beds_bucket)::int AS comps_count,
				CASE
					WHEN ranked.ppsf_percent_rank IS NULL THEN NULL
					ELSE round((ranked.ppsf_percent_rank * 100)::numeric, 2)::double precision
				END AS ppsf_percentile,
				CASE
					WHEN ranked.ppsf_percent_rank IS NULL THEN NULL
					ELSE round(((1 - ranked.ppsf_percent_rank) * 100)::numeric, 2)::double precision
				END AS value_score
			FROM enriched
			LEFT JOIN ranked ON ranked.property_key = enriched.property_key
		)
		SELECT
			property_key,
			property_type::text,
			address,
			unit,
			city,
			province,
			postal_code,
			lat,
			lon,
			beds,
			baths::double precision,
			sqft,
			current_price::double precision,
			ppsf,
			ppsf_percentile,
			value_score,
			comps_count,
			url,
			source,
			first_seen_at,
			last_seen_at,
			updated_at,
			source_listing_id
		FROM scored
		ORDER BY %s
		LIMIT $%d OFFSET $%d`, whereSQL, orderByClause(filter.Sort), limitPos, offsetPos)

	rows, err := s.db.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []model.Listing
	for rows.Next() {
		item, err := scanListing(rows)
		if err != nil {
			return nil, 0, err
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (s *Store) GetListing(ctx context.Context, propertyKey string) (model.Listing, error) {
	query := `
		WITH filtered AS (
			SELECT
				property_key,
				property_type,
				address,
				unit,
				city,
				province,
				postal_code,
				lat,
				lon,
				beds,
				baths,
				sqft,
				current_price,
				url,
				source,
				first_seen_at,
				last_seen_at,
				updated_at,
				source_listing_id
			FROM listings
		),
		enriched AS (
			SELECT
				filtered.*,
				CASE
					WHEN beds IS NULL THEN 'unknown'
					WHEN beds <= 1 THEN '0-1'
					WHEN beds = 2 THEN '2'
					WHEN beds = 3 THEN '3'
					ELSE '4+'
				END AS beds_bucket,
				CASE
					WHEN current_price IS NULL OR sqft IS NULL OR sqft = 0 THEN NULL
					ELSE (current_price::double precision / sqft)
				END AS ppsf
			FROM filtered
		),
		ranked AS (
			SELECT
				property_key,
				percent_rank() OVER (PARTITION BY property_type, beds_bucket ORDER BY ppsf ASC) AS ppsf_percent_rank
			FROM enriched
			WHERE ppsf IS NOT NULL
		),
		scored AS (
			SELECT
				enriched.*,
				ranked.ppsf_percent_rank,
				COUNT(ppsf) OVER (PARTITION BY property_type, beds_bucket)::int AS comps_count,
				CASE
					WHEN ranked.ppsf_percent_rank IS NULL THEN NULL
					ELSE round((ranked.ppsf_percent_rank * 100)::numeric, 2)::double precision
				END AS ppsf_percentile,
				CASE
					WHEN ranked.ppsf_percent_rank IS NULL THEN NULL
					ELSE round(((1 - ranked.ppsf_percent_rank) * 100)::numeric, 2)::double precision
				END AS value_score
			FROM enriched
			LEFT JOIN ranked ON ranked.property_key = enriched.property_key
		)
		SELECT
			property_key,
			property_type::text,
			address,
			unit,
			city,
			province,
			postal_code,
			lat,
			lon,
			beds,
			baths::double precision,
			sqft,
			current_price::double precision,
			ppsf,
			ppsf_percentile,
			value_score,
			comps_count,
			url,
			source,
			first_seen_at,
			last_seen_at,
			updated_at,
			source_listing_id
		FROM scored
		WHERE property_key=$1`

	row := s.db.QueryRow(ctx, query, propertyKey)
	item, err := scanListing(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.Listing{}, store.ErrNotFound
		}
		return model.Listing{}, err
	}
	return item, nil
}

func (s *Store) GetPriceHistory(ctx context.Context, propertyKey string) ([]model.PricePoint, error) {
	rows, err := s.db.Query(ctx, `
		SELECT observed_at, price::double precision
		FROM price_history
		WHERE property_key=$1
		ORDER BY observed_at ASC`, propertyKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var series []model.PricePoint
	for rows.Next() {
		var observedAt time.Time
		var price float64
		if err := rows.Scan(&observedAt, &price); err != nil {
			return nil, err
		}
		series = append(series, model.PricePoint{
			ObservedAt: observedAt,
			Price:      price,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return series, nil
}

func (s *Store) ListPriceChanges(ctx context.Context, since time.Time) ([]model.PriceChange, error) {
	rows, err := s.db.Query(ctx, `
		SELECT property_key, observed_at, price::double precision, lag_price::double precision
		FROM (
			SELECT
				property_key,
				observed_at,
				price,
				LAG(price) OVER (PARTITION BY property_key ORDER BY observed_at) AS lag_price
			FROM price_history
		) history
		WHERE observed_at >= $1
		  AND lag_price IS NOT NULL
		  AND price <> lag_price
		ORDER BY observed_at ASC, property_key ASC`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []model.PriceChange
	for rows.Next() {
		var (
			propertyKey string
			observedAt  time.Time
			newPrice    float64
			oldPrice    float64
		)
		if err := rows.Scan(&propertyKey, &observedAt, &newPrice, &oldPrice); err != nil {
			return nil, err
		}
		changes = append(changes, model.PriceChange{
			PropertyKey: propertyKey,
			ObservedAt:  observedAt,
			OldPrice:    oldPrice,
			NewPrice:    newPrice,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return changes, nil
}

type filterBuilder struct {
	where []string
	args  []any
}

func newListingFilterBuilder(filter store.ListingsFilter) *filterBuilder {
	builder := &filterBuilder{}

	if filter.MinPrice != nil {
		builder.add("current_price >= $%d", *filter.MinPrice)
	}
	if filter.MaxPrice != nil {
		builder.add("current_price <= $%d", *filter.MaxPrice)
	}
	if filter.MinBeds != nil {
		builder.add("beds >= $%d", *filter.MinBeds)
	}
	if filter.MinBaths != nil {
		builder.add("baths >= $%d", *filter.MinBaths)
	}
	if filter.MinSqft != nil {
		builder.add("sqft >= $%d", *filter.MinSqft)
	}
	if filter.MaxSqft != nil {
		builder.add("sqft <= $%d", *filter.MaxSqft)
	}
	if filter.PropertyType != nil {
		builder.add("property_type = $%d::property_type", *filter.PropertyType)
	}
	if filter.PostalPrefix != nil {
		builder.add("postal_code LIKE $%d", *filter.PostalPrefix+"%")
	}
	if filter.City != nil {
		builder.add("city ILIKE $%d", *filter.City)
	}
	if filter.Query != nil {
		builder.add("address ILIKE $%d", "%"+*filter.Query+"%")
	}

	return builder
}

func (b *filterBuilder) add(template string, arg any) {
	b.args = append(b.args, arg)
	b.where = append(b.where, fmt.Sprintf(template, len(b.args)))
}

func (b *filterBuilder) whereClause() string {
	if len(b.where) == 0 {
		return ""
	}
	return "WHERE " + strings.Join(b.where, " AND ")
}

func orderByClause(sort store.ListingSort) string {
	switch sort {
	case store.SortPriceAsc:
		return "current_price ASC NULLS LAST, last_seen_at DESC, property_key ASC"
	case store.SortPriceDesc:
		return "current_price DESC NULLS LAST, last_seen_at DESC, property_key ASC"
	case store.SortPPSFAsc:
		return "CASE WHEN current_price IS NULL OR sqft IS NULL OR sqft=0 THEN 1 ELSE 0 END ASC, (current_price / NULLIF(sqft,0)) ASC, last_seen_at DESC, property_key ASC"
	case store.SortPPSFDesc:
		return "CASE WHEN current_price IS NULL OR sqft IS NULL OR sqft=0 THEN 1 ELSE 0 END ASC, (current_price / NULLIF(sqft,0)) DESC, last_seen_at DESC, property_key ASC"
	case store.SortValueDesc:
		return "value_score DESC NULLS LAST, last_seen_at DESC, property_key ASC"
	default:
		return "last_seen_at DESC, property_key ASC"
	}
}

func scanListing(row pgx.Row) (model.Listing, error) {
	var (
		propertyKey     string
		propertyType    string
		address         string
		unit            pgtype.Text
		city            pgtype.Text
		province        pgtype.Text
		postalCode      string
		lat             pgtype.Float8
		lon             pgtype.Float8
		beds            pgtype.Int4
		baths           pgtype.Float8
		sqft            pgtype.Int4
		currentPrice    pgtype.Float8
		ppsf            pgtype.Float8
		ppsfPercentile  pgtype.Float8
		valueScore      pgtype.Float8
		compsCount      int
		url             pgtype.Text
		source          string
		firstSeenAt     time.Time
		lastSeenAt      time.Time
		updatedAt       time.Time
		sourceListingID pgtype.Text
	)

	if err := row.Scan(
		&propertyKey,
		&propertyType,
		&address,
		&unit,
		&city,
		&province,
		&postalCode,
		&lat,
		&lon,
		&beds,
		&baths,
		&sqft,
		&currentPrice,
		&ppsf,
		&ppsfPercentile,
		&valueScore,
		&compsCount,
		&url,
		&source,
		&firstSeenAt,
		&lastSeenAt,
		&updatedAt,
		&sourceListingID,
	); err != nil {
		return model.Listing{}, err
	}

	return model.Listing{
		PropertyKey:     propertyKey,
		PropertyType:    propertyType,
		Address:         address,
		Unit:            textPtr(unit),
		City:            textPtr(city),
		Province:        textPtr(province),
		PostalCode:      postalCode,
		Lat:             floatPtr(lat),
		Lon:             floatPtr(lon),
		Beds:            intPtr(beds),
		Baths:           floatPtr(baths),
		Sqft:            intPtr(sqft),
		CurrentPrice:    floatPtr(currentPrice),
		PPSF:            floatPtr(ppsf),
		PPSFPercentile:  floatPtr(ppsfPercentile),
		ValueScore:      floatPtr(valueScore),
		CompsCount:      compsCount,
		URL:             textPtr(url),
		Source:          source,
		FirstSeenAt:     firstSeenAt,
		LastSeenAt:      lastSeenAt,
		UpdatedAt:       updatedAt,
		SourceListingID: textPtr(sourceListingID),
	}, nil
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
