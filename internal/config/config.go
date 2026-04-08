package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AppEnv               string
	Port                 string
	DatabaseURL          string
	APIAuthToken         string
	DefaultRegionID      string
	DefaultTenantID      string
	WebhookTimeoutMS     int
	WebhookPaymentSecret string
	HTTPTimeoutMS        int
	HTTPMaxBodyBytes     int64
	LogLevel             string
}

func Load() (Config, error) {
	webhookTimeoutMS, err := getEnvIntInRange("WEBHOOK_TIMEOUT_MS", 3000, 1, 120000)
	if err != nil {
		return Config{}, err
	}
	httpTimeoutMS, err := getEnvIntInRange("HTTP_TIMEOUT_MS", 10000, 1, 300000)
	if err != nil {
		return Config{}, err
	}
	httpMaxBodyBytes, err := getEnvIntInRange("HTTP_MAX_BODY_BYTES", 1048576, 1, 104857600)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:               getEnv("APP_ENV", "development"),
		Port:                 getEnv("PORT", "8080"),
		DatabaseURL:          getEnv("DATABASE_URL", ""),
		APIAuthToken:         getEnv("API_AUTH_TOKEN", ""),
		DefaultRegionID:      getEnv("DEFAULT_REGION_ID", "global"),
		DefaultTenantID:      getEnv("DEFAULT_TENANT_ID", "public"),
		WebhookTimeoutMS:     webhookTimeoutMS,
		WebhookPaymentSecret: getEnv("WEBHOOK_PAYMENT_SECRET", ""),
		HTTPTimeoutMS:        httpTimeoutMS,
		HTTPMaxBodyBytes:     int64(httpMaxBodyBytes),
		LogLevel:             getEnv("LOG_LEVEL", "info"),
	}
	if err := validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvIntInRange(key string, fallback, min, max int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	if parsed < min || parsed > max {
		return 0, fmt.Errorf("%s must be between %d and %d", key, min, max)
	}
	return parsed, nil
}

func validate(cfg Config) error {
	appEnv := strings.ToLower(strings.TrimSpace(cfg.AppEnv))
	if appEnv != "test" && appEnv != "testing" && strings.TrimSpace(cfg.DatabaseURL) == "" {
		return fmt.Errorf("DATABASE_URL is required when APP_ENV is %q", cfg.AppEnv)
	}
	return nil
}
