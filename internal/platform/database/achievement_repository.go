package database

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/achievement"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type achievementDefinitionRecord struct {
	ProjectID string    `gorm:"type:varchar(64);primaryKey;not null;uniqueIndex:uq_achievements_project_name,priority:1"`
	ID        string    `gorm:"type:varchar(64);primaryKey;not null"`
	Name      string    `gorm:"type:varchar(255);not null;uniqueIndex:uq_achievements_project_name,priority:2"`
	CounterID string    `gorm:"type:varchar(64);not null"`
	Target    int64     `gorm:"not null;check:chk_achievements_target_positive,target > 0"`
	CreatedAt time.Time `gorm:"not null"`
}

func (achievementDefinitionRecord) TableName() string { return "achievement_definitions" }

type achievementUnlockRecord struct {
	ID            string    `gorm:"type:varchar(64);primaryKey;not null"`
	ProjectID     string    `gorm:"type:varchar(64);not null;uniqueIndex:uq_achievement_unlock_player,priority:1"`
	PlayerID      string    `gorm:"type:varchar(64);not null;uniqueIndex:uq_achievement_unlock_player,priority:2"`
	AchievementID string    `gorm:"type:varchar(64);not null;uniqueIndex:uq_achievement_unlock_player,priority:3"`
	EventID       string    `gorm:"type:varchar(255);not null"`
	CounterValue  int64     `gorm:"not null;check:chk_achievement_unlock_counter_value_positive,counter_value > 0"`
	UnlockedAt    time.Time `gorm:"not null"`
}

func (achievementUnlockRecord) TableName() string { return "achievement_unlocks" }

// AchievementRepository persists immutable Project-scoped definitions.
type AchievementRepository struct{ db *gorm.DB }

func NewAchievementRepository(db *gorm.DB) *AchievementRepository {
	return &AchievementRepository{db: db}
}

func (r *AchievementRepository) Save(ctx context.Context, value *achievement.Definition) error {
	record := achievementDefinitionRecord{ProjectID: value.ProjectID(), ID: value.ID(), Name: value.Name(), CounterID: value.CounterID(), Target: value.Target(), CreatedAt: value.CreatedAt()}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mapped := mapPersistenceError(err)
		if isUniqueConstraint(mapped, "uq_achievements_project_name") {
			return achievement.ErrNameTaken
		}
		return fmt.Errorf("create achievement: %w", mapped)
	}
	return nil
}

func (r *AchievementRepository) GetByID(ctx context.Context, projectID, id string) (*achievement.Definition, error) {
	var record achievementDefinitionRecord
	err := r.db.WithContext(ctx).Where("project_id = ? AND id = ?", projectID, id).Take(&record).Error
	if err == gorm.ErrRecordNotFound {
		return nil, achievement.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query achievement: %w", err)
	}
	return restoreAchievementDefinition(record)
}

func (r *AchievementRepository) ListByProject(ctx context.Context, projectID string) ([]*achievement.Definition, error) {
	var records []achievementDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("name ASC, id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query achievements by project: %w", err)
	}
	return restoreAchievementDefinitions(records)
}

func (r *AchievementRepository) ListByCounter(ctx context.Context, projectID, counterID string) ([]*achievement.Definition, error) {
	var records []achievementDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND counter_id = ?", projectID, counterID).Order("id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query achievements by Counter: %w", err)
	}
	return restoreAchievementDefinitions(records)
}

// AchievementUnlockRepository stores immutable, idempotent Player unlock records.
type AchievementUnlockRepository struct{ db *gorm.DB }

func NewAchievementUnlockRepository(db *gorm.DB) *AchievementUnlockRepository {
	return &AchievementUnlockRepository{db: db}
}

func (r *AchievementUnlockRepository) SaveIfAbsent(ctx context.Context, value *achievement.Unlock) (bool, error) {
	record := achievementUnlockRecord{ID: value.ID(), ProjectID: value.ProjectID(), PlayerID: value.PlayerID(), AchievementID: value.AchievementID(), EventID: value.EventID(), CounterValue: value.CounterValue(), UnlockedAt: value.UnlockedAt()}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "project_id"}, {Name: "player_id"}, {Name: "achievement_id"}}, DoNothing: true}).Create(&record)
	if result.Error != nil {
		return false, fmt.Errorf("save achievement unlock: %w", mapPersistenceError(result.Error))
	}
	return result.RowsAffected == 1, nil
}

func (r *AchievementUnlockRepository) ListByPlayer(ctx context.Context, projectID, playerID string) ([]*achievement.Unlock, error) {
	var records []achievementUnlockRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND player_id = ?", projectID, playerID).Order("unlocked_at ASC, achievement_id ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query player achievement unlocks: %w", err)
	}
	values := make([]*achievement.Unlock, 0, len(records))
	for _, record := range records {
		value, err := achievement.RestoreUnlock(record.ID, record.ProjectID, record.PlayerID, record.AchievementID, record.EventID, record.CounterValue, record.UnlockedAt)
		if err != nil {
			return nil, fmt.Errorf("restore achievement unlock %s: %w", record.ID, err)
		}
		values = append(values, value)
	}
	return values, nil
}

func restoreAchievementDefinition(record achievementDefinitionRecord) (*achievement.Definition, error) {
	value, err := achievement.Restore(record.ID, record.ProjectID, record.Name, record.CounterID, record.Target, record.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("restore achievement %s: %w", record.ID, err)
	}
	return value, nil
}

func restoreAchievementDefinitions(records []achievementDefinitionRecord) ([]*achievement.Definition, error) {
	values := make([]*achievement.Definition, 0, len(records))
	for _, record := range records {
		value, err := restoreAchievementDefinition(record)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

var _ achievement.Repository = (*AchievementRepository)(nil)
var _ achievement.UnlockRepository = (*AchievementUnlockRepository)(nil)
