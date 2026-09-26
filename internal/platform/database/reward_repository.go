package database

import (
	"context"
	"fmt"
	"time"

	rewarddomain "github.com/putradwinandap/oke-gaas/internal/reward"
	"gorm.io/gorm"
)

type rewardGrantRecord struct {
	ID          string    `gorm:"type:varchar(64);primaryKey;not null"`
	ProjectID   string    `gorm:"type:varchar(64);not null;index"`
	PlayerID    string    `gorm:"type:varchar(64);not null;index"`
	EventID     string    `gorm:"type:varchar(255);not null;index"`
	RuleID      string    `gorm:"type:varchar(64);not null"`
	RuleVersion uint64    `gorm:"not null"`
	RewardType  string    `gorm:"type:varchar(32);not null"`
	Amount      int64     `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (rewardGrantRecord) TableName() string { return "reward_grants" }

type RewardGrantRepository struct{ db *gorm.DB }

func NewRewardGrantRepository(db *gorm.DB) *RewardGrantRepository {
	return &RewardGrantRepository{db: db}
}

func (r *RewardGrantRepository) Save(ctx context.Context, value *rewarddomain.Grant) error {
	record := rewardGrantRecord{
		ID:          value.ID(),
		ProjectID:   value.ProjectID(),
		PlayerID:    value.PlayerID(),
		EventID:     value.EventID(),
		RuleID:      value.RuleID(),
		RuleVersion: value.RuleVersion(),
		RewardType:  value.Type(),
		Amount:      value.Amount(),
		CreatedAt:   value.CreatedAt(),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mapped := mapPersistenceError(err)
		if isUniqueConstraint(mapped, "reward_grants_pkey") ||
			isUniqueConstraint(mapped, "idx_reward_grants_event_rule_type") {
			return rewarddomain.ErrAlreadyExists
		}
		return fmt.Errorf("create reward grant: %w", mapped)
	}
	return nil
}

func (r *RewardGrantRepository) ListByEvent(ctx context.Context, projectID, eventID string) ([]*rewarddomain.Grant, error) {
	var records []rewardGrantRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND event_id = ?", projectID, eventID).
		Order("created_at ASC, id ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query reward grants by event: %w", err)
	}

	values := make([]*rewarddomain.Grant, 0, len(records))
	for _, record := range records {
		value, err := rewarddomain.RestoreGrant(
			record.ID,
			record.ProjectID,
			record.PlayerID,
			record.EventID,
			record.RuleID,
			record.RuleVersion,
			record.RewardType,
			record.Amount,
			record.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("restore reward grant %s: %w", record.ID, err)
		}
		values = append(values, value)
	}
	return values, nil
}

var _ rewarddomain.Repository = (*RewardGrantRepository)(nil)
