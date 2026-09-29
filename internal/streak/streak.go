package streak

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
	ErrInvalidID         = errors.New("streak id is required")
	ErrInvalidProjectID  = errors.New("streak project id is required")
	ErrInvalidName       = errors.New("streak name is required")
	ErrNameTooLong       = errors.New("streak name must not exceed 255 characters")
	ErrNameTaken         = errors.New("streak name already exists in project")
	ErrInvalidEventType  = errors.New("streak event type is required")
	ErrEventTypeTooLong  = errors.New("streak event type must not exceed 255 characters")
	ErrInvalidConditions = errors.New("streak conditions must be valid JSON")
	ErrInvalidTimestamp  = errors.New("streak timestamp is required")
	ErrInvalidPlayerID   = errors.New("streak player id is required")
	ErrNotFound          = errors.New("streak not found")
)

// Definition is an immutable Project-scoped daily Streak definition.
type Definition struct {
	id, projectID, name, eventType string
	conditions                     map[string]any
	createdAt                      time.Time
}

func New(id, projectID, name, eventType string, conditions map[string]any, createdAt time.Time) (*Definition, error) {
	id, projectID, name, eventType = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name), strings.TrimSpace(eventType)
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
		return nil, ErrInvalidTimestamp
	}
	normalized, err := rule.NormalizeConditions(conditions)
	if err != nil {
		return nil, errors.Join(ErrInvalidConditions, err)
	}
	return &Definition{id: id, projectID: projectID, name: name, eventType: eventType, conditions: normalized, createdAt: createdAt.UTC().Truncate(time.Microsecond)}, nil
}

func Restore(id, projectID, name, eventType string, conditions map[string]any, createdAt time.Time) (*Definition, error) {
	return New(id, projectID, name, eventType, conditions, createdAt)
}
func (d Definition) ID() string                 { return d.id }
func (d Definition) ProjectID() string          { return d.projectID }
func (d Definition) Name() string               { return d.name }
func (d Definition) EventType() string          { return d.eventType }
func (d Definition) Conditions() map[string]any { return rule.CloneConditions(d.conditions) }
func (d Definition) CreatedAt() time.Time       { return d.createdAt }
func (d Definition) Matches(e *event.Event) bool {
	return e != nil && e.ProjectID() == d.projectID && e.Type() == d.eventType && rule.MatchesConditions(d.conditions, e.Properties())
}

type QualifiedDay struct {
	projectID, playerID, streakID, eventID string
	day                                    time.Time
	qualifiedAt                            time.Time
}

func NewQualifiedDay(projectID, playerID, streakID, eventID string, day, qualifiedAt time.Time) (*QualifiedDay, error) {
	projectID, playerID, streakID, eventID = strings.TrimSpace(projectID), strings.TrimSpace(playerID), strings.TrimSpace(streakID), strings.TrimSpace(eventID)
	if projectID == "" || playerID == "" || streakID == "" || eventID == "" || day.IsZero() || qualifiedAt.IsZero() {
		return nil, ErrInvalidTimestamp
	}
	day = day.UTC()
	return &QualifiedDay{projectID: projectID, playerID: playerID, streakID: streakID, eventID: eventID, day: time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC), qualifiedAt: qualifiedAt.UTC().Truncate(time.Microsecond)}, nil
}
func (q QualifiedDay) ProjectID() string      { return q.projectID }
func (q QualifiedDay) PlayerID() string       { return q.playerID }
func (q QualifiedDay) StreakID() string       { return q.streakID }
func (q QualifiedDay) EventID() string        { return q.eventID }
func (q QualifiedDay) Day() time.Time         { return q.day }
func (q QualifiedDay) QualifiedAt() time.Time { return q.qualifiedAt }

type State struct {
	Streak    *Definition
	Current   int64
	LatestDay *time.Time
}
type Repository interface {
	Save(context.Context, *Definition) error
	ListByProject(context.Context, string) ([]*Definition, error)
	ListByEventType(context.Context, string, string) ([]*Definition, error)
}
type ClaimRepository interface {
	Claim(context.Context, *QualifiedDay) (bool, error)
	DaysByPlayer(context.Context, string, string) (map[string][]time.Time, error)
}

// ProcessEvent records one qualifying UTC day per Streak. Both uniqueness keys are enforced by persistence.
func ProcessEvent(ctx context.Context, definitions Repository, claims ClaimRepository, e *event.Event) error {
	if e == nil {
		return errors.New("streak event is required")
	}
	defs, err := definitions.ListByEventType(ctx, e.ProjectID(), e.Type())
	if err != nil {
		return err
	}
	day := e.OccurredAt().UTC()
	day = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	for _, def := range defs {
		if def == nil || !def.Matches(e) {
			continue
		}
		claim, err := NewQualifiedDay(e.ProjectID(), e.PlayerID(), def.ID(), e.ID(), day, e.ReceivedAt())
		if err != nil {
			return err
		}
		if _, err := claims.Claim(ctx, claim); err != nil {
			return err
		}
	}
	return nil
}

// CurrentLength derives the active consecutive run from the persisted qualified UTC day set.
func CurrentLength(days []time.Time, now time.Time) (int64, *time.Time) {
	if now.IsZero() {
		return 0, nil
	}
	today := now.UTC()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	set := make(map[int64]struct{}, len(days))
	var latest time.Time
	for _, d := range days {
		d = d.UTC()
		d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
		set[d.Unix()/86400] = struct{}{}
		if d.After(latest) {
			latest = d
		}
	}
	if latest.IsZero() {
		return 0, nil
	}
	latestKey := latest.Unix() / 86400
	todayKey := today.Unix() / 86400
	if latestKey > todayKey || latestKey < todayKey-1 {
		return 0, &latest
	}
	var count int64
	for key := latestKey; ; key-- {
		if _, ok := set[key]; !ok {
			break
		}
		count++
	}
	return count, &latest
}
