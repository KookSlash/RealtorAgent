package strategy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/model"
	"golang.org/x/net/html"
)

func TestZoloExtractPage1CountAndFields(t *testing.T) {
	fixture := fixturePath(t,
		filepath.Join("..", "..", "testdata", "zolo", "Zolo-page1.html"),
		filepath.Join("..", "..", "testdata", "zolo", "page-1.html"),
	)
	htmlBytes := readFixture(t, fixture)

	expectedCount := countListings(t, htmlBytes)
	if expectedCount == 0 {
		t.Fatalf("expected at least 1 listing in fixture")
	}

	strategy := NewZoloHTMLStrategy("https://www.zolo.ca", "")
	scrapedAt := time.Date(2025, 2, 3, 10, 11, 12, 0, time.UTC)

	records, matched, err := strategy.TryExtract(context.Background(), htmlBytes, "https://www.zolo.ca/calgary-real-estate", scrapedAt)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if !matched {
		t.Fatalf("expected strategy to match")
	}
	if len(records) != expectedCount {
		t.Fatalf("expected %d records, got %d", expectedCount, len(records))
	}

	if len(records) < 2 {
		t.Fatalf("expected at least 2 records")
	}

	for _, record := range records[:2] {
		if record.URL == "" {
			t.Fatalf("expected url to be set")
		}
		if record.Price <= 0 {
			t.Fatalf("expected price > 0")
		}
		if record.Address == "" {
			t.Fatalf("expected address to be set")
		}
		if record.Source != model.SourceZoloCA {
			t.Fatalf("expected source %s, got %s", model.SourceZoloCA, record.Source)
		}
		if record.ScrapedAt != scrapedAt.Format(time.RFC3339) {
			t.Fatalf("expected scraped_at %s, got %s", scrapedAt.Format(time.RFC3339), record.ScrapedAt)
		}
		var payload map[string]any
		if err := json.Unmarshal(record.SourcePayload, &payload); err != nil {
			t.Fatalf("invalid source_payload json: %v", err)
		}
	}
}

func TestZoloExtractIncludesPropertyTypeHint(t *testing.T) {
	fixture := fixturePath(t,
		filepath.Join("..", "..", "testdata", "zolo", "Zolo-page1.html"),
		filepath.Join("..", "..", "testdata", "zolo", "page-1.html"),
	)
	htmlBytes := readFixture(t, fixture)

	strategy := NewZoloHTMLStrategy("https://www.zolo.ca", "CONDO")
	scrapedAt := time.Date(2025, 2, 3, 10, 11, 12, 0, time.UTC)

	records, matched, err := strategy.TryExtract(context.Background(), htmlBytes, "https://www.zolo.ca/calgary-real-estate", scrapedAt)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if !matched {
		t.Fatalf("expected strategy to match")
	}
	if len(records) == 0 {
		t.Fatalf("expected at least 1 record")
	}
	if records[0].PropertyTypeHint != "CONDO" {
		t.Fatalf("expected property_type_hint CONDO, got %q", records[0].PropertyTypeHint)
	}
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

func fixturePath(t *testing.T, candidates ...string) string {
	t.Helper()
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Fatalf("fixture not found: %v", candidates)
	return ""
}

func countListings(t *testing.T, htmlBytes []byte) int {
	t.Helper()
	normalized := normalizeZoloHTML(htmlBytes)
	doc, err := parseHTML(normalized)
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}
	nodes := findNodes(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "article" && hasClass(n, "card-listing")
	})
	return len(nodes)
}
