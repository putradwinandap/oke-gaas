package reward

import (
	"context"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/stretchr/testify/require"
)

type fakeRuleRepository struct{ rules []*rule.Rule }

func (r fakeRuleRepository) Save(context.Context, *rule.Rule) error { return nil }
func (r fakeRuleRepository) ListByEventType(context.Context, string, string) ([]*rule.Rule, error) {
	return r.rules, nil
}

type fakeGrantRepository struct{ saved []*Grant }

func (r *fakeGrantRepository) Save(_ context.Context, grant *Grant) error {
	r.saved = append(r.saved, grant)
	return nil
}
func (r *fakeGrantRepository) ListByEvent(context.Context, string, string) ([]*Grant, error) {
	return r.saved, nil
}

func TestServiceGrantsXPForMatchingExactEventRule(t *testing.T) {
	matching, err := rule.New("rule_lesson", "proj_1", 2, "lesson_completed", 100)
	require.NoError(t, err)
	nonMatching, err := rule.New("rule_login", "proj_1", 1, "daily_login", 25)
	require.NoError(t, err)

	repository := &fakeGrantRepository{}
	service := NewService(fakeRuleRepository{rules: []*rule.Rule{matching, nonMatching}}, repository)
	service.now = func() time.Time { return time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC) }
	service.newID = func() (string, error) { return "grant_1", nil }

	value, err := event.New(
		"evt_1", "proj_1", "player_1", "lesson_completed",
		time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC),
		time.Date(2026, 9, 26, 8, 0, 1, 0, time.UTC),
		nil,
	)
	require.NoError(t, err)

	grants, err := service.Process(context.Background(), value)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, int64(100), grants[0].Amount())
	require.Equal(t, uint64(2), grants[0].RuleVersion())
	require.Equal(t, "rule_lesson", grants[0].RuleID())
	require.Equal(t, "evt_1", grants[0].EventID())
	require.Equal(t, TypeXP, grants[0].Type())
}

func TestServiceDoesNotGrantForNonMatchingEvent(t *testing.T) {
	onlyLogin, err := rule.New("rule_login", "proj_1", 1, "daily_login", 25)
	require.NoError(t, err)
	repository := &fakeGrantRepository{}
	service := NewService(fakeRuleRepository{rules: []*rule.Rule{onlyLogin}}, repository)

	value, err := event.New("evt_1", "proj_1", "player_1", "lesson_completed", time.Now(), time.Now(), nil)
	require.NoError(t, err)

	grants, err := service.Process(context.Background(), value)
	require.NoError(t, err)
	require.Empty(t, grants)
	require.Empty(t, repository.saved)
}
