package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Server
	Port string

	// Postgres
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int

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

func Load() (*Config, error) {
	cfg := &Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           requireEnv("DATABASE_URL"),
		RedisAddr:             getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:         getEnv("REDIS_PASSWORD", ""),
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
