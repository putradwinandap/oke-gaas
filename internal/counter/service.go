package counter

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

// Service coordinates Counter definitions and Project-scoped Player progress.
type Service struct {
	projects    project.Repository
	players     player.Repository
	definitions Repository
	states      StateRepository
	now         func() time.Time
}

func NewService(projects project.Repository, players player.Repository, definitions Repository, states StateRepository) *Service {
	return &Service{projects: projects, players: players, definitions: definitions, states: states, now: time.Now}
}

// Create adds one immutable Counter definition to an existing Project.
func (s *Service) Create(ctx context.Context, projectID, name, eventType string, conditions map[string]any) (*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for counter: %w", err)
	}
	id, err := identity.New("counter")
	if err != nil {
		return nil, fmt.Errorf("generate counter id: %w", err)
	}
	value, err := New(id, projectID, name, eventType, conditions, s.now())
	if err != nil {
		return nil, err
	}
	if err := s.definitions.Save(ctx, value); err != nil {
		return nil, fmt.Errorf("save counter: %w", err)
	}
	return value, nil
}

// List returns all immutable Counter definitions for a Project.
func (s *Service) List(ctx context.Context, projectID string) ([]*Definition, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project for counters: %w", err)
	}
	values, err := s.definitions.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list counters: %w", err)
	}
	return values, nil
}

// PlayerProgress returns every Project Counter for a Player, including zero-valued progress.
func (s *Service) PlayerProgress(ctx context.Context, projectID, playerID string) ([]PlayerProgress, error) {
	if _, err := s.players.GetByID(ctx, projectID, playerID); err != nil {
		return nil, fmt.Errorf("verify player for counters: %w", err)
	}
	definitions, err := s.definitions.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list player counter definitions: %w", err)
	}
	states, err := s.states.ListByPlayer(ctx, projectID, playerID)
	if err != nil {
		return nil, fmt.Errorf("list player counter states: %w", err)
	}
	stateByCounter := make(map[string]*State, len(states))
	for _, state := range states {
		stateByCounter[state.CounterID()] = state
	}
	values := make([]PlayerProgress, 0, len(definitions))
	for _, definition := range definitions {
		progress := PlayerProgress{Counter: definition}
		if state, ok := stateByCounter[definition.ID()]; ok {
			progress.Value = state.Value()
			updatedAt := state.UpdatedAt()
			progress.UpdatedAt = &updatedAt
		}
		values = append(values, progress)
	}
	return values, nil
}
