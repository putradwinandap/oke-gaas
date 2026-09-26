package event

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSameLogicalEventTreatsEquivalentJSONNumbersAsEqual(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)

	left, err := New(
		"evt_1",
		"proj_1",
		"player_1",
		"lesson_completed",
		now,
		now,
		map[string]any{"score": json.Number("1e2")},
	)
	require.NoError(t, err)

	right, err := New(
		"evt_1",
		"proj_1",
		"player_1",
		"lesson_completed",
		now,
		now.Add(time.Second),
		map[string]any{"score": json.Number("100")},
	)
	require.NoError(t, err)

	require.True(t, left.SameLogicalEvent(right))
}

func TestPropertiesReturnsDeepCopy(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	value, err := New(
		"evt_1",
		"proj_1",
		"player_1",
		"lesson_completed",
		now,
		now,
		map[string]any{"nested": map[string]any{"enabled": true}},
	)
	require.NoError(t, err)

	first := value.Properties()
	first["nested"].(map[string]any)["enabled"] = false

	second := value.Properties()
	require.Equal(t, true, second["nested"].(map[string]any)["enabled"])
}
