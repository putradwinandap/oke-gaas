package project

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/access"
)

// ProvisionWork contains repositories bound to one Project provisioning transaction.
type ProvisionWork struct {
	Projects Repository
	Keys     access.Repository
}

// ProvisionTransactor makes Project creation and API-key creation atomic.
type ProvisionTransactor interface {
	WithinProvisionTransaction(ctx context.Context, fn func(ProvisionWork) error) error
}

type ProvisionResult struct {
	Project *Project
	APIKey  string
}

type ProvisionService struct {
	transactions ProvisionTransactor
	now          func() time.Time
}

func NewProvisionService(transactions ProvisionTransactor) *ProvisionService {
	return &ProvisionService{transactions: transactions, now: time.Now}
}

func (s *ProvisionService) Provision(ctx context.Context, name string) (*ProvisionResult, error) {
	if s == nil || s.transactions == nil {
		return nil, fmt.Errorf("provision project: transaction boundary is required")
	}

	value, err := New(name, s.now())
	if err != nil {
		return nil, err
	}
	key, secret, err := access.Generate(value.ID())
	if err != nil {
		return nil, fmt.Errorf("generate project api key: %w", err)
	}

	if err := s.transactions.WithinProvisionTransaction(ctx, func(work ProvisionWork) error {
		if err := work.Projects.Save(ctx, value); err != nil {
			return fmt.Errorf("save project: %w", err)
		}
		if err := work.Keys.Save(ctx, key); err != nil {
			return fmt.Errorf("save project api key: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return &ProvisionResult{Project: value, APIKey: secret}, nil
}
