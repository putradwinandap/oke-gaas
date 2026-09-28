package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/putradwinandap/oke-gaas/internal/access"
	"github.com/putradwinandap/oke-gaas/internal/achievement"
	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/counter"
	"github.com/putradwinandap/oke-gaas/internal/level"
	"github.com/putradwinandap/oke-gaas/internal/platform/config"
	"github.com/putradwinandap/oke-gaas/internal/platform/database"
	httpserver "github.com/putradwinandap/oke-gaas/internal/platform/http"
	"github.com/putradwinandap/oke-gaas/internal/platform/telemetry"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/rule"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg := config.Load()
	if len(cfg.AdminAPIKey) < 32 || strings.IndexFunc(cfg.AdminAPIKey, unicode.IsSpace) >= 0 {
		slog.Error("OKE_GAAS_ADMIN_API_KEY must be at least 32 non-whitespace-separated characters")
		return 1
	}

	db, err := database.OpenPostgres(cfg.DatabaseURL)
	if err != nil {
		slog.Error("open database", "error", err)
		return 1
	}
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("open database handle", "error", err)
		return 1
	}
	defer sqlDB.Close()

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelPing()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		slog.Error("ping database", "error", err)
		return 1
	}

	telemetryRuntime, err := telemetry.Configure(context.Background(), "oke-gaas-api")
	if err != nil {
		slog.Error("configure OpenTelemetry", "error", err)
		return 1
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := telemetryRuntime.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown OpenTelemetry", "error", err)
		}
	}()

	projects := database.NewProjectRepository(db)
	players := database.NewPlayerRepository(db)
	rules := database.NewRuleRepository(db)
	levels := database.NewLevelRepository(db)
	states := database.NewPlayerStateRepository(db)
	counters := database.NewCounterRepository(db)
	playerCounters := database.NewPlayerCounterRepository(db)
	badges := database.NewBadgeRepository(db)
	badgeGrants := database.NewBadgeGrantRepository(db)
	achievements := achievement.NewService(projects, players, counters, database.NewAchievementRepository(db), database.NewAchievementUnlockRepository(db))

	app := httpserver.New(httpserver.Dependencies{
		Projects:     project.NewProvisionService(database.NewProjectProvisionTransactor(db)),
		Players:      player.NewService(projects, players),
		Levels:       level.NewService(projects, levels),
		Counters:     counter.NewService(projects, players, counters, playerCounters),
		Badges:       badge.NewService(projects, players, badges, badgeGrants),
		Achievements: achievements,
		Rules:        rule.NewServiceWithBadges(projects, rules, badges),
		Progress:     progression.NewService(database.NewProgressionTransactor(db)),
		States:       states,
		Access:       access.NewService(database.NewProjectAPIKeyRepository(db)),
		AdminKey:     cfg.AdminAPIKey,
	})

	slog.Info("starting Oke Gaas API", "address", cfg.HTTPAddr)
	if err := app.Listen(cfg.HTTPAddr); err != nil {
		slog.Error("server stopped", "error", err)
		return 1
	}
	return 0
}
