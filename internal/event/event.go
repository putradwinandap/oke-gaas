package event

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"time"
)

var (
	// ErrInvalidID is returned when an Event has no stable external identity.
	ErrInvalidID = errors.New("event id is required")
	// ErrInvalidProjectID is returned when an Event has no owning Project.
	ErrInvalidProjectID = errors.New("project id is required")
	// ErrInvalidPlayerID is returned when an Event has no target Player.
	ErrInvalidPlayerID = errors.New("player id is required")
	// ErrInvalidType is returned when an Event type is empty.
	ErrInvalidType = errors.New("event type is required")
	// ErrInvalidOccurredAt is returned when occurrence time is missing.
	ErrInvalidOccurredAt = errors.New("event occurred_at is required")
	// ErrNotFound is returned when an Event is not found within the requested Project.
	ErrNotFound = errors.New("event not found")
	// ErrAlreadyExists is returned by persistence when an Event identity already exists in a Project.
	ErrAlreadyExists = errors.New("event already exists in project")
	// ErrIdentityConflict is returned when an existing Event identity is reused with different event data.
	ErrIdentityConflict = errors.New("event identity reused with different data")
	// ErrPlayerNotInProject is returned when the target Player is not owned by the Event Project.
	ErrPlayerNotInProject = errors.New("player does not belong to project")
)

// Event is an external application event owned by exactly one Project.
type Event struct {
	id         string
	projectID  string
	playerID   string
	eventType  string
	occurredAt time.Time
	receivedAt time.Time
	properties map[string]any
}

// New creates an Event with caller-supplied stable identity.
func New(
	id string,
	projectID string,
	playerID string,
	eventType string,
	occurredAt time.Time,
	receivedAt time.Time,
	properties map[string]any,
) (*Event, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalidID
	}

	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidProjectID
	}

	playerID = strings.TrimSpace(playerID)
	if playerID == "" {
		return nil, ErrInvalidPlayerID
	}

	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return nil, ErrInvalidType
	}

	if occurredAt.IsZero() {
		return nil, ErrInvalidOccurredAt
	}
	if receivedAt.IsZero() {
		return nil, errors.New("event received_at is required")
	}

	return &Event{
		id:         id,
		projectID:  projectID,
		playerID:   playerID,
		eventType:  eventType,
		occurredAt: occurredAt.UTC(),
		receivedAt: receivedAt.UTC(),
		properties: cloneProperties(properties),
	}, nil
}

// Restore reconstructs a persisted Event.
func Restore(
	id string,
	projectID string,
	playerID string,
	eventType string,
	occurredAt time.Time,
	receivedAt time.Time,
	properties map[string]any,
) (*Event, error) {
	return New(id, projectID, playerID, eventType, occurredAt, receivedAt, properties)
}

// ID returns the stable Event identity supplied by the integrating application.
func (e Event) ID() string { return e.id }

// ProjectID returns the immutable owning Project identifier.
func (e Event) ProjectID() string { return e.projectID }

// PlayerID returns the target Player identifier.
func (e Event) PlayerID() string { return e.playerID }

// Type returns the external event type.
func (e Event) Type() string { return e.eventType }

// OccurredAt returns when the event happened in the integrating application.
func (e Event) OccurredAt() time.Time { return e.occurredAt }

// ReceivedAt returns when Oke Gaas first accepted the event.
func (e Event) ReceivedAt() time.Time { return e.receivedAt }

// Properties returns a defensive copy of event-specific data.
func (e Event) Properties() map[string]any { return cloneProperties(e.properties) }

// SameLogicalEvent reports whether another Event is the same retry payload.
// ReceivedAt is intentionally excluded because it is server-generated.
func (e Event) SameLogicalEvent(other *Event) bool {
	if other == nil {
		return false
	}
	return e.id == other.id &&
		e.projectID == other.projectID &&
		e.playerID == other.playerID &&
		e.eventType == other.eventType &&
		e.occurredAt.Equal(other.occurredAt) &&
		reflect.DeepEqual(e.properties, other.properties)
}

func cloneProperties(properties map[string]any) map[string]any {
	if properties == nil {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(properties))
	for key, value := range properties {
		cloned[key] = value
	}
	return cloned
}

// Repository persists Events with Project-scoped identity.
type Repository interface {
	Save(ctx context.Context, event *Event) error
	GetByID(ctx context.Context, projectID, eventID string) (*Event, error)
}
