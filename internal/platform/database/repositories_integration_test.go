package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	db, err := OpenPostgres(dsn)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Exec("DROP TABLE IF EXISTS events").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS players").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS projects").Error)

	migrationSQL, err := os.ReadFile("../../../migrations/000001_core.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migrationSQL)).Error)

	eventMigrationSQL, err := os.ReadFile("../../../migrations/000002_events.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(eventMigrationSQL)).Error)

	return db
}

func TestProjectRepositoryPersistsAndRetrievesProject(t *testing.T) {
	db := openIntegrationDatabase(t)
	repository := NewProjectRepository(db)

	value, err := project.New("Acme Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, repository.Save(context.Background(), value))

	retrieved, err := repository.GetByID(context.Background(), value.ID())
	require.NoError(t, err)
	require.Equal(t, value.ID(), retrieved.ID())
	require.Equal(t, value.Name(), retrieved.Name())
}

func TestPlayerRepositoryEnforcesProjectScopedIdentityAndIsolation(t *testing.T) {
	db := openIntegrationDatabase(t)
	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	ctx := context.Background()

	projectA, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projectA))

	projectB, err := project.New("Project B", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projectB))

	playerA, err := player.New(projectA.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, playerA))

	playerB, err := player.New(projectB.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, playerB))

	retrieved, err := players.GetByExternalID(ctx, projectA.ID(), "customer-42")
	require.NoError(t, err)
	require.Equal(t, playerA.ID(), retrieved.ID())

	crossProjectRead, err := players.GetByID(ctx, projectB.ID(), playerA.ID())
	require.ErrorIs(t, err, player.ErrNotFound)
	require.Nil(t, crossProjectRead)

	duplicate, err := player.New(projectA.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.ErrorIs(t, players.Save(ctx, duplicate), player.ErrExternalIDTaken)
}

func TestPlayerRepositoryDoesNotMapOtherUniqueConstraintsToExternalIDTaken(t *testing.T) {
	db := openIntegrationDatabase(t)
	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	ctx := context.Background()

	owningProject, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, owningProject))

	existing, err := player.New(owningProject.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, existing))

	samePrimaryKey, err := player.Restore(
		existing.ID(),
		owningProject.ID(),
		"customer-43",
		time.Now(),
	)
	require.NoError(t, err)

	err = players.Save(ctx, samePrimaryKey)
	require.Error(t, err)
	require.False(t, errors.Is(err, player.ErrExternalIDTaken))

	var uniqueErr *UniqueConstraintError
	require.ErrorAs(t, err, &uniqueErr)
	require.Equal(t, "players_pkey", uniqueErr.Constraint)
}

func TestPlayerRepositoryRejectsUnknownProject(t *testing.T) {
	db := openIntegrationDatabase(t)
	players := NewPlayerRepository(db)

	value, err := player.New("proj_missing", "customer-42", time.Now())
	require.NoError(t, err)

	require.Error(t, players.Save(context.Background(), value))
}

func TestEventRepositoryPersistsIdempotentIdentityAndEnforcesPlayerOwnership(t *testing.T) {
	db := openIntegrationDatabase(t)
	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	events := NewEventRepository(db)
	ctx := context.Background()
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)

	projectA, err := project.New("Project A", now)
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projectA))

	projectB, err := project.New("Project B", now)
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projectB))

	playerA, err := player.New(projectA.ID(), "customer-42", now)
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, playerA))

	value, err := eventdomain.New(
		"evt_1",
		projectA.ID(),
		playerA.ID(),
		"lesson_completed",
		now.Add(-time.Hour),
		now,
		map[string]any{"lesson_id": "lesson_5"},
	)
	require.NoError(t, err)
	require.NoError(t, events.Save(ctx, value))

	retrieved, err := events.GetByID(ctx, projectA.ID(), "evt_1")
	require.NoError(t, err)
	require.True(t, retrieved.SameLogicalEvent(value))

	require.ErrorIs(t, events.Save(ctx, value), eventdomain.ErrAlreadyExists)

	crossProjectRead, err := events.GetByID(ctx, projectB.ID(), "evt_1")
	require.ErrorIs(t, err, eventdomain.ErrNotFound)
	require.Nil(t, crossProjectRead)

	invalidOwnership, err := eventdomain.New(
		"evt_2",
		projectB.ID(),
		playerA.ID(),
		"lesson_completed",
		now,
		now,
		nil,
	)
	require.NoError(t, err)
	require.Error(t, events.Save(ctx, invalidOwnership))
}

func TestEventServiceIdempotencySurvivesPostgresRoundTrip(t *testing.T) {
	db := openIntegrationDatabase(t)
	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	events := NewEventRepository(db)
	ctx := context.Background()

	owningProject, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, owningProject))

	target, err := player.New(owningProject.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, target))

	service := eventdomain.NewService(players, events)
	command := eventdomain.IngestCommand{
		ID:         "evt_round_trip",
		ProjectID:  owningProject.ID(),
		PlayerID:   target.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Date(2026, 9, 26, 8, 0, 0, 123456789, time.UTC),
		Properties: map[string]any{
			"score":  100,
			"tags":   []string{"go", "ddd"},
			"nested": map[string]any{"attempt": 2},
		},
	}

	first, err := service.Ingest(ctx, command)
	require.NoError(t, err)
	require.False(t, first.Duplicate)

	second, err := service.Ingest(ctx, command)
	require.NoError(t, err)
	require.True(t, second.Duplicate)
	require.True(t, first.Event.SameLogicalEvent(second.Event))
	require.Equal(t, command.OccurredAt.UTC().Truncate(time.Microsecond), second.Event.OccurredAt())

	var count int64
	require.NoError(t, db.Model(&eventRecord{}).
		Where("project_id = ? AND id = ?", owningProject.ID(), command.ID).
		Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestAutoMigrateDevelopmentEnforcesEventPlayerProjectIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	db, err := OpenPostgres(dsn)
	require.NoError(t, err)

	require.NoError(t, db.Exec("DROP TABLE IF EXISTS events").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS players").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS projects").Error)
	require.NoError(t, AutoMigrateCoreForDevelopment(db))

	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	events := NewEventRepository(db)
	ctx := context.Background()

	projectA, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projectA))

	projectB, err := project.New("Project B", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projectB))

	playerA, err := player.New(projectA.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, playerA))

	invalidOwnership, err := eventdomain.New(
		"evt_automigrate", projectB.ID(), playerA.ID(), "lesson_completed",
		time.Now(), time.Now(), nil,
	)
	require.NoError(t, err)
	require.Error(t, events.Save(ctx, invalidOwnership))

	missingProject, err := eventdomain.New(
		"evt_missing_project",
		"proj_missing",
		playerA.ID(),
		"lesson_completed",
		time.Now(),
		time.Now(),
		nil,
	)
	require.NoError(t, err)
	require.Error(t, events.Save(ctx, missingProject))
}

func TestAutoMigrateDevelopmentIgnoresSameNamedConstraintOnAnotherTable(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	db, err := OpenPostgres(dsn)
	require.NoError(t, err)

	require.NoError(t, db.Exec("DROP TABLE IF EXISTS events").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS players").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS projects").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS shadow_events").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS shadow_projects").Error)

	require.NoError(t, db.Exec(`
		CREATE TABLE shadow_projects (
			id varchar(64) PRIMARY KEY
		)
	`).Error)
	require.NoError(t, db.Exec(`
		CREATE TABLE shadow_events (
			project_id varchar(64) NOT NULL,
			CONSTRAINT fk_events_project
				FOREIGN KEY (project_id)
				REFERENCES shadow_projects(id)
		)
	`).Error)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS shadow_events").Error
		_ = db.Exec("DROP TABLE IF EXISTS shadow_projects").Error
	})

	require.NoError(t, AutoMigrateCoreForDevelopment(db))

	var constraintCount int64
	require.NoError(t, db.Raw(`
		SELECT COUNT(*)
		FROM pg_constraint
		WHERE conname = 'fk_events_project'
		  AND conrelid = 'events'::regclass
		  AND contype = 'f'
	`).Scan(&constraintCount).Error)
	require.EqualValues(t, 1, constraintCount)
}
