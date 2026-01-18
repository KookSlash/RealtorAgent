package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	SourceRealtorCA = "REALTOR_CA"
	SourceZoloCA    = "ZOLO_CA"
)

type ListingSnapshot struct {
	Source          string          `json:"source"`
	SourceListingID string          `json:"source_listing_id,omitempty"`
	Address         string          `json:"address"`
	PostalCode      string          `json:"postal_code"`
	Unit            string          `json:"unit,omitempty"`
	City            string          `json:"city,omitempty"`
	Province        string          `json:"province,omitempty"`
	PropertyType    string          `json:"property_type"`
	PropertyTypeHint string         `json:"property_type_hint,omitempty"`
	Price           float64         `json:"price"`
	Beds            int             `json:"beds,omitempty"`
	Baths           float64         `json:"baths,omitempty"`
	Sqft            int             `json:"sqft,omitempty"`
	Lat             float64         `json:"lat,omitempty"`
	Lon             float64         `json:"lon,omitempty"`
	URL             string          `json:"url"`
	ScrapedAt       string          `json:"scraped_at"`
	SourcePayload   json.RawMessage `json:"source_payload"`
}

func NewListingSnapshot(scrapedAt time.Time, sourcePayload json.RawMessage) ListingSnapshot {
	return ListingSnapshot{
		Source:        SourceRealtorCA,
		ScrapedAt:     scrapedAt.UTC().Format(time.RFC3339),
		SourcePayload: sourcePayload,
	}
}

func (l *ListingSnapshot) SetScrapedAt(scrapedAt time.Time) {
	l.ScrapedAt = scrapedAt.UTC().Format(time.RFC3339)
}

func (l ListingSnapshot) CanonicalIdentity() string {
	return CanonicalIdentity(l.Address, l.PostalCode)
}

func CanonicalIdentity(address, postalCode string) string {
	normalizedAddress := strings.ToUpper(strings.TrimSpace(address))
	normalizedPostal := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(postalCode), " ", ""))
	return fmt.Sprintf("%s|%s", normalizedAddress, normalizedPostal)
}
