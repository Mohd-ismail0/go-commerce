package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppEnv           string
	Port             string
	DatabaseURL      string
	DefaultRegionID  string
	DefaultTenantID  string
	WebhookTimeoutMS int
	HTTPTimeoutMS    int
	HTTPMaxBodyBytes int64
	LogLevel         string
}

func Load() Config {
	return Config{
		AppEnv:           getEnv("APP_ENV", "development"),
		Port:             getEnv("PORT", "8080"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		DefaultRegionID:  getEnv("DEFAULT_REGION_ID", "global"),
		DefaultTenantID:  getEnv("DEFAULT_TENANT_ID", "public"),
		WebhookTimeoutMS: getEnvInt("WEBHOOK_TIMEOUT_MS", 3000),
		HTTPTimeoutMS:    getEnvInt("HTTP_TIMEOUT_MS", 10000),
		HTTPMaxBodyBytes: int64(getEnvInt("HTTP_MAX_BODY_BYTES", 1048576)),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
