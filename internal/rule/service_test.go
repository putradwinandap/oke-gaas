package rule

import (
	"context"
	"testing"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/project"
	"github.com/stretchr/testify/require"
)

type memoryProjectRepository struct {
	values map[string]*project.Project
}

func (r *memoryProjectRepository) Save(_ context.Context, value *project.Project) error {
	r.values[value.ID()] = value
	return nil
}

func (r *memoryProjectRepository) GetByID(_ context.Context, id string) (*project.Project, error) {
	value, ok := r.values[id]
	if !ok {
		return nil, project.ErrNotFound
	}
	return value, nil
}

type memoryRuleRepository struct {
	values []*Rule
}

func (r *memoryRuleRepository) Save(_ context.Context, value *Rule) error {
	r.values = append(r.values, value)
	return nil
}

func (r *memoryRuleRepository) ListByEventType(_ context.Context, projectID, eventType string) ([]*Rule, error) {
	var matches []*Rule
	for _, value := range r.values {
		if value.ProjectID() == projectID && value.EventType() == eventType {
			matches = append(matches, value)
		}
	}
	return matches, nil
}

func TestCreateExactXPCreatesVersionOneRuleInExistingProject(t *testing.T) {
	projects := &memoryProjectRepository{values: make(map[string]*project.Project)}
	rules := &memoryRuleRepository{}

	proj, err := project.New("Learning", time.Now())
	require.NoError(t, err)
	require.NoError(t, projects.Save(context.Background(), proj))

	service := NewService(projects, rules)
	value, err := service.CreateExactXP(context.Background(), proj.ID(), "lesson_completed", 100)
	require.NoError(t, err)
	require.Equal(t, uint64(1), value.Version())
	require.Equal(t, "lesson_completed", value.EventType())
	require.Equal(t, int64(100), value.XPAmount())
	require.Len(t, rules.values, 1)
}

func TestCreateExactXPRequiresExistingProject(t *testing.T) {
	service := NewService(
		&memoryProjectRepository{values: make(map[string]*project.Project)},
		&memoryRuleRepository{},
	)

	value, err := service.CreateExactXP(context.Background(), "proj_missing", "lesson_completed", 100)
	require.ErrorIs(t, err, project.ErrNotFound)
	require.Nil(t, value)
}
