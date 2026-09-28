package counter

import (
	"context"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/stretchr/testify/require"
)

func TestServiceCreatesListsAndReturnsZeroAndCurrentPlayerProgress(t *testing.T) {
	ctx := context.Background()
	projects := &memoryProjectRepository{values: make(map[string]*project.Project)}
	players := &memoryPlayerRepository{values: make(map[string]*player.Player)}
	definitions := &memoryDefinitions{}
	states := &memoryStates{values: make(map[string]*State)}

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, proj))
	pl, err := player.New(proj.ID(), "student-1", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, pl))

	service := NewService(projects, players, definitions, states)
	created, err := service.Create(ctx, proj.ID(), "lessons_completed", "lesson_completed", nil)
	require.NoError(t, err)
	require.Contains(t, created.ID(), "counter_")

	listed, err := service.List(ctx, proj.ID())
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, created.ID(), listed[0].ID())

	initial, err := service.PlayerProgress(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, initial, 1)
	require.Zero(t, initial[0].Value)
	require.Nil(t, initial[0].UpdatedAt)

	updatedAt := time.Now()
	_, err = states.Increment(ctx, proj.ID(), pl.ID(), created.ID(), updatedAt)
	require.NoError(t, err)
	progress, err := service.PlayerProgress(ctx, proj.ID(), pl.ID())
	require.NoError(t, err)
	require.Len(t, progress, 1)
	require.Equal(t, int64(1), progress[0].Value)
	require.NotNil(t, progress[0].UpdatedAt)
}

func TestServiceRejectsCrossProjectPlayerProgress(t *testing.T) {
	ctx := context.Background()
	projects := &memoryProjectRepository{values: make(map[string]*project.Project)}
	players := &memoryPlayerRepository{values: make(map[string]*player.Player)}
	projA, err := project.New("Project A", time.Now())
	require.NoError(t, err)
	projB, err := project.New("Project B", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(ctx, projA))
	require.NoError(t, projects.Save(ctx, projB))
	pl, err := player.New(projA.ID(), "student-1", time.Now())
	require.NoError(t, err)
	require.NoError(t, players.Save(ctx, pl))

	service := NewService(projects, players, &memoryDefinitions{}, &memoryStates{values: make(map[string]*State)})
	progress, err := service.PlayerProgress(ctx, projB.ID(), pl.ID())
	require.ErrorIs(t, err, player.ErrNotFound)
	require.Nil(t, progress)
}

type memoryProjectRepository struct{ values map[string]*project.Project }

func (r *memoryProjectRepository) Save(_ context.Context, value *project.Project) error {
	r.values[value.ID()] = value
	return nil
}
func (r *memoryProjectRepository) GetByID(_ context.Context, id string) (*project.Project, error) {
	value, ok := r.values[id]
	if !ok {
		return nil, project.ErrNotFound
	}
	return value, nil
}

type memoryPlayerRepository struct{ values map[string]*player.Player }

func (r *memoryPlayerRepository) Save(_ context.Context, value *player.Player) error {
	r.values[value.ID()] = value
	return nil
}
func (r *memoryPlayerRepository) GetByID(_ context.Context, projectID, playerID string) (*player.Player, error) {
	value, ok := r.values[playerID]
	if !ok || value.ProjectID() != projectID {
		return nil, player.ErrNotFound
	}
	return value, nil
}
func (r *memoryPlayerRepository) GetByExternalID(_ context.Context, projectID, externalID string) (*player.Player, error) {
	for _, value := range r.values {
		if value.ProjectID() == projectID && value.ExternalID() == externalID {
			return value, nil
		}
	}
	return nil, player.ErrNotFound
}
