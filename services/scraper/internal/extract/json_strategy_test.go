package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/strategy"
	"golang.org/x/net/html"
)

func TestJsonEmbeddedStrategyExtractsListings(t *testing.T) {
	html, err := os.ReadFile("../../testdata/calgary_page1.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	expectedCount := extractSEOResultsCount(t, html)
	if expectedCount == 0 {
		t.Fatalf("expected fixture to contain listings")
	}

	scrapedAt := time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)
	jsonStrategy := strategy.NewJsonEmbeddedStrategy(mapListingSnapshot)
	records, matched, err := jsonStrategy.TryExtract(context.Background(), html, "https://www.realtor.ca/ab/calgary/real-estate", scrapedAt)
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if !matched {
		t.Fatalf("expected json strategy to match")
	}
	if len(records) != expectedCount {
		t.Fatalf("expected %d records, got %d", expectedCount, len(records))
	}
	if len(records) < 3 {
		t.Fatalf("expected at least 3 records for sample assertions")
	}

	expectedScrapedAt := scrapedAt.Format(time.RFC3339)
	for _, record := range records {
		var payload map[string]any
		if err := json.Unmarshal(record.SourcePayload, &payload); err != nil {
			t.Fatalf("invalid source_payload JSON: %v", err)
		}
	}

	postalPaths := [][]string{
		{"PostalCode"},
		{"Property", "Address", "PostalCode"},
		{"Property", "PostalCode"},
	}
	pricePaths := [][]string{
		{"Property", "Price"},
		{"Property", "PriceUnformattedValue"},
		{"Price"},
		{"price"},
	}
	propertyTypePaths := [][]string{
		{"Property", "Type"},
		{"PropertyType"},
		{"property_type"},
	}
	urlPaths := [][]string{
		{"RelativeDetailsURL"},
		{"RelativeDetailsUrl"},
		{"URL"},
		{"Url"},
		{"url"},
	}

	for i := 0; i < 3; i++ {
		record := records[i]
		if strings.TrimSpace(record.Address) == "" {
			t.Fatalf("expected address on record %d", i)
		}
		if strings.TrimSpace(record.URL) == "" {
			t.Fatalf("expected url on record %d", i)
		}
		if record.ScrapedAt != expectedScrapedAt {
			t.Fatalf("expected scraped_at %s, got %s", expectedScrapedAt, record.ScrapedAt)
		}

		payload := decodePayload(t, record.SourcePayload)
		if hasAnyPath(payload, postalPaths...) && strings.TrimSpace(record.PostalCode) == "" {
			t.Fatalf("expected postal_code on record %d", i)
		}
		if hasAnyPath(payload, pricePaths...) && record.Price <= 0 {
			t.Fatalf("expected price on record %d", i)
		}
		if hasAnyPath(payload, propertyTypePaths...) && strings.TrimSpace(record.PropertyType) == "" {
			t.Fatalf("expected property_type on record %d", i)
		}
		if hasAnyPath(payload, urlPaths...) && strings.TrimSpace(record.URL) == "" {
			t.Fatalf("expected url on record %d", i)
		}
	}
}

func TestJsonEmbeddedStrategyDoesNotMatchPage2(t *testing.T) {
	html, err := os.ReadFile("../../testdata/calgary_page2.html")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	jsonStrategy := strategy.NewJsonEmbeddedStrategy(mapListingSnapshot)
	records, matched, err := jsonStrategy.TryExtract(context.Background(), html, "https://www.realtor.ca/ab/calgary/real-estate", time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC))
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected no records, got %d", len(records))
	}
	if matched && len(records) > 0 {
		t.Fatalf("expected empty results when json payload is unusable")
	}
}

func extractSEOResultsCount(t *testing.T, htmlBytes []byte) int {
	t.Helper()

	doc, err := html.Parse(bytes.NewReader(htmlBytes))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	script := findScriptByID(doc, "SEOLandingPageInitialResponse")
	if script == nil {
		t.Fatalf("missing seo json script")
	}

	payload := strings.TrimSpace(scriptText(script))
	if payload == "" {
		t.Fatalf("empty seo json payload")
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("unmarshal seo json: %v", err)
	}
	raw, ok := decoded["Results"].([]any)
	if !ok {
		t.Fatalf("missing Results array in seo payload")
	}
	return len(raw)
}

func findScriptByID(root *html.Node, id string) *html.Node {
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "script" {
			for _, attr := range n.Attr {
				if attr.Key == "id" && attr.Val == id {
					found = n
					return
				}
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return found
}

func scriptText(node *html.Node) string {
	var builder strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			builder.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	return builder.String()
}

func decodePayload(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	return payload
}

func hasAnyPath(raw map[string]any, paths ...[]string) bool {
	for _, path := range paths {
		if value, ok := lookupPath(raw, path...); ok && value != nil {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return true
				}
			default:
				return true
			}
		}
	}
	return false
}

func lookupPath(raw map[string]any, path ...string) (any, bool) {
	current := any(raw)
	for _, key := range path {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, ok := obj[key]
		if !ok {
			return nil, false
		}
		current = value
	}
	return current, true
}
