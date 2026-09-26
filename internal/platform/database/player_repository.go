package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/player"
	"gorm.io/gorm"
)

type playerRecord struct {
	ID         string    `gorm:"type:varchar(64);primaryKey"`
	ProjectID  string    `gorm:"type:varchar(64);not null;index;index:idx_players_project_external,unique"`
	ExternalID string    `gorm:"type:varchar(255);not null;index:idx_players_project_external,unique"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (playerRecord) TableName() string { return "players" }

// PlayerRepository is the GORM/PostgreSQL adapter for player.Repository.
type PlayerRepository struct {
	db *gorm.DB
}

// NewPlayerRepository creates a GORM-backed Player repository.
func NewPlayerRepository(db *gorm.DB) *PlayerRepository {
	return &PlayerRepository{db: db}
}

// Save persists a Player without allowing its owning Project to change.
func (r *PlayerRepository) Save(ctx context.Context, value *player.Player) error {
	record := playerRecord{
		ID:         value.ID(),
		ProjectID:  value.ProjectID(),
		ExternalID: value.ExternalID(),
		CreatedAt:  value.CreatedAt(),
	}

	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return player.ErrExternalIDTaken
		}
		return fmt.Errorf("create player: %w", err)
	}

	return nil
}

// GetByID retrieves a Player only within the supplied Project scope.
func (r *PlayerRepository) GetByID(ctx context.Context, projectID, playerID string) (*player.Player, error) {
	var record playerRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND id = ?", projectID, playerID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, player.ErrNotFound
		}
		return nil, fmt.Errorf("query player by id: %w", err)
	}

	return restorePlayer(record)
}

// GetByExternalID retrieves a Player by its Project-scoped external identifier.
func (r *PlayerRepository) GetByExternalID(ctx context.Context, projectID, externalID string) (*player.Player, error) {
	var record playerRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND external_id = ?", projectID, externalID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, player.ErrNotFound
		}
		return nil, fmt.Errorf("query player by external id: %w", err)
	}

	return restorePlayer(record)
}

func restorePlayer(record playerRecord) (*player.Player, error) {
	value, err := player.Restore(record.ID, record.ProjectID, record.ExternalID, record.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("restore player: %w", err)
	}
	return value, nil
}

var _ player.Repository = (*PlayerRepository)(nil)
