package progression

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRestoreStateValidatesMaterializedXP(t *testing.T) {
	now := time.Now()

	_, err := RestoreState("project_1", "player_1", -1, now)
	require.ErrorIs(t, err, ErrInvalidXP)

	state, err := RestoreState("project_1", "player_1", 0, now)
	require.NoError(t, err)
	require.Equal(t, "project_1", state.ProjectID())
	require.Equal(t, "player_1", state.PlayerID())
	require.Equal(t, int64(0), state.XP())
}

func TestRestoreStateRequiresTimestamp(t *testing.T) {
	_, err := RestoreState("project_1", "player_1", 0, time.Time{})
	require.ErrorIs(t, err, ErrInvalidUpdatedAt)
}
