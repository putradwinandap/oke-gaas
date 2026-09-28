package counter

import (
	"context"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/stretchr/testify/require"
)

func TestCounterMatchesProjectEventTypeAndExactProperties(t *testing.T) {
	now := time.Now()
	value, err := New("counter_lessons", "proj_a", "lessons_completed", "lesson_completed", map[string]any{
		"course_id":  "course_7",
		"difficulty": 1,
		"metadata":   map[string]any{"required": true},
	}, now)
	require.NoError(t, err)

	matching := counterEvent(t, "proj_a", "lesson_completed", map[string]any{
		"course_id":  "course_7",
		"difficulty": 1.0,
		"metadata":   map[string]any{"required": true},
		"extra":      "ignored",
	})
	require.True(t, value.Matches(matching))
	require.False(t, value.Matches(counterEvent(t, "proj_b", "lesson_completed", matching.Properties())))
	require.False(t, value.Matches(counterEvent(t, "proj_a", "lesson_started", matching.Properties())))
	require.False(t, value.Matches(counterEvent(t, "proj_a", "lesson_completed", map[string]any{
		"course_id":  "course_7",
		"difficulty": 2,
		"metadata":   map[string]any{"required": true},
	})))
	require.False(t, value.Matches(nil))
}

func TestCounterConditionsAreDefensiveCopiesAndJSONValidated(t *testing.T) {
	conditions := map[string]any{"metadata": map[string]any{"required": true}}
	value, err := New("counter_lessons", "proj_a", "lessons_completed", "lesson_completed", conditions, time.Now())
	require.NoError(t, err)

	conditions["metadata"].(map[string]any)["required"] = false
	require.Equal(t, true, value.Conditions()["metadata"].(map[string]any)["required"])

	copy := value.Conditions()
	copy["metadata"].(map[string]any)["required"] = false
	require.Equal(t, true, value.Conditions()["metadata"].(map[string]any)["required"])

	_, err = New("counter_invalid", "proj_a", "invalid", "lesson_completed", map[string]any{"bad": func() {}}, time.Now())
	require.ErrorIs(t, err, ErrInvalidConditions)
}

func TestCounterRejectsMissingFieldsAndInvalidState(t *testing.T) {
	_, err := New(" ", "proj_a", "lessons_completed", "lesson_completed", nil, time.Now())
	require.ErrorIs(t, err, ErrInvalidID)
	_, err = New("counter_1", "proj_a", " ", "lesson_completed", nil, time.Now())
	require.ErrorIs(t, err, ErrInvalidName)
	_, err = New("counter_1", "proj_a", "lessons_completed", " ", nil, time.Now())
	require.ErrorIs(t, err, ErrInvalidEventType)

	_, err = RestoreState("proj_a", "player_a", "counter_1", -1, time.Now())
	require.ErrorIs(t, err, ErrInvalidValue)
	_, err = RestoreState("proj_a", "player_a", "counter_1", 1, time.Time{})
	require.ErrorIs(t, err, ErrInvalidUpdatedAt)
}

func TestProcessEventIncrementsOnlyMatchingCounters(t *testing.T) {
	now := time.Now()
	matching, err := New("counter_lessons", "proj_a", "lessons_completed", "lesson_completed", map[string]any{"course_id": "course_7"}, now)
	require.NoError(t, err)
	nonMatching, err := New("counter_wrong_course", "proj_a", "wrong_course", "lesson_completed", map[string]any{"course_id": "course_8"}, now)
	require.NoError(t, err)
	otherType, err := New("counter_started", "proj_a", "started", "lesson_started", nil, now)
	require.NoError(t, err)

	definitions := &memoryDefinitions{values: []*Definition{matching, nonMatching, otherType}}
	states := &memoryStates{values: make(map[string]*State)}
	value := counterEvent(t, "proj_a", "lesson_completed", map[string]any{"course_id": "course_7"})

	require.NoError(t, ProcessEvent(context.Background(), definitions, states, value))
	require.Equal(t, int64(1), states.values[matching.ID()].Value())
	require.NotContains(t, states.values, nonMatching.ID())
	require.NotContains(t, states.values, otherType.ID())
}

func counterEvent(t *testing.T, projectID, eventType string, properties map[string]any) *event.Event {
	t.Helper()
	value, err := event.New("evt_counter", projectID, "player_a", eventType, time.Now(), time.Now(), properties)
	require.NoError(t, err)
	return value
}

type memoryDefinitions struct{ values []*Definition }

func (r *memoryDefinitions) Save(_ context.Context, value *Definition) error {
	r.values = append(r.values, value)
	return nil
}
func (r *memoryDefinitions) ListByProject(_ context.Context, projectID string) ([]*Definition, error) {
	values := make([]*Definition, 0)
	for _, value := range r.values {
		if value.ProjectID() == projectID {
			values = append(values, value)
		}
	}
	return values, nil
}
func (r *memoryDefinitions) ListByEventType(_ context.Context, projectID, eventType string) ([]*Definition, error) {
	values := make([]*Definition, 0)
	for _, value := range r.values {
		if value.ProjectID() == projectID && value.EventType() == eventType {
			values = append(values, value)
		}
	}
	return values, nil
}

type memoryStates struct{ values map[string]*State }

func (r *memoryStates) Increment(_ context.Context, projectID, playerID, counterID string, updatedAt time.Time) (*State, error) {
	value := int64(1)
	if current := r.values[counterID]; current != nil {
		value = current.Value() + 1
	}
	state, err := RestoreState(projectID, playerID, counterID, value, updatedAt)
	if err != nil {
		return nil, err
	}
	r.values[counterID] = state
	return state, nil
}
func (r *memoryStates) ListByPlayer(_ context.Context, projectID, playerID string) ([]*State, error) {
	values := make([]*State, 0)
	for _, value := range r.values {
		if value.ProjectID() == projectID && value.PlayerID() == playerID {
			values = append(values, value)
		}
	}
	return values, nil
}
