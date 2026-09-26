package project

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/shared/identity"
)

var (
	// ErrNotFound is returned when a project does not exist.
	ErrNotFound = errors.New("project not found")
	// ErrInvalidName is returned when a project name is empty.
	ErrInvalidName = errors.New("project name is required")
)

// Project is the tenant and integration boundary for Oke Gaas data.
type Project struct {
	id        string
	name      string
	createdAt time.Time
}

// New creates a new Project.
func New(name string, createdAt time.Time) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrInvalidName
	}

	id, err := identity.New("proj")
	if err != nil {
		return nil, err
	}

	return &Project{id: id, name: name, createdAt: createdAt.UTC()}, nil
}

// Restore reconstructs a persisted Project.
func Restore(id, name string, createdAt time.Time) (*Project, error) {
	if strings.TrimSpace(id) == "" {
		return nil, errors.New("project id is required")
	}
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidName
	}

	return &Project{id: id, name: name, createdAt: createdAt.UTC()}, nil
}

// ID returns the Project identifier.
func (p Project) ID() string { return p.id }

// Name returns the Project name.
func (p Project) Name() string { return p.name }

// CreatedAt returns when the Project was created.
func (p Project) CreatedAt() time.Time { return p.createdAt }

// Repository persists and retrieves Projects without exposing persistence details.
type Repository interface {
	Save(ctx context.Context, project *Project) error
	GetByID(ctx context.Context, id string) (*Project, error)
}
