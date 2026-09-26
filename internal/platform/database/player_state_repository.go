package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/progression"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type playerStateRecord struct {
	ProjectID string    `gorm:"type:varchar(64);primaryKey;not null"`
	PlayerID  string    `gorm:"type:varchar(64);primaryKey;not null"`
	XP        int64     `gorm:"not null;default:0"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (playerStateRecord) TableName() string { return "player_states" }

type PlayerStateRepository struct{ db *gorm.DB }

func NewPlayerStateRepository(db *gorm.DB) *PlayerStateRepository {
	return &PlayerStateRepository{db: db}
}

func (r *PlayerStateRepository) Ensure(ctx context.Context, projectID, playerID string, updatedAt time.Time) (*progression.State, error) {
	if updatedAt.IsZero() {
		return nil, progression.ErrInvalidUpdatedAt
	}
	record := playerStateRecord{
		ProjectID: projectID,
		PlayerID:  playerID,
		XP:        0,
		UpdatedAt: updatedAt.UTC().Truncate(time.Microsecond),
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}, {Name: "player_id"}},
			DoNothing: true,
		}).
		Create(&record).Error; err != nil {
		return nil, fmt.Errorf("ensure player state: %w", mapPersistenceError(err))
	}
	return r.Get(ctx, projectID, playerID)
}

func (r *PlayerStateRepository) AddXP(ctx context.Context, projectID, playerID string, amount int64, updatedAt time.Time) (*progression.State, error) {
	if amount <= 0 {
		return nil, progression.ErrInvalidXPIncrease
	}
	if updatedAt.IsZero() {
		return nil, progression.ErrInvalidUpdatedAt
	}

	var record playerStateRecord
	if err := r.db.WithContext(ctx).Raw(
		"INSERT INTO player_states (project_id, player_id, xp, updated_at) VALUES (?, ?, ?, ?) "+
			"ON CONFLICT (project_id, player_id) DO UPDATE SET "+
			"xp = player_states.xp + EXCLUDED.xp, "+
			"updated_at = GREATEST(player_states.updated_at, EXCLUDED.updated_at) "+
			"RETURNING project_id, player_id, xp, updated_at",
		projectID,
		playerID,
		amount,
		updatedAt.UTC().Truncate(time.Microsecond),
	).Scan(&record).Error; err != nil {
		return nil, fmt.Errorf("add player xp: %w", mapPersistenceError(err))
	}
	return restorePlayerState(record)
}

func (r *PlayerStateRepository) Get(ctx context.Context, projectID, playerID string) (*progression.State, error) {
	var record playerStateRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND player_id = ?", projectID, playerID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, progression.ErrStateNotFound
		}
		return nil, fmt.Errorf("query player state: %w", err)
	}
	return restorePlayerState(record)
}

func restorePlayerState(record playerStateRecord) (*progression.State, error) {
	value, err := progression.RestoreState(record.ProjectID, record.PlayerID, record.XP, record.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("restore player state: %w", err)
	}
	return value, nil
}

var _ progression.Repository = (*PlayerStateRepository)(nil)
