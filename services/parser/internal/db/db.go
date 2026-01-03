package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/sylvain/realtoragent/services/parser/internal/model"
)

type DB struct {
	conn                 *sql.DB
	stmtProcessedExists  *sql.Stmt
	stmtInsertProcessed  *sql.Stmt
	stmtEnsureAttempt    *sql.Stmt
	stmtGetAttempt       *sql.Stmt
	stmtIncrementAttempt *sql.Stmt
	stmtDeleteAttempt    *sql.Stmt
	stmtUpsertListing    *sql.Stmt
	stmtLatestSnapshot   *sql.Stmt
	stmtInsertHistory    *sql.Stmt
}

func New(ctx context.Context, dsn string) (*DB, error) {
	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := conn.PingContext(ctx); err != nil {
		return nil, err
	}
	d, err := newWithDB(ctx, conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return d, nil
}

func newWithDB(ctx context.Context, conn *sql.DB) (*DB, error) {
	d := &DB{conn: conn}
	if err := d.prepare(ctx); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) prepare(ctx context.Context) error {
	var err error
	d.stmtProcessedExists, err = d.conn.PrepareContext(ctx, `
		SELECT 1 FROM processed_files
		WHERE s3_bucket=$1 AND s3_key=$2 AND etag=$3
		LIMIT 1`)
	if err != nil {
		return err
	}
	d.stmtInsertProcessed, err = d.conn.PrepareContext(ctx, `
		INSERT INTO processed_files (
			s3_bucket, s3_key, etag, size_bytes, status,
			vault_raw_key, vault_normalized_key, vault_errors_key, error_message
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`)
	if err != nil {
		return err
	}
	d.stmtEnsureAttempt, err = d.conn.PrepareContext(ctx, `
		INSERT INTO processing_attempts (s3_bucket, s3_key, etag, attempt_count, last_attempt_at, updated_at)
		VALUES ($1,$2,$3,0,now(),now())
		ON CONFLICT (s3_bucket, s3_key, etag) DO NOTHING`)
	if err != nil {
		return err
	}
	d.stmtGetAttempt, err = d.conn.PrepareContext(ctx, `
		SELECT attempt_count FROM processing_attempts
		WHERE s3_bucket=$1 AND s3_key=$2 AND etag=$3`)
	if err != nil {
		return err
	}
	d.stmtIncrementAttempt, err = d.conn.PrepareContext(ctx, `
		UPDATE processing_attempts
		SET attempt_count = attempt_count + 1,
			last_error = $4,
			last_attempt_at = now(),
			updated_at = now()
		WHERE s3_bucket=$1 AND s3_key=$2 AND etag=$3
		RETURNING attempt_count`)
	if err != nil {
		return err
	}
	d.stmtDeleteAttempt, err = d.conn.PrepareContext(ctx, `
		DELETE FROM processing_attempts
		WHERE s3_bucket=$1 AND s3_key=$2 AND etag=$3`)
	if err != nil {
		return err
	}
	d.stmtUpsertListing, err = d.conn.PrepareContext(ctx, `
		INSERT INTO listings (
			property_key, property_type, address, unit, city, province, postal_code,
			lat, lon, beds, baths, sqft, current_price, url, source_listing_id,
			first_seen_at, last_seen_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT (property_key) DO UPDATE SET
			property_type=EXCLUDED.property_type,
			address=EXCLUDED.address,
			unit=EXCLUDED.unit,
			city=EXCLUDED.city,
			province=EXCLUDED.province,
			postal_code=EXCLUDED.postal_code,
			lat=EXCLUDED.lat,
			lon=EXCLUDED.lon,
			beds=EXCLUDED.beds,
			baths=EXCLUDED.baths,
			sqft=EXCLUDED.sqft,
			current_price=EXCLUDED.current_price,
			url=EXCLUDED.url,
			source_listing_id=EXCLUDED.source_listing_id,
			last_seen_at=EXCLUDED.last_seen_at,
			updated_at=now(),
			first_seen_at=LEAST(listings.first_seen_at, EXCLUDED.first_seen_at)`)
	if err != nil {
		return err
	}
	d.stmtLatestSnapshot, err = d.conn.PrepareContext(ctx, `
		SELECT snapshot_hash FROM price_history
		WHERE property_key=$1
		ORDER BY observed_at DESC
		LIMIT 1`)
	if err != nil {
		return err
	}
	d.stmtInsertHistory, err = d.conn.PrepareContext(ctx, `
		INSERT INTO price_history (
			property_key, observed_at, price, beds, baths, sqft, source_listing_id, snapshot_hash
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}

	return nil
}

func (d *DB) ProcessedExists(ctx context.Context, bucket, key, etag string) (bool, error) {
	row := d.stmtProcessedExists.QueryRowContext(ctx, bucket, key, etag)
	var one int
	if err := row.Scan(&one); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DB) EnsureAttempt(ctx context.Context, bucket, key, etag string) (int, error) {
	if _, err := d.stmtEnsureAttempt.ExecContext(ctx, bucket, key, etag); err != nil {
		return 0, err
	}
	row := d.stmtGetAttempt.QueryRowContext(ctx, bucket, key, etag)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (d *DB) IncrementAttempt(ctx context.Context, bucket, key, etag, lastError string) (int, error) {
	row := d.stmtIncrementAttempt.QueryRowContext(ctx, bucket, key, etag, lastError)
	var count int
	if err := row.Scan(&count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return d.createAttemptOnFailure(ctx, bucket, key, etag, lastError)
		}
		return 0, err
	}
	return count, nil
}

func (d *DB) createAttemptOnFailure(ctx context.Context, bucket, key, etag, lastError string) (int, error) {
	_, err := d.conn.ExecContext(ctx, `
		INSERT INTO processing_attempts (s3_bucket, s3_key, etag, attempt_count, last_error, last_attempt_at, updated_at)
		VALUES ($1,$2,$3,1,$4,now(),now())
		ON CONFLICT (s3_bucket, s3_key, etag) DO UPDATE SET
			attempt_count = processing_attempts.attempt_count + 1,
			last_error = EXCLUDED.last_error,
			last_attempt_at = now(),
			updated_at = now()`, bucket, key, etag, lastError)
	if err != nil {
		return 0, err
	}
	return 1, nil
}

func (d *DB) DeleteAttempt(ctx context.Context, bucket, key, etag string) error {
	_, err := d.stmtDeleteAttempt.ExecContext(ctx, bucket, key, etag)
	return err
}

func (d *DB) InsertProcessed(ctx context.Context, bucket, key, etag string, size int64, status string, vaultRaw, vaultNormalized, vaultErrors string, errorMessage *string) error {
	_, err := d.stmtInsertProcessed.ExecContext(ctx, bucket, key, etag, size, status, vaultRaw, vaultNormalized, vaultErrors, errorMessage)
	return err
}

func (d *DB) UpsertListing(ctx context.Context, tx *sql.Tx, record model.NormalizedRecord, scrapedAt time.Time) error {
	stmt := tx.StmtContext(ctx, d.stmtUpsertListing)
	_, err := stmt.ExecContext(
		ctx,
		record.PropertyKey,
		record.PropertyType,
		record.Address,
		toNullString(record.Unit),
		toNullString(nil),
		toNullString(nil),
		record.PostalCode,
		toNullFloat(record.Lat),
		toNullFloat(record.Lon),
		toNullInt(record.Beds),
		toNullFloat(record.Baths),
		toNullInt(record.Sqft),
		record.Price,
		toNullString(record.URL),
		toNullString(record.SourceListingID),
		scrapedAt,
		scrapedAt,
	)
	return err
}

func (d *DB) LatestSnapshotHash(ctx context.Context, tx *sql.Tx, propertyKey string) (string, bool, error) {
	stmt := tx.StmtContext(ctx, d.stmtLatestSnapshot)
	row := stmt.QueryRowContext(ctx, propertyKey)
	var hash string
	if err := row.Scan(&hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return hash, true, nil
}

func (d *DB) InsertPriceHistory(ctx context.Context, tx *sql.Tx, record model.NormalizedRecord, scrapedAt time.Time) error {
	stmt := tx.StmtContext(ctx, d.stmtInsertHistory)
	_, err := stmt.ExecContext(
		ctx,
		record.PropertyKey,
		scrapedAt,
		record.Price,
		toNullInt(record.Beds),
		toNullFloat(record.Baths),
		toNullInt(record.Sqft),
		toNullString(record.SourceListingID),
		record.SnapshotHash,
	)
	return err
}

func (d *DB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return d.conn.BeginTx(ctx, nil)
}

func toNullString(val *string) sql.NullString {
	if val == nil || *val == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *val, Valid: true}
}

func toNullInt(val *int) sql.NullInt64 {
	if val == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*val), Valid: true}
}

func toNullFloat(val *float64) sql.NullFloat64 {
	if val == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *val, Valid: true}
}
