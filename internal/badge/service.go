package badge

import (
	"context"
	"fmt"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
	"time"
)

type Service struct {
	projects    project.Repository
	players     player.Repository
	definitions Repository
	grants      GrantRepository
	now         func() time.Time
}

func NewService(projects project.Repository, players player.Repository, definitions Repository, grants GrantRepository) *Service {
	return &Service{projects: projects, players: players, definitions: definitions, grants: grants, now: time.Now}
}
func (s *Service) Create(ctx context.Context, projectID, name, description string) (*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for badge: %w", err)
	}
	id, err := identity.New("badge")
	if err != nil {
		return nil, fmt.Errorf("generate badge id: %w", err)
	}
	value, err := New(id, projectID, name, description, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.definitions.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save badge: %w", err)
	}
	return value, nil
}
func (s *Service) List(ctx context.Context, projectID string) ([]*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for badges: %w", err)
	}
	return s.definitions.ListByProject(ctx, projectID)
}
func (s *Service) PlayerBadges(ctx context.Context, projectID, playerID string) ([]PlayerBadge, error) {
	if _, err := s.players.GetByID(ctx, projectID, playerID); err != nil {
		return nil, fmt.Errorf("verify player for badges: %w", err)
	}
	grants, err := s.grants.ListByPlayer(ctx, projectID, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player badge grants: %w", err)
	}
	values := make([]PlayerBadge, 0, len(grants))
	for _, grant := range grants {
		definition, err := s.definitions.Get(ctx, projectID, grant.BadgeID())
		if err != nil {
			return nil, fmt.Errorf("load granted badge definition: %w", err)
		}
		values = append(values, PlayerBadge{Definition: definition, Grant: grant})
	}
	return values, nil
}
