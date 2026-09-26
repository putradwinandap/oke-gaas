package rule

import (
	"context"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

type Service struct {
	projects project.Repository
	rules    Repository
}

func NewService(projects project.Repository, rules Repository) *Service {
	return &Service{projects: projects, rules: rules}
}

// CreateExactXP creates version 1 of the smallest supported exact-event XP rule.
func (s *Service) CreateExactXP(ctx context.Context, projectID, eventType string, xpAmount int64) (*Rule, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	id, err := identity.New("rule")
	if err != nil {
		return nil, fmt.Errorf("generate rule id: %w", err)
	}
	value, err := New(id, projectID, 1, eventType, xpAmount)
	if err != nil {
		return nil, err
	}
	if err := s.rules.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save rule: %w", err)
	}
	return value, nil
}
