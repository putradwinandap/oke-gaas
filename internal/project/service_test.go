package project

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memoryRepository struct {
	projects map[string]*Project
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{projects: make(map[string]*Project)}
}

func (r *memoryRepository) Save(_ context.Context, project *Project) error {
	r.projects[project.ID()] = project
	return nil
}

func (r *memoryRepository) GetByID(_ context.Context, id string) (*Project, error) {
	project, ok := r.projects[id]
	if !ok {
		return nil, ErrNotFound
	}
	return project, nil
}

func TestServiceCreatePersistsAndRetrievesProject(t *testing.T) {
	repository := newMemoryRepository()
	service := NewService(repository)

	created, err := service.Create(context.Background(), "Acme Learning")
	require.NoError(t, err)
	require.NotEmpty(t, created.ID())
	require.Equal(t, "Acme Learning", created.Name())

	retrieved, err := service.Get(context.Background(), created.ID())
	require.NoError(t, err)
	require.Equal(t, created.ID(), retrieved.ID())
	require.Equal(t, created.Name(), retrieved.Name())
}

func TestNewRejectsBlankProjectName(t *testing.T) {
	project, err := New("   ", serviceTestTime())

	require.ErrorIs(t, err, ErrInvalidName)
	require.Nil(t, project)
}

func serviceTestTime() (zeroTime time.Time) {
	return zeroTime
}
