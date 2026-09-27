package progression

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/putradwinandap/oke-gaas/internal/project"
)

var (
	ErrInvalidLevelProjectID       = errors.New("level project id is required")
	ErrInvalidLevelNumber          = errors.New("level number must be greater than zero")
	ErrInvalidLevelMinXP           = errors.New("level min_xp is invalid")
	ErrLevelNotConfigured          = errors.New("no configured level threshold")
	ErrLevelThresholdNotIncreasing = errors.New("level min_xp must be greater than the previous threshold")
	ErrLevelThresholdConflict      = errors.New("level threshold conflicts with an existing level")
)

// LevelThreshold is one immutable Project-scoped XP boundary.
// Level 1 at 0 XP is implicit; persisted thresholds begin at level 2.
type LevelThreshold struct {
	projectID string
	level     uint32
	minXP     int64
}

func RestoreLevelThreshold(projectID string, level uint32, minXP int64) (*LevelThreshold, error) {
	projectID = strings.TrimSpace(projectID)
	switch {
	case projectID == "":
		return nil, ErrInvalidLevelProjectID
	case level == 0:
		return nil, ErrInvalidLevelNumber
	case level == 1 && minXP != 0:
		return nil, ErrInvalidLevelMinXP
	case level > 1 && minXP <= 0:
		return nil, ErrInvalidLevelMinXP
	}
	return &LevelThreshold{projectID: projectID, level: level, minXP: minXP}, nil
}

func (l LevelThreshold) ProjectID() string { return l.projectID }
func (l LevelThreshold) Level() uint32     { return l.level }
func (l LevelThreshold) MinXP() int64      { return l.minXP }

// LevelRepository persists configured thresholds and resolves the current level.
// Level 1 is implicit and therefore is never persisted.
type LevelRepository interface {
	Save(ctx context.Context, threshold *LevelThreshold) error
	Latest(ctx context.Context, projectID string) (*LevelThreshold, error)
	List(ctx context.Context, projectID string) ([]*LevelThreshold, error)
	Resolve(ctx context.Context, projectID string, xp int64) (uint32, error)
}

// LevelService owns append-only Project level configuration and current-level resolution.
type LevelService struct {
	projects project.Repository
	levels   LevelRepository
}

func NewLevelService(projects project.Repository, levels LevelRepository) *LevelService {
	return &LevelService{projects: projects, levels: levels}
}

func (s *LevelService) Add(ctx context.Context, projectID string, minXP int64) (*LevelThreshold, error) {
	if s == nil || s.projects == nil || s.levels == nil {
		return nil, fmt.Errorf("level service dependencies are required")
	}
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("get project for level threshold: %w", err)
	}
	if minXP <= 0 {
		return nil, ErrInvalidLevelMinXP
	}

	nextLevel := uint32(2)
	previousMinXP := int64(0)
	latest, err := s.levels.Latest(ctx, projectID)
	if err != nil && !errors.Is(err, ErrLevelNotConfigured) {
		return nil, fmt.Errorf("load latest level threshold: %w", err)
	}
	if latest != nil {
		if latest.Level() == ^uint32(0) {
			return nil, ErrInvalidLevelNumber
		}
		nextLevel = latest.Level() + 1
		previousMinXP = latest.MinXP()
	}
	if minXP <= previousMinXP {
		return nil, ErrLevelThresholdNotIncreasing
	}

	threshold, err := RestoreLevelThreshold(projectID, nextLevel, minXP)
	if err != nil {
		return nil, err
	}
	if err := s.levels.Save(ctx, threshold); err != nil {
		return nil, fmt.Errorf("save level threshold: %w", err)
	}
	return threshold, nil
}

func (s *LevelService) List(ctx context.Context, projectID string) ([]*LevelThreshold, error) {
	if s == nil || s.projects == nil || s.levels == nil {
		return nil, fmt.Errorf("level service dependencies are required")
	}
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return nil, fmt.Errorf("get project for level thresholds: %w", err)
	}
	configured, err := s.levels.List(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list level thresholds: %w", err)
	}
	implicit, err := RestoreLevelThreshold(projectID, 1, 0)
	if err != nil {
		return nil, err
	}
	result := make([]*LevelThreshold, 0, len(configured)+1)
	result = append(result, implicit)
	result = append(result, configured...)
	return result, nil
}

func (s *LevelService) Resolve(ctx context.Context, projectID string, xp int64) (uint32, error) {
	if s == nil || s.projects == nil || s.levels == nil {
		return 0, fmt.Errorf("level service dependencies are required")
	}
	if xp < 0 {
		return 0, ErrInvalidXP
	}
	if _, err := s.projects.GetByID(ctx, projectID); err != nil {
		return 0, fmt.Errorf("get project for level resolution: %w", err)
	}
	level, err := s.levels.Resolve(ctx, projectID, xp)
	if err != nil {
		return 0, fmt.Errorf("resolve player level: %w", err)
	}
	if level == 0 {
		return 0, ErrInvalidLevelNumber
	}
	return level, nil
}
