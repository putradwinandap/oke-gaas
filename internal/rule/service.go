package rule

import (
	"context"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

type Service struct {
	projects project.Repository
	rules    Repository
	badges   badge.Repository
}

func NewService(projects project.Repository, rules Repository) *Service {
	return &Service{projects: projects, rules: rules}
}

// NewServiceWithBadges enables Badge references for Badge-reward Rules.
func NewServiceWithBadges(projects project.Repository, rules Repository, badges badge.Repository) *Service {
	return &Service{projects: projects, rules: rules, badges: badges}
}

// CreateExactXP creates version 1 of a conditionless exact-event XP rule.
func (s *Service) CreateExactXP(ctx context.Context, projectID, eventType string, xpAmount int64) (*Rule, error) {
	return s.CreateXP(ctx, projectID, eventType, xpAmount, nil)
}

// CreateXP creates version 1 of an exact-event XP rule with optional property conditions.
func (s *Service) CreateXP(ctx context.Context, projectID, eventType string, xpAmount int64, conditions map[string]any) (*Rule, error) {
	return s.CreateAggregateXP(ctx, projectID, eventType, xpAmount, conditions, 1)
}

// CreateAggregateXP creates version 1 of an exact-event XP rule that grants on
// every matchEvery-th matching Event for each Player.
func (s *Service) CreateAggregateXP(ctx context.Context, projectID, eventType string, xpAmount int64, conditions map[string]any, matchEvery uint64) (*Rule, error) {
	return s.CreateTimedXP(ctx, projectID, eventType, xpAmount, conditions, matchEvery, false)
}

// CreateTimedXP creates version 1 of an exact-event XP rule with the currently
// supported time-aware option.
func (s *Service) CreateTimedXP(ctx context.Context, projectID, eventType string, xpAmount int64, conditions map[string]any, matchEvery uint64, oncePerUTCDay bool) (*Rule, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	id, err := identity.New("rule")
	if err != nil {
		return nil, fmt.Errorf("generate rule id: %w", err)
	}
	value, err := NewTimed(id, projectID, 1, eventType, xpAmount, conditions, matchEvery, oncePerUTCDay)
	if err != nil {
		return nil, err
	}
	if err := s.rules.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save rule: %w", err)
	}
	return value, nil
}

// CreateBadge creates a Rule version that grants the specified Project Badge.
func (s *Service) CreateBadge(ctx context.Context, projectID, eventType, badgeID string, conditions map[string]any) (*Rule, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}
	if s.badges == nil {
		return nil, fmt.Errorf("badge repository is required")
	}
	if _, err := s.badges.Get(ctx, projectID, badgeID); err != nil {
		return nil, fmt.Errorf("verify badge belongs to project: %w", err)
	}
	id, err := identity.New("rule")
	if err != nil {
		return nil, fmt.Errorf("generate rule id: %w", err)
	}
	value, err := NewBadge(id, projectID, 1, eventType, badgeID, conditions)
	if err != nil {
		return nil, err
	}
	if err := s.rules.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save rule: %w", err)
	}
	return value, nil
}
