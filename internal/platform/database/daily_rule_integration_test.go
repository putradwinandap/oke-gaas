package database

import (
	"context"
	"sync"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	ruledomain "github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/stretchr/testify/require"
)

func seedDailyRuleFixture(t *testing.T, xp int64) (*project.Project, *player.Player, *ruledomain.Rule) {
	t.Helper()
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Daily Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	pl, err := player.New(proj.ID(), "daily-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))

	daily, err := ruledomain.NewTimed("rule_daily", proj.ID(), 1, "daily_login", xp, nil, 1, true)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, daily))
	return proj, pl, daily
}

func TestDailyRuleGrantsOncePerUTCDayAndAgainNextDay(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Daily Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))
	pl, err := player.New(proj.ID(), "daily-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))
	daily, err := ruledomain.NewTimed("rule_daily", proj.ID(), 1, "daily_login", 25, nil, 1, true)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, daily))

	service := progression.NewService(NewProgressionTransactor(db))
	send := func(id string, occurredAt time.Time) *progression.ProcessResult {
		result, err := service.Process(ctx, eventdomain.IngestCommand{
			ID: id, ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "daily_login", OccurredAt: occurredAt,
		})
		require.NoError(t, err)
		return result
	}

	first := send("evt_daily_first", time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC))
	require.Len(t, first.Grants, 1)
	require.Equal(t, int64(25), first.State.XP())

	second := send("evt_daily_second", time.Date(2026, 9, 27, 23, 59, 0, 0, time.UTC))
	require.Empty(t, second.Grants)
	require.Equal(t, int64(25), second.State.XP())

	nextDay := send("evt_daily_next", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	require.Len(t, nextDay.Grants, 1)
	require.Equal(t, int64(50), nextDay.State.XP())

	var claims int64
	require.NoError(t, db.Table("rule_daily_claims").
		Where("project_id = ? AND player_id = ? AND rule_id = ?", proj.ID(), pl.ID(), daily.ID()).
		Count(&claims).Error)
	require.Equal(t, int64(2), claims)
}

func TestConcurrentSameDayEventsGrantDailyRuleOnce(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Concurrent Daily", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))
	pl, err := player.New(proj.ID(), "concurrent-daily-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))
	daily, err := ruledomain.NewTimed("rule_daily_concurrent", proj.ID(), 1, "daily_login", 100, nil, 1, true)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, daily))

	service := progression.NewService(NewProgressionTransactor(db))
	occurredAt := time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)
	commands := []eventdomain.IngestCommand{
		{ID: "evt_daily_a", ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "daily_login", OccurredAt: occurredAt},
		{ID: "evt_daily_b", ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "daily_login", OccurredAt: occurredAt.Add(time.Hour)},
	}

	results := make(chan *progression.ProcessResult, len(commands))
	errs := make(chan error, len(commands))
	var wg sync.WaitGroup
	wg.Add(len(commands))
	for _, command := range commands {
		command := command
		go func() {
			defer wg.Done()
			result, err := service.Process(ctx, command)
			results <- result
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	grants := 0
	for result := range results {
		require.NotNil(t, result)
		grants += len(result.Grants)
	}
	require.Equal(t, 1, grants)

	state, err := NewPlayerStateRepository(db).Get(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Equal(t, int64(100), state.XP())

	var claims int64
	require.NoError(t, db.Table("rule_daily_claims").
		Where("project_id = ? AND player_id = ? AND rule_id = ?", proj.ID(), pl.ID(), daily.ID()).
		Count(&claims).Error)
	require.Equal(t, int64(1), claims)
}

func TestDailyClaimRejectsCrossProjectScope(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	projectA, err := project.New("Daily A", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, projectA))
	projectB, err := project.New("Daily B", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, projectB))

	playerA, err := player.New(projectA.ID(), "daily-a", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, playerA))
	ruleA, err := ruledomain.NewTimed("rule_daily_a", projectA.ID(), 1, "daily_login", 25, nil, 1, true)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, ruleA))

	claimed, err := NewRuleDailyClaimRepository(db).Claim(
		ctx, projectB.ID(), playerA.ID(), ruleA.ID(), ruleA.Version(),
		time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
	)
	require.Error(t, err)
	require.False(t, claimed)

	var claims int64
	require.NoError(t, db.Table("rule_daily_claims").Count(&claims).Error)
	require.Zero(t, claims)
}

func TestDailyClaimRollsBackWhenPlayerStateUpdateFails(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Daily Rollback", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))
	pl, err := player.New(proj.ID(), "daily-rollback-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))
	daily, err := ruledomain.NewTimed("rule_daily_rollback", proj.ID(), 1, "daily_login", 1, nil, 1, true)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, daily))

	states := NewPlayerStateRepository(db)
	_, err = states.AddXP(ctx, proj.ID(), pl.ID(), int64(^uint64(0)>>1), time.Now())
	require.NoError(t, err)

	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{
		ID: "evt_daily_overflow", ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "daily_login",
		OccurredAt: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC),
	}
	_, err = service.Process(ctx, command)
	require.Error(t, err)

	var claims int64
	require.NoError(t, db.Table("rule_daily_claims").Count(&claims).Error)
	require.Zero(t, claims)

	require.NoError(t, db.Model(&playerStateRecord{}).
		Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).
		Update("xp", 0).Error)

	retry, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.Len(t, retry.Grants, 1)
	require.Equal(t, int64(1), retry.State.XP())

	require.NoError(t, db.Table("rule_daily_claims").Count(&claims).Error)
	require.Equal(t, int64(1), claims)
}
