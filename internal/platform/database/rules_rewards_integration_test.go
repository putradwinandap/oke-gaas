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

	require.NoError(t, db.Exec("DROP FUNCTION IF EXISTS require_counter_aware_event_processing() CASCADE").Error)
	require.NoError(t, db.Exec("DROP FUNCTION IF EXISTS require_badge_aware_event_processing() CASCADE").Error)
	require.NoError(t, db.Exec("DROP FUNCTION IF EXISTS require_streak_aware_event_processing() CASCADE").Error)
	for _, table := range []string{"project_api_keys", "streak_days", "streak_event_claims", "streak_definitions", "event_processing", "badge_definitions", "rule_daily_claims", "rule_match_counts", "player_states", "reward_grants", "rules", "events", "players", "projects"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	for _, path := range []string{
		"../../../migrations/000001_core.up.sql",
		"../../../migrations/000002_events.up.sql",
		"../../../migrations/000003_rules_rewards.up.sql",
		"../../../migrations/000005_event_processing.up.sql",
		"../../../migrations/000007_rule_conditions.up.sql",
		"../../../migrations/000008_rule_match_counts.up.sql",
		"../../../migrations/000009_rule_daily_claims.up.sql",
		"../../../migrations/000013_badge_rewards.up.sql",
		"../../../migrations/000014_streaks.up.sql",
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

func TestRuleRepositoryRoundTripsPropertyConditions(t *testing.T) {
	db := openRulesRewardsIntegrationDatabase(t)
	ctx := context.Background()
	projects := NewProjectRepository(db)
	rules := NewRuleRepository(db)

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, proj))

	value, err := ruledomain.NewAggregate(
		"rule_course",
		proj.ID(),
		1,
		"lesson_completed",
		75,
		map[string]any{
			"course_id":  "course_7",
			"difficulty": 1,
			"metadata":   map[string]any{"required": true},
		},
		4,
	)
	require.NoError(t, err)
	require.NoError(t, rules.Save(ctx, value))

	current, err := rules.ListByEventType(ctx, proj.ID(), "lesson_completed")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.Equal(t, value.ID(), current[0].ID())
	require.Equal(t, value.Conditions(), current[0].Conditions())
	require.Equal(t, uint64(4), current[0].MatchEvery())

	matching, err := eventdomain.New(
		"evt_match", proj.ID(), "player_1", "lesson_completed",
		time.Now(), time.Now(),
		map[string]any{
			"course_id":  "course_7",
			"difficulty": 1.0,
			"metadata":   map[string]any{"required": true},
		},
	)
	require.NoError(t, err)
	require.True(t, current[0].Matches(matching))
}

func TestAggregateMigrationRollbackFailsClosedWhenAggregateRulesExist(t *testing.T) {
	db := openRulesRewardsIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Rollback Safety", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	aggregate, err := ruledomain.NewAggregate(
		"rule_rollback_guard",
		proj.ID(),
		1,
		"lesson_completed",
		100,
		nil,
		2,
	)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, aggregate))

	dailyDownSQL, err := os.ReadFile("../../../migrations/000009_rule_daily_claims.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(dailyDownSQL)).Error)

	downSQL, err := os.ReadFile("../../../migrations/000008_rule_match_counts.down.sql")
	require.NoError(t, err)

	err = db.Exec(string(downSQL)).Error
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot roll back migration 000008 while aggregate rules exist")

	var persistedMatchEvery int64
	require.NoError(t, db.Raw(
		"SELECT match_every FROM rules WHERE project_id = ? AND id = ? AND version = ?",
		proj.ID(),
		aggregate.ID(),
		aggregate.Version(),
	).Scan(&persistedMatchEvery).Error)
	require.Equal(t, int64(2), persistedMatchEvery)

	require.NoError(t, db.Exec(
		"DELETE FROM rules WHERE project_id = ? AND id = ? AND version = ?",
		proj.ID(),
		aggregate.ID(),
		aggregate.Version(),
	).Error)
	require.NoError(t, db.Exec(string(downSQL)).Error)

	var matchEveryColumnExists bool
	require.NoError(t, db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'rules'
			  AND column_name = 'match_every'
		)
	`).Scan(&matchEveryColumnExists).Error)
	require.False(t, matchEveryColumnExists)

	var matchCountTableExists bool
	require.NoError(t, db.Raw(
		"SELECT to_regclass(current_schema() || '.rule_match_counts') IS NOT NULL",
	).Scan(&matchCountTableExists).Error)
	require.False(t, matchCountTableExists)
}

func TestDailyRuleMigrationRollbackFailsClosedWhenDailyRulesExist(t *testing.T) {
	db := openRulesRewardsIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Daily Rollback Safety", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	daily, err := ruledomain.NewTimed(
		"rule_daily_rollback_guard",
		proj.ID(),
		1,
		"daily_login",
		25,
		nil,
		1,
		true,
	)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, daily))

	downSQL, err := os.ReadFile("../../../migrations/000009_rule_daily_claims.down.sql")
	require.NoError(t, err)

	err = db.Exec(string(downSQL)).Error
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot roll back migration 000009 while daily rules exist")

	current, err := NewRuleRepository(db).ListByEventType(ctx, proj.ID(), "daily_login")
	require.NoError(t, err)
	require.Len(t, current, 1)
	require.True(t, current[0].OncePerUTCDay())

	require.NoError(t, db.Exec(
		"DELETE FROM rules WHERE project_id = ? AND id = ? AND version = ?",
		proj.ID(),
		daily.ID(),
		daily.Version(),
	).Error)
	require.NoError(t, db.Exec(string(downSQL)).Error)

	var dailyColumnExists bool
	require.NoError(t, db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
			  AND table_name = 'rules'
			  AND column_name = 'once_per_utc_day'
		)
	`).Scan(&dailyColumnExists).Error)
	require.False(t, dailyColumnExists)

	var dailyClaimsTableExists bool
	require.NoError(t, db.Raw(
		"SELECT to_regclass(current_schema() || '.rule_daily_claims') IS NOT NULL",
	).Scan(&dailyClaimsTableExists).Error)
	require.False(t, dailyClaimsTableExists)
}
