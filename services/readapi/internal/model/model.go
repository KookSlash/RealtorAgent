package model

import "time"

type Listing struct {
	PropertyKey     string
	PropertyType    string
	Address         string
	Unit            *string
	City            *string
	Province        *string
	PostalCode      string
	Lat             *float64
	Lon             *float64
	Beds            *int
	Baths           *float64
	Sqft            *int
	CurrentPrice    *float64
	PPSF            *float64
	PPSFPercentile  *float64
	ValueScore      *float64
	CompsCount      int
	URL             *string
	Source          string
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	UpdatedAt       time.Time
	SourceListingID *string
}

type PricePoint struct {
	ObservedAt time.Time
	Price      float64
}

type PriceChange struct {
	PropertyKey string
	ObservedAt  time.Time
	OldPrice    float64
	NewPrice    float64
}
