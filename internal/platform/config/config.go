package config

import (
	"os"
	"strings"
)

const defaultHTTPAddr = ":8080"

// Config contains process-level configuration for the Oke Gaas server.
type Config struct {
	HTTPAddr    string
	DatabaseURL string
	AdminAPIKey string
}

// Load reads configuration from environment variables and applies safe local defaults.
func Load() Config {
	httpAddr := os.Getenv("OKE_GAAS_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = defaultHTTPAddr
	}

	return Config{
		HTTPAddr:    httpAddr,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		AdminAPIKey: strings.TrimSpace(os.Getenv("OKE_GAAS_ADMIN_API_KEY")),
	}
}
