package achievement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/stretchr/testify/require"
)

type fakeRepository struct {
	values []*Definition
	err    error
}

func (r *fakeRepository) Save(context.Context, *Definition) error { return nil }
func (r *fakeRepository) GetByID(context.Context, string, string) (*Definition, error) {
	return nil, ErrNotFound
}
func (r *fakeRepository) ListByProject(context.Context, string) ([]*Definition, error) {
	return r.values, nil
}
func (r *fakeRepository) ListByCounter(_ context.Context, projectID, counterID string) ([]*Definition, error) {
	if r.err != nil {
		return nil, r.err
	}
	values := make([]*Definition, 0)
	for _, value := range r.values {
		if value.ProjectID() == projectID && value.CounterID() == counterID {
			values = append(values, value)
		}
	}
	return values, nil
}

type fakeUnlockRepository struct{ values map[string]*Unlock }

func (r *fakeUnlockRepository) SaveIfAbsent(_ context.Context, value *Unlock) (bool, error) {
	key := value.ProjectID() + "/" + value.PlayerID() + "/" + value.AchievementID()
	if r.values == nil {
		r.values = map[string]*Unlock{}
	}
	if _, ok := r.values[key]; ok {
		return false, nil
	}
	r.values[key] = value
	return true, nil
}
func (r *fakeUnlockRepository) ListByPlayer(_ context.Context, projectID, playerID string) ([]*Unlock, error) {
	values := make([]*Unlock, 0)
	for _, value := range r.values {
		if value.ProjectID() == projectID && value.PlayerID() == playerID {
			values = append(values, value)
		}
	}
	return values, nil
}

func TestNewRequiresPositiveTargetAndNormalizes(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 123456789, time.FixedZone("JKT", 7*60*60))
	value, err := New(" achievement_1 ", " proj_1 ", " First ten lessons ", " counter_1 ", 10, now)
	require.NoError(t, err)
	require.Equal(t, "achievement_1", value.ID())
	require.Equal(t, "First ten lessons", value.Name())
	require.Equal(t, int64(10), value.Target())
	require.Equal(t, now.UTC().Truncate(time.Microsecond), value.CreatedAt())
	_, err = New("achievement_1", "proj_1", "invalid", "counter_1", 0, now)
	require.ErrorIs(t, err, ErrInvalidTarget)
	_, err = New("achievement_1", "proj_1", "invalid", "counter_1", -1, now)
	require.ErrorIs(t, err, ErrInvalidTarget)
}

func TestProcessEventUnlocksOnceAtCounterThreshold(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.UTC)
	definition, err := New("achievement_1", "proj_1", "First two lessons", "counter_1", 2, now)
	require.NoError(t, err)
	definitions := &fakeRepository{values: []*Definition{definition}}
	unlocks := &fakeUnlockRepository{}
	newID := func() (string, error) { return "unlock_1", nil }
	event1, err := event.New("evt_1", "proj_1", "player_1", "lesson_completed", now, now, nil)
	require.NoError(t, err)

	require.NoError(t, ProcessEvent(ctx, definitions, unlocks, map[string]int64{"counter_1": 1}, event1, now, newID))
	require.Empty(t, unlocks.values)

	require.NoError(t, ProcessEvent(ctx, definitions, unlocks, map[string]int64{"counter_1": 2}, event1, now, newID))
	require.Len(t, unlocks.values, 1)
	stored := unlocks.values["proj_1/player_1/achievement_1"]
	require.Equal(t, "evt_1", stored.EventID())
	require.Equal(t, int64(2), stored.CounterValue())

	event2, err := event.New("evt_2", "proj_1", "player_1", "lesson_completed", now.Add(time.Minute), now.Add(time.Minute), nil)
	require.NoError(t, err)
	require.NoError(t, ProcessEvent(ctx, definitions, unlocks, map[string]int64{"counter_1": 3}, event2, now.Add(time.Minute), newID))
	require.Len(t, unlocks.values, 1)
	require.Equal(t, "evt_1", unlocks.values["proj_1/player_1/achievement_1"].EventID())
}

func TestProcessEventReturnsPersistenceFailures(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	definition, err := New("achievement_1", "proj_1", "First lesson", "counter_1", 1, now)
	require.NoError(t, err)
	expected := errors.New("database unavailable")
	repository := &fakeRepository{values: []*Definition{definition}, err: expected}
	value, err := event.New("evt_1", "proj_1", "player_1", "lesson_completed", now, now, nil)
	require.NoError(t, err)
	err = ProcessEvent(ctx, repository, &fakeUnlockRepository{}, map[string]int64{"counter_1": 1}, value, now, func() (string, error) { return "unlock_1", nil })
	require.ErrorIs(t, err, expected)
}
