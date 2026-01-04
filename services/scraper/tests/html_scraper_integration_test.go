package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/extract"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/fetch"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/write"
	"golang.org/x/net/html"
)

func TestHTMLScraperPaginationIntegration(t *testing.T) {
	t.Setenv("RAW_BUCKET", "test-bucket")
	t.Setenv("SCRAPE_DATE", "2025-01-01")
	t.Setenv("RUN_ID", "20250101T000000Z")
	t.Setenv("RATE_LIMIT_MS", "0")
	t.Setenv("MAX_PAGES", "5")
	t.Setenv("USER_AGENT", "TestAgent/1.0")
	t.Setenv("SEED_COOKIES", "false")
	t.Setenv("FETCH_MODE", "http")
	t.Setenv("SCRAPER_STRATEGY", "realtor_ca")

	basePage := readFixture(t, "../testdata/calgary_page1.html")
	page1 := withNextPointer(basePage, "/page2")
	page2 := withNextPointer(withListingIDPrefix(basePage, "page2-"), "/page3")
	page3 := withListingIDPrefix(basePage, "page3-")

	mux := http.NewServeMux()
	mux.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page1)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page2)
	})
	mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page3)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("SEARCH_ENTRYPOINT_URL", server.URL+"/entry")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	scrapedAt := time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)
	result := runScrapeWithFetcher(t, cfg, testNextResolver, scrapedAt)
	defer os.Remove(result.path)

	expectedPerPage := extractSEOResultsCount(t, basePage)
	if result.pagesFetched != 3 {
		t.Fatalf("expected 3 pages fetched, got %d", result.pagesFetched)
	}
	if result.recordsExtracted != expectedPerPage*3 {
		t.Fatalf("expected %d records extracted, got %d", expectedPerPage*3, result.recordsExtracted)
	}

	lines := readJSONLLines(t, result.path)
	if len(lines) != result.recordsExtracted {
		t.Fatalf("expected %d jsonl lines, got %d", result.recordsExtracted, len(lines))
	}
	assertJSONLLines(t, lines, scrapedAt)
}

func TestHTMLScraperDoesNotAbortOnEmptyPage(t *testing.T) {
	t.Setenv("RAW_BUCKET", "test-bucket")
	t.Setenv("SCRAPE_DATE", "2025-01-01")
	t.Setenv("RUN_ID", "20250101T000000Z")
	t.Setenv("RATE_LIMIT_MS", "0")
	t.Setenv("MAX_PAGES", "5")
	t.Setenv("USER_AGENT", "TestAgent/1.0")
	t.Setenv("SEED_COOKIES", "false")
	t.Setenv("FETCH_MODE", "http")
	t.Setenv("SCRAPER_STRATEGY", "realtor_ca")

	basePage := readFixture(t, "../testdata/calgary_page1.html")
	page1 := withNextPointer(basePage, "/page2")
	page2 := withNextPointer(readFixture(t, "../testdata/calgary_page2.html"), "/page3")
	page3 := withListingIDPrefix(basePage, "page3-")

	mux := http.NewServeMux()
	mux.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page1)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page2)
	})
	mux.HandleFunc("/page3", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(page3)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("SEARCH_ENTRYPOINT_URL", server.URL+"/entry")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	scrapedAt := time.Date(2025, 1, 2, 15, 4, 5, 0, time.UTC)
	result := runScrapeWithFetcher(t, cfg, testNextResolver, scrapedAt)
	defer os.Remove(result.path)

	expectedPerPage := extractSEOResultsCount(t, basePage)
	if result.pagesFetched != 3 {
		t.Fatalf("expected 3 pages fetched, got %d", result.pagesFetched)
	}
	if result.recordsExtracted < expectedPerPage*2 {
		t.Fatalf("expected at least %d records, got %d", expectedPerPage*2, result.recordsExtracted)
	}

	lines := readJSONLLines(t, result.path)
	if len(lines) != result.recordsExtracted {
		t.Fatalf("expected %d jsonl lines, got %d", result.recordsExtracted, len(lines))
	}
	if !hasSourceListingPrefix(lines, "page3-") {
		t.Fatalf("expected records from page3 to be present")
	}
	assertJSONLLines(t, lines, scrapedAt)
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return data
}

type scrapeResult struct {
	pagesFetched     int
	recordsExtracted int
	path             string
}

func runScrapeWithFetcher(t *testing.T, cfg config.Config, resolver fetch.NextPageResolver, scrapedAt time.Time) scrapeResult {
	t.Helper()

	fetcher, err := fetch.NewRealtorFetcherWithResolver(cfg, nil, resolver)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	pages, err := fetcher.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("fetch pages: %v", err)
	}

	file, err := os.CreateTemp("", "scraper-integration-*.jsonl")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer file.Close()

	writer := write.NewJSONLWriter(file)
	extractor := extract.NewExtractor()

	count := 0
	for _, page := range pages {
		records, _, err := extractor.ExtractWithPayload(context.Background(), page.HTML, page.CapturedJSON, page.URL, scrapedAt)
		if err != nil {
			continue
		}
		for _, record := range records {
			if err := writer.Write(record); err != nil {
				t.Fatalf("write record: %v", err)
			}
			count++
		}
	}

	return scrapeResult{
		pagesFetched:     len(pages),
		recordsExtracted: count,
		path:             file.Name(),
	}
}

func withNextPointer(htmlBytes []byte, next string) []byte {
	if strings.TrimSpace(next) == "" {
		return htmlBytes
	}
	replacement := fmt.Sprintf("<body data-test-next=\"%s\"", next)
	updated := strings.Replace(string(htmlBytes), "<body", replacement, 1)
	return []byte(updated)
}

func withListingIDPrefix(htmlBytes []byte, prefix string) []byte {
	if strings.TrimSpace(prefix) == "" {
		return htmlBytes
	}
	updated := strings.Replace(string(htmlBytes), `"Id":"`, `"Id":"`+prefix, 1)
	return []byte(updated)
}

func testNextResolver(htmlBytes []byte, baseURL string) string {
	doc, err := html.Parse(strings.NewReader(string(htmlBytes)))
	if err != nil {
		return ""
	}
	next := findAttrValue(doc, "data-test-next")
	if strings.TrimSpace(next) == "" {
		return ""
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	ref, err := url.Parse(next)
	if err != nil {
		return ""
	}
	return base.ResolveReference(ref).String()
}

func findAttrValue(root *html.Node, attr string) string {
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != "" {
			return
		}
		if n.Type == html.ElementNode {
			for _, a := range n.Attr {
				if a.Key == attr {
					found = a.Val
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

func extractSEOResultsCount(t *testing.T, htmlBytes []byte) int {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(string(htmlBytes)))
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
	results, ok := decoded["Results"].([]any)
	if !ok {
		t.Fatalf("missing Results array in seo payload")
	}
	return len(results)
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

func readJSONLLines(t *testing.T, path string) []map[string]any {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []map[string]any
	for scanner.Scan() {
		var decoded map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &decoded); err != nil {
			t.Fatalf("invalid json line: %v", err)
		}
		lines = append(lines, decoded)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan error: %v", err)
	}
	return lines
}

func assertJSONLLines(t *testing.T, lines []map[string]any, scrapedAt time.Time) {
	t.Helper()
	expectedScrapedAt := scrapedAt.Format(time.RFC3339)
	for _, decoded := range lines {
		for _, field := range []string{"address", "postal_code", "property_type", "price", "url", "scraped_at"} {
			value, ok := decoded[field]
			if !ok {
				t.Fatalf("missing required field %s", field)
			}
			if str, ok := value.(string); ok && strings.TrimSpace(str) == "" {
				t.Fatalf("empty required field %s", field)
			}
		}
		if scrapedAtStr, ok := decoded["scraped_at"].(string); ok {
			if scrapedAtStr != expectedScrapedAt {
				t.Fatalf("expected scraped_at %s, got %s", expectedScrapedAt, scrapedAtStr)
			}
		} else {
			t.Fatalf("scraped_at missing or not a string")
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

func hasSourceListingPrefix(lines []map[string]any, prefix string) bool {
	for _, decoded := range lines {
		value, ok := decoded["source_listing_id"].(string)
		if ok && strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}
