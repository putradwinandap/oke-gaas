package database

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/streak"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openStreakIntegrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	return openProgressionIntegrationDatabase(t)
}

func TestStreakClaimsAreIdempotentConcurrentAuditableAndProjectScoped(t *testing.T) {
	db := openStreakIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := streak.New("streak_test_"+proj.ID(), proj.ID(), "Daily login", "daily_login", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewStreakRepository(db).Save(ctx, definition))
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	events := NewEventRepository(db)
	for i := 0; i < 12; i++ {
		eventID := fmt.Sprintf("evt_streak_%d", i)
		e, err := eventdomain.New(eventID, proj.ID(), pl.ID(), "daily_login", day.Add(12*time.Hour), time.Now(), nil)
		require.NoError(t, err)
		require.NoError(t, events.Save(ctx, e))
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			eventID := fmt.Sprintf("evt_streak_%d", i)
			claim, e := streak.NewQualifiedDay(proj.ID(), pl.ID(), definition.ID(), eventID, day, time.Now())
			if e == nil {
				_, e = NewStreakDayRepository(db).Claim(ctx, claim)
			}
			errs <- e
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	var rows int64
	require.NoError(t, db.Table("streak_days").Where("project_id = ? AND player_id = ? AND streak_id = ? AND qualified_day = ?", proj.ID(), pl.ID(), definition.ID(), day).Count(&rows).Error)
	require.Equal(t, int64(1), rows)
	var eventRows int64
	require.NoError(t, db.Table("streak_event_claims").Where("project_id = ? AND player_id = ? AND streak_id = ?", proj.ID(), pl.ID(), definition.ID()).Count(&eventRows).Error)
	require.Equal(t, int64(12), eventRows)
	claim, err := streak.NewQualifiedDay("proj_other", pl.ID(), definition.ID(), "evt_other_project", day, day)
	require.NoError(t, err)
	_, err = NewStreakDayRepository(db).Claim(ctx, claim)
	require.Error(t, err)
}

func TestStreakMigrationRejectsUnawareProcessingAndDowngrade(t *testing.T) {
	db := openStreakIntegrationDatabase(t)
	ctx := context.Background()
	proj, pl := seedProgressionFixture(t, db, 1)
	definition, err := streak.New("streak_guard_"+proj.ID(), proj.ID(), "Daily login", "daily_login", nil, time.Now())
	require.NoError(t, err)
	require.NoError(t, NewStreakRepository(db).Save(ctx, definition))
	require.NoError(t, db.Exec("SELECT set_config('oke_gaas.counter_aware','true',true), set_config('oke_gaas.achievement_aware','true',true), set_config('oke_gaas.badge_aware','true',true)").Error)
	claimed, claimErr := NewEventProcessingRepository(db).Claim(ctx, proj.ID(), "evt_unaware_streak", time.Now())
	require.False(t, claimed)
	require.ErrorContains(t, claimErr, "Streak-aware")
	_ = pl
	down, err := os.ReadFile("../../../migrations/000014_streaks.down.sql")
	require.NoError(t, err)
	require.ErrorContains(t, db.Exec(string(down)).Error, "cannot roll back 000014")
}
