package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRegion          = "us-west-2"
	defaultOutputKeyPrefix = "raw/realtorca"
	defaultUserAgent       = "RealtorAgentScraper/0.1"
	defaultRateLimitMs     = 500
	defaultMaxPages        = 20
)

type Config struct {
	AWSRegion          string
	LocalstackEndpoint string
	RawBucket          string
	ScrapeDate         string
	RunID              string
	OutputKeyPrefix    string
	UserAgent          string
	RateLimitMs        int
	MaxPages           int
}

func Load() (Config, error) {
	cfg := Config{}

	cfg.AWSRegion = getEnvOrDefault("AWS_REGION", defaultRegion)
	cfg.LocalstackEndpoint = strings.TrimSpace(os.Getenv("LOCALSTACK_ENDPOINT"))
	cfg.RawBucket = strings.TrimSpace(os.Getenv("RAW_BUCKET"))
	cfg.UserAgent = getEnvOrDefault("USER_AGENT", defaultUserAgent)

	if cfg.RawBucket == "" {
		return cfg, fmt.Errorf("RAW_BUCKET is required")
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		return cfg, fmt.Errorf("USER_AGENT must be non-empty")
	}

	scrapeDate, err := resolveScrapeDate()
	if err != nil {
		return cfg, err
	}
	cfg.ScrapeDate = scrapeDate

	runID := strings.TrimSpace(os.Getenv("RUN_ID"))
	if runID == "" {
		runID = time.Now().UTC().Format("20060102T150405Z")
	}
	cfg.RunID = runID

	prefix, err := resolvePrefix()
	if err != nil {
		return cfg, err
	}
	cfg.OutputKeyPrefix = prefix

	cfg.RateLimitMs, err = resolveIntEnv("RATE_LIMIT_MS", defaultRateLimitMs)
	if err != nil {
		return cfg, err
	}
	cfg.MaxPages, err = resolveIntEnv("MAX_PAGES", defaultMaxPages)
	if err != nil {
		return cfg, err
	}

	return cfg, nil
}

func BuildOutputKey(prefix, scrapeDate, runID string) string {
	cleanPrefix := strings.TrimSuffix(prefix, "/")
	return fmt.Sprintf("%s/%s/run-%s.jsonl", cleanPrefix, scrapeDate, runID)
}

func resolveScrapeDate() (string, error) {
	value, ok := os.LookupEnv("SCRAPE_DATE")
	value = strings.TrimSpace(value)
	if !ok || value == "" {
		return time.Now().UTC().Format("2006-01-02"), nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", fmt.Errorf("SCRAPE_DATE must be YYYY-MM-DD: %w", err)
	}
	return value, nil
}

func resolvePrefix() (string, error) {
	value, ok := os.LookupEnv("OUTPUT_KEY_PREFIX")
	if ok {
		value = strings.TrimSpace(value)
		if value == "" {
			return "", fmt.Errorf("OUTPUT_KEY_PREFIX must not be empty")
		}
	} else {
		value = defaultOutputKeyPrefix
	}

	if strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("OUTPUT_KEY_PREFIX must not start with '/'")
	}
	return value, nil
}

func resolveIntEnv(key string, defaultValue int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	return value
}
