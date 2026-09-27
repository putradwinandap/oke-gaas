package database

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/reward"
	"gorm.io/gorm"
)

type ruleDailyClaimRecord struct {
	ProjectID   string    `gorm:"type:varchar(64);primaryKey;not null"`
	PlayerID    string    `gorm:"type:varchar(64);primaryKey;not null"`
	RuleID      string    `gorm:"type:varchar(64);primaryKey;not null"`
	RuleVersion uint64    `gorm:"primaryKey;not null"`
	ClaimDay    time.Time `gorm:"type:date;primaryKey;not null"`
	ClaimedAt   time.Time `gorm:"not null"`
}

func (ruleDailyClaimRecord) TableName() string { return "rule_daily_claims" }

// RuleDailyClaimRepository atomically claims UTC calendar days for time-aware Rules.
type RuleDailyClaimRepository struct{ db *gorm.DB }

func NewRuleDailyClaimRepository(db *gorm.DB) *RuleDailyClaimRepository {
	return &RuleDailyClaimRepository{db: db}
}

func (r *RuleDailyClaimRepository) Claim(
	ctx context.Context,
	projectID, playerID, ruleID string,
	ruleVersion uint64,
	occurredAt time.Time,
) (bool, error) {
	if occurredAt.IsZero() {
		return false, fmt.Errorf("claim rule UTC day: occurred_at is required")
	}

	utc := occurredAt.UTC()
	claimDay := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	claimedAt := time.Now().UTC().Truncate(time.Microsecond)

	result := r.db.WithContext(ctx).Exec(
		"INSERT INTO rule_daily_claims (project_id, player_id, rule_id, rule_version, claim_day, claimed_at) "+
			"VALUES (?, ?, ?, ?, ?, ?) "+
			"ON CONFLICT (project_id, player_id, rule_id, rule_version, claim_day) DO NOTHING",
		projectID,
		playerID,
		ruleID,
		ruleVersion,
		claimDay,
		claimedAt,
	)
	if result.Error != nil {
		return false, fmt.Errorf("claim rule UTC day: %w", mapPersistenceError(result.Error))
	}
	return result.RowsAffected == 1, nil
}

var _ reward.DailyClaimer = (*RuleDailyClaimRepository)(nil)
