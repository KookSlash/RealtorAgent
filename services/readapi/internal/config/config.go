package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	defaultPort            = 8080
	defaultLogLevel        = "info"
	defaultPostgresSSLMode = "disable"
	defaultCORSAllowOrigin = "*"
)

type Config struct {
	Port             int
	LogLevel         string
	CORSAllowOrigin  string
	DBEnabled        bool
	PostgresHost     string
	PostgresPort     int
	PostgresDB       string
	PostgresUser     string
	PostgresPassword string
	PostgresSSLMode  string
}

func Load() (Config, error) {
	cfg := Config{
		Port:            defaultPort,
		LogLevel:        defaultLogLevel,
		CORSAllowOrigin: defaultCORSAllowOrigin,
		PostgresSSLMode: defaultPostgresSSLMode,
	}

	if value := strings.TrimSpace(os.Getenv("PORT")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return cfg, fmt.Errorf("PORT must be an integer")
		}
		if parsed <= 0 || parsed > 65535 {
			return cfg, fmt.Errorf("PORT must be between 1 and 65535")
		}
		cfg.Port = parsed
	}

	if value := strings.TrimSpace(os.Getenv("LOG_LEVEL")); value != "" {
		value = strings.ToLower(value)
		switch value {
		case "debug", "info", "warn", "error":
			cfg.LogLevel = value
		default:
			return cfg, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error")
		}
	}

	if value := strings.TrimSpace(os.Getenv("READAPI_CORS_ALLOW_ORIGIN")); value != "" {
		cfg.CORSAllowOrigin = value
	}

	dbEnabled, err := resolveBoolEnv("DB_ENABLED", false)
	if err != nil {
		return cfg, err
	}
	cfg.DBEnabled = dbEnabled

	if cfg.DBEnabled {
		cfg.PostgresHost = strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
		cfg.PostgresDB = strings.TrimSpace(os.Getenv("POSTGRES_DB"))
		cfg.PostgresUser = strings.TrimSpace(os.Getenv("POSTGRES_USER"))
		cfg.PostgresPassword = strings.TrimSpace(os.Getenv("POSTGRES_PASSWORD"))
		if value := strings.TrimSpace(os.Getenv("POSTGRES_SSLMODE")); value != "" {
			cfg.PostgresSSLMode = value
		}

		missing := []string{}
		if cfg.PostgresHost == "" {
			missing = append(missing, "POSTGRES_HOST")
		}
		portValue := strings.TrimSpace(os.Getenv("POSTGRES_PORT"))
		if portValue == "" {
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

		parsedPort, err := strconv.Atoi(portValue)
		if err != nil || parsedPort <= 0 || parsedPort > 65535 {
			return cfg, fmt.Errorf("POSTGRES_PORT must be a valid port")
		}
		cfg.PostgresPort = parsedPort
	}

	return cfg, nil
}

func (c Config) PostgresDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.PostgresHost,
		c.PostgresPort,
		c.PostgresDB,
		c.PostgresUser,
		c.PostgresPassword,
		c.PostgresSSLMode,
	)
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
