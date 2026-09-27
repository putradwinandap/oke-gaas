package database

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/reward"
	"gorm.io/gorm"
)

type ruleMatchCountRecord struct {
	ProjectID   string    `gorm:"type:varchar(64);primaryKey;not null"`
	PlayerID    string    `gorm:"type:varchar(64);primaryKey;not null"`
	RuleID      string    `gorm:"type:varchar(64);primaryKey;not null"`
	RuleVersion uint64    `gorm:"primaryKey;not null"`
	MatchCount  uint64    `gorm:"not null;default:0"`
	UpdatedAt   time.Time `gorm:"not null"`
}

func (ruleMatchCountRecord) TableName() string { return "rule_match_counts" }

// RuleMatchCounter atomically materializes aggregate Rule progress.
type RuleMatchCounter struct{ db *gorm.DB }

func NewRuleMatchCounter(db *gorm.DB) *RuleMatchCounter {
	return &RuleMatchCounter{db: db}
}

func (r *RuleMatchCounter) Increment(
	ctx context.Context,
	projectID, playerID, ruleID string,
	ruleVersion uint64,
	matchedAt time.Time,
) (uint64, error) {
	if matchedAt.IsZero() {
		return 0, fmt.Errorf("increment rule match count: matched_at is required")
	}

	var record ruleMatchCountRecord
	if err := r.db.WithContext(ctx).Raw(
		"INSERT INTO rule_match_counts (project_id, player_id, rule_id, rule_version, match_count, updated_at) "+
			"VALUES (?, ?, ?, ?, 1, ?) "+
			"ON CONFLICT (project_id, player_id, rule_id, rule_version) DO UPDATE SET "+
			"match_count = rule_match_counts.match_count + 1, "+
			"updated_at = GREATEST(rule_match_counts.updated_at, EXCLUDED.updated_at) "+
			"RETURNING project_id, player_id, rule_id, rule_version, match_count, updated_at",
		projectID,
		playerID,
		ruleID,
		ruleVersion,
		matchedAt.UTC().Truncate(time.Microsecond),
	).Scan(&record).Error; err != nil {
		return 0, fmt.Errorf("increment rule match count: %w", mapPersistenceError(err))
	}
	return record.MatchCount, nil
}

var _ reward.MatchCounter = (*RuleMatchCounter)(nil)
