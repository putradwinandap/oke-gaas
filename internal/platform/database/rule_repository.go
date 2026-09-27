package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ruledomain "github.com/putradwinandap/oke-gaas/internal/rule"
	"gorm.io/gorm"
)

type ruleRecord struct {
	ProjectID     string `gorm:"type:varchar(64);primaryKey;not null"`
	ID            string `gorm:"type:varchar(64);primaryKey;not null"`
	Version       uint64 `gorm:"primaryKey;not null"`
	EventType     string `gorm:"type:varchar(255);not null;index"`
	XPAmount      int64  `gorm:"not null"`
	Conditions    []byte `gorm:"type:jsonb;not null;default:'{}'"`
	MatchEvery    uint64 `gorm:"not null;default:1"`
	OncePerUTCDay bool   `gorm:"not null;default:false;check:chk_rules_daily_not_aggregate,NOT once_per_utc_day OR match_every = 1"`
}

func (ruleRecord) TableName() string { return "rules" }

type RuleRepository struct{ db *gorm.DB }

func NewRuleRepository(db *gorm.DB) *RuleRepository { return &RuleRepository{db: db} }

func (r *RuleRepository) Save(ctx context.Context, value *ruledomain.Rule) error {
	conditions, err := json.Marshal(value.Conditions())
	if err != nil {
		return fmt.Errorf("encode rule conditions: %w", err)
	}
	record := ruleRecord{
		ProjectID:     value.ProjectID(),
		ID:            value.ID(),
		Version:       value.Version(),
		EventType:     value.EventType(),
		XPAmount:      value.XPAmount(),
		Conditions:    conditions,
		MatchEvery:    value.MatchEvery(),
		OncePerUTCDay: value.OncePerUTCDay(),
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
func (r *RuleRepository) ListByEventType(ctx context.Context, projectID, eventType string) ([]*ruledomain.Rule, error) {
	var records []ruleRecord
	if err := r.db.WithContext(ctx).Raw(`
		SELECT project_id, id, version, event_type, xp_amount, conditions, match_every, once_per_utc_day
		FROM (
			SELECT
				project_id,
				id,
				version,
				event_type,
				xp_amount,
				conditions,
				match_every,
				once_per_utc_day,
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
		var conditions map[string]any
		if len(record.Conditions) > 0 {
			decoder := json.NewDecoder(strings.NewReader(string(record.Conditions)))
			decoder.UseNumber()
			if err := decoder.Decode(&conditions); err != nil {
				return nil, fmt.Errorf("decode rule %s version %d conditions: %w", record.ID, record.Version, err)
			}
		}
		value, err := ruledomain.Restore(record.ID, record.ProjectID, record.Version, record.EventType, record.XPAmount, conditions, record.MatchEvery, record.OncePerUTCDay)
		if err != nil {
			return nil, fmt.Errorf("restore rule %s version %d: %w", record.ID, record.Version, err)
		}
		values = append(values, value)
	}
	return values, nil
}

var _ ruledomain.Repository = (*RuleRepository)(nil)
