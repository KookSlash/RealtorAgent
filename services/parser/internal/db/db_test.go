package db

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/sylvain/realtoragent/services/parser/internal/model"
)

type preparedStatements struct {
	processedExists  *sqlmock.ExpectedPrepare
	insertProcessed  *sqlmock.ExpectedPrepare
	ensureAttempt    *sqlmock.ExpectedPrepare
	getAttempt       *sqlmock.ExpectedPrepare
	incrementAttempt *sqlmock.ExpectedPrepare
	deleteAttempt    *sqlmock.ExpectedPrepare
	upsertListing    *sqlmock.ExpectedPrepare
	latestSnapshot   *sqlmock.ExpectedPrepare
	insertHistory    *sqlmock.ExpectedPrepare
}

func setupMockDB(t *testing.T, configure func(preparedStatements, sqlmock.Sqlmock)) (*DB, sqlmock.Sqlmock) {
	t.Helper()

	conn, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}

	prep := expectPrepareStatements(mock)
	if configure != nil {
		configure(prep, mock)
	}

	db, err := newWithDB(context.Background(), conn)
	if err != nil {
		t.Fatalf("newWithDB: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("expectations not met: %v", err)
		}
	})

	return db, mock
}

func expectPrepareStatements(mock sqlmock.Sqlmock) preparedStatements {
	return preparedStatements{
		processedExists:  mock.ExpectPrepare(regexp.QuoteMeta("SELECT 1 FROM processed_files")),
		insertProcessed:  mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO processed_files")),
		ensureAttempt:    mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO processing_attempts")),
		getAttempt:       mock.ExpectPrepare(regexp.QuoteMeta("SELECT attempt_count FROM processing_attempts")),
		incrementAttempt: mock.ExpectPrepare(regexp.QuoteMeta("UPDATE processing_attempts")),
		deleteAttempt:    mock.ExpectPrepare(regexp.QuoteMeta("DELETE FROM processing_attempts")),
		upsertListing:    mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO listings")),
		latestSnapshot:   mock.ExpectPrepare(regexp.QuoteMeta("SELECT snapshot_hash FROM price_history")),
		insertHistory:    mock.ExpectPrepare(regexp.QuoteMeta("INSERT INTO price_history")),
	}
}

func TestProcessedExists(t *testing.T) {
	db, _ := setupMockDB(t, func(prep preparedStatements, _ sqlmock.Sqlmock) {
		prep.processedExists.ExpectQuery().
			WithArgs("bucket", "key", "etag").
			WillReturnRows(sqlmock.NewRows([]string{"one"}).AddRow(1))
	})

	exists, err := db.ProcessedExists(context.Background(), "bucket", "key", "etag")
	if err != nil {
		t.Fatalf("ProcessedExists error: %v", err)
	}
	if !exists {
		t.Fatalf("expected processed to be true")
	}
}

func TestProcessedExistsFalse(t *testing.T) {
	db, _ := setupMockDB(t, func(prep preparedStatements, _ sqlmock.Sqlmock) {
		prep.processedExists.ExpectQuery().
			WithArgs("bucket", "key", "etag").
			WillReturnRows(sqlmock.NewRows([]string{"one"}))
	})

	exists, err := db.ProcessedExists(context.Background(), "bucket", "key", "etag")
	if err != nil {
		t.Fatalf("ProcessedExists error: %v", err)
	}
	if exists {
		t.Fatalf("expected processed to be false")
	}
}

func TestEnsureAttempt(t *testing.T) {
	db, _ := setupMockDB(t, func(prep preparedStatements, _ sqlmock.Sqlmock) {
		prep.ensureAttempt.ExpectExec().
			WithArgs("bucket", "key", "etag").
			WillReturnResult(sqlmock.NewResult(1, 1))
		prep.getAttempt.ExpectQuery().
			WithArgs("bucket", "key", "etag").
			WillReturnRows(sqlmock.NewRows([]string{"attempt_count"}).AddRow(1))
	})

	count, err := db.EnsureAttempt(context.Background(), "bucket", "key", "etag")
	if err != nil {
		t.Fatalf("EnsureAttempt error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected attempt_count 1, got %d", count)
	}
}

func TestIncrementAttempt(t *testing.T) {
	db, _ := setupMockDB(t, func(prep preparedStatements, _ sqlmock.Sqlmock) {
		prep.incrementAttempt.ExpectQuery().
			WithArgs("bucket", "key", "etag", "fail").
			WillReturnRows(sqlmock.NewRows([]string{"attempt_count"}).AddRow(2))
	})

	count, err := db.IncrementAttempt(context.Background(), "bucket", "key", "etag", "fail")
	if err != nil {
		t.Fatalf("IncrementAttempt error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected attempt_count 2, got %d", count)
	}
}

func TestInsertProcessed(t *testing.T) {
	db, _ := setupMockDB(t, func(prep preparedStatements, _ sqlmock.Sqlmock) {
		prep.insertProcessed.ExpectExec().
			WithArgs("bucket", "key", "etag", int64(10), "PROCESSED", "vault/raw/key", "vault/normalized/key", "vault/errors/key", nil).
			WillReturnResult(sqlmock.NewResult(1, 1))
	})

	err := db.InsertProcessed(context.Background(), "bucket", "key", "etag", 10, "PROCESSED", "vault/raw/key", "vault/normalized/key", "vault/errors/key", nil)
	if err != nil {
		t.Fatalf("InsertProcessed error: %v", err)
	}
}

func TestUpsertListingAndInsertHistory(t *testing.T) {
	db, _ := setupMockDB(t, func(prep preparedStatements, mock sqlmock.Sqlmock) {
		mock.ExpectBegin()
		prep.upsertListing.ExpectExec().
			WithArgs(
				"prop-key",
				"HOUSE",
				"123 Main St",
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				"T2P1A1",
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		prep.insertHistory.ExpectExec().
			WithArgs(
				"prop-key",
				sqlmock.AnyArg(),
				500000.0,
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				sqlmock.AnyArg(),
				"hash",
			).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	})

	tx, err := db.BeginTx(context.Background())
	if err != nil {
		t.Fatalf("BeginTx error: %v", err)
	}

	record := model.NormalizedRecord{
		PropertyKey:  "prop-key",
		PropertyType: "HOUSE",
		Address:      "123 Main St",
		PostalCode:   "T2P1A1",
		Price:        500000.0,
		SnapshotHash: "hash",
		ScrapedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	scrapedAt, _ := time.Parse(time.RFC3339, record.ScrapedAt)
	if err := db.UpsertListing(context.Background(), tx, record, scrapedAt); err != nil {
		t.Fatalf("UpsertListing error: %v", err)
	}
	if err := db.InsertPriceHistory(context.Background(), tx, record, scrapedAt); err != nil {
		t.Fatalf("InsertPriceHistory error: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit error: %v", err)
	}
}
