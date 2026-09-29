package streak

import (
	"context"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/event"
	"github.com/stretchr/testify/require"
)

func TestCurrentLengthConsecutiveGapAndExpiry(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, 9, n, 0, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		name string
		days []time.Time
		now  time.Time
		want int64
	}{
		{"single", []time.Time{day(1)}, day(1), 1},
		{"consecutive", []time.Time{day(1), day(2), day(3)}, day(3), 3},
		{"gap", []time.Time{day(1), day(3)}, day(3), 1},
		{"yesterday active", []time.Time{day(2)}, day(3), 1},
		{"old inactive", []time.Time{day(1)}, day(3), 0},
	} {
		t.Run(tc.name, func(t *testing.T) { got, _ := CurrentLength(tc.days, tc.now); require.Equal(t, tc.want, got) })
	}
}

func TestCurrentLengthTodayYesterdayAndHistoricalRepair(t *testing.T) {
	day := func(n int) time.Time { return time.Date(2026, 9, n, 0, 0, 0, 0, time.UTC) }
	current, latest := CurrentLength([]time.Time{day(26), day(28)}, day(29))
	require.Equal(t, int64(1), current)
	require.Equal(t, day(28), *latest)
	current, _ = CurrentLength([]time.Time{day(26), day(27), day(28)}, day(29))
	require.Equal(t, int64(3), current)
	current, _ = CurrentLength([]time.Time{day(27), day(28)}, day(29))
	require.Equal(t, int64(2), current)
	current, latest = CurrentLength([]time.Time{day(27)}, day(29))
	require.Zero(t, current)
	require.Equal(t, day(27), *latest)
	current, latest = CurrentLength([]time.Time{day(30)}, day(29))
	require.Zero(t, current)
	require.Equal(t, day(30), *latest)
}

func TestProcessEventMatchesConditionsAndUsesOccurredUTCDate(t *testing.T) {
	loc := time.FixedZone("west", -7*3600)
	e, err := event.New("evt_1", "proj_1", "player_1", "daily_login", time.Date(2026, 9, 2, 1, 0, 0, 0, loc), time.Date(2026, 9, 3, 1, 0, 0, 0, time.UTC), map[string]any{"source": "mobile"})
	require.NoError(t, err)
	def, err := New("streak_1", "proj_1", "login", "daily_login", map[string]any{"source": "mobile"}, time.Now())
	require.NoError(t, err)
	r := &memoryRepository{definitions: []*Definition{def}}
	c := &memoryClaims{}
	require.NoError(t, ProcessEvent(context.Background(), r, c, e))
	require.Len(t, c.values, 1)
	require.Equal(t, time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC), c.values[0].Day())
}

type memoryRepository struct{ definitions []*Definition }

func (m *memoryRepository) Save(_ context.Context, d *Definition) error {
	m.definitions = append(m.definitions, d)
	return nil
}
func (m *memoryRepository) ListByProject(_ context.Context, p string) ([]*Definition, error) {
	out := []*Definition{}
	for _, d := range m.definitions {
		if d.ProjectID() == p {
			out = append(out, d)
		}
	}
	return out, nil
}
func (m *memoryRepository) ListByEventType(_ context.Context, p, e string) ([]*Definition, error) {
	out := []*Definition{}
	for _, d := range m.definitions {
		if d.ProjectID() == p && d.EventType() == e {
			out = append(out, d)
		}
	}
	return out, nil
}

type memoryClaims struct{ values []*QualifiedDay }

func (m *memoryClaims) Claim(_ context.Context, q *QualifiedDay) (bool, error) {
	for _, v := range m.values {
		if v.ProjectID() == q.ProjectID() && v.PlayerID() == q.PlayerID() && v.StreakID() == q.StreakID() && v.Day().Equal(q.Day()) {
			return false, nil
		}
	}
	m.values = append(m.values, q)
	return true, nil
}
func (m *memoryClaims) DaysByPlayer(_ context.Context, p, pl string) (map[string][]time.Time, error) {
	out := map[string][]time.Time{}
	for _, v := range m.values {
		if v.ProjectID() == p && v.PlayerID() == pl {
			out[v.StreakID()] = append(out[v.StreakID()], v.Day())
		}
	}
	return out, nil
}
