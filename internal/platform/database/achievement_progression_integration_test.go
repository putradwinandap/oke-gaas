package database

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/achievement"
	"github.com/putradwinandap/oke-gaas/internal/counter"
	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openAchievementIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db := openProgressionIntegrationDatabase(t)
	return db
}

func createCounterAchievement(t *testing.T, db *gorm.DB, projectID string, target int64) *achievement.Definition {
	t.Helper()
	ctx := context.Background()
	definition, err := counter.New("counter_achievements_"+projectID, projectID, "lessons_completed", "lesson_completed", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(ctx, definition))
	value, err := achievement.New("achievement_"+projectID, projectID, "First lessons", definition.ID(), target, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewAchievementRepository(db).Save(ctx, value))
	return value
}

func TestAchievementUnlockIsAuditableIdempotentAndProjectScoped(t *testing.T) {
	db := openAchievementIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition := createCounterAchievement(t, db, proj.ID(), 2)
	service := progression.NewService(NewProgressionTransactor(db))
	process := func(id string) error {
		occurredAt := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
		_, err := service.Process(ctx, eventdomain.IngestCommand{ID: id, ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "lesson_completed", OccurredAt: occurredAt})
		return err
	}
	require.NoError(t, process("evt_achievement_1"))
	require.NoError(t, process("evt_achievement_1"))
	unlocks, err := NewAchievementUnlockRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Empty(t, unlocks)
	require.NoError(t, process("evt_achievement_2"))
	require.NoError(t, process("evt_achievement_3"))
	unlocks, err = NewAchievementUnlockRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, unlocks, 1)
	require.Equal(t, definition.ID(), unlocks[0].AchievementID())
	require.Equal(t, "evt_achievement_2", unlocks[0].EventID())
	require.Equal(t, int64(2), unlocks[0].CounterValue())
	require.False(t, unlocks[0].UnlockedAt().IsZero())
	stored, err := NewAchievementRepository(db).GetByID(ctx, proj.ID(), definition.ID())
	require.NoError(t, err)
	require.Equal(t, int64(2), stored.Target())
}

func TestConcurrentEventsCreateOneAchievementUnlock(t *testing.T) {
	db := openAchievementIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	createCounterAchievement(t, db, proj.ID(), 1)
	service := progression.NewService(NewProgressionTransactor(db))
	const events = 12
	errs := make(chan error, events)
	var wg sync.WaitGroup
	for i := 0; i < events; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.Process(ctx, eventdomain.IngestCommand{ID: fmt.Sprintf("evt_achievement_concurrent_%d", index), ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "lesson_completed", OccurredAt: time.Now()})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	unlocks, err := NewAchievementUnlockRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, unlocks, 1)
	var count int64
	require.NoError(t, db.Table("player_counters").Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var counterValue int64
	require.NoError(t, db.Table("player_counters").Select("value").Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).Scan(&counterValue).Error)
	require.EqualValues(t, events, counterValue)
}

func TestAchievementUnlockAndCounterRollBackWithEventTransaction(t *testing.T) {
	db := openAchievementIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	createCounterAchievement(t, db, proj.ID(), 1)
	_, err := NewPlayerStateRepository(db).AddXP(ctx, proj.ID(), pl.ID(), int64(^uint64(0)>>1), time.Now())
	require.NoError(t, err)
	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{ID: "evt_achievement_rollback", ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "lesson_completed", OccurredAt: time.Now()}
	_, err = service.Process(ctx, command)
	require.Error(t, err)
	unlocks, err := NewAchievementUnlockRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Empty(t, unlocks)
	states, err := NewPlayerCounterRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Empty(t, states)
	var grants int64
	require.NoError(t, db.Table("reward_grants").Where("project_id = ? AND event_id = ?", proj.ID(), command.ID).Count(&grants).Error)
	require.Zero(t, grants)
	require.NoError(t, db.Model(&playerStateRecord{}).Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).Update("xp", 0).Error)
	_, err = service.Process(ctx, command)
	require.NoError(t, err)
	unlocks, err = NewAchievementUnlockRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, unlocks, 1)
}

func TestAchievementMigrationBlocksUnawareProcessingAndDowngrade(t *testing.T) {
	db := openAchievementIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition := createCounterAchievement(t, db, proj.ID(), 1)
	eventValue, err := eventdomain.New("evt_unaware_achievement", proj.ID(), pl.ID(), "lesson_completed", time.Now(), time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, NewEventRepository(db).Save(ctx, eventValue))
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT set_config('oke_gaas.counter_aware', 'true', true)").Error; err != nil {
			return err
		}
		_, err := NewEventProcessingRepository(tx).Claim(ctx, proj.ID(), eventValue.ID(), time.Now())
		return err
	})
	require.ErrorContains(t, err, "Achievement-aware application binary")
	require.NotEmpty(t, definition.ID())
	var claims int64
	require.NoError(t, db.Table("event_processing").Where("project_id = ? AND event_id = ?", proj.ID(), eventValue.ID()).Count(&claims).Error)
	require.Zero(t, claims)

	downSQL, err := os.ReadFile("../../../migrations/000012_achievements.down.sql")
	require.NoError(t, err)
	err = db.Exec(string(downSQL)).Error
	require.ErrorContains(t, err, "cannot roll back 000012")
	var relation *string
	require.NoError(t, db.Raw("SELECT to_regclass('achievement_definitions')").Scan(&relation).Error)
	require.NotNil(t, relation)
}
