package event

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrInvalidID is returned when an Event has no stable external identity.
	ErrInvalidID = errors.New("event id is required")
	ErrIDTooLong = errors.New("event id must not exceed 255 characters")
	// ErrInvalidProjectID is returned when an Event has no owning Project.
	ErrInvalidProjectID = errors.New("project id is required")
	// ErrInvalidPlayerID is returned when an Event has no target Player.
	ErrInvalidPlayerID = errors.New("player id is required")
	// ErrInvalidType is returned when an Event type is empty.
	ErrInvalidType = errors.New("event type is required")
	ErrTypeTooLong = errors.New("event type must not exceed 255 characters")
	// ErrInvalidOccurredAt is returned when the source occurrence time is missing.
	ErrInvalidOccurredAt = errors.New("event occurred_at is required")
	// ErrInvalidReceivedAt is returned when the server receive time is missing.
	ErrInvalidReceivedAt = errors.New("event received_at is required")
	// ErrInvalidProperties is returned when Event properties cannot be represented as JSON.
	ErrInvalidProperties = errors.New("event properties must be valid JSON")
	// ErrNotFound is returned when an Event is not found within the requested Project.
	ErrNotFound = errors.New("event not found")
	// ErrAlreadyExists indicates that the Project-scoped Event identity is already persisted.
	ErrAlreadyExists = errors.New("event already exists in project")
	// ErrIdentityConflict indicates reuse of a Project-scoped Event identity for different logical data.
	ErrIdentityConflict = errors.New("event identity reused with different data")
	// ErrPlayerNotInProject indicates that the target Player is not owned by the Event's Project.
	ErrPlayerNotInProject = errors.New("player does not belong to project")
)

// Event is an immutable external application event owned by one Project.
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

// New validates and creates an external Event. Identity is scoped by ProjectID + ID.
func New(id, projectID, playerID, eventType string, occurredAt, receivedAt time.Time, properties map[string]any) (*Event, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalidID
	}
	if utf8.RuneCountInString(id) > 255 {
		return nil, ErrIDTooLong
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
	if utf8.RuneCountInString(eventType) > 255 {
		return nil, ErrTypeTooLong
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

// Restore reconstructs a persisted Event while applying the same invariants as New.
func Restore(id, projectID, playerID, eventType string, occurredAt, receivedAt time.Time, properties map[string]any) (*Event, error) {
	return New(id, projectID, playerID, eventType, occurredAt, receivedAt, properties)
}

// ID returns the stable external Event identifier.
func (e Event) ID() string { return e.id }

// ProjectID returns the owning Project identifier.
func (e Event) ProjectID() string { return e.projectID }

// PlayerID returns the target Player identifier.
func (e Event) PlayerID() string { return e.playerID }

// Type returns the external Event type.
func (e Event) Type() string { return e.eventType }

// OccurredAt returns when the integrating application says the Event occurred.
func (e Event) OccurredAt() time.Time { return e.occurredAt }

// ReceivedAt returns when Oke Gaas received the Event.
func (e Event) ReceivedAt() time.Time { return e.receivedAt }

// Properties returns a deep copy of the normalized Event properties.
func (e Event) Properties() map[string]any {
	return cloneJSONObject(e.properties)
}

// PropertiesJSON returns a defensive copy of the JSON representation used for persistence.
func (e Event) PropertiesJSON() []byte { return append([]byte(nil), e.propertiesJSON...) }

// SameLogicalEvent reports whether two Events represent the same idempotent logical input.
// ReceivedAt is intentionally excluded because retries are compared with the originally persisted Event.
func (e Event) SameLogicalEvent(other *Event) bool {
	if other == nil {
		return false
	}
	return e.id == other.id &&
		e.projectID == other.projectID &&
		e.playerID == other.playerID &&
		e.eventType == other.eventType &&
		e.occurredAt.Equal(other.occurredAt) &&
		sameJSONValue(e.properties, other.properties)
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

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
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

func cloneJSONObject(source map[string]any) map[string]any {
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = cloneJSONValue(value)
	}
	return cloned
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneJSONObject(typed)
	case []any:
		cloned := make([]any, len(typed))
		for index, item := range typed {
			cloned[index] = cloneJSONValue(item)
		}
		return cloned
	default:
		return typed
	}
}

func sameJSONValue(left, right any) bool {
	switch leftValue := left.(type) {
	case nil:
		return right == nil
	case bool:
		rightValue, ok := right.(bool)
		return ok && leftValue == rightValue
	case string:
		rightValue, ok := right.(string)
		return ok && leftValue == rightValue
	case json.Number:
		rightValue, ok := right.(json.Number)
		if !ok {
			return false
		}
		leftNumber, leftOK := new(big.Rat).SetString(leftValue.String())
		rightNumber, rightOK := new(big.Rat).SetString(rightValue.String())
		return leftOK && rightOK && leftNumber.Cmp(rightNumber) == 0
	case []any:
		rightValue, ok := right.([]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for index := range leftValue {
			if !sameJSONValue(leftValue[index], rightValue[index]) {
				return false
			}
		}
		return true
	case map[string]any:
		rightValue, ok := right.(map[string]any)
		if !ok || len(leftValue) != len(rightValue) {
			return false
		}
		for key, leftItem := range leftValue {
			rightItem, exists := rightValue[key]
			if !exists || !sameJSONValue(leftItem, rightItem) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Repository persists and retrieves Events using Project-scoped identity.
type Repository interface {
	Save(ctx context.Context, event *Event) error
	GetByID(ctx context.Context, projectID, eventID string) (*Event, error)
}
