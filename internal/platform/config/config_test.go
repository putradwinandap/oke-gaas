package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadUsesDefaultHTTPAddress(t *testing.T) {
	t.Setenv("OKE_GAAS_HTTP_ADDR", "")
	t.Setenv("DATABASE_URL", "")

	cfg := Load()

	require.Equal(t, ":8080", cfg.HTTPAddr)
	require.Empty(t, cfg.DatabaseURL)
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("OKE_GAAS_HTTP_ADDR", ":9090")
	t.Setenv("DATABASE_URL", "postgres://example")

	cfg := Load()

	require.Equal(t, ":9090", cfg.HTTPAddr)
	require.Equal(t, "postgres://example", cfg.DatabaseURL)
}
