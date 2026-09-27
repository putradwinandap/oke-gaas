package level

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrInvalidProjectID       = errors.New("level project id is required")
	ErrInvalidMinXP           = errors.New("level min_xp must be greater than zero")
	ErrThresholdNotIncreasing = errors.New("level min_xp must be greater than the previous threshold")
)

type Threshold struct {
	projectID string
	number    uint64
	minXP     int64
}

func RestoreThreshold(projectID string, number uint64, minXP int64) (*Threshold, error) {
	if strings.TrimSpace(projectID) == "" {
		return nil, ErrInvalidProjectID
	}
	if number < 2 {
		return nil, errors.New("configured level number must be at least 2")
	}
	if minXP <= 0 {
		return nil, ErrInvalidMinXP
	}
	return &Threshold{projectID: projectID, number: number, minXP: minXP}, nil
}

func (t Threshold) ProjectID() string { return t.projectID }
func (t Threshold) Number() uint64    { return t.number }
func (t Threshold) MinXP() int64      { return t.minXP }

type Repository interface {
	Append(ctx context.Context, projectID string, minXP int64) (*Threshold, error)
	List(ctx context.Context, projectID string) ([]*Threshold, error)
	Resolve(ctx context.Context, projectID string, xp int64) (uint64, error)
}
