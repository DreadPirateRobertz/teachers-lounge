package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for user-service, populated from environment variables.
type Config struct {
	// Server
	Port string

	// Postgres
	DatabaseURL string

	// Redis
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	// RedisTLSEnabled enables TLS for Memorystore (prod). Set REDIS_TLS_ENABLED=true.
	RedisTLSEnabled    bool
	// RedisTLSServerName is the Memorystore host for TLS SNI verification.
	// Typically the host portion of REDIS_ADDR without the port.
	RedisTLSServerName string

	// JWT
	JWTSecret            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration

	// Stripe
	StripeSecretKey     string
	StripeWebhookSecret string

	// Plans — Stripe Price IDs (set in environment per deployment)
	StripePriceMonthly    string
	StripePriceQuarterly  string
	StripePriceSemesterly string

	// Trial
	TrialDays int

	// App
	Env string // development | staging | production
}

// Load reads configuration from environment variables and returns a validated Config.
// It panics if any required variable is absent and returns an error for invalid values.
func Load() (*Config, error) {
	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           requireEnv("DATABASE_URL"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:         getEnv("REDIS_PASSWORD", ""),
		RedisTLSEnabled:       getEnv("REDIS_TLS_ENABLED", "false") == "true",
		RedisTLSServerName:    getEnv("REDIS_TLS_SERVER_NAME", ""),
		JWTSecret:             requireEnv("JWT_SECRET"),
		AccessTokenDuration:   15 * time.Minute,
		RefreshTokenDuration:  30 * 24 * time.Hour,
		StripeSecretKey:       requireEnv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:   requireEnv("STRIPE_WEBHOOK_SECRET"),
		StripePriceMonthly:    requireEnv("STRIPE_PRICE_MONTHLY"),
		StripePriceQuarterly:  requireEnv("STRIPE_PRICE_QUARTERLY"),
		StripePriceSemesterly: requireEnv("STRIPE_PRICE_SEMESTERLY"),
		Env:                   getEnv("ENV", "development"),
	}

	var err error
	cfg.RedisDB, err = strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB: %w", err)
	}
	cfg.TrialDays, err = strconv.Atoi(getEnv("TRIAL_DAYS", "14"))
	if err != nil {
		return nil, fmt.Errorf("invalid TRIAL_DAYS: %w", err)
	}

	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters (got %d)", len(cfg.JWTSecret))
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set", key))
	}
	return v
}
