package database_test

import (
	"context"
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
