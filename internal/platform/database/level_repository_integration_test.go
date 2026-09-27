package database_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	database "github.com/putradwinandap/oke-gaas/internal/platform/database"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/stretchr/testify/require"
)

func TestConcurrentLevelAppendKeepsOneContiguousNextLevel(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Concurrent Levels", time.Now())
	require.NoError(t, err)
	require.NoError(t, database.NewProjectRepository(db).Save(ctx, proj))

	repository := database.NewLevelRepository(db)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			defer wg.Done()
			_, err := repository.Append(ctx, proj.ID(), 100)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	successes := 0
	failures := 0
	for err := range errs {
		if err == nil {
			successes++
			continue
		}
		failures++
		require.ErrorContains(t, err, "greater than the previous threshold")
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, failures)

	values, err := repository.List(ctx, proj.ID())
	require.NoError(t, err)
	require.Len(t, values, 1)
	require.Equal(t, uint64(2), values[0].Number())
	require.Equal(t, int64(100), values[0].MinXP())
}

func TestLevelMigrationDownFailsClosedWhenThresholdsExist(t *testing.T) {
	db := openHTTPIntegrationDatabase(t)
	ctx := context.Background()

	proj, err := project.New("Rollback Levels", time.Now())
	require.NoError(t, err)
	require.NoError(t, database.NewProjectRepository(db).Save(ctx, proj))

	repository := database.NewLevelRepository(db)
	_, err = repository.Append(ctx, proj.ID(), 100)
	require.NoError(t, err)

	downSQL, err := os.ReadFile("../../../migrations/000010_level_thresholds.down.sql")
	require.NoError(t, err)
	err = db.Exec(string(downSQL)).Error
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot roll back 000010")

	var count int64
	require.NoError(t, db.Table("level_thresholds").Where("project_id = ?", proj.ID()).Count(&count).Error)
	require.Equal(t, int64(1), count)
}
