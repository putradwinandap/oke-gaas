package database

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/counter"
	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/progression"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProgressionCounterConditionsAndDuplicateEventRetry(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := counter.New("counter_lessons", proj.ID(), "lessons_completed", "lesson_completed", map[string]any{
		"course_id": "course_7",
	}, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(ctx, definition))

	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{
		ID: "evt_counter_match", ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "lesson_completed",
		OccurredAt: time.Now().Add(-time.Minute), Properties: map[string]any{"course_id": "course_7"},
	}
	first, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.False(t, first.Duplicate)

	retry, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.True(t, retry.Duplicate)

	_, err = service.Process(ctx, eventdomain.IngestCommand{
		ID: "evt_counter_miss", ProjectID: proj.ID(), PlayerID: pl.ID(), Type: "lesson_completed",
		OccurredAt: time.Now(), Properties: map[string]any{"course_id": "course_8"},
	})
	require.NoError(t, err)

	states, err := NewPlayerCounterRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.Equal(t, definition.ID(), states[0].CounterID())
	require.Equal(t, int64(1), states[0].Value())
}

func TestConcurrentDistinctEventsDoNotLoseCounterIncrements(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := counter.New("counter_lessons", proj.ID(), "lessons_completed", "lesson_completed", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(ctx, definition))
	service := progression.NewService(NewProgressionTransactor(db))

	const eventCount = 12
	errs := make(chan error, eventCount)
	var wg sync.WaitGroup
	for i := 0; i < eventCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := service.Process(ctx, eventdomain.IngestCommand{
				ID: fmt.Sprintf("evt_counter_%d", index), ProjectID: proj.ID(), PlayerID: pl.ID(),
				Type: "lesson_completed", OccurredAt: time.Now(),
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	states, err := NewPlayerCounterRepository(db).ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, states, 1)
	require.Equal(t, int64(eventCount), states[0].Value())
}

func TestCounterIncrementRollsBackWithFailedEventProcessing(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := counter.New("counter_lessons", proj.ID(), "lessons_completed", "lesson_completed", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(ctx, definition))
	states := NewPlayerStateRepository(db)
	_, err = states.AddXP(ctx, proj.ID(), pl.ID(), int64(^uint64(0)>>1), time.Now())
	require.NoError(t, err)

	service := progression.NewService(NewProgressionTransactor(db))
	command := eventdomain.IngestCommand{
		ID: "evt_counter_rollback", ProjectID: proj.ID(), PlayerID: pl.ID(),
		Type: "lesson_completed", OccurredAt: time.Now(),
	}
	_, err = service.Process(ctx, command)
	require.Error(t, err)

	_, err = NewEventRepository(db).GetByID(ctx, proj.ID(), command.ID)
	require.ErrorIs(t, err, eventdomain.ErrNotFound)
	playerCounters := NewPlayerCounterRepository(db)
	progress, err := playerCounters.ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Empty(t, progress)

	require.NoError(t, db.Model(&playerStateRecord{}).
		Where("project_id = ? AND player_id = ?", proj.ID(), pl.ID()).
		Update("xp", 0).Error)
	retry, err := service.Process(ctx, command)
	require.NoError(t, err)
	require.False(t, retry.Duplicate)
	progress, err = playerCounters.ListByPlayer(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, progress, 1)
	require.Equal(t, int64(1), progress[0].Value())
}

func TestPlayerCounterRepositoryRejectsOverflow(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := counter.New("counter_lessons", proj.ID(), "lessons_completed", "lesson_completed", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(ctx, definition))
	require.NoError(t, db.Exec(
		"INSERT INTO player_counters (project_id, player_id, counter_id, value, updated_at) VALUES (?, ?, ?, ?, ?)",
		proj.ID(), pl.ID(), definition.ID(), int64(^uint64(0)>>1), time.Now(),
	).Error)

	_, err = NewPlayerCounterRepository(db).Increment(ctx, proj.ID(), pl.ID(), definition.ID(), time.Now())
	require.ErrorIs(t, err, counter.ErrValueOverflow)
}

func TestCounterMigrationBlocksUnawareEventProcessingForConfiguredProject(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := counter.New("counter_lessons", proj.ID(), "lessons_completed", "lesson_completed", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(ctx, definition))
	eventValue, err := eventdomain.New("evt_unaware_binary", proj.ID(), pl.ID(), "lesson_completed", time.Now(), time.Now(), nil)
	require.NoError(t, err)
	require.NoError(t, NewEventRepository(db).Save(ctx, eventValue))

	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := NewEventProcessingRepository(tx).Claim(ctx, proj.ID(), eventValue.ID(), time.Now())
		return err
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "requires a Counter-aware application binary")

	var count int64
	require.NoError(t, db.Table("event_processing").Where("project_id = ? AND event_id = ?", proj.ID(), eventValue.ID()).Count(&count).Error)
	require.Zero(t, count)
}

func TestCounterMigrationDownFailsClosedWhenDefinitionsExist(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	proj, _ := seedProgressionFixture(t, db, 1)
	definition, err := counter.New("counter_lessons", proj.ID(), "lessons_completed", "lesson_completed", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewCounterRepository(db).Save(context.Background(), definition))

	downSQL, err := os.ReadFile("../../../migrations/000011_counters.down.sql")
	require.NoError(t, err)
	err = db.Exec(string(downSQL)).Error
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot roll back 000011")
	var relation *string
	require.NoError(t, db.Raw("SELECT to_regclass('counter_definitions')").Scan(&relation).Error)
	require.NotNil(t, relation)
}

func TestCounterMigrationDownSucceedsWithoutDefinitions(t *testing.T) {
	db := openProgressionIntegrationDatabase(t)
	downSQL, err := os.ReadFile("../../../migrations/000011_counters.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(downSQL)).Error)
	var relation *string
	require.NoError(t, db.Raw("SELECT to_regclass('counter_definitions')").Scan(&relation).Error)
	require.Nil(t, relation)
}
