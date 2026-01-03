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
		t.Setenv("SEARCH_ENTRYPOINT_URL", "")

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
}
