package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/progression"
	"gorm.io/gorm"
)

const (
	levelThresholdPrimaryKeyConstraint       = "level_thresholds_pkey"
	levelThresholdProjectMinXPUniqueConstraint = "idx_level_thresholds_project_min_xp"
)

type levelThresholdRecord struct {
	ProjectID string `gorm:"type:varchar(64);primaryKey;not null;index:idx_level_thresholds_project_min_xp,unique"`
	Level     uint32 `gorm:"primaryKey;not null"`
	MinXP     int64  `gorm:"not null;index:idx_level_thresholds_project_min_xp,unique"`
}

func (levelThresholdRecord) TableName() string { return "level_thresholds" }

type LevelThresholdRepository struct{ db *gorm.DB }

func NewLevelThresholdRepository(db *gorm.DB) *LevelThresholdRepository {
	return &LevelThresholdRepository{db: db}
}

func (r *LevelThresholdRepository) Save(ctx context.Context, threshold *progression.LevelThreshold) error {
	record := levelThresholdRecord{
		ProjectID: threshold.ProjectID(),
		Level:     threshold.Level(),
		MinXP:     threshold.MinXP(),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mappedErr := mapPersistenceError(err)
		if isUniqueConstraint(mappedErr, levelThresholdPrimaryKeyConstraint) ||
			isUniqueConstraint(mappedErr, levelThresholdProjectMinXPUniqueConstraint) {
			return progression.ErrLevelThresholdConflict
		}
		return fmt.Errorf("create level threshold: %w", mappedErr)
	}
	return nil
}

func (r *LevelThresholdRepository) Latest(ctx context.Context, projectID string) (*progression.LevelThreshold, error) {
	var record levelThresholdRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("level DESC").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, progression.ErrLevelNotConfigured
		}
		return nil, fmt.Errorf("query latest level threshold: %w", err)
	}
	return restoreLevelThreshold(record)
}

func (r *LevelThresholdRepository) List(ctx context.Context, projectID string) ([]*progression.LevelThreshold, error) {
	var records []levelThresholdRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("level ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list level thresholds: %w", err)
	}
	result := make([]*progression.LevelThreshold, 0, len(records))
	for _, record := range records {
		value, err := restoreLevelThreshold(record)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

func (r *LevelThresholdRepository) Resolve(ctx context.Context, projectID string, xp int64) (uint32, error) {
	var record levelThresholdRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND min_xp <= ?", projectID, xp).
		Order("min_xp DESC").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 1, nil
		}
		return 0, fmt.Errorf("resolve level threshold: %w", err)
	}
	return record.Level, nil
}

func restoreLevelThreshold(record levelThresholdRecord) (*progression.LevelThreshold, error) {
	value, err := progression.RestoreLevelThreshold(record.ProjectID, record.Level, record.MinXP)
	if err != nil {
		return nil, fmt.Errorf("restore level threshold: %w", err)
	}
	return value, nil
}

var _ progression.LevelRepository = (*LevelThresholdRepository)(nil)
