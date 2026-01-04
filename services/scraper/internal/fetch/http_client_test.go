package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/KookSlash/RealtorAgent/services/scraper/internal/config"
)

func TestFetchAddsBrowserHeaders(t *testing.T) {
	var gotHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html></html>"))
	}))
	defer server.Close()

	cfg := config.Config{
		SearchEntrypointURL: server.URL,
		UserAgent:           "TestAgent/1.0",
		Referer:             "https://www.realtor.ca/",
		RateLimitMs:         0,
		MaxPages:            1,
		SeedCookies:         false,
	}

	fetcher, err := NewRealtorFetcherWithResolver(cfg, nil, func(_ []byte, _ string) string { return "" })
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	_, err = fetcher.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("fetch pages: %v", err)
	}

	assertHeader(t, gotHeaders, "User-Agent", "TestAgent/1.0")
	assertHeader(t, gotHeaders, "Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	assertHeader(t, gotHeaders, "Accept-Language", "en-CA,en;q=0.9")
	assertHeader(t, gotHeaders, "Accept-Encoding", "gzip, deflate, br")
	assertHeader(t, gotHeaders, "Connection", "keep-alive")
	assertHeader(t, gotHeaders, "Upgrade-Insecure-Requests", "1")
	assertHeader(t, gotHeaders, "Sec-Fetch-Dest", "document")
	assertHeader(t, gotHeaders, "Sec-Fetch-Mode", "navigate")
	assertHeader(t, gotHeaders, "Sec-Fetch-Site", "none")
	assertHeader(t, gotHeaders, "Sec-Fetch-User", "?1")
	assertHeader(t, gotHeaders, "Referer", "https://www.realtor.ca/")
}

func TestSeedCookiesAllowsEntryFetch(t *testing.T) {
	var seedHits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/seed":
			seedHits.Add(1)
			http.SetCookie(w, &http.Cookie{Name: "seed", Value: "1", Path: "/"})
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>seed</html>"))
		case "/entry":
			if _, err := r.Cookie("seed"); err != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>entry</html>"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	cfg := config.Config{
		SearchEntrypointURL: server.URL + "/entry",
		UserAgent:           "TestAgent/1.0",
		Referer:             server.URL + "/seed",
		RateLimitMs:         0,
		MaxPages:            1,
		SeedCookies:         true,
	}

	fetcher, err := NewRealtorFetcherWithResolver(cfg, nil, func(_ []byte, _ string) string { return "" })
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}

	pages, err := fetcher.FetchAll(context.Background())
	if err != nil {
		t.Fatalf("fetch pages: %v", err)
	}
	if seedHits.Load() == 0 {
		t.Fatalf("expected seed request to be made")
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestFetchRetriesOnServerError(t *testing.T) {
	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("error"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer server.Close()

	cfg := config.Config{
		SearchEntrypointURL: server.URL,
		UserAgent:           "TestAgent/1.0",
		Referer:             "",
		RateLimitMs:         0,
		MaxPages:            1,
		SeedCookies:         false,
	}

	fetcher, err := NewRealtorFetcherWithResolver(cfg, nil, func(_ []byte, _ string) string { return "" })
	if err != nil {
		t.Fatalf("new fetcher: %v", err)
	}
	fetcher.maxRetries = 2

	result, err := fetcher.fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if hits.Load() < 2 {
		t.Fatalf("expected at least 2 attempts, got %d", hits.Load())
	}
	if len(result.body) == 0 {
		t.Fatalf("expected response body")
	}
}

func assertHeader(t *testing.T, headers http.Header, key, expected string) {
	t.Helper()
	if headers.Get(key) != expected {
		t.Fatalf("expected header %s=%q, got %q", key, expected, headers.Get(key))
	}
}
