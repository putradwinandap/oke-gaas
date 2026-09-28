package counter

import (
	"context"
	"errors"

	"github.com/putradwinandap/oke-gaas/internal/event"
)

// ProcessEvent increments every matching Counter in the caller's transaction.
func ProcessEvent(ctx context.Context, definitions Repository, states StateRepository, value *event.Event) error {
	_, err := ProcessEventWithValues(ctx, definitions, states, value)
	return err
}

// ProcessEventWithValues increments matching Counters and returns their post-increment values.
func ProcessEventWithValues(ctx context.Context, definitions Repository, states StateRepository, value *event.Event) (map[string]int64, error) {
	if value == nil {
		return nil, errors.New("counter event is required")
	}
	matching, err := definitions.ListByEventType(ctx, value.ProjectID(), value.Type())
	if err != nil {
		return nil, err
	}
	values := make(map[string]int64, len(matching))
	for _, definition := range matching {
		if definition == nil || !definition.Matches(value) {
			continue
		}
		state, err := states.Increment(ctx, value.ProjectID(), value.PlayerID(), definition.ID(), value.ReceivedAt())
		if err != nil {
			return nil, err
		}
		if state == nil {
			return nil, errors.New("counter increment returned no state")
		}
		values[definition.ID()] = state.Value()
	}
	return values, nil
}
