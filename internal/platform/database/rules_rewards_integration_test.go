package database

import (
	"context"
	"os"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	rewarddomain "github.com/putradwinandap/oke-gaas/internal/reward"
	ruledomain "github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openRulesRewardsIntegrationDatabase(t *testing.T) *gorm.DB {
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
	} {
		sql, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(sql)).Error)
	}
	return db
}

func TestRuleRepositoryReturnsOnlyLatestVersionPerRule(t *testing.T) {
	db := openRulesRewardsIntegrationDatabase(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	rules := NewRuleRepository(db)

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, proj))

	v1, err := ruledomain.New("rule_lesson", proj.ID(), 1, "lesson_completed", 100)
	require.NoError(t, err)
	require.NoError(t, rules.Save(ctx, v1))

	v2, err := ruledomain.New("rule_lesson", proj.ID(), 2, "lesson_completed", 50)
	require.NoError(t, err)
	require.NoError(t, rules.Save(ctx, v2))

	current, err := rules.ListByEventType(ctx, proj.ID(), "lesson_completed")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, uint64(2), current[0].Version())
	require.Equal(t, int64(50), current[0].XPAmount())
}

func TestRewardGrantRepositoryPreservesRuleVersionAuditTrail(t *testing.T) {
	db := openRulesRewardsIntegrationDatabase(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	events := NewEventRepository(db)
	rules := NewRuleRepository(db)
	grants := NewRewardGrantRepository(db)

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, proj))

	pl, err := player.New(proj.ID(), "player-ext-1", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, pl))

	ev, err := eventdomain.New(
		"evt_1", proj.ID(), pl.ID(), "lesson_completed",
		time.Now().Add(-time.Minute), time.Now(), nil,
	)
	require.NoError(t, err)
	require.NoError(t, events.Save(ctx, ev))

	ruleV2, err := ruledomain.New("rule_lesson", proj.ID(), 2, "lesson_completed", 50)
	require.NoError(t, err)
	require.NoError(t, rules.Save(ctx, ruleV2))

	grant, err := rewarddomain.NewGrant(
		"grant_1", proj.ID(), pl.ID(), ev.ID(), ruleV2.ID(), ruleV2.Version(), ruleV2.XPAmount(), time.Now(),
	)
	require.NoError(t, err)
	require.NoError(t, grants.Save(ctx, grant))

	history, err := grants.ListByEvent(ctx, proj.ID(), ev.ID())
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, uint64(2), history[0].RuleVersion())
	require.Equal(t, "rule_lesson", history[0].RuleID())
	require.Equal(t, int64(50), history[0].Amount())
}

func TestRewardGrantRepositoryRejectsSameEventRuleAcrossVersions(t *testing.T) {
	db := openRulesRewardsIntegrationDatabase(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)
	events := NewEventRepository(db)
	rules := NewRuleRepository(db)
	grants := NewRewardGrantRepository(db)

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, proj))

	pl, err := player.New(proj.ID(), "player-ext-1", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, pl))

	ev, err := eventdomain.New(
		"evt_1", proj.ID(), pl.ID(), "lesson_completed",
		time.Now().Add(-time.Minute), time.Now(), nil,
	)
	require.NoError(t, err)
	require.NoError(t, events.Save(ctx, ev))

	v1, err := ruledomain.New("rule_lesson", proj.ID(), 1, "lesson_completed", 100)
	require.NoError(t, err)
	require.NoError(t, rules.Save(ctx, v1))

	v2, err := ruledomain.New("rule_lesson", proj.ID(), 2, "lesson_completed", 50)
	require.NoError(t, err)
	require.NoError(t, rules.Save(ctx, v2))

	first, err := rewarddomain.NewGrant(
		"grant_1", proj.ID(), pl.ID(), ev.ID(), v1.ID(), v1.Version(), v1.XPAmount(), time.Now(),
	)
	require.NoError(t, err)
	require.NoError(t, grants.Save(ctx, first))

	reprocessed, err := rewarddomain.NewGrant(
		"grant_2", proj.ID(), pl.ID(), ev.ID(), v2.ID(), v2.Version(), v2.XPAmount(), time.Now(),
	)
	require.NoError(t, err)
	require.ErrorIs(t, grants.Save(ctx, reprocessed), rewarddomain.ErrAlreadyExists)

	history, err := grants.ListByEvent(ctx, proj.ID(), ev.ID())
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, uint64(1), history[0].RuleVersion())
	require.Equal(t, int64(100), history[0].Amount())
}
