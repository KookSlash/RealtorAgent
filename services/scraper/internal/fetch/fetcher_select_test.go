package fetch

import (
	"testing"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
)

func TestNewFetcherHTTPMode(t *testing.T) {
	cfg := config.Config{
		FetchMode: "http",
		UserAgent: "TestAgent/1.0",
	}

	fetcher, err := NewFetcher(cfg, nil)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	if _, ok := fetcher.(*RealtorFetcher); !ok {
		t.Fatalf("expected http fetcher type, got %T", fetcher)
	}
}

func TestNewFetcherBrowserMode(t *testing.T) {
	cfg := config.Config{
		FetchMode: "browser",
		UserAgent: "TestAgent/1.0",
	}

	fetcher, err := NewFetcher(cfg, nil)
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	if _, ok := fetcher.(*BrowserFetcher); !ok {
		t.Fatalf("expected browser fetcher type, got %T", fetcher)
	}
}
