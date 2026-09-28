package badge

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefinitionTrimsAndValidatesDisplayMetadata(t *testing.T) {
	created := time.Date(2026, 9, 28, 12, 0, 0, 1234, time.FixedZone("test", 3600))
	value, err := New(" badge_1 ", " proj_1 ", " Early Adopter ", " First cohort ", created)
	require.NoError(t, err)
	require.Equal(t, "badge_1", value.ID())
	require.Equal(t, "Early Adopter", value.Name())
	require.Equal(t, "First cohort", value.Description())
	require.Equal(t, created.UTC().Truncate(time.Microsecond), value.CreatedAt())
	_, err = New("badge_2", "proj_1", " ", "", created)
	require.ErrorIs(t, err, ErrInvalidName)
}

func TestBadgeGrantRetainsRuleAndEventAudit(t *testing.T) {
	grantedAt := time.Date(2026, 9, 28, 12, 0, 0, 1234, time.UTC)
	value, err := NewGrant("proj_1", "player_1", "badge_1", "evt_1", "rule_1", 2, grantedAt)
	require.NoError(t, err)
	require.Equal(t, "badge_1", value.BadgeID())
	require.Equal(t, "evt_1", value.EventID())
	require.Equal(t, "rule_1", value.RuleID())
	require.Equal(t, uint64(2), value.RuleVersion())
}
