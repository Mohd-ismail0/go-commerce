package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresDatabaseURLOutsideTestEnv(t *testing.T) {
	t.Setenv("APP_ENV", "development")
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error when DATABASE_URL is missing in non-test environment")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("expected DATABASE_URL required error, got: %v", err)
	}
}

func TestLoadAllowsMissingDatabaseURLInTestEnv(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DATABASE_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("expected no error for test env, got: %v", err)
	}
	if cfg.AppEnv != "test" {
		t.Fatalf("expected APP_ENV=test, got: %s", cfg.AppEnv)
	}
}

func TestLoadRejectsInvalidIntValues(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("WEBHOOK_TIMEOUT_MS", "abc")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error for invalid WEBHOOK_TIMEOUT_MS")
	}
	if !strings.Contains(err.Error(), "WEBHOOK_TIMEOUT_MS must be an integer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsOutOfRangeValues(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("HTTP_MAX_BODY_BYTES", "0")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected error for out-of-range HTTP_MAX_BODY_BYTES")
	}
	if !strings.Contains(err.Error(), "HTTP_MAX_BODY_BYTES must be between") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadParsesLegacyBypassFlag(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("ALLOW_LEGACY_ROLE_BYPASS", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AllowLegacyRoleBypass {
		t.Fatalf("expected AllowLegacyRoleBypass to be true")
	}
}
