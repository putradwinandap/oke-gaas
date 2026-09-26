package rule

import (
	"strings"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/stretchr/testify/require"
)

func TestRuleMatchesExactProjectAndEventType(t *testing.T) {
	value, err := New("rule_lesson", "proj_1", 1, "lesson_completed", 100)
	require.NoError(t, err)

	matching, err := event.New("evt_1", "proj_1", "player_1", "lesson_completed", time.Now(), time.Now(), nil)
	require.NoError(t, err)
	otherProject, err := event.New("evt_2", "proj_2", "player_1", "lesson_completed", time.Now(), time.Now(), nil)
	require.NoError(t, err)
	otherType, err := event.New("evt_3", "proj_1", "player_1", "lesson_started", time.Now(), time.Now(), nil)
	require.NoError(t, err)

	require.True(t, value.Matches(matching))
	require.False(t, value.Matches(otherProject))
	require.False(t, value.Matches(otherType))
}

func TestRuleRequiresExplicitPositiveVersionAndXP(t *testing.T) {
	_, err := New("rule_lesson", "proj_1", 0, "lesson_completed", 100)
	require.ErrorIs(t, err, ErrInvalidVersion)

	_, err = New("rule_lesson", "proj_1", 1, "lesson_completed", 0)
	require.ErrorIs(t, err, ErrInvalidXPAmount)
}

func TestRuleRejectsEventTypeLongerThanPersistenceLimit(t *testing.T) {
	value, err := New("rule_1", "proj_1", 1, strings.Repeat("x", 256), 100)
	require.ErrorIs(t, err, ErrEventTypeTooLong)
	require.Nil(t, value)
}
