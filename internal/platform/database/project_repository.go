package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/project"
	"gorm.io/gorm"
)

type projectRecord struct {
	ID        string    `gorm:"type:varchar(64);primaryKey"`
	Name      string    `gorm:"type:varchar(255);not null"`
	CreatedAt time.Time `gorm:"not null"`
}

func (projectRecord) TableName() string { return "projects" }

// ProjectRepository is the GORM/PostgreSQL adapter for project.Repository.
type ProjectRepository struct {
	db *gorm.DB
}

// NewProjectRepository creates a GORM-backed Project repository.
func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

// Save persists a Project.
func (r *ProjectRepository) Save(ctx context.Context, value *project.Project) error {
	record := projectRecord{
		ID:        value.ID(),
		Name:      value.Name(),
		CreatedAt: value.CreatedAt(),
	}

	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create project: %w", err)
	}

	return nil
}

// GetByID retrieves a Project by identifier.
func (r *ProjectRepository) GetByID(ctx context.Context, id string) (*project.Project, error) {
	var record projectRecord
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, project.ErrNotFound
		}
		return nil, fmt.Errorf("query project: %w", err)
	}

	value, err := project.Restore(record.ID, record.Name, record.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("restore project: %w", err)
	}

	return value, nil
}

var _ project.Repository = (*ProjectRepository)(nil)
