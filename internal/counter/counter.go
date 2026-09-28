package counter

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/rule"
)

var (
	ErrInvalidID         = errors.New("counter id is required")
	ErrInvalidProjectID  = errors.New("counter project id is required")
	ErrInvalidPlayerID   = errors.New("counter player id is required")
	ErrInvalidName       = errors.New("counter name is required")
	ErrNameTooLong       = errors.New("counter name must not exceed 255 characters")
	ErrNameTaken         = errors.New("counter name already exists in project")
	ErrInvalidEventType  = errors.New("counter event type is required")
	ErrEventTypeTooLong  = errors.New("counter event type must not exceed 255 characters")
	ErrInvalidConditions = errors.New("counter conditions must be valid JSON")
	ErrInvalidValue      = errors.New("counter value cannot be negative")
	ErrInvalidUpdatedAt  = errors.New("counter updated_at is required")
	ErrValueOverflow     = errors.New("counter value overflow")
	ErrCounterNotFound   = errors.New("counter not found")
)

// Definition is an immutable Project-scoped Counter definition.
type Definition struct {
	id         string
	projectID  string
	name       string
	eventType  string
	conditions map[string]any
	createdAt  time.Time
}

// New creates a Counter definition after validating its public fields.
func New(id, projectID, name, eventType string, conditions map[string]any, createdAt time.Time) (*Definition, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	name = strings.TrimSpace(name)
	eventType = strings.TrimSpace(eventType)
	switch {
	case id == "":
		return nil, ErrInvalidID
	case projectID == "":
		return nil, ErrInvalidProjectID
	case name == "":
		return nil, ErrInvalidName
	case utf8.RuneCountInString(name) > 255:
		return nil, ErrNameTooLong
	case eventType == "":
		return nil, ErrInvalidEventType
	case utf8.RuneCountInString(eventType) > 255:
		return nil, ErrEventTypeTooLong
	case createdAt.IsZero():
		return nil, ErrInvalidUpdatedAt
	}

	normalized, err := rule.NormalizeConditions(conditions)
	if err != nil {
		return nil, errors.Join(ErrInvalidConditions, err)
	}
	return &Definition{
		id:         id,
		projectID:  projectID,
		name:       name,
		eventType:  eventType,
		conditions: normalized,
		createdAt:  createdAt.UTC().Truncate(time.Microsecond),
	}, nil
}

// Restore reconstructs a persisted Counter definition.
func Restore(id, projectID, name, eventType string, conditions map[string]any, createdAt time.Time) (*Definition, error) {
	return New(id, projectID, name, eventType, conditions, createdAt)
}

func (d Definition) ID() string                 { return d.id }
func (d Definition) ProjectID() string          { return d.projectID }
func (d Definition) Name() string               { return d.name }
func (d Definition) EventType() string          { return d.eventType }
func (d Definition) CreatedAt() time.Time       { return d.createdAt }
func (d Definition) Conditions() map[string]any { return rule.CloneConditions(d.conditions) }

// Matches reports whether an Event matches this Counter's Project, type, and conditions.
func (d Definition) Matches(value *event.Event) bool {
	return value != nil &&
		value.ProjectID() == d.projectID &&
		value.Type() == d.eventType &&
		rule.MatchesConditions(d.conditions, value.Properties())
}

// State is the current materialized value of one Player Counter.
type State struct {
	projectID string
	playerID  string
	counterID string
	value     int64
	updatedAt time.Time
}

// RestoreState reconstructs persisted Player Counter progress.
func RestoreState(projectID, playerID, counterID string, value int64, updatedAt time.Time) (*State, error) {
	projectID = strings.TrimSpace(projectID)
	playerID = strings.TrimSpace(playerID)
	counterID = strings.TrimSpace(counterID)
	switch {
	case projectID == "":
		return nil, ErrInvalidProjectID
	case playerID == "":
		return nil, ErrInvalidPlayerID
	case counterID == "":
		return nil, ErrInvalidID
	case value < 0:
		return nil, ErrInvalidValue
	case updatedAt.IsZero():
		return nil, ErrInvalidUpdatedAt
	}
	return &State{
		projectID: projectID,
		playerID:  playerID,
		counterID: counterID,
		value:     value,
		updatedAt: updatedAt.UTC().Truncate(time.Microsecond),
	}, nil
}

func (s State) ProjectID() string    { return s.projectID }
func (s State) PlayerID() string     { return s.playerID }
func (s State) CounterID() string    { return s.counterID }
func (s State) Value() int64         { return s.value }
func (s State) UpdatedAt() time.Time { return s.updatedAt }

// PlayerProgress combines an immutable Counter definition with its current value.
// A nil UpdatedAt means the Player has not matched the Counter yet.
type PlayerProgress struct {
	Counter   *Definition
	Value     int64
	UpdatedAt *time.Time
}

// Repository persists immutable Counter definitions.
type Repository interface {
	Save(ctx context.Context, value *Definition) error
	ListByProject(ctx context.Context, projectID string) ([]*Definition, error)
	ListByEventType(ctx context.Context, projectID, eventType string) ([]*Definition, error)
}

// StateRepository persists current materialized Player Counter values.
type StateRepository interface {
	Increment(ctx context.Context, projectID, playerID, counterID string, updatedAt time.Time) (*State, error)
	ListByPlayer(ctx context.Context, projectID, playerID string) ([]*State, error)
}

// ProcessEvent increments every matching Counter within the caller's transaction.
func ProcessEvent(ctx context.Context, definitions Repository, states StateRepository, value *event.Event) error {
	if value == nil {
		return errors.New("counter event is required")
	}
	matching, err := definitions.ListByEventType(ctx, value.ProjectID(), value.Type())
	if err != nil {
		return err
	}
	for _, definition := range matching {
		if definition == nil || !definition.Matches(value) {
			continue
		}
		if _, err := states.Increment(ctx, value.ProjectID(), value.PlayerID(), definition.ID(), value.ReceivedAt()); err != nil {
			return err
		}
	}
	return nil
}
