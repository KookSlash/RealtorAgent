package testutil

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func EnsureIntegrationDBEnv(t *testing.T) {
	t.Helper()

	rawValue, ok := os.LookupEnv("DB_ENABLED")
	if !ok || strings.TrimSpace(rawValue) == "" {
		t.Fatalf("DB_ENABLED must be true for integration tests")
	}
	enabled, err := strconv.ParseBool(strings.TrimSpace(rawValue))
	if err != nil || !enabled {
		t.Fatalf("DB_ENABLED must be true for integration tests")
	}

	setDefaultEnv(t, "POSTGRES_HOST", "127.0.0.1")
	setDefaultEnv(t, "POSTGRES_PORT", "5432")
	setDefaultEnv(t, "POSTGRES_DB", "realestate_it")
	setDefaultEnv(t, "POSTGRES_USER", "realestate")
	setDefaultEnv(t, "POSTGRES_PASSWORD", "realestate")
}

func setDefaultEnv(t *testing.T, key, value string) {
	t.Helper()

	if existing, ok := os.LookupEnv(key); ok && strings.TrimSpace(existing) != "" {
		return
	}
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}
}
