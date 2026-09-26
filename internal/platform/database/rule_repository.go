package database

import (
	"context"
	"fmt"

	ruledomain "github.com/putradwinandap/oke-gaas/internal/rule"
	"gorm.io/gorm"
)

type ruleRecord struct {
	ProjectID string `gorm:"type:varchar(64);primaryKey;not null"`
	ID        string `gorm:"type:varchar(64);primaryKey;not null"`
	Version   uint64 `gorm:"primaryKey;not null"`
	EventType string `gorm:"type:varchar(255);not null;index"`
	XPAmount  int64  `gorm:"not null"`
}

func (ruleRecord) TableName() string { return "rules" }

type RuleRepository struct{ db *gorm.DB }

func NewRuleRepository(db *gorm.DB) *RuleRepository { return &RuleRepository{db: db} }

func (r *RuleRepository) Save(ctx context.Context, value *ruledomain.Rule) error {
	record := ruleRecord{
		ProjectID: value.ProjectID(),
		ID:        value.ID(),
		Version:   value.Version(),
		EventType: value.EventType(),
		XPAmount:  value.XPAmount(),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mapped := mapPersistenceError(err)
		if isUniqueConstraint(mapped, "rules_pkey") {
			return ruledomain.ErrAlreadyExists
		}
		return fmt.Errorf("create rule: %w", mapped)
	}
	return nil
}

// ListByEventType returns only the latest version of each Rule identity.
// Older versions remain persisted for Reward Grant auditability but are not re-evaluated.
func (r *RuleRepository) ListByEventType(ctx context.Context, projectID, eventType string) ([]*ruledomain.Rule, error) {
	var records []ruleRecord
	if err := r.db.WithContext(ctx).Raw(`
		SELECT project_id, id, version, event_type, xp_amount
		FROM (
			SELECT
				project_id,
				id,
				version,
				event_type,
				xp_amount,
				ROW_NUMBER() OVER (
					PARTITION BY project_id, id
					ORDER BY version DESC
				) AS version_rank
			FROM rules
			WHERE project_id = ?
		) latest
		WHERE version_rank = 1
		  AND event_type = ?
		ORDER BY id ASC
	`, projectID, eventType).Scan(&records).Error; err != nil {
		return nil, fmt.Errorf("query current rules by event type: %w", err)
	}

	values := make([]*ruledomain.Rule, 0, len(records))
	for _, record := range records {
		value, err := ruledomain.Restore(record.ID, record.ProjectID, record.Version, record.EventType, record.XPAmount)
		if err != nil {
			return nil, fmt.Errorf("restore rule %s version %d: %w", record.ID, record.Version, err)
		}
		values = append(values, value)
	}
	return values, nil
}

var _ ruledomain.Repository = (*RuleRepository)(nil)
