package rule

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/putradwinandap/oke-gaas/internal/event"
)

const maxMatchEvery = uint64(1<<63 - 1)

var (
	ErrInvalidID              = errors.New("rule id is required")
	ErrInvalidProjectID       = errors.New("project id is required")
	ErrInvalidVersion         = errors.New("rule version must be greater than zero")
	ErrInvalidEventType       = errors.New("rule event type is required")
	ErrEventTypeTooLong       = errors.New("rule event type must not exceed 255 characters")
	ErrInvalidXPAmount        = errors.New("rule xp amount must be greater than zero")
	ErrInvalidConditions      = errors.New("rule conditions must be valid JSON")
	ErrInvalidMatchEvery      = errors.New("rule match_every must fit a positive signed 64-bit integer")
	ErrIncompatibleTimeWindow = errors.New("once_per_utc_day requires match_every=1")
	ErrAlreadyExists          = errors.New("rule version already exists")
)

// Rule is one immutable version of an exact-event XP rule.
// Conditions optionally require exact matches on top-level Event properties.
type Rule struct {
	id            string
	projectID     string
	version       uint64
	eventType     string
	xpAmount      int64
	conditions    map[string]any
	matchEvery    uint64
	oncePerUTCDay bool
}

// New validates and creates one immutable conditionless Rule version.
func New(id, projectID string, version uint64, eventType string, xpAmount int64) (*Rule, error) {
	return NewConditional(id, projectID, version, eventType, xpAmount, nil)
}

// NewConditional validates and creates one immutable Rule version with optional
// exact top-level property conditions. Every configured condition must match.
func NewConditional(id, projectID string, version uint64, eventType string, xpAmount int64, conditions map[string]any) (*Rule, error) {
	return NewAggregate(id, projectID, version, eventType, xpAmount, conditions, 1)
}

// NewAggregate validates and creates one immutable Rule version with an optional
// count threshold. matchEvery=1 preserves immediate reward behavior.
func NewAggregate(id, projectID string, version uint64, eventType string, xpAmount int64, conditions map[string]any, matchEvery uint64) (*Rule, error) {
	return NewTimed(id, projectID, version, eventType, xpAmount, conditions, matchEvery, false)
}

// NewTimed validates and creates one immutable Rule version with the currently
// supported time-aware option. oncePerUTCDay is intentionally limited to
// immediate rules so this slice does not introduce aggregate/time compositions.
func NewTimed(id, projectID string, version uint64, eventType string, xpAmount int64, conditions map[string]any, matchEvery uint64, oncePerUTCDay bool) (*Rule, error) {
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
	if matchEvery == 0 || matchEvery > maxMatchEvery {
		return nil, ErrInvalidMatchEvery
	}
	if oncePerUTCDay && matchEvery != 1 {
		return nil, ErrIncompatibleTimeWindow
	}

	normalizedConditions, err := normalizeJSONObject(conditions)
	if err != nil {
		return nil, err
	}

	return &Rule{
		id:            id,
		projectID:     projectID,
		version:       version,
		eventType:     eventType,
		xpAmount:      xpAmount,
		conditions:    normalizedConditions,
		matchEvery:    matchEvery,
		oncePerUTCDay: oncePerUTCDay,
	}, nil
}

// Restore reconstructs a persisted Rule version.
func Restore(id, projectID string, version uint64, eventType string, xpAmount int64, conditions map[string]any, matchEvery uint64, oncePerUTCDay bool) (*Rule, error) {
	return NewTimed(id, projectID, version, eventType, xpAmount, conditions, matchEvery, oncePerUTCDay)
}

func (r Rule) ID() string          { return r.id }
func (r Rule) ProjectID() string   { return r.projectID }
func (r Rule) Version() uint64     { return r.version }
func (r Rule) EventType() string   { return r.eventType }
func (r Rule) XPAmount() int64     { return r.xpAmount }
func (r Rule) MatchEvery() uint64  { return r.matchEvery }
func (r Rule) OncePerUTCDay() bool { return r.oncePerUTCDay }

// Conditions returns a deep copy of the exact top-level property conditions.
func (r Rule) Conditions() map[string]any { return cloneJSONObject(r.conditions) }

// Matches reports whether this rule applies to the supplied Event.
func (r Rule) Matches(value *event.Event) bool {
	if value == nil || value.ProjectID() != r.projectID || value.Type() != r.eventType {
		return false
	}
	if len(r.conditions) == 0 {
		return true
	}

	properties := value.Properties()
	for key, expected := range r.conditions {
		actual, ok := properties[key]
		if !ok || !sameJSONValue(expected, actual) {
			return false
		}
	}
	return true
}

// Repository persists immutable rule versions and returns rules applicable to an event type.
type Repository interface {
	Save(ctx context.Context, rule *Rule) error
	ListByEventType(ctx context.Context, projectID, eventType string) ([]*Rule, error)
}

func normalizeJSONObject(value map[string]any) (map[string]any, error) {
	if value == nil {
		return map[string]any{}, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, errors.Join(ErrInvalidConditions, err)
	}

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	var normalized map[string]any
	if err := decoder.Decode(&normalized); err != nil {
		return nil, errors.Join(ErrInvalidConditions, err)
	}
	if normalized == nil {
		normalized = map[string]any{}
	}
	return normalized, nil
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
