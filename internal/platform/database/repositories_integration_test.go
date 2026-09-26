package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

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

	require.NoError(t, db.Exec("DROP TABLE IF EXISTS players").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS projects").Error)
	require.NoError(t, AutoMigrateCoreForDevelopment(db))

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
