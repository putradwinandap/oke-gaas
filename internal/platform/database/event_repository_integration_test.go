package database

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestEventRetryRemainsUsableInsideTransactionAndAfterJSONBRoundTrip(t *testing.T) {
	db := openIntegrationDatabase(t)
	ctx := context.Background()

	projects := NewProjectRepository(db)
	players := NewPlayerRepository(db)

	owningProject, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, owningProject))

	targetPlayer, err := player.New(owningProject.ID(), "customer-42", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, targetPlayer))

	command := eventdomain.IngestCommand{
		ID:         "evt_retry_in_tx",
		ProjectID:  owningProject.ID(),
		PlayerID:   targetPlayer.ID(),
		Type:       "lesson_completed",
		OccurredAt: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
		Properties: map[string]any{"score": json.Number("1e2")},
	}

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		service := eventdomain.NewService(
			NewPlayerRepository(tx),
			NewEventRepository(tx),
		)

		first, err := service.Ingest(ctx, command)
		require.NoError(t, err)
		require.False(t, first.Duplicate)

		retry, err := service.Ingest(ctx, command)
		require.NoError(t, err)
		require.True(t, retry.Duplicate)
		require.True(t, first.Event.SameLogicalEvent(retry.Event))

		var count int64
		require.NoError(t, tx.Model(&eventRecord{}).
			Where("project_id = ? AND id = ?", owningProject.ID(), command.ID).
			Count(&count).Error)
		require.EqualValues(t, 1, count)

		return nil
	}))
}
