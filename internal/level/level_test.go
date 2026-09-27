package level

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRestoreThresholdValidatesConfiguredLevel(t *testing.T) {
	_, err := RestoreThreshold("", 2, 100)
	require.ErrorIs(t, err, ErrInvalidProjectID)

	_, err = RestoreThreshold("proj_1", 1, 100)
	require.Error(t, err)

	_, err = RestoreThreshold("proj_1", 2, 0)
	require.ErrorIs(t, err, ErrInvalidMinXP)

	value, err := RestoreThreshold("proj_1", 2, 100)
	require.NoError(t, err)
	require.Equal(t, "proj_1", value.ProjectID())
	require.Equal(t, uint64(2), value.Number())
	require.Equal(t, int64(100), value.MinXP())
}
