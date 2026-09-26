package rule

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/putradwinandap/oke-gaas/internal/event"
)

var (
	ErrInvalidID        = errors.New("rule id is required")
	ErrInvalidProjectID = errors.New("project id is required")
	ErrInvalidVersion   = errors.New("rule version must be greater than zero")
	ErrInvalidEventType = errors.New("rule event type is required")
	ErrEventTypeTooLong = errors.New("rule event type must not exceed 255 characters")
	ErrInvalidXPAmount  = errors.New("rule xp amount must be greater than zero")
	ErrAlreadyExists    = errors.New("rule version already exists")
)

// Rule is one immutable version of an exact-event XP rule.
type Rule struct {
	id        string
	projectID string
	version   uint64
	eventType string
	xpAmount  int64
}

// New validates and creates one immutable Rule version.
func New(id, projectID string, version uint64, eventType string, xpAmount int64) (*Rule, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, ErrInvalidID
	}
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, ErrInvalidProjectID
	}
	if version == 0 {
		return nil, ErrInvalidVersion
	}
	eventType = strings.TrimSpace(eventType)
	if eventType == "" {
		return nil, ErrInvalidEventType
	}
	if utf8.RuneCountInString(eventType) > 255 {
		return nil, ErrEventTypeTooLong
	}
	if xpAmount <= 0 {
		return nil, ErrInvalidXPAmount
	}
	return &Rule{id: id, projectID: projectID, version: version, eventType: eventType, xpAmount: xpAmount}, nil
}

// Restore reconstructs a persisted Rule version.
func Restore(id, projectID string, version uint64, eventType string, xpAmount int64) (*Rule, error) {
	return New(id, projectID, version, eventType, xpAmount)
}

func (r Rule) ID() string        { return r.id }
func (r Rule) ProjectID() string { return r.projectID }
func (r Rule) Version() uint64   { return r.version }
func (r Rule) EventType() string { return r.eventType }
func (r Rule) XPAmount() int64   { return r.xpAmount }

// Matches reports whether this rule applies to the supplied Event.
func (r Rule) Matches(value *event.Event) bool {
	return value != nil && value.ProjectID() == r.projectID && value.Type() == r.eventType
}

// Repository persists immutable rule versions and returns rules applicable to an event type.
type Repository interface {
	Save(ctx context.Context, rule *Rule) error
	ListByEventType(ctx context.Context, projectID, eventType string) ([]*Rule, error)
}
