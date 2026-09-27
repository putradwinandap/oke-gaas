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

func TestRuleMatchesEveryConfiguredTopLevelPropertyCondition(t *testing.T) {
	value, err := NewConditional(
		"rule_lesson",
		"proj_1",
		1,
		"lesson_completed",
		100,
		map[string]any{
			"course_id":  "course_7",
			"difficulty": 1,
			"metadata":   map[string]any{"required": true},
			"tags":       []any{"go", "backend"},
		},
	)
	require.NoError(t, err)

	matching, err := event.New(
		"evt_1",
		"proj_1",
		"player_1",
		"lesson_completed",
		time.Now(),
		time.Now(),
		map[string]any{
			"course_id":  "course_7",
			"difficulty": 1.0,
			"metadata":   map[string]any{"required": true},
			"tags":       []any{"go", "backend"},
			"extra":      "ignored",
		},
	)
	require.NoError(t, err)
	require.True(t, value.Matches(matching))

	for name, properties := range map[string]map[string]any{
		"missing": {
			"course_id":  "course_7",
			"difficulty": 1,
			"metadata":   map[string]any{"required": true},
		},
		"different scalar": {
			"course_id":  "course_8",
			"difficulty": 1,
			"metadata":   map[string]any{"required": true},
			"tags":       []any{"go", "backend"},
		},
		"different nested value": {
			"course_id":  "course_7",
			"difficulty": 1,
			"metadata":   map[string]any{"required": false},
			"tags":       []any{"go", "backend"},
		},
		"different array order": {
			"course_id":  "course_7",
			"difficulty": 1,
			"metadata":   map[string]any{"required": true},
			"tags":       []any{"backend", "go"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate, err := event.New(
				"evt_"+strings.ReplaceAll(name, " ", "_"),
				"proj_1",
				"player_1",
				"lesson_completed",
				time.Now(),
				time.Now(),
				properties,
			)
			require.NoError(t, err)
			require.False(t, value.Matches(candidate))
		})
	}
}

func TestRuleConditionsAreDefensiveCopies(t *testing.T) {
	input := map[string]any{"metadata": map[string]any{"required": true}}
	value, err := NewConditional("rule_lesson", "proj_1", 1, "lesson_completed", 100, input)
	require.NoError(t, err)

	input["metadata"].(map[string]any)["required"] = false
	require.Equal(t, true, value.Conditions()["metadata"].(map[string]any)["required"])

	copy := value.Conditions()
	copy["metadata"].(map[string]any)["required"] = false
	require.Equal(t, true, value.Conditions()["metadata"].(map[string]any)["required"])
}

func TestRuleRejectsNonJSONConditions(t *testing.T) {
	value, err := NewConditional(
		"rule_lesson",
		"proj_1",
		1,
		"lesson_completed",
		100,
		map[string]any{"bad": func() {}},
	)
	require.ErrorIs(t, err, ErrInvalidConditions)
	require.Nil(t, value)
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
