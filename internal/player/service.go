package player

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/project"
)

// Service orchestrates Player application behavior.
type Service struct {
	projects project.Repository
	players  Repository
	now      func() time.Time
}

// NewService creates a Player application service.
func NewService(projects project.Repository, players Repository) *Service {
	return &Service{
		projects: projects,
		players:  players,
		now:      time.Now,
	}
}

// Register creates a Player under exactly one existing Project.
func (s *Service) Register(ctx context.Context, projectID, externalID string) (*Player, error) {
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("verify project: %w", err)
	}

	if _, err := s.players.GetByExternalID(ctx, projectID, externalID); err == nil {
		return nil, ErrExternalIDTaken
	} else if !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("check existing player: %w", err)
	}

	player, err := New(projectID, externalID, s.now())
	if err != nil {
		return nil, err
	}

	if err := s.players.Save(ctx, player); err != nil {
		return nil, fmt.Errorf("save player: %w", err)
	}

	return player, nil
}

// Get returns a Player only when it belongs to the supplied Project.
func (s *Service) Get(ctx context.Context, projectID, playerID string) (*Player, error) {
	player, err := s.players.GetByID(ctx, projectID, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player: %w", err)
	}

	return player, nil
}
