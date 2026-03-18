package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port     int
	Host     string
	AppHost  string
	BaseURL  string
	Dev      bool

	DatabaseURL string
	DataDir     string

	GoogleClientID     string
	GoogleClientSecret string
	GithubClientID     string
	GithubClientSecret string

	SessionSecret string

	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceID       string

	SESFrom   string
	SESRegion string

	MaxPoolSize int
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getenv("PORT", "8080"))
	maxPool, _ := strconv.Atoi(getenv("MAX_POOL_SIZE", "50"))

	host := getenv("HOST", "writekit.dev")
	appHost := getenv("APP_HOST", "app."+host)

	scheme := "https"
	dev := getenv("DEV", "") == "true"
	if dev {
		scheme = "http"
	}

	cfg := &Config{
		Port:    port,
		Host:    host,
		AppHost: appHost,
		BaseURL: fmt.Sprintf("%s://%s", scheme, appHost),
		Dev:     dev,

		DatabaseURL: getenv("DATABASE_URL", ""),
		DataDir:     getenv("DATA_DIR", "./data/tenants"),

		GoogleClientID:     getenv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getenv("GOOGLE_CLIENT_SECRET", ""),
		GithubClientID:     getenv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret: getenv("GITHUB_CLIENT_SECRET", ""),

		SessionSecret: getenv("SESSION_SECRET", ""),

		StripeSecretKey:     getenv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getenv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceID:       getenv("STRIPE_PRICE_ID", ""),

		SESFrom:   getenv("SES_FROM", ""),
		SESRegion: getenv("SES_REGION", ""),

		MaxPoolSize: maxPool,
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
