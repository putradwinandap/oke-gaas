package reward

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/rule"
)

// MatchCounter atomically materializes per-Player progress for aggregate Rule versions.
type MatchCounter interface {
	Increment(ctx context.Context, projectID, playerID, ruleID string, ruleVersion uint64, matchedAt time.Time) (uint64, error)
}

// DailyClaimer atomically claims one UTC calendar day for a Player and Rule version.
type DailyClaimer interface {
	Claim(ctx context.Context, projectID, playerID, ruleID string, ruleVersion uint64, occurredAt time.Time) (bool, error)
}

// Service evaluates matching rules and records auditable grants.
type Service struct {
	rules            rule.Repository
	grants           Repository
	counter          MatchCounter
	dailyClaims      DailyClaimer
	now              func() time.Time
	newID            func() (string, error)
	badgeDefinitions badge.Repository
}

// WithBadgeDefinitions enables Badge rules in the same single Rule evaluation pass.
func (s *Service) WithBadgeDefinitions(definitions badge.Repository) *Service {
	s.badgeDefinitions = definitions
	return s
}

func NewService(rules rule.Repository, grants Repository, counters ...MatchCounter) *Service {
	var counter MatchCounter
	if len(counters) > 0 {
		counter = counters[0]
	}
	return &Service{rules: rules, grants: grants, counter: counter, now: time.Now, newID: randomGrantID}
}

// NewServiceWithDailyClaims wires both aggregate counters and UTC-day claims.
func NewServiceWithDailyClaims(rules rule.Repository, grants Repository, counter MatchCounter, dailyClaims DailyClaimer) *Service {
	return &Service{
		rules: rules, grants: grants, counter: counter, dailyClaims: dailyClaims,
		now: time.Now, newID: randomGrantID,
	}
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
		if candidate.RewardType() == rule.TypeBadge {
			if s.badgeDefinitions == nil {
				return nil, fmt.Errorf("process badge rule %s: badge definitions are required", candidate.ID())
			}
			if _, err := s.badgeDefinitions.Get(ctx, value.ProjectID(), candidate.BadgeID()); err != nil {
				return nil, fmt.Errorf("verify badge rule %s definition: %w", candidate.ID(), err)
			}
			id, err := s.newID()
			if err != nil {
				return nil, fmt.Errorf("create badge grant id: %w", err)
			}
			grant, err := NewBadgeGrant(id, value.ProjectID(), value.PlayerID(), value.ID(), candidate.ID(), candidate.Version(), candidate.BadgeID(), s.now())
			if err != nil {
				return nil, fmt.Errorf("build badge grant: %w", err)
			}
			if err := s.grants.Save(ctx, grant); err != nil {
				if errors.Is(err, ErrAlreadyExists) {
					continue
				}
				return nil, fmt.Errorf("save badge reward grant: %w", err)
			}
			grants = append(grants, grant)
			continue
		}
		if candidate.OncePerUTCDay() {
			if s.dailyClaims == nil {
				return nil, fmt.Errorf("process daily rule %s: daily claimer is required", candidate.ID())
			}
			claimed, err := s.dailyClaims.Claim(
				ctx,
				value.ProjectID(),
				value.PlayerID(),
				candidate.ID(),
				candidate.Version(),
				value.OccurredAt(),
			)
			if err != nil {
				return nil, fmt.Errorf("claim rule %s UTC day: %w", candidate.ID(), err)
			}
			if !claimed {
				continue
			}
		}
		if candidate.MatchEvery() > 1 {
			if s.counter == nil {
				return nil, fmt.Errorf("process aggregate rule %s: match counter is required", candidate.ID())
			}
			count, err := s.counter.Increment(
				ctx,
				value.ProjectID(),
				value.PlayerID(),
				candidate.ID(),
				candidate.Version(),
				value.ReceivedAt(),
			)
			if err != nil {
				return nil, fmt.Errorf("increment rule %s match count: %w", candidate.ID(), err)
			}
			if count%candidate.MatchEvery() != 0 {
				continue
			}
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
