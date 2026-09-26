package project

import (
	"context"
	"testing"

	"github.com/putradwinandap/oke-gaas/internal/access"
	"github.com/stretchr/testify/require"
)

type memoryAccessRepository struct {
	keys map[string]*access.ProjectKey
}

func newMemoryAccessRepository() *memoryAccessRepository {
	return &memoryAccessRepository{keys: make(map[string]*access.ProjectKey)}
}

func (r *memoryAccessRepository) Save(_ context.Context, key *access.ProjectKey) error {
	r.keys[key.ProjectID()] = key
	return nil
}

func (r *memoryAccessRepository) GetByProjectID(_ context.Context, projectID string) (*access.ProjectKey, error) {
	key, ok := r.keys[projectID]
	if !ok {
		return nil, access.ErrNotFound
	}
	return key, nil
}

type memoryProvisionTransactor struct {
	projects *memoryRepository
	keys     *memoryAccessRepository
}

func (t *memoryProvisionTransactor) WithinProvisionTransaction(_ context.Context, fn func(ProvisionWork) error) error {
	return fn(ProvisionWork{Projects: t.projects, Keys: t.keys})
}

func TestProvisionCreatesProjectAndUsableProjectKey(t *testing.T) {
	projects := newMemoryRepository()
	keys := newMemoryAccessRepository()
	service := NewProvisionService(&memoryProvisionTransactor{projects: projects, keys: keys})

	result, err := service.Provision(context.Background(), "Learning")
	require.NoError(t, err)
	require.NotNil(t, result.Project)
	require.NotEmpty(t, result.APIKey)

	persisted, err := projects.GetByID(context.Background(), result.Project.ID())
	require.NoError(t, err)
	require.Equal(t, result.Project.ID(), persisted.ID())

	auth := access.NewService(keys)
	require.NoError(t, auth.Authenticate(context.Background(), result.Project.ID(), result.APIKey))
	require.ErrorIs(t, auth.Authenticate(context.Background(), result.Project.ID(), "wrong"), access.ErrUnauthorized)
}
