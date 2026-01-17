//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/config"
	"github.com/sylvain/realtoragent/services/readapi/internal/db"
	"github.com/sylvain/realtoragent/services/readapi/internal/testutil"
)

func TestListingsRepositoryCountIntegration(t *testing.T) {
	testutil.EnsureIntegrationDBEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if !cfg.DBEnabled {
		t.Fatalf("DB_ENABLED must be true for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbClient, err := db.New(ctx, cfg)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	defer dbClient.Close()

	repo := NewListingsRepository(dbClient)
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count < 0 {
		t.Fatalf("expected count >= 0, got %d", count)
	}
}

func TestListingsRepositoryListIntegration(t *testing.T) {
	testutil.EnsureIntegrationDBEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if !cfg.DBEnabled {
		t.Fatalf("DB_ENABLED must be true for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbClient, err := db.New(ctx, cfg)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	defer dbClient.Close()

	if _, err := dbClient.Exec(ctx, "TRUNCATE price_history, listings RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate listings: %v", err)
	}

	newest := time.Date(2026, 1, 3, 10, 0, 0, 0, time.UTC)
	shared := time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

	insert := `
		INSERT INTO listings (
			property_key, property_type, address, postal_code, current_price, beds, baths, sqft, url,
			first_seen_at, last_seen_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	if _, err := dbClient.Exec(ctx, insert, "key-c", "HOUSE", "789 Pine St", "T2P3C3", 600000.0, 4, 2.0, 1600, "http://example.com/3", newest, newest); err != nil {
		t.Fatalf("insert key-c: %v", err)
	}
	if _, err := dbClient.Exec(ctx, insert, "key-a", "CONDO", "123 Main St", "T2P1A1", 500000.0, 2, 1.5, 900, "http://example.com/1", shared, shared); err != nil {
		t.Fatalf("insert key-a: %v", err)
	}
	if _, err := dbClient.Exec(ctx, insert, "key-b", "TOWNHOUSE", "456 Elm St", "T2P2B2", 550000.0, 3, 2.0, 1200, "http://example.com/2", shared, shared); err != nil {
		t.Fatalf("insert key-b: %v", err)
	}

	repo := NewListingsRepository(dbClient)

	items, err := repo.List(ctx, 2, 0)
	if err != nil {
		t.Fatalf("list query: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].PropertyKey != "key-c" {
		t.Fatalf("expected first item key-c, got %q", items[0].PropertyKey)
	}
	if items[1].PropertyKey != "key-a" {
		t.Fatalf("expected second item key-a, got %q", items[1].PropertyKey)
	}

	items, err = repo.List(ctx, 2, 2)
	if err != nil {
		t.Fatalf("list query offset: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].PropertyKey != "key-b" {
		t.Fatalf("expected item key-b, got %q", items[0].PropertyKey)
	}
}
