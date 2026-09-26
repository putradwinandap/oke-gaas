package progression

import (
	"context"
	"errors"
	"strings"
	"time"
)

var (
	ErrStateNotFound     = errors.New("player state not found")
	ErrInvalidProjectID  = errors.New("player state project id is required")
	ErrInvalidPlayerID   = errors.New("player state player id is required")
	ErrInvalidXP         = errors.New("player state xp cannot be negative")
	ErrInvalidUpdatedAt  = errors.New("player state updated_at is required")
	ErrInvalidXPIncrease = errors.New("player state xp increase must be greater than zero")
)

// State is the materialized current gamification state for one Player.
type State struct {
	projectID string
	playerID  string
	xp        int64
	updatedAt time.Time
}

func RestoreState(projectID, playerID string, xp int64, updatedAt time.Time) (*State, error) {
	projectID = strings.TrimSpace(projectID)
	playerID = strings.TrimSpace(playerID)
	switch {
	case projectID == "":
		return nil, ErrInvalidProjectID
	case playerID == "":
		return nil, ErrInvalidPlayerID
	case xp < 0:
		return nil, ErrInvalidXP
	case updatedAt.IsZero():
		return nil, ErrInvalidUpdatedAt
	}
	return &State{
		projectID: projectID,
		playerID:  playerID,
		xp:        xp,
		updatedAt: updatedAt.UTC().Truncate(time.Microsecond),
	}, nil
}

func (s State) ProjectID() string    { return s.projectID }
func (s State) PlayerID() string     { return s.playerID }
func (s State) XP() int64            { return s.xp }
func (s State) UpdatedAt() time.Time { return s.updatedAt }

// Repository persists materialized Player State within Project scope.
type Repository interface {
	Ensure(ctx context.Context, projectID, playerID string, updatedAt time.Time) (*State, error)
	AddXP(ctx context.Context, projectID, playerID string, amount int64, updatedAt time.Time) (*State, error)
	Get(ctx context.Context, projectID, playerID string) (*State, error)
}
