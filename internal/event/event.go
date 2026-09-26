package event

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidID          = errors.New("event id is required")
	ErrInvalidProjectID   = errors.New("project id is required")
	ErrInvalidPlayerID    = errors.New("player id is required")
	ErrInvalidType        = errors.New("event type is required")
	ErrInvalidOccurredAt  = errors.New("event occurred_at is required")
	ErrInvalidReceivedAt  = errors.New("event received_at is required")
	ErrInvalidProperties  = errors.New("event properties must be valid JSON")
	ErrNotFound           = errors.New("event not found")
	ErrAlreadyExists      = errors.New("event already exists in project")
	ErrIdentityConflict   = errors.New("event identity reused with different data")
	ErrPlayerNotInProject = errors.New("player does not belong to project")
)

type Event struct {
	id             string
	projectID      string
	playerID       string
	eventType      string
	occurredAt     time.Time
	receivedAt     time.Time
	properties     map[string]any
	propertiesJSON []byte
}

func New(id, projectID, playerID, eventType string, occurredAt, receivedAt time.Time, properties map[string]any) (*Event, error) {
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
		return nil, ErrInvalidReceivedAt
	}

	normalized, canonical, err := normalizeProperties(properties)
	if err != nil {
		return nil, err
	}

	return &Event{
		id:             id,
		projectID:      projectID,
		playerID:       playerID,
		eventType:      eventType,
		occurredAt:     normalizeDatabaseTime(occurredAt),
		receivedAt:     normalizeDatabaseTime(receivedAt),
		properties:     normalized,
		propertiesJSON: canonical,
	}, nil
}

func Restore(id, projectID, playerID, eventType string, occurredAt, receivedAt time.Time, properties map[string]any) (*Event, error) {
	return New(id, projectID, playerID, eventType, occurredAt, receivedAt, properties)
}

func (e Event) ID() string            { return e.id }
func (e Event) ProjectID() string     { return e.projectID }
func (e Event) PlayerID() string      { return e.playerID }
func (e Event) Type() string          { return e.eventType }
func (e Event) OccurredAt() time.Time { return e.occurredAt }
func (e Event) ReceivedAt() time.Time { return e.receivedAt }

func (e Event) Properties() map[string]any {
	normalized, _, err := normalizeProperties(e.properties)
	if err != nil {
		panic("event contains invalid normalized properties")
	}
	return normalized
}

func (e Event) PropertiesJSON() []byte { return append([]byte(nil), e.propertiesJSON...) }

func (e Event) SameLogicalEvent(other *Event) bool {
	if other == nil {
		return false
	}
	return e.id == other.id &&
		e.projectID == other.projectID &&
		e.playerID == other.playerID &&
		e.eventType == other.eventType &&
		e.occurredAt.Equal(other.occurredAt) &&
		bytes.Equal(e.propertiesJSON, other.propertiesJSON)
}

func normalizeDatabaseTime(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func normalizeProperties(properties map[string]any) (map[string]any, []byte, error) {
	if properties == nil {
		properties = map[string]any{}
	}
	raw, err := json.Marshal(properties)
	if err != nil {
		return nil, nil, errors.Join(ErrInvalidProperties, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized map[string]any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, nil, errors.Join(ErrInvalidProperties, err)
	}
	if normalized == nil {
		normalized = map[string]any{}
	}
	canonical, err := json.Marshal(normalized)
	if err != nil {
		return nil, nil, errors.Join(ErrInvalidProperties, err)
	}
	return normalized, canonical, nil
}

type Repository interface {
	Save(ctx context.Context, event *Event) error
	GetByID(ctx context.Context, projectID, eventID string) (*Event, error)
}
