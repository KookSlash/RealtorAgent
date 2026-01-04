package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/extract"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/fetch"
	"github.com/KookSlash/RealtorAgent/services/scraper/internal/strategy"
)

func TestZoloPaginationAndExtractionIntegration(t *testing.T) {
	page1 := readFixture(t, fixturePath(t,
		filepath.Join("..", "testdata", "zolo", "Zolo-page1.html"),
		filepath.Join("..", "testdata", "zolo", "page-1.html"),
	))
	page2 := readFixture(t, fixturePath(t,
		filepath.Join("..", "testdata", "zolo", "zolo-page2.html"),
		filepath.Join("..", "testdata", "zolo", "page-2.html"),
	))

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(rewriteRelNext(page1, serverURL(server.URL, "/page-2")))
	})
	mux.HandleFunc("/page-2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(rewriteRelNext(page2, ""))
	})

	t.Setenv("SCRAPER_STRATEGY", "zolo_ca")
	t.Setenv("ZOLO_ENTRYPOINT_URL", server.URL+"/")
	t.Setenv("ZOLO_BASE_URL", server.URL)
	t.Setenv("USER_AGENT", "TestAgent/1.0")
	t.Setenv("RATE_LIMIT_MS", "0")
	t.Setenv("MAX_PAGES", "5")
	t.Setenv("RAW_BUCKET", "test-bucket")
	t.Setenv("DRY_RUN", "true")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	fetcher, err := fetch.NewZoloHTTPFetcher(cfg, nil)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	pages, err := fetcher.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("fetch pages: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages fetched, got %d", len(pages))
	}

	extractor := extract.NewExtractor(strategy.NewZoloHTMLStrategy(cfg.ZoloBaseURL))
	scrapedAt := time.Date(2025, 2, 3, 10, 11, 12, 0, time.UTC)
	records := 0
	for _, page := range pages {
		recs, _, err := extractor.Extract(context.Background(), page.HTML, page.URL, scrapedAt)
		if err != nil {
			t.Fatalf("extract page: %v", err)
		}
		records += len(recs)
	}

	if records == 0 {
		t.Fatalf("expected records extracted")
	}
}

func rewriteRelNext(html []byte, next string) []byte {
	s := string(html)
	if strings.TrimSpace(next) == "" {
		next = "#next"
	}

	absolute := regexp.MustCompile(`https://www\.zolo\.ca/calgary-real-estate/page-\d+/?`)
	relative := regexp.MustCompile(`/calgary-real-estate/page-\d+/?`)
	s = absolute.ReplaceAllString(s, next)
	s = relative.ReplaceAllString(s, next)

	if next == "#next" {
		s = strings.ReplaceAll(s, `rel="next"`, `rel="nofollow"`)
		s = strings.ReplaceAll(s, `rel='next'`, `rel='nofollow'`)
	}
	return []byte(s)
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

func serverURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}
