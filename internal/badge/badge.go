package badge

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidID          = errors.New("badge id is required")
	ErrInvalidProjectID   = errors.New("badge project id is required")
	ErrInvalidName        = errors.New("badge name is required")
	ErrNameTooLong        = errors.New("badge name must not exceed 255 characters")
	ErrDescriptionTooLong = errors.New("badge description must not exceed 1024 characters")
	ErrAlreadyExists      = errors.New("badge already exists")
	ErrNotFound           = errors.New("badge not found")
	ErrInvalidPlayerID    = errors.New("badge player id is required")
)

// Definition is an immutable, Project-scoped collectible Badge.
type Definition struct {
	id, projectID, name, description string
	createdAt                        time.Time
}

func New(id, projectID, name, description string, createdAt time.Time) (*Definition, error) {
	id, projectID = strings.TrimSpace(id), strings.TrimSpace(projectID)
	name, description = strings.TrimSpace(name), strings.TrimSpace(description)
	switch {
	case id == "":
		return nil, ErrInvalidID
	case projectID == "":
		return nil, ErrInvalidProjectID
	case name == "":
		return nil, ErrInvalidName
	case utf8.RuneCountInString(name) > 255:
		return nil, ErrNameTooLong
	case utf8.RuneCountInString(description) > 1024:
		return nil, ErrDescriptionTooLong
	case createdAt.IsZero():
		return nil, errors.New("badge creation time is required")
	}
	return &Definition{id: id, projectID: projectID, name: name, description: description, createdAt: createdAt.UTC().Truncate(time.Microsecond)}, nil
}
func (d Definition) ID() string           { return d.id }
func (d Definition) ProjectID() string    { return d.projectID }
func (d Definition) Name() string         { return d.name }
func (d Definition) Description() string  { return d.description }
func (d Definition) CreatedAt() time.Time { return d.createdAt }

// Grant records a Player's unique ownership of one Badge.
type Grant struct {
	projectID, playerID, badgeID, eventID, ruleID string
	ruleVersion                                   uint64
	grantedAt                                     time.Time
}

// PlayerBadge combines the immutable Badge definition and its first grant audit.
type PlayerBadge struct {
	Definition *Definition
	Grant      *Grant
}

func NewGrant(projectID, playerID, badgeID, eventID, ruleID string, version uint64, grantedAt time.Time) (*Grant, error) {
	projectID, playerID, badgeID = strings.TrimSpace(projectID), strings.TrimSpace(playerID), strings.TrimSpace(badgeID)
	eventID, ruleID = strings.TrimSpace(eventID), strings.TrimSpace(ruleID)
	switch {
	case projectID == "":
		return nil, ErrInvalidProjectID
	case playerID == "":
		return nil, ErrInvalidPlayerID
	case badgeID == "" || eventID == "" || ruleID == "" || version == 0 || grantedAt.IsZero():
		return nil, errors.New("badge grant audit fields are required")
	}
	return &Grant{projectID: projectID, playerID: playerID, badgeID: badgeID, eventID: eventID, ruleID: ruleID, ruleVersion: version, grantedAt: grantedAt.UTC().Truncate(time.Microsecond)}, nil
}
func (g Grant) ProjectID() string    { return g.projectID }
func (g Grant) PlayerID() string     { return g.playerID }
func (g Grant) BadgeID() string      { return g.badgeID }
func (g Grant) EventID() string      { return g.eventID }
func (g Grant) RuleID() string       { return g.ruleID }
func (g Grant) RuleVersion() uint64  { return g.ruleVersion }
func (g Grant) GrantedAt() time.Time { return g.grantedAt }

type Repository interface {
	Save(context.Context, *Definition) error
	ListByProject(context.Context, string) ([]*Definition, error)
	Get(context.Context, string, string) (*Definition, error)
}
type GrantRepository interface {
	ListByPlayer(context.Context, string, string) ([]*Grant, error)
}
