package database

import (
	"context"
	"fmt"
	"time"

	badgedomain "github.com/putradwinandap/oke-gaas/internal/badge"
	rewarddomain "github.com/putradwinandap/oke-gaas/internal/reward"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type rewardGrantRecord struct {
	ID          string    `gorm:"type:varchar(64);primaryKey;not null"`
	ProjectID   string    `gorm:"type:varchar(64);not null;index"`
	PlayerID    string    `gorm:"type:varchar(64);not null;index"`
	EventID     string    `gorm:"type:varchar(255);not null;index"`
	RuleID      string    `gorm:"type:varchar(64);not null"`
	RuleVersion uint64    `gorm:"not null"`
	RewardType  string    `gorm:"type:varchar(32);not null"`
	BadgeID     *string   `gorm:"type:varchar(64)"`
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
	if value.BadgeID() != "" {
		badgeID := value.BadgeID()
		record.BadgeID = &badgeID
	}
	conflict := clause.OnConflict{DoNothing: true}
	if value.Type() == rewarddomain.TypeBadge {
		conflict.Columns = []clause.Column{{Name: "project_id"}, {Name: "player_id"}, {Name: "badge_id"}}
		conflict.TargetWhere = clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "reward_type = 'badge'"}}}
	}
	result := r.db.WithContext(ctx).Clauses(conflict).Create(&record)
	if result.Error != nil {
		return fmt.Errorf("create reward grant: %w", mapPersistenceError(result.Error))
	}
	if result.RowsAffected == 0 {
		return rewarddomain.ErrAlreadyExists
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
		var value *rewarddomain.Grant
		var err error
		if record.RewardType == rewarddomain.TypeBadge && record.BadgeID != nil {
			value, err = rewarddomain.RestoreBadgeGrant(record.ID, record.ProjectID, record.PlayerID, record.EventID, record.RuleID, record.RuleVersion, *record.BadgeID, record.CreatedAt)
		} else {
			value, err = rewarddomain.RestoreGrant(
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
		}
		if err != nil {
			return nil, fmt.Errorf("restore reward grant %s: %w", record.ID, err)
		}
		values = append(values, value)
	}
	return values, nil
}

var _ rewarddomain.Repository = (*RewardGrantRepository)(nil)

var _ badgedomain.GrantRepository = (*BadgeGrantRepository)(nil)
