package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/stretchr/testify/require"
)

type fakePlayerRepository struct {
	players map[string]*player.Player
}

func (r *fakePlayerRepository) Save(_ context.Context, value *player.Player) error {
	if r.players == nil {
		r.players = map[string]*player.Player{}
	}
	r.players[value.ProjectID()+"/"+value.ID()] = value
	return nil
}

func (r *fakePlayerRepository) GetByID(_ context.Context, projectID, playerID string) (*player.Player, error) {
	value, ok := r.players[projectID+"/"+playerID]
	if !ok {
		return nil, player.ErrNotFound
	}
	return value, nil
}

func (r *fakePlayerRepository) GetByExternalID(_ context.Context, projectID, externalID string) (*player.Player, error) {
	for _, value := range r.players {
		if value.ProjectID() == projectID && value.ExternalID() == externalID {
			return value, nil
		}
	}
	return nil, player.ErrNotFound
}

type fakeEventRepository struct {
	events map[string]*Event
}

func (r *fakeEventRepository) Save(_ context.Context, value *Event) error {
	if r.events == nil {
		r.events = map[string]*Event{}
	}
	key := value.ProjectID() + "/" + value.ID()
	if _, exists := r.events[key]; exists {
		return ErrAlreadyExists
	}
	r.events[key] = value
	return nil
}

func (r *fakeEventRepository) GetByID(_ context.Context, projectID, eventID string) (*Event, error) {
	value, ok := r.events[projectID+"/"+eventID]
	if !ok {
		return nil, ErrNotFound
	}
	return value, nil
}

func TestIngestPersistsEventAndMarksRetryAsDuplicate(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	occurredAt := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	target, err := player.Restore("player_1", "proj_1", "customer-1", now)
	require.NoError(t, err)

	players := &fakePlayerRepository{players: map[string]*player.Player{
		"proj_1/player_1": target,
	}}
	events := &fakeEventRepository{}
	service := NewService(players, events)
	service.now = func() time.Time { return now }

	command := IngestCommand{
		ID:         "evt_1",
		ProjectID:  "proj_1",
		PlayerID:   "player_1",
		Type:       "lesson_completed",
		OccurredAt: occurredAt,
		Properties: map[string]any{"lesson_id": "lesson_5"},
	}

	first, err := service.Ingest(context.Background(), command)
	require.NoError(t, err)
	require.False(t, first.Duplicate)
	require.Equal(t, now, first.Event.ReceivedAt())

	second, err := service.Ingest(context.Background(), command)
	require.NoError(t, err)
	require.True(t, second.Duplicate)
	require.Equal(t, first.Event.ReceivedAt(), second.Event.ReceivedAt())
	require.Len(t, events.events, 1)
}

func TestIngestRejectsCrossProjectPlayer(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	target, err := player.Restore("player_1", "proj_a", "customer-1", now)
	require.NoError(t, err)

	service := NewService(
		&fakePlayerRepository{players: map[string]*player.Player{"proj_a/player_1": target}},
		&fakeEventRepository{},
	)

	result, err := service.Ingest(context.Background(), IngestCommand{
		ID:         "evt_1",
		ProjectID:  "proj_b",
		PlayerID:   "player_1",
		Type:       "lesson_completed",
		OccurredAt: now,
	})
	require.ErrorIs(t, err, ErrPlayerNotInProject)
	require.Nil(t, result)
}

func TestIngestRejectsConflictingReuseOfEventIdentity(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	target, err := player.Restore("player_1", "proj_1", "customer-1", now)
	require.NoError(t, err)

	players := &fakePlayerRepository{players: map[string]*player.Player{
		"proj_1/player_1": target,
	}}
	events := &fakeEventRepository{}
	service := NewService(players, events)
	service.now = func() time.Time { return now }

	base := IngestCommand{
		ID:         "evt_1",
		ProjectID:  "proj_1",
		PlayerID:   "player_1",
		Type:       "lesson_completed",
		OccurredAt: now,
		Properties: map[string]any{"lesson_id": "lesson_5"},
	}
	_, err = service.Ingest(context.Background(), base)
	require.NoError(t, err)

	base.Properties = map[string]any{"lesson_id": "lesson_6"}
	result, err := service.Ingest(context.Background(), base)
	require.ErrorIs(t, err, ErrIdentityConflict)
	require.Nil(t, result)
	require.Len(t, events.events, 1)
}

func TestEventIdentityIsProjectScoped(t *testing.T) {
	now := time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)
	repository := &fakeEventRepository{}

	eventA, err := New("evt_same", "proj_a", "player_a", "lesson_completed", now, now, nil)
	require.NoError(t, err)
	eventB, err := New("evt_same", "proj_b", "player_b", "lesson_completed", now, now, nil)
	require.NoError(t, err)

	require.NoError(t, repository.Save(context.Background(), eventA))
	require.NoError(t, repository.Save(context.Background(), eventB))
	require.Len(t, repository.events, 2)
}

func TestIngestPropagatesUnexpectedPlayerRepositoryErrors(t *testing.T) {
	repositoryErr := errors.New("database unavailable")
	players := &failingPlayerRepository{err: repositoryErr}
	service := NewService(players, &fakeEventRepository{})

	result, err := service.Ingest(context.Background(), IngestCommand{})
	require.ErrorIs(t, err, repositoryErr)
	require.Nil(t, result)
}

type failingPlayerRepository struct {
	err error
}

func (r *failingPlayerRepository) Save(context.Context, *player.Player) error { return r.err }
func (r *failingPlayerRepository) GetByID(context.Context, string, string) (*player.Player, error) {
	return nil, r.err
}
func (r *failingPlayerRepository) GetByExternalID(context.Context, string, string) (*player.Player, error) {
	return nil, r.err
}
