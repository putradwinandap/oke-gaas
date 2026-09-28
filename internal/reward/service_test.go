package reward

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/badge"
	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/putradwinandap/oke-gaas/internal/rule"
	"github.com/stretchr/testify/require"
)

type fakeRuleRepository struct{ rules []*rule.Rule }

func (r fakeRuleRepository) Save(context.Context, *rule.Rule) error { return nil }
func (r fakeRuleRepository) ListByEventType(context.Context, string, string) ([]*rule.Rule, error) {
	return r.rules, nil
}

type fakeMatchCounter struct {
	count uint64
}

func (c *fakeMatchCounter) Increment(context.Context, string, string, string, uint64, time.Time) (uint64, error) {
	c.count++
	return c.count, nil
}

type fakeDailyClaimer struct {
	claimed map[string]bool
}

func (c *fakeDailyClaimer) Claim(_ context.Context, _, _, _ string, _ uint64, occurredAt time.Time) (bool, error) {
	if c.claimed == nil {
		c.claimed = make(map[string]bool)
	}
	day := occurredAt.UTC().Format("2006-01-02")
	if c.claimed[day] {
		return false, nil
	}
	c.claimed[day] = true
	return true, nil
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

func TestServiceGrantsBadgeWithRuleAudit(t *testing.T) {
	badgeRule, err := rule.NewBadge("rule_badge", "proj_1", 3, "lesson_completed", "badge_1", map[string]any{"course": "go"})
	require.NoError(t, err)
	repository := &fakeGrantRepository{}
	definitions := fakeBadgeRepository{}
	service := NewService(fakeRuleRepository{rules: []*rule.Rule{badgeRule}}, repository).WithBadgeDefinitions(definitions)
	service.now = func() time.Time { return time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC) }
	service.newID = func() (string, error) { return "grant_badge_1", nil }
	value, err := event.New("evt_badge", "proj_1", "player_1", "lesson_completed", time.Now(), time.Now(), map[string]any{"course": "go"})
	require.NoError(t, err)
	grants, err := service.Process(context.Background(), value)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, TypeBadge, grants[0].Type())
	require.Equal(t, "badge_1", grants[0].BadgeID())
	require.Zero(t, grants[0].Amount())
	require.Equal(t, "evt_badge", grants[0].EventID())
	require.Equal(t, uint64(3), grants[0].RuleVersion())
}

type fakeBadgeRepository struct{}

func (fakeBadgeRepository) Save(context.Context, *badge.Definition) error { return nil }
func (fakeBadgeRepository) ListByProject(context.Context, string) ([]*badge.Definition, error) {
	return nil, nil
}
func (fakeBadgeRepository) Get(_ context.Context, projectID, id string) (*badge.Definition, error) {
	return badge.New(id, projectID, "Test badge", "", time.Now())
}

func TestServiceGrantsOnlyOnAggregateThreshold(t *testing.T) {
	aggregate, err := rule.NewAggregate("rule_streak", "proj_1", 1, "lesson_completed", 250, nil, 3)
	require.NoError(t, err)

	repository := &fakeGrantRepository{}
	counter := &fakeMatchCounter{}
	service := NewService(fakeRuleRepository{rules: []*rule.Rule{aggregate}}, repository, counter)
	service.now = func() time.Time { return time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC) }
	service.newID = func() (string, error) { return "grant_threshold", nil }

	for index := 1; index <= 3; index++ {
		value, err := event.New(
			fmt.Sprintf("evt_%d", index), "proj_1", "player_1", "lesson_completed",
			time.Now(), time.Now(), nil,
		)
		require.NoError(t, err)
		grants, err := service.Process(context.Background(), value)
		require.NoError(t, err)
		if index < 3 {
			require.Empty(t, grants)
		} else {
			require.Len(t, grants, 1)
			require.Equal(t, int64(250), grants[0].Amount())
		}
	}
	require.Equal(t, uint64(3), counter.count)
	require.Len(t, repository.saved, 1)
}

func TestServiceGrantsDailyRuleAtMostOncePerUTCDay(t *testing.T) {
	daily, err := rule.NewTimed("rule_daily", "proj_1", 1, "daily_login", 25, nil, 1, true)
	require.NoError(t, err)

	repository := &fakeGrantRepository{}
	claims := &fakeDailyClaimer{}
	service := NewServiceWithDailyClaims(
		fakeRuleRepository{rules: []*rule.Rule{daily}},
		repository,
		nil,
		claims,
	)
	service.newID = func() (string, error) { return fmt.Sprintf("grant_%d", len(repository.saved)+1), nil }

	events := []struct {
		id         string
		occurredAt time.Time
		wantGrant  bool
	}{
		{"evt_day_1_first", time.Date(2026, 9, 27, 1, 0, 0, 0, time.UTC), true},
		{"evt_day_1_second", time.Date(2026, 9, 27, 23, 0, 0, 0, time.UTC), false},
		{"evt_day_2", time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC), true},
	}
	for _, test := range events {
		value, err := event.New(
			test.id, "proj_1", "player_1", "daily_login",
			test.occurredAt, test.occurredAt.Add(time.Second), nil,
		)
		require.NoError(t, err)
		grants, err := service.Process(context.Background(), value)
		require.NoError(t, err)
		if test.wantGrant {
			require.Len(t, grants, 1)
		} else {
			require.Empty(t, grants)
		}
	}
	require.Len(t, repository.saved, 2)
}

func TestServiceDoesNotAdvanceAggregateCounterForNonMatchingConditions(t *testing.T) {
	aggregate, err := rule.NewAggregate(
		"rule_course", "proj_1", 1, "lesson_completed", 250,
		map[string]any{"course_id": "course_7"}, 2,
	)
	require.NoError(t, err)
	counter := &fakeMatchCounter{}
	service := NewService(fakeRuleRepository{rules: []*rule.Rule{aggregate}}, &fakeGrantRepository{}, counter)

	value, err := event.New(
		"evt_miss", "proj_1", "player_1", "lesson_completed", time.Now(), time.Now(),
		map[string]any{"course_id": "course_8"},
	)
	require.NoError(t, err)
	grants, err := service.Process(context.Background(), value)
	require.NoError(t, err)
	require.Empty(t, grants)
	require.Zero(t, counter.count)
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
