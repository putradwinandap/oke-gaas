package main

import (
	"log/slog"
	"os"

	"github.com/putradwinandap/oke-gaas/internal/platform/config"
	httpserver "github.com/putradwinandap/oke-gaas/internal/platform/http"
)

func main() {
	cfg := config.Load()

	app := httpserver.New()

	slog.Info("starting Oke Gaas API", "address", cfg.HTTPAddr)

	if err := app.Listen(cfg.HTTPAddr); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
