package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/putradwinandap/oke-gaas/internal/access"
	"github.com/putradwinandap/oke-gaas/internal/platform/config"
	"github.com/putradwinandap/oke-gaas/internal/platform/database"
	httpserver "github.com/putradwinandap/oke-gaas/internal/platform/http"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/rule"
)

func main() {
	cfg := config.Load()
	if len(cfg.AdminAPIKey) < 32 || strings.IndexFunc(cfg.AdminAPIKey, unicode.IsSpace) >= 0 {
		slog.Error("OKE_GAAS_ADMIN_API_KEY must be at least 32 non-whitespace-separated characters")
		os.Exit(1)
	}

	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("open database handle", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		slog.Error("ping database", "error", err)
		os.Exit(1)
	}

	projects := database.NewProjectRepository(db)
	players := database.NewPlayerRepository(db)
	rules := database.NewRuleRepository(db)
	states := database.NewPlayerStateRepository(db)

	app := httpserver.New(httpserver.Dependencies{
		Projects: project.NewProvisionService(database.NewProjectProvisionTransactor(db)),
		Players:  player.NewService(projects, players),
		Rules:    rule.NewService(projects, rules),
		Progress: progression.NewService(database.NewProgressionTransactor(db)),
		States:   states,
		Access:   access.NewService(database.NewProjectAPIKeyRepository(db)),
		AdminKey: cfg.AdminAPIKey,
	})

	slog.Info("starting Oke Gaas API", "address", cfg.HTTPAddr)
	if err := app.Listen(cfg.HTTPAddr); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
