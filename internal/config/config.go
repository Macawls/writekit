package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Port       int
	Host       string
	BaseURL    string
	MCPHost    string
	MCPBaseURL string
	Dev        bool
	Local      bool

	DatabaseURL string
	DataDir     string

	GoogleClientID      string
	GoogleClientSecret  string
	GithubClientID      string
	GithubClientSecret  string
	DiscordClientID     string
	DiscordClientSecret string

	SessionSecret string

	StripeSecretKey     string
	StripeWebhookSecret string
	StripePriceID       string

	SESFrom   string
	SESRegion string

	MaxPoolSize int

	AdminEmails []string
}

func Load() (*Config, error) {
	port, _ := strconv.Atoi(getenv("PORT", "8080"))
	maxPool, _ := strconv.Atoi(getenv("MAX_POOL_SIZE", "50"))

	host := getenv("HOST", "writekit.dev")

	local := getenv("LOCAL", "") == "true"
	dev := getenv("DEV", "") == "true"
	scheme := "https"
	if dev || local {
		scheme = "http"
	}

	baseURL := fmt.Sprintf("%s://%s", scheme, host)
	if dev {
		baseURL = fmt.Sprintf("http://localhost:%d", port)
	}
	if local {
		baseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}

	mcpHost := getenv("MCP_HOST", "mcp."+host)
	mcpBaseURL := fmt.Sprintf("%s://%s", scheme, mcpHost)
	if dev {
		mcpBaseURL = fmt.Sprintf("http://mcp.localhost:%d", port)
	}
	if local {
		mcpBaseURL = baseURL
	}

	dataDirDefault := "./data/tenants"
	if local {
		if ucd, err := os.UserConfigDir(); err == nil {
			dataDirDefault = filepath.Join(ucd, "WriteKit", "tenants")
		}
	}

	cfg := &Config{
		Port:       port,
		Host:       host,
		BaseURL:    baseURL,
		MCPHost:    mcpHost,
		MCPBaseURL: mcpBaseURL,
		Dev:        dev,
		Local:      local,

		DatabaseURL: getenv("DATABASE_URL", ""),
		DataDir:     getenv("DATA_DIR", dataDirDefault),

		GoogleClientID:      getenv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret:  getenv("GOOGLE_CLIENT_SECRET", ""),
		GithubClientID:      getenv("GITHUB_CLIENT_ID", ""),
		GithubClientSecret:  getenv("GITHUB_CLIENT_SECRET", ""),
		DiscordClientID:     getenv("DISCORD_CLIENT_ID", ""),
		DiscordClientSecret: getenv("DISCORD_CLIENT_SECRET", ""),

		SessionSecret: getenv("SESSION_SECRET", ""),

		StripeSecretKey:     getenv("STRIPE_SECRET_KEY", ""),
		StripeWebhookSecret: getenv("STRIPE_WEBHOOK_SECRET", ""),
		StripePriceID:       getenv("STRIPE_PRICE_ID", ""),

		SESFrom:   getenv("SES_FROM", ""),
		SESRegion: getenv("SES_REGION", ""),

		MaxPoolSize: maxPool,
	}

	if !local {
		if cfg.DatabaseURL == "" {
			return nil, fmt.Errorf("DATABASE_URL is required")
		}
		if cfg.SessionSecret == "" {
			return nil, fmt.Errorf("SESSION_SECRET is required")
		}
	}

	if adminRaw := getenv("ADMIN_EMAILS", ""); adminRaw != "" {
		for _, e := range strings.Split(adminRaw, ",") {
			if trimmed := strings.TrimSpace(strings.ToLower(e)); trimmed != "" {
				cfg.AdminEmails = append(cfg.AdminEmails, trimmed)
			}
		}
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
