package database

import (
	"context"
	"os"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	ruledomain "github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openProgressionIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	db, err := OpenPostgres(dsn)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, table := range []string{"event_processing", "player_states", "reward_grants", "rules", "events", "players", "projects"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	for _, path := range []string{
		"../../../migrations/000001_core.up.sql",
		"../../../migrations/000002_events.up.sql",
		"../../../migrations/000003_rules_rewards.up.sql",
		"../../../migrations/000004_player_state.up.sql",
		"../../../migrations/000005_event_processing.up.sql",
	} {
		sql, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(sql)).Error)
	}
	return db
}

func seedProgressionFixture(t *testing.T, db *gorm.DB, xpAmount int64) (*project.Project, *player.Player) {
	t.Helper()
	ctx := context.Background()

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	pl, err := player.New(proj.ID(), "player-ext-1", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))

	rule, err := ruledomain.New("rule_lesson", proj.ID(), 1, "lesson_completed", xpAmount)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, rule))

	return proj, pl
}

func TestProgressionTransactionIsIdempotentAndMaterializesXP(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 100)

	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{
		ID:         "evt_1",
		ProjectID:  proj.ID(),
		PlayerID:   pl.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Now().Add(-time.Minute),
	}

	first, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.False(t, first.Duplicate)
	require.Len(t, first.Grants, 1)
	require.Equal(t, int64(100), first.State.XP())

	retry, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.True(t, retry.Duplicate)
	require.Len(t, retry.Grants, 1)
	require.Equal(t, int64(100), retry.State.XP())

	grants, err := NewRewardGrantRepository(db).ListByEvent(ctx, proj.ID(), "evt_1")
	require.NoError(t, err)
	require.Len(t, grants, 1)
}

func TestProgressionTransactionRollsBackGrantStateAndProcessingClaim(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)

	states := NewPlayerStateRepository(db)
	_, err := states.AddXP(ctx, proj.ID(), pl.ID(), int64(^uint64(0)>>1), time.Now())
	require.NoError(t, err)

	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{
		ID:         "evt_overflow",
		ProjectID:  proj.ID(),
		PlayerID:   pl.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Now().Add(-time.Minute),
	}

	_, err = service.Process(ctx, command)
	require.Error(t, err)

	_, err = NewEventRepository(db).GetByID(ctx, proj.ID(), "evt_overflow")
	require.ErrorIs(t, err, eventdomain.ErrNotFound)

	grants, err := NewRewardGrantRepository(db).ListByEvent(ctx, proj.ID(), "evt_overflow")
	require.NoError(t, err)
	require.Empty(t, grants)

	var claimCount int64
	require.NoError(t, db.Table("event_processing").
		Where("project_id = ? AND event_id = ?", proj.ID(), "evt_overflow").
		Count(&claimCount).Error)
	require.Zero(t, claimCount)

	require.NoError(t, db.Model(&playerStateRecord{}).
		Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).
		Update("xp", 0).Error)

	retry, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.False(t, retry.Duplicate)
	require.Len(t, retry.Grants, 1)
	require.Equal(t, int64(1), retry.State.XP())
}

func TestEventProcessingClaimPreventsReprocessingExistingEvent(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 100)

	ev, err := eventdomain.New(
		"evt_legacy",
		proj.ID(),
		pl.ID(),
		"lesson_completed",
		time.Now().Add(-time.Hour),
		time.Now().Add(-time.Minute),
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, NewEventRepository(db).Save(ctx, ev))

	require.NoError(t, db.Exec(
		"INSERT INTO event_processing (project_id, event_id, processed_at) VALUES (?, ?, ?)",
		proj.ID(),
		ev.ID(),
		ev.ReceivedAt(),
	).Error)

	states := NewPlayerStateRepository(db)
	_, err = states.Ensure(ctx, proj.ID(), pl.ID(), ev.ReceivedAt())
	require.NoError(t, err)

	service := progression.NewService(NewProgressionTransactor(db))
	result, err := service.Process(ctx, eventdomain.IngestCommand{
		ID:         ev.ID(),
		ProjectID:  ev.ProjectID(),
		PlayerID:   ev.PlayerID(),
		Type:       ev.Type(),
		OccurredAt: ev.OccurredAt(),
	})
	require.NoError(t, err)
	require.True(t, result.Duplicate)
	require.Empty(t, result.Grants)
	require.Equal(t, int64(0), result.State.XP())
}
