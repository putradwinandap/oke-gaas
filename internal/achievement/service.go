package achievement

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/counter"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

// Service coordinates immutable Achievement definitions and Player unlock reads.
type Service struct {
	projects    project.Repository
	players     player.Repository
	counters    counter.Repository
	definitions Repository
	unlocks     UnlockRepository
	now         func() time.Time
}

func NewService(projects project.Repository, players player.Repository, counters counter.Repository, definitions Repository, unlocks UnlockRepository) *Service {
	return &Service{projects: projects, players: players, counters: counters, definitions: definitions, unlocks: unlocks, now: time.Now}
}

// Create adds an immutable Achievement linked to a Counter in the same Project.
func (s *Service) Create(ctx context.Context, projectID, name, counterID string, target int64) (*Definition, error) {
	id, err := identity.New("achievement")
	if err != nil {
		return nil, fmt.Errorf("generate achievement id: %w", err)
	}
	value, err := New(id, projectID, name, counterID, target, s.now())
	if err != nil {
		return nil, err
	}
	if _, err := s.projects.GetByID(ctx, value.ProjectID()); err != nil {
		return nil, fmt.Errorf("verify project for achievement: %w", err)
	}
	counters, err := s.counters.ListByProject(ctx, value.ProjectID())
	if err != nil {
		return nil, fmt.Errorf("list project counters for achievement: %w", err)
	}
	owned := false
	for _, existing := range counters {
		if existing != nil && existing.ID() == value.CounterID() {
			owned = true
			break
		}
	}
	if !owned {
		return nil, counter.ErrCounterNotFound
	}
	if err := s.definitions.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save achievement: %w", err)
	}
	return value, nil
}

// List returns immutable Achievement definitions for a Project.
func (s *Service) List(ctx context.Context, projectID string) ([]*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for achievements: %w", err)
	}
	values, err := s.definitions.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list achievements: %w", err)
	}
	return values, nil
}

// PlayerProgress returns every Project Achievement with its optional unlock record.
func (s *Service) PlayerProgress(ctx context.Context, projectID, playerID string) ([]PlayerAchievement, error) {
	if _, err := s.players.GetByID(ctx, projectID, playerID); err != nil {
		return nil, fmt.Errorf("verify player for achievements: %w", err)
	}
	definitions, err := s.definitions.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list player achievements: %w", err)
	}
	unlocks, err := s.unlocks.ListByPlayer(ctx, projectID, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player achievement unlocks: %w", err)
	}
	byAchievement := make(map[string]*Unlock, len(unlocks))
	for _, unlock := range unlocks {
		if unlock != nil {
			byAchievement[unlock.AchievementID()] = unlock
		}
	}
	values := make([]PlayerAchievement, 0, len(definitions))
	for _, definition := range definitions {
		values = append(values, PlayerAchievement{Achievement: definition, Unlock: byAchievement[definition.ID()]})
	}
	return values, nil
}
