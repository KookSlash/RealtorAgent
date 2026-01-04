package config

import (
	"regexp"
	"testing"
	"time"
)

func TestBuildOutputKey(t *testing.T) {
	key := BuildOutputKey("raw/realtorca", "2025-01-01", "20250101T000000Z")
	if key != "raw/realtorca/2025-01-01/run-20250101T000000Z.jsonl" {
		t.Fatalf("unexpected key: %s", key)
	}

	key = BuildOutputKey("raw/realtorca/", "2025-01-01", "run-1")
	if key != "raw/realtorca/2025-01-01/run-run-1.jsonl" {
		t.Fatalf("unexpected key with trailing slash: %s", key)
	}
}

func TestConfigDefaultsAndValidation(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("AWS_REGION", "")
		t.Setenv("OUTPUT_KEY_PREFIX", defaultOutputKeyPrefix)
		t.Setenv("USER_AGENT", "")
		t.Setenv("RATE_LIMIT_MS", "")
		t.Setenv("MAX_PAGES", "")
		t.Setenv("SCRAPE_DATE", "")
		t.Setenv("RUN_ID", "")
		t.Setenv("DRY_RUN", "")
		t.Setenv("SEED_COOKIES", "")
		t.Setenv("RETRY_FORBIDDEN", "")
		t.Setenv("SEARCH_ENTRYPOINT_URL", "")
		t.Setenv("SCRAPER_STRATEGY", "")
		t.Setenv("ZOLO_ENTRYPOINT_URL", "")
		t.Setenv("ZOLO_BASE_URL", "")
		t.Setenv("FETCH_MODE", "")
		t.Setenv("BROWSER_HEADLESS", "")
		t.Setenv("BROWSER_TIMEOUT_MS", "")
		t.Setenv("BROWSER_WAIT_MS", "")
		t.Setenv("BROWSER_USER_AGENT", "")
		t.Setenv("SCRAPER_SAVE_HTML_DIR", "")
		t.Setenv("FORCE_UPLOAD_EMPTY", "")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.AWSRegion != defaultRegion {
			t.Fatalf("expected default region %s, got %s", defaultRegion, cfg.AWSRegion)
		}
		if cfg.OutputKeyPrefix != defaultOutputKeyPrefix {
			t.Fatalf("expected default prefix %s, got %s", defaultOutputKeyPrefix, cfg.OutputKeyPrefix)
		}
		if cfg.UserAgent == "" {
			t.Fatalf("expected non-empty user agent")
		}
		if cfg.Referer != defaultReferer {
			t.Fatalf("expected default referer %s, got %s", defaultReferer, cfg.Referer)
		}
		if cfg.ScraperStrategy != defaultScraperStrategy {
			t.Fatalf("expected default scraper strategy %s, got %s", defaultScraperStrategy, cfg.ScraperStrategy)
		}
		if cfg.ZoloBaseURL != defaultZoloBaseURL {
			t.Fatalf("expected default zolo base url %s, got %s", defaultZoloBaseURL, cfg.ZoloBaseURL)
		}
		if !cfg.SeedCookies {
			t.Fatalf("expected seed cookies default true")
		}
		if cfg.RetryForbidden {
			t.Fatalf("expected retry forbidden default false")
		}
		if cfg.RateLimitMs != defaultRateLimitMs {
			t.Fatalf("expected default rate limit %d, got %d", defaultRateLimitMs, cfg.RateLimitMs)
		}
		if cfg.MaxPages != defaultMaxPages {
			t.Fatalf("expected default max pages %d, got %d", defaultMaxPages, cfg.MaxPages)
		}
		if _, err := time.Parse("2006-01-02", cfg.ScrapeDate); err != nil {
			t.Fatalf("expected valid scrape date, got %s", cfg.ScrapeDate)
		}
		if !regexp.MustCompile(`^\d{8}T\d{6}Z$`).MatchString(cfg.RunID) {
			t.Fatalf("expected run id format 20060102T150405Z, got %s", cfg.RunID)
		}
		if cfg.DryRun {
			t.Fatalf("expected dry run default false")
		}
		if cfg.FetchMode != defaultFetchMode {
			t.Fatalf("expected default fetch mode %s, got %s", defaultFetchMode, cfg.FetchMode)
		}
		if !cfg.BrowserHeadless {
			t.Fatalf("expected browser headless default true")
		}
		if cfg.BrowserTimeoutMs != defaultBrowserTimeout {
			t.Fatalf("expected browser timeout %d, got %d", defaultBrowserTimeout, cfg.BrowserTimeoutMs)
		}
		if cfg.BrowserWaitMs != defaultBrowserWaitMs {
			t.Fatalf("expected browser wait %d, got %d", defaultBrowserWaitMs, cfg.BrowserWaitMs)
		}
		if cfg.BrowserUserAgent != "" {
			t.Fatalf("expected empty browser user agent, got %q", cfg.BrowserUserAgent)
		}
		if cfg.ForceUploadEmpty {
			t.Fatalf("expected force upload empty default false")
		}
	})

	t.Run("invalid scrape date", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("SCRAPE_DATE", "2025-13-40")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for invalid SCRAPE_DATE")
		}
	})

	t.Run("invalid prefix", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("OUTPUT_KEY_PREFIX", "/raw/realtorca")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for invalid OUTPUT_KEY_PREFIX")
		}
	})

	t.Run("empty prefix", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("OUTPUT_KEY_PREFIX", " ")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for empty OUTPUT_KEY_PREFIX")
		}
	})

	t.Run("dry run true", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("OUTPUT_KEY_PREFIX", defaultOutputKeyPrefix)
		t.Setenv("DRY_RUN", "true")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !cfg.DryRun {
			t.Fatalf("expected dry run true")
		}
	})

	t.Run("invalid entrypoint url", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("SEARCH_ENTRYPOINT_URL", "not-a-url")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for invalid SEARCH_ENTRYPOINT_URL")
		}
	})

	t.Run("zolo entrypoint required", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("SCRAPER_STRATEGY", "zolo_ca")
		t.Setenv("ZOLO_ENTRYPOINT_URL", "")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for missing ZOLO_ENTRYPOINT_URL")
		}
	})

	t.Run("zolo entrypoint valid", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("SCRAPER_STRATEGY", "zolo_ca")
		t.Setenv("ZOLO_ENTRYPOINT_URL", "https://www.zolo.ca/index.php?sarea=Calgary&filter=1")

		_, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid fetch mode", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("FETCH_MODE", "invalid")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for invalid FETCH_MODE")
		}
	})

	t.Run("empty referer allowed", func(t *testing.T) {
		t.Setenv("RAW_BUCKET", "bucket")
		t.Setenv("REFERER", " ")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Referer != "" {
			t.Fatalf("expected empty referer, got %q", cfg.Referer)
		}
	})

	t.Run("dry run skips raw bucket", func(t *testing.T) {
		t.Setenv("DRY_RUN", "true")
		t.Setenv("RAW_BUCKET", "")

		_, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-dry run requires raw bucket", func(t *testing.T) {
		t.Setenv("DRY_RUN", "false")
		t.Setenv("RAW_BUCKET", "")

		_, err := Load()
		if err == nil {
			t.Fatalf("expected error for missing RAW_BUCKET")
		}
	})
}
