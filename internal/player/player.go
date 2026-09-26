package player

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

var (
	// ErrNotFound is returned when a Player is not found within the requested Project.
	ErrNotFound = errors.New("player not found")
	// ErrInvalidProjectID is returned when a Player has no owning Project.
	ErrInvalidProjectID = errors.New("project id is required")
	// ErrInvalidExternalID is returned when a Player has no project-scoped external identifier.
	ErrInvalidExternalID = errors.New("player external id is required")
	ErrExternalIDTooLong = errors.New("player external id must not exceed 255 characters")
	// ErrExternalIDTaken is returned when the same external identifier already exists in a Project.
	ErrExternalIDTaken = errors.New("player external id already exists in project")
)

// Player is a gamified end user owned by exactly one Project.
type Player struct {
	id         string
	projectID  string
	externalID string
	createdAt  time.Time
}

// New creates a Player owned by one Project.
func New(projectID, externalID string, createdAt time.Time) (*Player, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidProjectID
	}

	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, ErrInvalidExternalID
	}
	if utf8.RuneCountInString(externalID) > 255 {
		return nil, ErrExternalIDTooLong
	}

	id, err := identity.New("player")
	if err != nil {
		return nil, err
	}

	return &Player{
		id:         id,
		projectID:  projectID,
		externalID: externalID,
		createdAt:  createdAt.UTC(),
	}, nil
}

// Restore reconstructs a persisted Player.
func Restore(id, projectID, externalID string, createdAt time.Time) (*Player, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("player id is required")
	}
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrInvalidProjectID
	}
	externalID = strings.TrimSpace(externalID)
	if externalID == "" {
		return nil, ErrInvalidExternalID
	}
	if utf8.RuneCountInString(externalID) > 255 {
		return nil, ErrExternalIDTooLong
	}

	return &Player{
		id:         id,
		projectID:  projectID,
		externalID: externalID,
		createdAt:  createdAt.UTC(),
	}, nil
}

// ID returns the Player identifier.
func (p Player) ID() string { return p.id }

// ProjectID returns the immutable owning Project identifier.
func (p Player) ProjectID() string { return p.projectID }

// ExternalID returns the identifier supplied by the integrating Project.
func (p Player) ExternalID() string { return p.externalID }

// CreatedAt returns when the Player was registered.
func (p Player) CreatedAt() time.Time { return p.createdAt }

// Repository persists Players and requires Project scope for reads.
type Repository interface {
	Save(ctx context.Context, player *Player) error
	GetByID(ctx context.Context, projectID, playerID string) (*Player, error)
	GetByExternalID(ctx context.Context, projectID, externalID string) (*Player, error)
}
