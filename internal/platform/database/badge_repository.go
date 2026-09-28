package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/badge"
	"gorm.io/gorm"
)

type badgeDefinitionRecord struct {
	ProjectID   string    `gorm:"type:varchar(64);primaryKey;not null;uniqueIndex:uq_badges_project_name,priority:1"`
	ID          string    `gorm:"type:varchar(64);primaryKey;not null"`
	Name        string    `gorm:"type:varchar(255);not null;uniqueIndex:uq_badges_project_name,priority:2"`
	Description string    `gorm:"type:varchar(1024);not null;default:''"`
	CreatedAt   time.Time `gorm:"not null"`
}

func (badgeDefinitionRecord) TableName() string { return "badge_definitions" }

type BadgeRepository struct{ db *gorm.DB }

func NewBadgeRepository(db *gorm.DB) *BadgeRepository { return &BadgeRepository{db: db} }
func (r *BadgeRepository) Save(ctx context.Context, value *badge.Definition) error {
	record := badgeDefinitionRecord{ProjectID: value.ProjectID(), ID: value.ID(), Name: value.Name(), Description: value.Description(), CreatedAt: value.CreatedAt()}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mapped := mapPersistenceError(err)
		if isUniqueConstraint(mapped, "uq_badges_project_name") {
			return badge.ErrAlreadyExists
		}
		return fmt.Errorf("create badge: %w", mapped)
	}
	return nil
}
func (r *BadgeRepository) ListByProject(ctx context.Context, projectID string) ([]*badge.Definition, error) {
	var records []badgeDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("name ASC, id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list badges: %w", err)
	}
	values := make([]*badge.Definition, 0, len(records))
	for _, record := range records {
		value, err := badge.New(record.ID, record.ProjectID, record.Name, record.Description, record.CreatedAt)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}
func (r *BadgeRepository) Get(ctx context.Context, projectID, id string) (*badge.Definition, error) {
	var record badgeDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND id = ?", projectID, id).First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, badge.ErrNotFound
		}
		return nil, fmt.Errorf("get badge: %w", err)
	}
	return badge.New(record.ID, record.ProjectID, record.Name, record.Description, record.CreatedAt)
}

// BadgeGrantRepository persists badge ownership in the common auditable reward ledger.
type BadgeGrantRepository struct{ db *gorm.DB }

func NewBadgeGrantRepository(db *gorm.DB) *BadgeGrantRepository { return &BadgeGrantRepository{db: db} }
func (r *BadgeGrantRepository) ListByPlayer(ctx context.Context, projectID, playerID string) ([]*badge.Grant, error) {
	var records []rewardGrantRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND player_id = ? AND reward_type = ?", projectID, playerID, "badge").Order("created_at ASC, id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list player badges: %w", err)
	}
	values := make([]*badge.Grant, 0, len(records))
	for _, record := range records {
		if record.BadgeID == nil {
			return nil, fmt.Errorf("badge grant %s has no Badge ID", record.ID)
		}
		value, err := badge.NewGrant(record.ProjectID, record.PlayerID, *record.BadgeID, record.EventID, record.RuleID, record.RuleVersion, record.CreatedAt)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

var _ badge.Repository = (*BadgeRepository)(nil)
