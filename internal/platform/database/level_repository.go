package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/level"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type levelThresholdRecord struct {
	ProjectID string `gorm:"type:varchar(64);primaryKey;not null"`
	Number    uint64 `gorm:"primaryKey;not null"`
	MinXP     int64  `gorm:"not null"`
}

func (levelThresholdRecord) TableName() string { return "level_thresholds" }

type LevelRepository struct{ db *gorm.DB }

func NewLevelRepository(db *gorm.DB) *LevelRepository { return &LevelRepository{db: db} }

func (r *LevelRepository) Append(ctx context.Context, projectID string, minXP int64) (*level.Threshold, error) {
	if minXP <= 0 {
		return nil, level.ErrInvalidMinXP
	}

	var created levelThresholdRecord
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var project projectRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&project, "id = ?", projectID).Error; err != nil {
			return err
		}

		var last levelThresholdRecord
		err := tx.Where("project_id = ?", projectID).Order("number DESC").First(&last).Error
		next := uint64(2)
		if err == nil {
			if minXP <= last.MinXP {
				return level.ErrThresholdNotIncreasing
			}
			next = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		created = levelThresholdRecord{ProjectID: projectID, Number: next, MinXP: minXP}
		return tx.Create(&created).Error
	})
	if err != nil {
		return nil, fmt.Errorf("append level threshold: %w", mapPersistenceError(err))
	}
	return restoreLevelThreshold(created)
}

func (r *LevelRepository) List(ctx context.Context, projectID string) ([]*level.Threshold, error) {
	var records []levelThresholdRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", projectID).Order("number ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list level thresholds: %w", err)
	}
	values := make([]*level.Threshold, 0, len(records))
	for _, record := range records {
		value, err := restoreLevelThreshold(record)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func (r *LevelRepository) Resolve(ctx context.Context, projectID string, xp int64) (uint64, error) {
	var record levelThresholdRecord
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND min_xp <= ?", projectID, xp).
		Order("number DESC").
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, nil
	}
	if err != nil {
		return 0, fmt.Errorf("resolve level threshold: %w", err)
	}
	return record.Number, nil
}

func restoreLevelThreshold(record levelThresholdRecord) (*level.Threshold, error) {
	value, err := level.RestoreThreshold(record.ProjectID, record.Number, record.MinXP)
	if err != nil {
		return nil, fmt.Errorf("restore level threshold: %w", err)
	}
	return value, nil
}

var _ level.Repository = (*LevelRepository)(nil)
