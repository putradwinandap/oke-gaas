package achievement

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/putradwinandap/oke-gaas/internal/event"
)

var (
	ErrInvalidID           = errors.New("achievement id is required")
	ErrInvalidProjectID    = errors.New("achievement project id is required")
	ErrInvalidName         = errors.New("achievement name is required")
	ErrNameTooLong         = errors.New("achievement name must not exceed 255 characters")
	ErrInvalidCounterID    = errors.New("achievement counter id is required")
	ErrCounterIDTooLong    = errors.New("achievement counter id must not exceed 64 characters")
	ErrInvalidTarget       = errors.New("achievement target must be positive")
	ErrNameTaken           = errors.New("achievement name already exists in project")
	ErrNotFound            = errors.New("achievement not found")
	ErrInvalidPlayerID     = errors.New("achievement player id is required")
	ErrInvalidEventID      = errors.New("achievement event id is required")
	ErrInvalidCounterValue = errors.New("achievement counter value must be positive")
	ErrInvalidUnlockedAt   = errors.New("achievement unlocked_at is required")
)

// Definition is an immutable Project-scoped milestone based on one Counter.
type Definition struct {
	id        string
	projectID string
	name      string
	counterID string
	target    int64
	createdAt time.Time
}

// New creates a validated immutable Achievement definition.
func New(id, projectID, name, counterID string, target int64, createdAt time.Time) (*Definition, error) {
	id, projectID = strings.TrimSpace(id), strings.TrimSpace(projectID)
	name, counterID = strings.TrimSpace(name), strings.TrimSpace(counterID)
	switch {
	case id == "":
		return nil, ErrInvalidID
	case projectID == "":
		return nil, ErrInvalidProjectID
	case name == "":
		return nil, ErrInvalidName
	case utf8.RuneCountInString(name) > 255:
		return nil, ErrNameTooLong
	case counterID == "":
		return nil, ErrInvalidCounterID
	case utf8.RuneCountInString(counterID) > 64:
		return nil, ErrCounterIDTooLong
	case target <= 0:
		return nil, ErrInvalidTarget
	case createdAt.IsZero():
		return nil, ErrInvalidUnlockedAt
	}
	return &Definition{
		id: id, projectID: projectID, name: name, counterID: counterID,
		target: target, createdAt: createdAt.UTC().Truncate(time.Microsecond),
	}, nil
}

// Restore reconstructs a persisted immutable Achievement definition.
func Restore(id, projectID, name, counterID string, target int64, createdAt time.Time) (*Definition, error) {
	return New(id, projectID, name, counterID, target, createdAt)
}

func (d Definition) ID() string           { return d.id }
func (d Definition) ProjectID() string    { return d.projectID }
func (d Definition) Name() string         { return d.name }
func (d Definition) CounterID() string    { return d.counterID }
func (d Definition) Target() int64        { return d.target }
func (d Definition) CreatedAt() time.Time { return d.createdAt }

// Unlock is the immutable audit record for a Player crossing an Achievement threshold.
type Unlock struct {
	id            string
	projectID     string
	playerID      string
	achievementID string
	eventID       string
	counterValue  int64
	unlockedAt    time.Time
}

// NewUnlock creates a validated Achievement unlock record.
func NewUnlock(id, projectID, playerID, achievementID, eventID string, counterValue int64, unlockedAt time.Time) (*Unlock, error) {
	id, projectID, playerID = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(playerID)
	achievementID, eventID = strings.TrimSpace(achievementID), strings.TrimSpace(eventID)
	switch {
	case id == "":
		return nil, ErrInvalidID
	case projectID == "":
		return nil, ErrInvalidProjectID
	case playerID == "":
		return nil, ErrInvalidPlayerID
	case achievementID == "":
		return nil, ErrInvalidID
	case eventID == "":
		return nil, ErrInvalidEventID
	case counterValue <= 0:
		return nil, ErrInvalidCounterValue
	case unlockedAt.IsZero():
		return nil, ErrInvalidUnlockedAt
	}
	return &Unlock{
		id: id, projectID: projectID, playerID: playerID, achievementID: achievementID,
		eventID: eventID, counterValue: counterValue,
		unlockedAt: unlockedAt.UTC().Truncate(time.Microsecond),
	}, nil
}

// RestoreUnlock reconstructs a persisted unlock.
func RestoreUnlock(id, projectID, playerID, achievementID, eventID string, counterValue int64, unlockedAt time.Time) (*Unlock, error) {
	return NewUnlock(id, projectID, playerID, achievementID, eventID, counterValue, unlockedAt)
}

func (u Unlock) ID() string            { return u.id }
func (u Unlock) ProjectID() string     { return u.projectID }
func (u Unlock) PlayerID() string      { return u.playerID }
func (u Unlock) AchievementID() string { return u.achievementID }
func (u Unlock) EventID() string       { return u.eventID }
func (u Unlock) CounterValue() int64   { return u.counterValue }
func (u Unlock) UnlockedAt() time.Time { return u.unlockedAt }

// PlayerAchievement combines an immutable definition with its unlock status.
type PlayerAchievement struct {
	Achievement *Definition
	Unlock      *Unlock
}

// Repository persists immutable Achievement definitions.
type Repository interface {
	Save(ctx context.Context, value *Definition) error
	GetByID(ctx context.Context, projectID, id string) (*Definition, error)
	ListByProject(ctx context.Context, projectID string) ([]*Definition, error)
	ListByCounter(ctx context.Context, projectID, counterID string) ([]*Definition, error)
}

// UnlockRepository persists one immutable unlock per Project + Player + Achievement.
type UnlockRepository interface {
	SaveIfAbsent(ctx context.Context, value *Unlock) (bool, error)
	ListByPlayer(ctx context.Context, projectID, playerID string) ([]*Unlock, error)
}

// ProcessEvent unlocks reached Achievements after matching Counters have incremented.
// It runs inside the event-processing transaction, so the Counter and unlock commit together.
func ProcessEvent(ctx context.Context, definitions Repository, unlocks UnlockRepository, counterValues map[string]int64, value *event.Event, unlockedAt time.Time, newID func() (string, error)) error {
	if value == nil {
		return errors.New("achievement event is required")
	}
	for counterID, count := range counterValues {
		if count <= 0 {
			continue
		}
		achievements, err := definitions.ListByCounter(ctx, value.ProjectID(), counterID)
		if err != nil {
			return err
		}
		for _, definition := range achievements {
			if definition == nil || count < definition.Target() {
				continue
			}
			id, err := newID()
			if err != nil {
				return err
			}
			unlock, err := NewUnlock(id, value.ProjectID(), value.PlayerID(), definition.ID(), value.ID(), count, unlockedAt)
			if err != nil {
				return err
			}
			if _, err := unlocks.SaveIfAbsent(ctx, unlock); err != nil {
				return err
			}
		}
	}
	return nil
}
