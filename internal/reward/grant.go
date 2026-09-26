package reward

import (
	"context"
	"errors"
	"strings"
	"time"
)

const TypeXP = "xp"

var (
	ErrInvalidID          = errors.New("reward grant id is required")
	ErrInvalidProjectID   = errors.New("reward grant project id is required")
	ErrInvalidPlayerID    = errors.New("reward grant player id is required")
	ErrInvalidEventID     = errors.New("reward grant event id is required")
	ErrInvalidRuleID      = errors.New("reward grant rule id is required")
	ErrInvalidRuleVersion = errors.New("reward grant rule version must be greater than zero")
	ErrInvalidType        = errors.New("reward grant type is invalid")
	ErrInvalidAmount      = errors.New("reward grant amount must be greater than zero")
	ErrInvalidCreatedAt   = errors.New("reward grant created_at is required")
	ErrAlreadyExists      = errors.New("reward grant already exists")
)

// Grant is an immutable audit record explaining why XP was granted.
type Grant struct {
	id          string
	projectID   string
	playerID    string
	eventID     string
	ruleID      string
	ruleVersion uint64
	rewardType  string
	amount      int64
	createdAt   time.Time
}

func NewGrant(id, projectID, playerID, eventID, ruleID string, ruleVersion uint64, amount int64, createdAt time.Time) (*Grant, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	playerID = strings.TrimSpace(playerID)
	eventID = strings.TrimSpace(eventID)
	ruleID = strings.TrimSpace(ruleID)
	switch {
	case id == "":
		return nil, ErrInvalidID
	case projectID == "":
		return nil, ErrInvalidProjectID
	case playerID == "":
		return nil, ErrInvalidPlayerID
	case eventID == "":
		return nil, ErrInvalidEventID
	case ruleID == "":
		return nil, ErrInvalidRuleID
	case ruleVersion == 0:
		return nil, ErrInvalidRuleVersion
	case amount <= 0:
		return nil, ErrInvalidAmount
	case createdAt.IsZero():
		return nil, ErrInvalidCreatedAt
	}
	return &Grant{
		id: id, projectID: projectID, playerID: playerID, eventID: eventID,
		ruleID: ruleID, ruleVersion: ruleVersion, rewardType: TypeXP,
		amount: amount, createdAt: createdAt.UTC().Truncate(time.Microsecond),
	}, nil
}

func RestoreGrant(id, projectID, playerID, eventID, ruleID string, ruleVersion uint64, rewardType string, amount int64, createdAt time.Time) (*Grant, error) {
	if strings.TrimSpace(rewardType) != TypeXP {
		return nil, ErrInvalidType
	}
	return NewGrant(id, projectID, playerID, eventID, ruleID, ruleVersion, amount, createdAt)
}

func (g Grant) ID() string           { return g.id }
func (g Grant) ProjectID() string    { return g.projectID }
func (g Grant) PlayerID() string     { return g.playerID }
func (g Grant) EventID() string      { return g.eventID }
func (g Grant) RuleID() string       { return g.ruleID }
func (g Grant) RuleVersion() uint64  { return g.ruleVersion }
func (g Grant) Type() string         { return g.rewardType }
func (g Grant) Amount() int64        { return g.amount }
func (g Grant) CreatedAt() time.Time { return g.createdAt }

// Repository stores immutable Reward Grants.
type Repository interface {
	Save(ctx context.Context, grant *Grant) error
	ListByEvent(ctx context.Context, projectID, eventID string) ([]*Grant, error)
}
