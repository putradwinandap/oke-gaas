package database

import (
	"context"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/project"
	"gorm.io/gorm"
)

type ProjectProvisionTransactor struct {
	db *gorm.DB
}

func NewProjectProvisionTransactor(db *gorm.DB) *ProjectProvisionTransactor {
	return &ProjectProvisionTransactor{db: db}
}

func (t *ProjectProvisionTransactor) WithinProvisionTransaction(ctx context.Context, fn func(project.ProvisionWork) error) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("project provision database is required")
	}
	if fn == nil {
		return fmt.Errorf("project provision callback is required")
	}
	if err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(project.ProvisionWork{
			Projects: NewProjectRepository(tx),
			Keys:     NewProjectAPIKeyRepository(tx),
		})
	}); err != nil {
		return fmt.Errorf("project provision transaction: %w", err)
	}
	return nil
}

var _ project.ProvisionTransactor = (*ProjectProvisionTransactor)(nil)
