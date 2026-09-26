package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/access"
	"gorm.io/gorm"
)

type projectAPIKeyRecord struct {
	ProjectID  string        `gorm:"type:varchar(64);primaryKey;not null"`
	SecretHash []byte        `gorm:"type:bytea;not null"`
	Project    projectRecord `gorm:"foreignKey:ProjectID;references:ID;constraint:OnUpdate:RESTRICT,OnDelete:CASCADE"`
}

func (projectAPIKeyRecord) TableName() string { return "project_api_keys" }

type ProjectAPIKeyRepository struct{ db *gorm.DB }

func NewProjectAPIKeyRepository(db *gorm.DB) *ProjectAPIKeyRepository {
	return &ProjectAPIKeyRepository{db: db}
}

func (r *ProjectAPIKeyRepository) Save(ctx context.Context, key *access.ProjectKey) error {
	record := projectAPIKeyRecord{
		ProjectID:  key.ProjectID(),
		SecretHash: key.Hash(),
	}
	if err := r.db.WithContext(ctx).Omit("Project").Create(&record).Error; err != nil {
		return fmt.Errorf("create project api key: %w", mapPersistenceError(err))
	}
	return nil
}

func (r *ProjectAPIKeyRepository) GetByProjectID(ctx context.Context, projectID string) (*access.ProjectKey, error) {
	var record projectAPIKeyRecord
	if err := r.db.WithContext(ctx).First(&record, "project_id = ?", projectID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, access.ErrNotFound
		}
		return nil, fmt.Errorf("query project api key: %w", err)
	}
	value, err := access.Restore(record.ProjectID, record.SecretHash)
	if err != nil {
		return nil, fmt.Errorf("restore project api key: %w", err)
	}
	return value, nil
}

var _ access.Repository = (*ProjectAPIKeyRepository)(nil)
