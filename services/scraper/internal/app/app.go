package app

import (
	"encoding/json"
	"os"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/write"
)

func GenerateDummySnapshot(cfg config.Config) (string, string, int, error) {
	outputKey := config.BuildOutputKeyFromParts(cfg.SourceID, cfg.ScrapeDate, cfg.RunID)

	file, err := os.CreateTemp("", "scraper-*.jsonl")
	if err != nil {
		return "", "", 0, err
	}
	defer file.Close()

	writer := write.NewJSONLWriter(file)

	scrapeTime, err := time.Parse("2006-01-02", cfg.ScrapeDate)
	if err != nil {
		scrapeTime = time.Now().UTC()
	} else {
		scrapeTime = scrapeTime.UTC()
	}

	records := []model.ListingSnapshot{
		buildDummyRecord(scrapeTime.Add(10*time.Minute), "L-100", "123 Main St", "T2P 1A1", "101", "Condo", 500000, 2, 1.5, 900, 51.0447, -114.0719, "https://example.com/listing/100"),
		buildDummyRecord(scrapeTime.Add(20*time.Minute), "L-200", "456 Elm St", "T2P 2B2", "", "House", 750000, 3, 2.0, 1400, 51.05, -114.07, "https://example.com/listing/200"),
	}

	for _, record := range records {
		if err := writer.Write(record); err != nil {
			return "", "", 0, err
		}
	}

	return outputKey, file.Name(), len(records), nil
}

func buildDummyRecord(scrapedAt time.Time, listingID, address, postalCode, unit, propertyType string, price float64, beds int, baths float64, sqft int, lat, lon float64, url string) model.ListingSnapshot {
	payload := map[string]any{
		"listing_id":  listingID,
		"address":     address,
		"postal_code": postalCode,
		"unit":        unit,
		"price":       price,
		"property":    propertyType,
	}
	payloadBytes, _ := json.Marshal(payload)

	snapshot := model.NewListingSnapshot(scrapedAt, payloadBytes)
	snapshot.SourceListingID = listingID
	snapshot.Address = address
	snapshot.PostalCode = postalCode
	snapshot.Unit = unit
	snapshot.City = "Calgary"
	snapshot.Province = "AB"
	snapshot.PropertyType = propertyType
	snapshot.Price = price
	snapshot.Beds = beds
	snapshot.Baths = baths
	snapshot.Sqft = sqft
	snapshot.Lat = lat
	snapshot.Lon = lon
	snapshot.URL = url

	return snapshot
}
