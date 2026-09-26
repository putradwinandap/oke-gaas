package main

import (
	"log/slog"
	"os"

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
	if len(cfg.AdminAPIKey) < 32 {
		slog.Error("OKE_GAAS_ADMIN_API_KEY must be at least 32 characters")
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
