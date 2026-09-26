package event

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
)

// Service orchestrates external Event ingestion.
type Service struct {
	players player.Repository
	events  Repository
	now     func() time.Time
}

// NewService creates an Event ingestion service.
func NewService(players player.Repository, events Repository) *Service {
	return &Service{players: players, events: events, now: time.Now}
}

// IngestCommand contains the caller-supplied logical Event data.
type IngestCommand struct {
	ID         string
	ProjectID  string
	PlayerID   string
	Type       string
	OccurredAt time.Time
	Properties map[string]any
}

// IngestResult contains the persisted Event and whether the call was an idempotent retry.
type IngestResult struct {
	Event     *Event
	Duplicate bool
}

// Ingest validates Player ownership, persists a new Event, or resolves an identical retry.
// Reusing the same Project + Event identity with different logical data returns ErrIdentityConflict.
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
	if err != nil {
		return nil, err
	}

	if _, err := s.players.GetByID(ctx, candidate.ProjectID(), candidate.PlayerID()); err != nil {
		if errors.Is(err, player.ErrNotFound) {
			return nil, ErrPlayerNotInProject
		}
		return nil, fmt.Errorf("verify player ownership: %w", err)
	}

	if err := s.events.Save(ctx, candidate); err == nil {
		return &IngestResult{Event: candidate}, nil
	} else if !errors.Is(err, ErrAlreadyExists) {
		return nil, fmt.Errorf("save event: %w", err)
	}

	existing, err := s.events.GetByID(ctx, candidate.ProjectID(), candidate.ID())
	if err != nil {
		return nil, fmt.Errorf("load existing event after duplicate: %w", err)
	}
	if !existing.SameLogicalEvent(candidate) {
		return nil, ErrIdentityConflict
	}

	return &IngestResult{Event: existing, Duplicate: true}, nil
}
