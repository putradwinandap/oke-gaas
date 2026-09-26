package player

import (
	"context"
	"testing"

	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/stretchr/testify/require"
)

type memoryProjectRepository struct {
	projects map[string]*project.Project
}

func newMemoryProjectRepository() *memoryProjectRepository {
	return &memoryProjectRepository{projects: make(map[string]*project.Project)}
}

func (r *memoryProjectRepository) Save(_ context.Context, value *project.Project) error {
	r.projects[value.ID()] = value
	return nil
}

func (r *memoryProjectRepository) GetByID(_ context.Context, id string) (*project.Project, error) {
	value, ok := r.projects[id]
	if !ok {
		return nil, project.ErrNotFound
	}
	return value, nil
}

type memoryPlayerRepository struct {
	players map[string]*Player
}

func newMemoryPlayerRepository() *memoryPlayerRepository {
	return &memoryPlayerRepository{players: make(map[string]*Player)}
}

func playerKey(projectID, playerID string) string {
	return projectID + ":" + playerID
}

func externalPlayerKey(projectID, externalID string) string {
	return "external:" + projectID + ":" + externalID
}

func (r *memoryPlayerRepository) Save(_ context.Context, value *Player) error {
	externalKey := externalPlayerKey(value.ProjectID(), value.ExternalID())
	if _, exists := r.players[externalKey]; exists {
		return ErrExternalIDTaken
	}

	r.players[playerKey(value.ProjectID(), value.ID())] = value
	r.players[externalKey] = value
	return nil
}

func (r *memoryPlayerRepository) GetByID(_ context.Context, projectID, playerID string) (*Player, error) {
	value, ok := r.players[playerKey(projectID, playerID)]
	if !ok {
		return nil, ErrNotFound
	}
	return value, nil
}

func (r *memoryPlayerRepository) GetByExternalID(_ context.Context, projectID, externalID string) (*Player, error) {
	value, ok := r.players[externalPlayerKey(projectID, externalID)]
	if !ok {
		return nil, ErrNotFound
	}
	return value, nil
}

func createProject(t *testing.T, repository *memoryProjectRepository, name string) *project.Project {
	t.Helper()

	value, err := project.New(name, serviceTestNow())
	require.NoError(t, err)
	require.NoError(t, repository.Save(context.Background(), value))
	return value
}

func TestRegisterScopesExternalIdentifiersPerProject(t *testing.T) {
	projects := newMemoryProjectRepository()
	players := newMemoryPlayerRepository()
	service := NewService(projects, players)

	projectA := createProject(t, projects, "Project A")
	projectB := createProject(t, projects, "Project B")

	playerA, err := service.Register(context.Background(), projectA.ID(), "customer-42")
	require.NoError(t, err)

	playerB, err := service.Register(context.Background(), projectB.ID(), "customer-42")
	require.NoError(t, err)

	require.NotEqual(t, playerA.ID(), playerB.ID())
	require.Equal(t, projectA.ID(), playerA.ProjectID())
	require.Equal(t, projectB.ID(), playerB.ProjectID())
}

func TestRegisterRejectsDuplicateExternalIdentifierWithinProject(t *testing.T) {
	projects := newMemoryProjectRepository()
	players := newMemoryPlayerRepository()
	service := NewService(projects, players)

	projectA := createProject(t, projects, "Project A")

	_, err := service.Register(context.Background(), projectA.ID(), "customer-42")
	require.NoError(t, err)

	duplicate, err := service.Register(context.Background(), projectA.ID(), "customer-42")
	require.ErrorIs(t, err, ErrExternalIDTaken)
	require.Nil(t, duplicate)
}

func TestProjectCannotReadAnotherProjectsPlayer(t *testing.T) {
	projects := newMemoryProjectRepository()
	players := newMemoryPlayerRepository()
	service := NewService(projects, players)

	projectA := createProject(t, projects, "Project A")
	projectB := createProject(t, projects, "Project B")

	registered, err := service.Register(context.Background(), projectA.ID(), "customer-42")
	require.NoError(t, err)

	read, err := service.Get(context.Background(), projectB.ID(), registered.ID())
	require.ErrorIs(t, err, ErrNotFound)
	require.Nil(t, read)
}

func TestRegisterRequiresExistingProject(t *testing.T) {
	service := NewService(newMemoryProjectRepository(), newMemoryPlayerRepository())

	registered, err := service.Register(context.Background(), "proj_missing", "customer-42")

	require.ErrorIs(t, err, project.ErrNotFound)
	require.Nil(t, registered)
}

func serviceTestNow() (zeroTime time.Time) {
	return zeroTime
}
