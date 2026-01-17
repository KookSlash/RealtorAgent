//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sylvain/realtoragent/services/readapi/internal/config"
	"github.com/sylvain/realtoragent/services/readapi/internal/db"
	storepg "github.com/sylvain/realtoragent/services/readapi/internal/store/postgres"
	"github.com/sylvain/realtoragent/services/readapi/internal/testutil"
)

type integrationEnv struct {
	db     *db.DB
	server *httptest.Server
}

func setupIntegration(t *testing.T) *integrationEnv {
	t.Helper()

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

	store := storepg.New(dbClient)
	handler := NewHandler(store, true, dbClient.Ping)
	server := httptest.NewServer(NewRouter(handler, "*"))

	t.Cleanup(func() {
		server.Close()
		dbClient.Close()
	})

	return &integrationEnv{db: dbClient, server: server}
}

func resetListings(t *testing.T, env *integrationEnv) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := env.db.Exec(ctx, "TRUNCATE price_history, listings RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func insertListing(t *testing.T, env *integrationEnv, propertyKey, propertyType, address, postalCode string, price float64, sqft int, lastSeen time.Time) {
	beds := 3
	sqftVal := sqft
	insertListingRow(t, env, propertyKey, propertyType, address, postalCode, price, &sqftVal, &beds, lastSeen)
}

func insertListingRow(t *testing.T, env *integrationEnv, propertyKey, propertyType, address, postalCode string, price float64, sqft *int, beds *int, lastSeen time.Time) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var sqftArg any
	if sqft != nil {
		sqftArg = *sqft
	}
	var bedsArg any
	if beds != nil {
		bedsArg = *beds
	}

	_, err := env.db.Exec(ctx, `
		INSERT INTO listings (
			property_key, property_type, address, city, province, postal_code,
			beds, baths, sqft, current_price, url, source, first_seen_at, last_seen_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		propertyKey, propertyType, address, "Calgary", "AB", postalCode,
		bedsArg, 2.0, sqftArg, price, "http://example.com/"+propertyKey, "REALTOR_CA",
		lastSeen.Add(-24*time.Hour), lastSeen, lastSeen,
	)
	if err != nil {
		t.Fatalf("insert listing %s: %v", propertyKey, err)
	}
}

func insertHistory(t *testing.T, env *integrationEnv, propertyKey string, observedAt time.Time, price float64, hash string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := env.db.Exec(ctx, `
		INSERT INTO price_history (
			property_key, observed_at, price, beds, baths, sqft, source_listing_id, snapshot_hash
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		propertyKey, observedAt, price, 3, 2.0, 1000, "src-"+propertyKey, hash,
	)
	if err != nil {
		t.Fatalf("insert price history %s: %v", propertyKey, err)
	}
}

func TestListingsBrowseIntegration(t *testing.T) {
	env := setupIntegration(t)
	resetListings(t, env)

	t1 := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 1, 2, 8, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 1, 3, 8, 0, 0, 0, time.UTC)

	insertListing(t, env, "key-1", "HOUSE", "123 Main St", "T2P1A1", 500000, 1000, t3)
	insertListing(t, env, "key-2", "CONDO", "456 Elm St", "T2P2B2", 300000, 600, t2)
	insertListing(t, env, "key-3", "TOWNHOUSE", "789 Pine St", "T2P3C3", 700000, 0, t1)

	resp, err := http.Get(env.server.URL + "/v1/listings?page=1&page_size=2&sort=price_asc")
	if err != nil {
		t.Fatalf("get listings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload listingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode listings: %v", err)
	}
	if payload.Total != 3 {
		t.Fatalf("expected total 3, got %d", payload.Total)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload.Items))
	}
	if payload.Items[0].PropertyKey != "key-2" || payload.Items[1].PropertyKey != "key-1" {
		t.Fatalf("unexpected sort order: %s, %s", payload.Items[0].PropertyKey, payload.Items[1].PropertyKey)
	}
}

func TestPriceHistoryStatsIntegration(t *testing.T) {
	env := setupIntegration(t)
	resetListings(t, env)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insertListing(t, env, "key-1", "HOUSE", "123 Main St", "T2P1A1", 500000, 1000, base)

	insertHistory(t, env, "key-1", base.Add(0), 500000, "hash-1")
	insertHistory(t, env, "key-1", base.Add(9*24*time.Hour), 480000, "hash-2")
	insertHistory(t, env, "key-1", base.Add(19*24*time.Hour), 510000, "hash-3")

	resp, err := http.Get(env.server.URL + "/v1/listings/key-1/price-history")
	if err != nil {
		t.Fatalf("get price history: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload priceHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode price history: %v", err)
	}
	if payload.Stats.NumObservations != 3 {
		t.Fatalf("expected 3 observations, got %d", payload.Stats.NumObservations)
	}
	assertFloatApprox(t, payload.Stats.FirstPrice, 500000)
	assertFloatApprox(t, payload.Stats.LastPrice, 510000)
	assertFloatApprox(t, payload.Stats.AbsChange, 10000)
	assertFloatApprox(t, payload.Stats.PctChange, 2.0)
	assertFloatApprox(t, payload.Stats.MinPrice, 480000)
	assertFloatApprox(t, payload.Stats.MaxPrice, 510000)
	if payload.Stats.NumPriceChanges != 2 {
		t.Fatalf("expected 2 price changes, got %d", payload.Stats.NumPriceChanges)
	}
	if payload.Stats.DaysSinceLastChange != 0 {
		t.Fatalf("expected 0 days since last change, got %d", payload.Stats.DaysSinceLastChange)
	}
	assertFloatApprox(t, payload.Stats.MaxDrawdownPct, 4.0)
}

func TestChangesIntegration(t *testing.T) {
	env := setupIntegration(t)
	resetListings(t, env)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	insertListing(t, env, "key-1", "HOUSE", "123 Main St", "T2P1A1", 500000, 1000, base)

	insertHistory(t, env, "key-1", base.Add(0), 500000, "hash-1")
	insertHistory(t, env, "key-1", base.Add(24*time.Hour), 480000, "hash-2")
	insertHistory(t, env, "key-1", base.Add(48*time.Hour), 510000, "hash-3")

	resp, err := http.Get(env.server.URL + "/v1/changes?since=" + base.Format(time.RFC3339) + "&type=price_change")
	if err != nil {
		t.Fatalf("get changes: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload changesResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode changes: %v", err)
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(payload.Items))
	}
	if payload.Items[0].OldPrice != 500000 || payload.Items[0].NewPrice != 480000 {
		t.Fatalf("unexpected first change: %+v", payload.Items[0])
	}
	if payload.Items[1].OldPrice != 480000 || payload.Items[1].NewPrice != 510000 {
		t.Fatalf("unexpected second change: %+v", payload.Items[1])
	}
}

func TestListingsValueScoreIntegration(t *testing.T) {
	env := setupIntegration(t)
	resetListings(t, env)

	base := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	sqft := 1000

	insertListingRow(t, env, "key-a", "HOUSE", "123 Main St", "T2P1A1", 300000, &sqft, intPtr(3), base.Add(3*time.Hour))
	insertListingRow(t, env, "key-b", "HOUSE", "456 Elm St", "T2P2B2", 400000, &sqft, intPtr(3), base.Add(2*time.Hour))
	insertListingRow(t, env, "key-c", "HOUSE", "789 Pine St", "T2P3C3", 500000, &sqft, intPtr(3), base.Add(1*time.Hour))
	insertListingRow(t, env, "key-d", "HOUSE", "000 Null Sqft", "T2P4D4", 450000, nil, intPtr(3), base.Add(30*time.Minute))
	insertListingRow(t, env, "key-e", "CONDO", "999 Other Group", "T2P9Z9", 350000, &sqft, intPtr(1), base.Add(4*time.Hour))

	resp, err := http.Get(env.server.URL + "/v1/listings?sort=value_desc&page=1&page_size=10")
	if err != nil {
		t.Fatalf("get listings: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var payload listingsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode listings: %v", err)
	}

	indexes := map[string]int{}
	for i, item := range payload.Items {
		indexes[item.PropertyKey] = i
	}
	if !(indexes["key-a"] < indexes["key-b"] && indexes["key-b"] < indexes["key-c"]) {
		t.Fatalf("expected value order key-a, key-b, key-c, got indexes %+v", indexes)
	}
	if payload.Items[len(payload.Items)-1].PropertyKey != "key-d" {
		t.Fatalf("expected key-d last due to NULL value_score, got %s", payload.Items[len(payload.Items)-1].PropertyKey)
	}

	keyA := findListing(payload.Items, "key-a")
	keyB := findListing(payload.Items, "key-b")
	keyC := findListing(payload.Items, "key-c")
	keyD := findListing(payload.Items, "key-d")
	keyE := findListing(payload.Items, "key-e")

	if keyA == nil || keyB == nil || keyC == nil || keyD == nil || keyE == nil {
		t.Fatalf("missing expected listing keys in response")
	}

	if keyA.CompsCount != 3 || keyB.CompsCount != 3 || keyC.CompsCount != 3 || keyD.CompsCount != 3 {
		t.Fatalf("expected comps_count 3 for key-a/b/c/d, got %d/%d/%d/%d", keyA.CompsCount, keyB.CompsCount, keyC.CompsCount, keyD.CompsCount)
	}
	if keyE.CompsCount != 1 {
		t.Fatalf("expected comps_count 1 for key-e, got %d", keyE.CompsCount)
	}

	if keyA.ValueScore == nil || keyB.ValueScore == nil || keyC.ValueScore == nil {
		t.Fatalf("expected non-nil value_score for key-a/b/c")
	}
	assertFloatApprox(t, *keyA.ValueScore, 100)
	assertFloatApprox(t, *keyB.ValueScore, 50)
	assertFloatApprox(t, *keyC.ValueScore, 0)
	if keyD.ValueScore != nil {
		t.Fatalf("expected nil value_score for key-d")
	}
	if keyA.PPSFPercentile == nil || keyB.PPSFPercentile == nil || keyC.PPSFPercentile == nil {
		t.Fatalf("expected non-nil ppsf_percentile for key-a/b/c")
	}
	assertFloatApprox(t, *keyA.PPSFPercentile, 0)
	assertFloatApprox(t, *keyB.PPSFPercentile, 50)
	assertFloatApprox(t, *keyC.PPSFPercentile, 100)
	if keyD.PPSFPercentile != nil {
		t.Fatalf("expected nil ppsf_percentile for key-d")
	}
}

func assertFloatApprox(t *testing.T, got, want float64) {
	t.Helper()
	diff := got - want
	if diff < 0 {
		diff = -diff
	}
	if diff > 0.01 {
		t.Fatalf("expected %.4f, got %.4f", want, got)
	}
}

func findListing(items []listingItem, key string) *listingItem {
	for i := range items {
		if items[i].PropertyKey == key {
			return &items[i]
		}
	}
	return nil
}
