package project

import (
	"context"
	"fmt"
	"time"
)

// Service orchestrates Project application behavior.
type Service struct {
	repository Repository
	now        func() time.Time
}

// NewService creates a Project application service.
func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
		now:        time.Now,
	}
}

// Create creates and persists a Project.
func (s *Service) Create(ctx context.Context, name string) (*Project, error) {
	project, err := New(name, s.now())
	if err != nil {
		return nil, err
	}

	if err := s.repository.Save(ctx, project); err != nil {
		return nil, fmt.Errorf("save project: %w", err)
	}

	return project, nil
}

// Get returns a Project by identifier.
func (s *Service) Get(ctx context.Context, id string) (*Project, error) {
	project, err := s.repository.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	return project, nil
}
