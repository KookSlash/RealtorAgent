package model

type NormalizedRecord struct {
	PropertyKey     string   `json:"property_key"`
	PropertyType    string   `json:"property_type"`
	Address         string   `json:"address"`
	PostalCode      string   `json:"postal_code"`
	Unit            *string  `json:"unit,omitempty"`
	Price           float64  `json:"price"`
	Beds            *int     `json:"beds,omitempty"`
	Baths           *float64 `json:"baths,omitempty"`
	Sqft            *int     `json:"sqft,omitempty"`
	Lat             *float64 `json:"lat,omitempty"`
	Lon             *float64 `json:"lon,omitempty"`
	URL             *string  `json:"url,omitempty"`
	ScrapedAt       string   `json:"scraped_at"`
	SourceListingID *string  `json:"source_listing_id,omitempty"`
	SnapshotHash    string   `json:"snapshot_hash"`
}

type ErrorRecord struct {
	LineNumber int    `json:"line_number"`
	Reason     string `json:"reason"`
	RawLine    string `json:"raw_line"`
}
