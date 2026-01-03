package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	AWSRegion            string
	LocalstackEndpoint   string
	RawBucket            string
	VaultBucket          string
	RawEventsQueue       string
	PostgresDSN          string
	PostgresHost         string
	PostgresPort         string
	PostgresDB           string
	PostgresUser         string
	PostgresPassword     string
	MaxRetries           int
	DryRun               bool
	LogLevel             string
	Once                 bool
	FailOnKey            string
	SQSVisibilityTimeout int32
}

func Load() (Config, error) {
	cfg := Config{}
	cfg.AWSRegion = os.Getenv("AWS_REGION")
	cfg.RawBucket = os.Getenv("RAW_BUCKET")
	cfg.VaultBucket = os.Getenv("VAULT_BUCKET")
	cfg.RawEventsQueue = os.Getenv("RAW_EVENTS_QUEUE")
	cfg.LocalstackEndpoint = os.Getenv("LOCALSTACK_ENDPOINT")
	cfg.PostgresDSN = os.Getenv("POSTGRES_DSN")
	cfg.PostgresHost = os.Getenv("POSTGRES_HOST")
	cfg.PostgresPort = os.Getenv("POSTGRES_PORT")
	cfg.PostgresDB = os.Getenv("POSTGRES_DB")
	cfg.PostgresUser = os.Getenv("POSTGRES_USER")
	cfg.PostgresPassword = os.Getenv("POSTGRES_PASSWORD")
	cfg.LogLevel = os.Getenv("LOG_LEVEL")
	cfg.FailOnKey = os.Getenv("FAIL_ON_KEY")

	if cfg.LocalstackEndpoint == "" {
		if port := os.Getenv("LOCALSTACK_PORT"); port != "" {
			cfg.LocalstackEndpoint = fmt.Sprintf("http://localhost:%s", port)
		}
	}

	cfg.MaxRetries = getIntEnv("MAX_RETRIES", 3)
	cfg.DryRun = getBoolEnv("DRY_RUN", false)
	cfg.Once = getBoolEnv("ONCE", false)
	cfg.SQSVisibilityTimeout = int32(getIntEnv("SQS_VISIBILITY_TIMEOUT", 0))

	if cfg.AWSRegion == "" {
		return cfg, fmt.Errorf("AWS_REGION is required")
	}
	if cfg.RawBucket == "" {
		return cfg, fmt.Errorf("RAW_BUCKET is required")
	}
	if cfg.VaultBucket == "" {
		return cfg, fmt.Errorf("VAULT_BUCKET is required")
	}
	if cfg.RawEventsQueue == "" {
		return cfg, fmt.Errorf("RAW_EVENTS_QUEUE is required")
	}
	if cfg.LocalstackEndpoint == "" {
		return cfg, fmt.Errorf("LOCALSTACK_ENDPOINT is required (or LOCALSTACK_PORT)")
	}

	if cfg.PostgresDSN == "" {
		missing := []string{}
		if cfg.PostgresHost == "" {
			missing = append(missing, "POSTGRES_HOST")
		}
		if cfg.PostgresPort == "" {
			missing = append(missing, "POSTGRES_PORT")
		}
		if cfg.PostgresDB == "" {
			missing = append(missing, "POSTGRES_DB")
		}
		if cfg.PostgresUser == "" {
			missing = append(missing, "POSTGRES_USER")
		}
		if cfg.PostgresPassword == "" {
			missing = append(missing, "POSTGRES_PASSWORD")
		}
		if len(missing) > 0 {
			return cfg, fmt.Errorf("missing required env vars: %v", missing)
		}
		cfg.PostgresDSN = fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable", cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresDB, cfg.PostgresUser, cfg.PostgresPassword)
	}

	if cfg.MaxRetries < 0 {
		return cfg, fmt.Errorf("MAX_RETRIES must be >= 0")
	}

	return cfg, nil
}

func getBoolEnv(key string, defaultValue bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	parsed, err := strconv.ParseBool(val)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getIntEnv(key string, defaultValue int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return parsed
}
