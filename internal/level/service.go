package level

import (
	"context"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/project"
)

type Service struct {
	projects project.Repository
	levels   Repository
}

func NewService(projects project.Repository, levels Repository) *Service {
	return &Service{projects: projects, levels: levels}
}

func (s *Service) Append(ctx context.Context, projectID string, minXP int64) (*Threshold, error) {
	if minXP <= 0 {
		return nil, ErrInvalidMinXP
	}
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("get project for level: %w", err)
	}
	value, err := s.levels.Append(ctx, projectID, minXP)
	if err != nil {
		return nil, fmt.Errorf("append level threshold: %w", err)
	}
	return value, nil
}

func (s *Service) List(ctx context.Context, projectID string) ([]*Threshold, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("get project for levels: %w", err)
	}
	values, err := s.levels.List(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list level thresholds: %w", err)
	}
	return values, nil
}

func (s *Service) Resolve(ctx context.Context, projectID string, xp int64) (uint64, error) {
	if xp < 0 {
		return 0, fmt.Errorf("resolve level: xp cannot be negative")
	}
	value, err := s.levels.Resolve(ctx, projectID, xp)
	if err != nil {
		return 0, fmt.Errorf("resolve level: %w", err)
	}
	return value, nil
}
