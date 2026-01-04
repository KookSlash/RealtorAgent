package tests

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/app"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
)

func TestScraperProducesParserCompatibleJSONL(t *testing.T) {
	t.Setenv("RAW_BUCKET", "test-bucket")
	t.Setenv("SCRAPE_DATE", "2025-01-01")
	t.Setenv("RUN_ID", "20250101T120000Z")
	t.Setenv("FETCH_MODE", "http")
	t.Setenv("SCRAPER_STRATEGY", "realtor_ca")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	_, path, count, err := app.GenerateDummySnapshot(cfg)
	if err != nil {
		t.Fatalf("generate snapshot: %v", err)
	}
	defer os.Remove(path)

	if count != 2 {
		t.Fatalf("expected 2 records, got %d", count)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer file.Close()

	lines := []string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	for _, line := range lines {
		var decoded map[string]any
		if err := json.Unmarshal([]byte(line), &decoded); err != nil {
			t.Fatalf("invalid json line: %v", err)
		}

		required := []string{"address", "postal_code", "property_type", "price", "url", "scraped_at"}
		for _, field := range required {
			value, ok := decoded[field]
			if !ok {
				t.Fatalf("missing required field %s", field)
			}
			if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
				t.Fatalf("empty required field %s", field)
			}
		}

		payload, ok := decoded["source_payload"]
		if !ok {
			t.Fatalf("missing source_payload")
		}
		if _, err := json.Marshal(payload); err != nil {
			t.Fatalf("invalid source_payload JSON: %v", err)
		}
	}
}
