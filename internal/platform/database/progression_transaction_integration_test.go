package database

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/putradwinandap/oke-gaas/internal/project"
	rewarddomain "github.com/putradwinandap/oke-gaas/internal/reward"
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

	for _, table := range []string{"project_api_keys", "event_processing", "rule_match_counts", "player_states", "reward_grants", "rules", "events", "players", "projects"} {
		require.NoError(t, db.Exec("DROP TABLE IF EXISTS "+table+" CASCADE").Error)
	}
	for _, path := range []string{
		"../../../migrations/000001_core.up.sql",
		"../../../migrations/000002_events.up.sql",
		"../../../migrations/000003_rules_rewards.up.sql",
		"../../../migrations/000004_player_state.up.sql",
		"../../../migrations/000005_event_processing.up.sql",
		"../../../migrations/000007_rule_conditions.up.sql",
		"../../../migrations/000008_rule_match_counts.up.sql",
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

type aggregateCounterCoordinator struct {
	mu           sync.Mutex
	calls        int
	firstLocked  chan struct{}
	releaseFirst chan struct{}
	secondPID    chan int
	counts       chan uint64
}

func newAggregateCounterCoordinator() *aggregateCounterCoordinator {
	return &aggregateCounterCoordinator{
		firstLocked:  make(chan struct{}),
		releaseFirst: make(chan struct{}),
		secondPID:    make(chan int, 1),
		counts:       make(chan uint64, 2),
	}
}

func (c *aggregateCounterCoordinator) nextCall() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.calls
}

type coordinatedAggregateCounter struct {
	base        *RuleMatchCounter
	coordinator *aggregateCounterCoordinator
}

func (c *coordinatedAggregateCounter) Increment(
	ctx context.Context,
	projectID, playerID, ruleID string,
	ruleVersion uint64,
	matchedAt time.Time,
) (uint64, error) {
	call := c.coordinator.nextCall()
	if call == 1 {
		count, err := c.base.Increment(ctx, projectID, playerID, ruleID, ruleVersion, matchedAt)
		close(c.coordinator.firstLocked)
		if err != nil {
			return 0, err
		}
		c.coordinator.counts <- count
		select {
		case <-c.coordinator.releaseFirst:
			return count, nil
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}

	if call == 2 {
		select {
		case <-c.coordinator.firstLocked:
		case <-ctx.Done():
			return 0, ctx.Err()
		}

		var backendPID int
		if err := c.base.db.WithContext(ctx).Raw("SELECT pg_backend_pid()").Scan(&backendPID).Error; err != nil {
			return 0, err
		}
		select {
		case c.coordinator.secondPID <- backendPID:
		case <-ctx.Done():
			return 0, ctx.Err()
		}

		count, err := c.base.Increment(ctx, projectID, playerID, ruleID, ruleVersion, matchedAt)
		if err == nil {
			c.coordinator.counts <- count
		}
		return count, err
	}

	count, err := c.base.Increment(ctx, projectID, playerID, ruleID, ruleVersion, matchedAt)
	if err == nil {
		c.coordinator.counts <- count
	}
	return count, err
}

type coordinatedProgressionTransactor struct {
	db          *gorm.DB
	coordinator *aggregateCounterCoordinator
}

func (t *coordinatedProgressionTransactor) WithinTransaction(
	ctx context.Context,
	fn func(progression.Work) error,
) error {
	return t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(progression.Work{
			Players: NewPlayerRepository(tx),
			Events:  NewEventRepository(tx),
			Rules:   NewRuleRepository(tx),
			Grants:  NewRewardGrantRepository(tx),
			Counters: &coordinatedAggregateCounter{
				base:        NewRuleMatchCounter(tx),
				coordinator: t.coordinator,
			},
			States: NewPlayerStateRepository(tx),
			Claims: NewEventProcessingRepository(tx),
		})
	})
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

func TestPlayerStateConcurrentXPIncrementsDoNotLoseUpdates(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)

	states := NewPlayerStateRepository(db)
	_, err := states.Ensure(ctx, proj.ID(), pl.ID(), time.Now())
	require.NoError(t, err)

	const workers = 10
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := states.AddXP(ctx, proj.ID(), pl.ID(), 1, time.Now())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	state, err := states.Get(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Equal(t, int64(workers), state.XP())
}

func TestPlayerStateRejectsCrossProjectMutation(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)

	otherProject, err := project.New("Other", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, otherProject))

	states := NewPlayerStateRepository(db)
	_, err = states.Ensure(ctx, proj.ID(), pl.ID(), time.Now())
	require.NoError(t, err)

	_, err = states.Get(ctx, otherProject.ID(), pl.ID())
	require.ErrorIs(t, err, progression.ErrStateNotFound)

	_, err = states.AddXP(ctx, otherProject.ID(), pl.ID(), 10, time.Now())
	require.Error(t, err)

	state, err := states.Get(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Equal(t, int64(0), state.XP())
}

func TestConcurrentDuplicateEventProcessesExactlyOnce(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 100)

	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{
		ID:         "evt_concurrent",
		ProjectID:  proj.ID(),
		PlayerID:   pl.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Now().Add(-time.Minute),
	}

	const workers = 2
	results := make(chan *progression.ProcessResult, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
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

	var duplicateCount int
	for result := range results {
		require.NotNil(t, result)
		if result.Duplicate {
			duplicateCount++
		}
		require.Equal(t, int64(100), result.State.XP())
	}
	require.Equal(t, 1, duplicateCount)

	grants, err := NewRewardGrantRepository(db).ListByEvent(ctx, proj.ID(), command.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)

	state, err := NewPlayerStateRepository(db).Get(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Equal(t, int64(100), state.XP())

	var claimCount int64
	require.NoError(t, db.Table("event_processing").
		Where("project_id = ? AND event_id = ?", proj.ID(), command.ID).
		Count(&claimCount).Error)
	require.Equal(t, int64(1), claimCount)
}

func TestAutoMigrateDevelopmentBackfillsHistoricalProgressionState(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}

	db, err := OpenPostgres(dsn)
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, table := range []string{"project_api_keys", "event_processing", "rule_match_counts", "player_states", "reward_grants", "rules", "events", "players", "projects"} {
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

	ctx := context.Background()
	proj, err := project.New("Historical", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	pl, err := player.New(proj.ID(), "legacy-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))

	ev, err := eventdomain.New(
		"evt_historical",
		proj.ID(),
		pl.ID(),
		"lesson_completed",
		time.Now().Add(-2*time.Hour),
		time.Now().Add(-time.Hour),
		nil,
	)
	require.NoError(t, err)
	require.NoError(t, NewEventRepository(db).Save(ctx, ev))

	rule, err := ruledomain.New("rule_historical", proj.ID(), 1, "lesson_completed", 75)
	require.NoError(t, err)
	require.NoError(t, db.Exec(
		"INSERT INTO rules (project_id, id, version, event_type, xp_amount) VALUES (?, ?, ?, ?, ?)",
		rule.ProjectID(),
		rule.ID(),
		rule.Version(),
		rule.EventType(),
		rule.XPAmount(),
	).Error)

	grant, err := rewarddomain.NewGrant(
		"grant_historical",
		proj.ID(),
		pl.ID(),
		ev.ID(),
		rule.ID(),
		rule.Version(),
		rule.XPAmount(),
		time.Now().Add(-30*time.Minute),
	)
	require.NoError(t, err)
	require.NoError(t, NewRewardGrantRepository(db).Save(ctx, grant))

	require.NoError(t, AutoMigrateCoreForDevelopment(db))

	state, err := NewPlayerStateRepository(db).Get(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Equal(t, int64(75), state.XP())

	var claimCount int64
	require.NoError(t, db.Table("event_processing").
		Where("project_id = ? AND event_id = ?", proj.ID(), ev.ID()).
		Count(&claimCount).Error)
	require.Equal(t, int64(1), claimCount)
}

func TestAggregateCounterRollsBackWhenPlayerStateUpdateFails(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Aggregate Rollback", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	pl, err := player.New(proj.ID(), "aggregate-rollback-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))

	aggregate, err := ruledomain.NewAggregate(
		"rule_aggregate_rollback",
		proj.ID(),
		1,
		"lesson_completed",
		1,
		nil,
		2,
	)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, aggregate))

	states := NewPlayerStateRepository(db)
	_, err = states.AddXP(ctx, proj.ID(), pl.ID(), int64(^uint64(0)>>1), time.Now())
	require.NoError(t, err)

	service := progression.NewService(NewProgressionTransactor(db))
	first := eventdomain.IngestCommand{
		ID:         "evt_aggregate_first",
		ProjectID:  proj.ID(),
		PlayerID:   pl.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Now().Add(-2 * time.Minute),
	}
	firstResult, err := service.Process(ctx, first)
	require.NoError(t, err)
	require.Empty(t, firstResult.Grants)

	second := eventdomain.IngestCommand{
		ID:         "evt_aggregate_overflow",
		ProjectID:  proj.ID(),
		PlayerID:   pl.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Now().Add(-time.Minute),
	}
	_, err = service.Process(ctx, second)
	require.Error(t, err)

	_, err = NewEventRepository(db).GetByID(ctx, proj.ID(), second.ID)
	require.ErrorIs(t, err, eventdomain.ErrNotFound)

	grants, err := NewRewardGrantRepository(db).ListByEvent(ctx, proj.ID(), second.ID)
	require.NoError(t, err)
	require.Empty(t, grants)

	var matchCount uint64
	require.NoError(t, db.Table("rule_match_counts").
		Select("match_count").
		Where(
			"project_id = ? AND player_id = ? AND rule_id = ? AND rule_version = ?",
			proj.ID(), pl.ID(), aggregate.ID(), aggregate.Version(),
		).
		Scan(&matchCount).Error)
	require.Equal(t, uint64(1), matchCount)

	require.NoError(t, db.Model(&playerStateRecord{}).
		Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).
		Update("xp", 0).Error)

	retry, err := service.Process(ctx, second)
	require.NoError(t, err)
	require.False(t, retry.Duplicate)
	require.Len(t, retry.Grants, 1)
	require.Equal(t, int64(1), retry.State.XP())

	require.NoError(t, db.Table("rule_match_counts").
		Select("match_count").
		Where(
			"project_id = ? AND player_id = ? AND rule_id = ? AND rule_version = ?",
			proj.ID(), pl.ID(), aggregate.ID(), aggregate.Version(),
		).
		Scan(&matchCount).Error)
	require.Equal(t, uint64(2), matchCount)
}

func TestAggregateRuleConcurrentEventsCrossThresholdOnce(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Aggregate Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, proj))

	pl, err := player.New(proj.ID(), "aggregate-player", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, pl))

	aggregate, err := ruledomain.NewAggregate(
		"rule_aggregate",
		proj.ID(),
		1,
		"lesson_completed",
		100,
		nil,
		2,
	)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, aggregate))

	coordinator := newAggregateCounterCoordinator()
	service := progression.NewService(&coordinatedProgressionTransactor{
		db:          db,
		coordinator: coordinator,
	})
	commands := []eventdomain.IngestCommand{
		{
			ID:         "evt_aggregate_a",
			ProjectID:  proj.ID(),
			PlayerID:   pl.ID(),
			Type:       "lesson_completed",
			OccurredAt: time.Now().Add(-2 * time.Minute),
		},
		{
			ID:         "evt_aggregate_b",
			ProjectID:  proj.ID(),
			PlayerID:   pl.ID(),
			Type:       "lesson_completed",
			OccurredAt: time.Now().Add(-time.Minute),
		},
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

	var secondPID int
	select {
	case secondPID = <-coordinator.secondPID:
	case <-time.After(5 * time.Second):
		close(coordinator.releaseFirst)
		require.FailNow(t, "second aggregate transaction did not reach the shared counter")
	}

	blocked := false
	deadline := time.Now().Add(5 * time.Second)
	for !blocked && time.Now().Before(deadline) {
		require.NoError(t, db.Raw(
			"SELECT cardinality(pg_blocking_pids(?)) > 0",
			secondPID,
		).Scan(&blocked).Error)
		if !blocked {
			time.Sleep(10 * time.Millisecond)
		}
	}
	close(coordinator.releaseFirst)
	require.True(t, blocked, "second aggregate transaction never blocked on the first counter update")

	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	counts := []uint64{<-coordinator.counts, <-coordinator.counts}
	require.ElementsMatch(t, []uint64{1, 2}, counts)

	var granted int
	for result := range results {
		require.NotNil(t, result)
		granted += len(result.Grants)
	}
	require.Equal(t, 1, granted)

	state, err := NewPlayerStateRepository(db).Get(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Equal(t, int64(100), state.XP())

	var matchCount uint64
	require.NoError(t, db.Table("rule_match_counts").
		Select("match_count").
		Where(
			"project_id = ? AND player_id = ? AND rule_id = ? AND rule_version = ?",
			proj.ID(), pl.ID(), aggregate.ID(), aggregate.Version(),
		).
		Scan(&matchCount).Error)
	require.Equal(t, uint64(2), matchCount)

	var grantCount int64
	require.NoError(t, db.Table("reward_grants").
		Where("project_id = ? AND player_id = ? AND rule_id = ?", proj.ID(), pl.ID(), aggregate.ID()).
		Count(&grantCount).Error)
	require.Equal(t, int64(1), grantCount)
}

func TestRuleMatchCounterRejectsCrossProjectScope(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()

	projectA, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, projectA))

	projectB, err := project.New("Project B", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewProjectRepository(db).Save(ctx, projectB))

	playerA, err := player.New(projectA.ID(), "player-a", time.Now())
	require.NoError(t, err)
	require.NoError(t, NewPlayerRepository(db).Save(ctx, playerA))

	ruleA, err := ruledomain.NewAggregate("rule_a", projectA.ID(), 1, "lesson_completed", 100, nil, 2)
	require.NoError(t, err)
	require.NoError(t, NewRuleRepository(db).Save(ctx, ruleA))

	_, err = NewRuleMatchCounter(db).Increment(
		ctx,
		projectB.ID(),
		playerA.ID(),
		ruleA.ID(),
		ruleA.Version(),
		time.Now(),
	)
	require.Error(t, err)

	var count int64
	require.NoError(t, db.Table("rule_match_counts").Count(&count).Error)
	require.Zero(t, count)
}
