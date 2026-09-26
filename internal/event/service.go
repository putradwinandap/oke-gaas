package event

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
)

type Service struct {
	players player.Repository
	events Repository
	now func() time.Time
}

func NewService(players player.Repository, events Repository) *Service {
	return &Service{players: players, events: events, now: time.Now}
}

type IngestCommand struct {
	ID string
	ProjectID string
	PlayerID string
	Type string
	OccurredAt time.Time
	Properties map[string]any
}

type IngestResult struct {
	Event *Event
	Duplicate bool
}

func (s *Service) Ingest(ctx context.Context, command IngestCommand) (*IngestResult, error) {
	candidate, err := New(
		command.ID,
		command.ProjectID,
		command.PlayerID,
		command.Type,
		command.OccurredAt,
		s.now(),
		command.Properties,
	)
	if err != nil { return nil, err }

	if _, err := s.players.GetByID(ctx, candidate.ProjectID(), candidate.PlayerID()); err != nil {
		if errors.Is(err, player.ErrNotFound) { return nil, ErrPlayerNotInProject }
		return nil, fmt.Errorf("verify player ownership: %w", err)
	}

	if err := s.events.Save(ctx, candidate); err == nil {
		return &IngestResult{Event:candidate}, nil
	} else if !errors.Is(err, ErrAlreadyExists) {
		return nil, fmt.Errorf("save event: %w", err)
	}

	existing, err := s.events.GetByID(ctx, candidate.ProjectID(), candidate.ID())
	if err != nil { return nil, fmt.Errorf("load existing event after duplicate: %w", err) }
	if !existing.SameLogicalEvent(candidate) { return nil, ErrIdentityConflict }

	return &IngestResult{Event:existing, Duplicate:true}, nil
}
