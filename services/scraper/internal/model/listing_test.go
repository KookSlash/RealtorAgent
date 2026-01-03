package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestListingSnapshotMarshal(t *testing.T) {
	sourcePayload := json.RawMessage(`{"foo":"bar"}`)
	scrapedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)

	snapshot := NewListingSnapshot(scrapedAt, sourcePayload)
	snapshot.Address = "123 Main St"
	snapshot.PostalCode = "T2P 1A1"
	snapshot.PropertyType = "Condo"
	snapshot.Price = 500000
	snapshot.URL = "https://example.com/listing/1"

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded["source"] != SourceRealtorCA {
		t.Fatalf("expected source %s, got %v", SourceRealtorCA, decoded["source"])
	}

	scrapedAtStr, ok := decoded["scraped_at"].(string)
	if !ok {
		t.Fatalf("scraped_at missing or not a string")
	}
	if _, err := time.Parse(time.RFC3339, scrapedAtStr); err != nil {
		t.Fatalf("scraped_at not RFC3339: %v", err)
	}

	payloadBytes, err := json.Marshal(decoded["source_payload"])
	if err != nil {
		t.Fatalf("marshal source_payload: %v", err)
	}
	if string(payloadBytes) != string(sourcePayload) {
		t.Fatalf("source_payload round-trip mismatch")
	}
}

func TestCanonicalIdentityUsesAddressPostal(t *testing.T) {
	snapshot := ListingSnapshot{
		Address:         "123 Main St",
		PostalCode:      "T2P 1A1",
		SourceListingID: "L-100",
	}

	keyA := snapshot.CanonicalIdentity()
	snapshot.SourceListingID = "L-200"
	keyB := snapshot.CanonicalIdentity()

	if keyA != keyB {
		t.Fatalf("canonical identity should ignore listing id")
	}
	if keyA != "123 MAIN ST|T2P1A1" {
		t.Fatalf("unexpected canonical identity: %s", keyA)
	}
}
