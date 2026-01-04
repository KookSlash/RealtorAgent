package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRegion          = "us-west-2"
	defaultUserAgent       = "RealtorAgentScraper/0.1"
	defaultRateLimitMs     = 500
	defaultMaxPages        = 20
	defaultReferer         = "https://www.realtor.ca/"
	defaultScraperStrategy = "realtor_ca"
	defaultZoloBaseURL     = "https://www.zolo.ca"
	defaultFetchMode       = "http"
	defaultBrowserTimeout  = 30000
	defaultBrowserWaitMs   = 5000
	sourceIDRealtor        = "realtorca"
	sourceIDZolo           = "zolo"
)

type Config struct {
	AWSRegion           string
	LocalstackEndpoint  string
	RawBucket           string
	ScrapeDate          string
	RunID               string
	SourceID            string
	UserAgent           string
	Referer             string
	ScraperStrategy     string
	SeedCookies         bool
	RetryForbidden      bool
	ScraperSaveHTMLDir  string
	FetchMode           string
	BrowserHeadless     bool
	BrowserTimeoutMs    int
	BrowserWaitMs       int
	BrowserUserAgent    string
	RateLimitMs         int
	MaxPages            int
	DryRun              bool
	ForceUploadEmpty    bool
	SearchEntrypointURL string
	ZoloEntrypointURL   string
	ZoloBaseURL         string
}

func Load() (Config, error) {
	cfg := Config{}
	var err error

	cfg.AWSRegion = getEnvOrDefault("AWS_REGION", defaultRegion)
	cfg.LocalstackEndpoint = strings.TrimSpace(os.Getenv("LOCALSTACK_ENDPOINT"))
	cfg.RawBucket = strings.TrimSpace(os.Getenv("RAW_BUCKET"))
	cfg.UserAgent = getEnvOrDefault("USER_AGENT", defaultUserAgent)
	cfg.SearchEntrypointURL = strings.TrimSpace(os.Getenv("SEARCH_ENTRYPOINT_URL"))
	cfg.ZoloEntrypointURL = strings.TrimSpace(os.Getenv("ZOLO_ENTRYPOINT_URL"))
	cfg.Referer = resolveOptionalStringEnv("REFERER", defaultReferer)
	cfg.ScraperStrategy = resolveScraperStrategy()
	cfg.ZoloBaseURL = getEnvOrDefault("ZOLO_BASE_URL", defaultZoloBaseURL)
	cfg.ScraperSaveHTMLDir = strings.TrimSpace(os.Getenv("SCRAPER_SAVE_HTML_DIR"))
	cfg.FetchMode = resolveFetchMode()
	cfg.BrowserUserAgent = strings.TrimSpace(os.Getenv("BROWSER_USER_AGENT"))

	cfg.DryRun, err = resolveBoolEnv("DRY_RUN", false)
	if err != nil {
		return cfg, err
	}
	if !cfg.DryRun && cfg.RawBucket == "" {
		return cfg, fmt.Errorf("RAW_BUCKET is required")
	}
	if strings.TrimSpace(cfg.UserAgent) == "" {
		return cfg, fmt.Errorf("USER_AGENT must be non-empty")
	}
	if err := validateScraperStrategy(cfg.ScraperStrategy); err != nil {
		return cfg, err
	}
	sourceID, err := sourceIDForStrategy(cfg.ScraperStrategy)
	if err != nil {
		return cfg, err
	}
	cfg.SourceID = sourceID
	if err := validateEntrypoints(cfg); err != nil {
		return cfg, err
	}
	if err := validateFetchMode(cfg.FetchMode); err != nil {
		return cfg, err
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

	cfg.RateLimitMs, err = resolveIntEnv("RATE_LIMIT_MS", defaultRateLimitMs)
	if err != nil {
		return cfg, err
	}
	cfg.MaxPages, err = resolveIntEnv("MAX_PAGES", defaultMaxPages)
	if err != nil {
		return cfg, err
	}
	cfg.SeedCookies, err = resolveBoolEnv("SEED_COOKIES", true)
	if err != nil {
		return cfg, err
	}
	cfg.RetryForbidden, err = resolveBoolEnv("RETRY_FORBIDDEN", false)
	if err != nil {
		return cfg, err
	}
	cfg.BrowserHeadless, err = resolveBoolEnv("BROWSER_HEADLESS", true)
	if err != nil {
		return cfg, err
	}
	cfg.BrowserTimeoutMs, err = resolveIntEnv("BROWSER_TIMEOUT_MS", defaultBrowserTimeout)
	if err != nil {
		return cfg, err
	}
	cfg.BrowserWaitMs, err = resolveIntEnv("BROWSER_WAIT_MS", defaultBrowserWaitMs)
	if err != nil {
		return cfg, err
	}
	cfg.ForceUploadEmpty, err = resolveBoolEnv("FORCE_UPLOAD_EMPTY", false)
	if err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateEntrypoint(value string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("SEARCH_ENTRYPOINT_URL must be a valid URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("SEARCH_ENTRYPOINT_URL must include scheme and host")
	}
	return nil
}

func validateScraperStrategy(value string) error {
	_, err := sourceIDForStrategy(value)
	return err
}

func validateEntrypoints(cfg Config) error {
	if strings.TrimSpace(cfg.SearchEntrypointURL) != "" {
		if err := validateURL(cfg.SearchEntrypointURL, "SEARCH_ENTRYPOINT_URL"); err != nil {
			return err
		}
	}
	switch strings.ToLower(strings.TrimSpace(cfg.ScraperStrategy)) {
	case "zolo_ca":
		if strings.TrimSpace(cfg.ZoloEntrypointURL) == "" {
			return fmt.Errorf("ZOLO_ENTRYPOINT_URL is required for zolo_ca strategy")
		}
		if err := validateURL(cfg.ZoloEntrypointURL, "ZOLO_ENTRYPOINT_URL"); err != nil {
			return err
		}
		if strings.TrimSpace(cfg.ZoloBaseURL) != "" {
			if err := validateURL(cfg.ZoloBaseURL, "ZOLO_BASE_URL"); err != nil {
				return err
			}
		}
	default:
		if cfg.SearchEntrypointURL != "" {
			if err := validateURL(cfg.SearchEntrypointURL, "SEARCH_ENTRYPOINT_URL"); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateURL(value, field string) error {
	parsed, err := url.ParseRequestURI(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", field, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%s must include scheme and host", field)
	}
	return nil
}

func validateFetchMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "http", "browser":
		return nil
	default:
		return fmt.Errorf("FETCH_MODE must be http or browser")
	}
}

func BuildOutputKey(sourceID string, t time.Time) string {
	t = t.UTC()
	return BuildOutputKeyFromParts(sourceID, t.Format("2006-01-02"), t.Format("20060102T150405Z"))
}

func BuildOutputKeyFromParts(sourceID, scrapeDate, runID string) string {
	cleanSourceID := sanitizeSourceID(sourceID)
	cleanDate := strings.TrimSpace(scrapeDate)
	cleanRunID := strings.TrimSpace(runID)
	return fmt.Sprintf("raw/%s/%s/run-%s.jsonl", cleanSourceID, cleanDate, cleanRunID)
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

func resolveBoolEnv(key string, defaultValue bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return defaultValue, nil
	}
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
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

func resolveOptionalStringEnv(key, defaultValue string) string {
	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return strings.TrimSpace(value)
}

func resolveFetchMode() string {
	value := strings.TrimSpace(os.Getenv("FETCH_MODE"))
	if value == "" {
		return defaultFetchMode
	}
	return strings.ToLower(value)
}

func resolveScraperStrategy() string {
	value := strings.TrimSpace(os.Getenv("SCRAPER_STRATEGY"))
	if value == "" {
		return defaultScraperStrategy
	}
	return strings.ToLower(value)
}

func sourceIDForStrategy(strategy string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "realtor_ca":
		return sourceIDRealtor, nil
	case "zolo_ca":
		return sourceIDZolo, nil
	default:
		return "", fmt.Errorf("SCRAPER_STRATEGY must be realtor_ca or zolo_ca")
	}
}

func sanitizeSourceID(sourceID string) string {
	lower := strings.ToLower(strings.TrimSpace(sourceID))
	if lower == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-' || r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('-')
		}
	}
	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return "unknown"
	}
	return cleaned
}
