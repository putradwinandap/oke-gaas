package access

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type memoryKeyRepository struct {
	keys map[string]*ProjectKey
}

func newMemoryKeyRepository() *memoryKeyRepository {
	return &memoryKeyRepository{keys: make(map[string]*ProjectKey)}
}

func (r *memoryKeyRepository) Save(_ context.Context, key *ProjectKey) error {
	r.keys[key.ProjectID()] = key
	return nil
}

func (r *memoryKeyRepository) GetByProjectID(_ context.Context, projectID string) (*ProjectKey, error) {
	key, ok := r.keys[projectID]
	if !ok {
		return nil, ErrNotFound
	}
	return key, nil
}

func TestProjectKeyAuthenticatesOnlyMatchingSecretAndProject(t *testing.T) {
	repository := newMemoryKeyRepository()
	key, secret, err := Generate("proj_1")
	require.NoError(t, err)
	require.NotEqual(t, secret, string(key.Hash()))
	require.NoError(t, repository.Save(context.Background(), key))

	service := NewService(repository)
	require.NoError(t, service.Authenticate(context.Background(), "proj_1", secret))
	require.ErrorIs(t, service.Authenticate(context.Background(), "proj_1", secret+"x"), ErrUnauthorized)
	require.ErrorIs(t, service.Authenticate(context.Background(), "proj_2", secret), ErrUnauthorized)
}

func TestGenerateProjectKeyRequiresProject(t *testing.T) {
	key, secret, err := Generate("   ")

	require.ErrorIs(t, err, ErrInvalidProjectID)
	require.Nil(t, key)
	require.Empty(t, secret)
}

func TestServiceAuthenticateRejectsMissingRepositoryWithoutPanic(t *testing.T) {
	service := NewService(nil)

	err := service.Authenticate(context.Background(), "proj_1", "gaas_secret")

	require.Error(t, err)
	require.NotErrorIs(t, err, ErrUnauthorized)
}
