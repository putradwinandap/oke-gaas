package streak

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

type Service struct {
	projects    project.Repository
	players     player.Repository
	definitions Repository
	claims      ClaimRepository
	now         func() time.Time
}

func NewService(projects project.Repository, players player.Repository, definitions Repository, claims ClaimRepository) *Service {
	return NewServiceWithClock(projects, players, definitions, claims, time.Now)
}

// NewServiceWithClock creates a Streak service that uses the supplied clock for active-state calculations.
func NewServiceWithClock(projects project.Repository, players player.Repository, definitions Repository, claims ClaimRepository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{projects: projects, players: players, definitions: definitions, claims: claims, now: now}
}
func (s *Service) Create(ctx context.Context, projectID, name, eventType string, conditions map[string]any) (*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for streak: %w", err)
	}
	id, err := identity.New("streak")
	if err != nil {
		return nil, fmt.Errorf("generate streak id: %w", err)
	}
	value, err := New(id, projectID, name, eventType, conditions, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.definitions.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save streak: %w", err)
	}
	return value, nil
}
func (s *Service) List(ctx context.Context, projectID string) ([]*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for streaks: %w", err)
	}
	values, err := s.definitions.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list streaks: %w", err)
	}
	return values, nil
}
func (s *Service) PlayerProgress(ctx context.Context, projectID, playerID string) ([]State, error) {
	if _, err := s.players.GetByID(ctx, projectID, playerID); err != nil {
		return nil, fmt.Errorf("verify player for streaks: %w", err)
	}
	defs, err := s.definitions.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list player streak definitions: %w", err)
	}
	days, err := s.claims.DaysByPlayer(ctx, projectID, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player streak claims: %w", err)
	}
	now := s.now().UTC()
	values := make([]State, 0, len(defs))
	for _, def := range defs {
		current, latest := CurrentLength(days[def.ID()], now)
		values = append(values, State{Streak: def, Current: current, LatestDay: latest})
	}
	return values, nil
}
