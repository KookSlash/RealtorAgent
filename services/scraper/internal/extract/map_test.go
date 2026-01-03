package extract

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMapListingSnapshot(t *testing.T) {
	raw := map[string]any{
		"Id": "L-123",
		"Property": map[string]any{
			"Price": "$550,000",
			"Type":  "House",
			"Address": map[string]any{
				"AddressText": "123 Main St|Calgary, Alberta T2P 1A1",
				"PostalCode":  "T2P 1A1",
				"City":        "Calgary",
				"Province":    "AB",
				"Latitude":    51.1,
				"Longitude":   -114.1,
				"UnitNumber":  "5",
			},
		},
		"Building": map[string]any{
			"Bedrooms":      3,
			"BathroomTotal": 2.0,
			"SizeInterior":  "1500",
		},
		"RelativeDetailsURL": "/listing/123",
	}

	scrapedAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	snapshot, ok := mapListingSnapshot(raw, "https://www.realtor.ca/ab/calgary/real-estate", scrapedAt)
	if !ok {
		t.Fatalf("expected snapshot to map")
	}

	if snapshot.SourceListingID != "L-123" {
		t.Fatalf("expected listing id L-123, got %s", snapshot.SourceListingID)
	}
	if snapshot.Address != "123 Main St" {
		t.Fatalf("expected address parsed from AddressText, got %s", snapshot.Address)
	}
	if snapshot.PostalCode != "T2P 1A1" {
		t.Fatalf("expected postal code, got %s", snapshot.PostalCode)
	}
	if snapshot.Unit != "5" {
		t.Fatalf("expected unit 5, got %s", snapshot.Unit)
	}
	if snapshot.PropertyType != "House" {
		t.Fatalf("expected property type House, got %s", snapshot.PropertyType)
	}
	if snapshot.Price != 550000 {
		t.Fatalf("expected price 550000, got %f", snapshot.Price)
	}
	if snapshot.Beds != 3 {
		t.Fatalf("expected 3 beds, got %d", snapshot.Beds)
	}
	if snapshot.Baths != 2 {
		t.Fatalf("expected 2 baths, got %f", snapshot.Baths)
	}
	if snapshot.Sqft != 1500 {
		t.Fatalf("expected 1500 sqft, got %d", snapshot.Sqft)
	}
	if snapshot.Lat != 51.1 || snapshot.Lon != -114.1 {
		t.Fatalf("expected lat/lon 51.1/-114.1, got %f/%f", snapshot.Lat, snapshot.Lon)
	}
	if snapshot.URL != "https://www.realtor.ca/listing/123" {
		t.Fatalf("expected resolved URL, got %s", snapshot.URL)
	}
	if _, err := time.Parse(time.RFC3339, snapshot.ScrapedAt); err != nil {
		t.Fatalf("scraped_at invalid: %v", err)
	}
	if _, err := json.Marshal(snapshot.SourcePayload); err != nil {
		t.Fatalf("source payload not valid json: %v", err)
	}
}
