package reward

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/rule"
)

// Service evaluates matching rules and records auditable grants.
// Player State mutation intentionally belongs to the next transactional slice.
type Service struct {
	rules  rule.Repository
	grants Repository
	now    func() time.Time
	newID  func() (string, error)
}

func NewService(rules rule.Repository, grants Repository) *Service {
	return &Service{rules: rules, grants: grants, now: time.Now, newID: randomGrantID}
}

// Process evaluates all exact-event rules for one Event and persists one Grant per match.
func (s *Service) Process(ctx context.Context, value *event.Event) ([]*Grant, error) {
	if value == nil {
		return nil, fmt.Errorf("process reward: event is required")
	}
	rules, err := s.rules.ListByEventType(ctx, value.ProjectID(), value.Type())
	if err != nil {
		return nil, fmt.Errorf("list matching rules: %w", err)
	}

	grants := make([]*Grant, 0, len(rules))
	for _, candidate := range rules {
		if candidate == nil || !candidate.Matches(value) {
			continue
		}
		id, err := s.newID()
		if err != nil {
			return nil, fmt.Errorf("create reward grant id: %w", err)
		}
		grant, err := NewGrant(
			id,
			value.ProjectID(),
			value.PlayerID(),
			value.ID(),
			candidate.ID(),
			candidate.Version(),
			candidate.XPAmount(),
			s.now(),
		)
		if err != nil {
			return nil, fmt.Errorf("build reward grant: %w", err)
		}
		if err := s.grants.Save(ctx, grant); err != nil {
			return nil, fmt.Errorf("save reward grant: %w", err)
		}
		grants = append(grants, grant)
	}
	return grants, nil
}

func randomGrantID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return "grant_" + hex.EncodeToString(bytes[:]), nil
}
