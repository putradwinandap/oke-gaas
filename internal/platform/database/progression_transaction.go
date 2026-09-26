package database

import (
	"context"
	"fmt"

	"github.com/putradwinandap/oke-gaas/internal/progression"
	"gorm.io/gorm"
)

// ProgressionTransactor binds all repositories used by the core processing flow
// to the same PostgreSQL transaction.
type ProgressionTransactor struct {
	db *gorm.DB
}

func NewProgressionTransactor(db *gorm.DB) *ProgressionTransactor {
	return &ProgressionTransactor{db: db}
}

func (t *ProgressionTransactor) WithinTransaction(ctx context.Context, fn func(progression.Work) error) error {
	if t == nil || t.db == nil {
		return fmt.Errorf("progression transaction database is required")
	}
	if fn == nil {
		return fmt.Errorf("progression transaction callback is required")
	}

	if err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(progression.Work{
			Players: NewPlayerRepository(tx),
			Events:  NewEventRepository(tx),
			Rules:   NewRuleRepository(tx),
			Grants:  NewRewardGrantRepository(tx),
			States:  NewPlayerStateRepository(tx),
			Claims:  NewEventProcessingRepository(tx),
		})
	}); err != nil {
		return fmt.Errorf("progression transaction: %w", err)
	}
	return nil
}

var _ progression.Transactor = (*ProgressionTransactor)(nil)
