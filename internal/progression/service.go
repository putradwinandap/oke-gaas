package progression

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/achievement"
	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/counter"
	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/player"
	"github.com/putradwinandap/oke-gaas/internal/reward"
	"github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
	"github.com/putradwinandap/oke-gaas/internal/streak"
)

// ProcessingClaims coordinates exactly-once processing attempts for a persisted Event.
// A successful claim lives in the same database transaction as grants and Player State.
type ProcessingClaims interface {
	Claim(ctx context.Context, projectID, eventID string, processedAt time.Time) (bool, error)
}

// Work contains repositories bound to one transaction.
type Work struct {
	Players            player.Repository
	Events             event.Repository
	Rules              rule.Repository
	Grants             reward.Repository
	Counters           reward.MatchCounter
	DailyClaims        reward.DailyClaimer
	CounterDefinitions counter.Repository
	CounterStates      counter.StateRepository
	Achievements       achievement.Repository
	AchievementUnlocks achievement.UnlockRepository
	BadgeDefinitions   badge.Repository
	StreakDefinitions  streak.Repository
	StreakClaims       streak.ClaimRepository
	States             Repository
	Claims             ProcessingClaims
}

// Transactor executes one callback atomically.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(Work) error) error
}

// Service processes the synchronous Event -> Rule -> Reward -> Player State flow.
type Service struct {
	transactions Transactor
}

func NewService(transactions Transactor) *Service {
	return &Service{transactions: transactions}
}

type ProcessResult struct {
	Event     *event.Event
	Grants    []*reward.Grant
	State     *State
	Duplicate bool
}

// Process ingests one Event and atomically persists its Reward Grants and Player State update.
// An identical Event retry returns existing persisted results without increasing XP again.
func (s *Service) Process(ctx context.Context, command event.IngestCommand) (*ProcessResult, error) {
	if s == nil || s.transactions == nil {
		return nil, fmt.Errorf("process progression: transaction boundary is required")
	}

	var result ProcessResult
	if err := s.transactions.WithinTransaction(ctx, func(work Work) error {
		events := event.NewService(work.Players, work.Events)
		ingested, err := events.Ingest(ctx, command)
		if err != nil {
			return fmt.Errorf("ingest event: %w", err)
		}
		result.Event = ingested.Event
		result.Duplicate = ingested.Duplicate

		claimed, err := work.Claims.Claim(
			ctx,
			ingested.Event.ProjectID(),
			ingested.Event.ID(),
			ingested.Event.ReceivedAt(),
		)
		if err != nil {
			return fmt.Errorf("claim event processing: %w", err)
		}
		if !claimed {
			result.Grants, err = work.Grants.ListByEvent(ctx, ingested.Event.ProjectID(), ingested.Event.ID())
			if err != nil {
				return fmt.Errorf("load existing reward grants: %w", err)
			}
			result.State, err = work.States.Get(ctx, ingested.Event.ProjectID(), ingested.Event.PlayerID())
			if err != nil {
				return fmt.Errorf("load existing player state: %w", err)
			}
			return nil
		}

		result.State, err = work.States.Ensure(
			ctx,
			ingested.Event.ProjectID(),
			ingested.Event.PlayerID(),
			ingested.Event.ReceivedAt(),
		)
		if err != nil {
			return fmt.Errorf("materialize player state: %w", err)
		}

		rewards := reward.NewServiceWithDailyClaims(work.Rules, work.Grants, work.Counters, work.DailyClaims)
		if work.BadgeDefinitions != nil {
			rewards.WithBadgeDefinitions(work.BadgeDefinitions)
		}
		result.Grants, err = rewards.Process(ctx, ingested.Event)
		if err != nil {
			return fmt.Errorf("process rewards: %w", err)
		}
		counterValues, err := counter.ProcessEventWithValues(ctx, work.CounterDefinitions, work.CounterStates, ingested.Event)
		if err != nil {
			return fmt.Errorf("process player counters: %w", err)
		}
		if err := achievement.ProcessEvent(ctx, work.Achievements, work.AchievementUnlocks, counterValues, ingested.Event, ingested.Event.ReceivedAt(), func() (string, error) {
			return identity.New("unlock")
		}); err != nil {
			return fmt.Errorf("process achievement unlocks: %w", err)
		}
		if work.StreakDefinitions != nil && work.StreakClaims != nil {
			if err := streak.ProcessEvent(ctx, work.StreakDefinitions, work.StreakClaims, ingested.Event); err != nil {
				return fmt.Errorf("process streaks: %w", err)
			}
		}

		var xp int64
		for _, grant := range result.Grants {
			if grant == nil || grant.Type() != reward.TypeXP {
				continue
			}
			if grant.Amount() > math.MaxInt64-xp {
				return fmt.Errorf("sum reward xp: overflow")
			}
			xp += grant.Amount()
		}
		if xp == 0 {
			return nil
		}

		result.State, err = work.States.AddXP(
			ctx,
			ingested.Event.ProjectID(),
			ingested.Event.PlayerID(),
			xp,
			ingested.Event.ReceivedAt(),
		)
		if err != nil {
			return fmt.Errorf("update player xp state: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &result, nil
}
