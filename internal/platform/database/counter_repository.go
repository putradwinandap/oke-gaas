package database

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/counter"
	"gorm.io/gorm"
)

type counterDefinitionRecord struct {
	ProjectID  string    `gorm:"type:varchar(64);primaryKey;not null;uniqueIndex:uq_counters_project_name,priority:1"`
	ID         string    `gorm:"type:varchar(64);primaryKey;not null"`
	Name       string    `gorm:"type:varchar(255);not null;uniqueIndex:uq_counters_project_name,priority:2"`
	EventType  string    `gorm:"type:varchar(255);not null;index"`
	Conditions []byte    `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (counterDefinitionRecord) TableName() string { return "counter_definitions" }

type playerCounterStateRecord struct {
	ProjectID string    `gorm:"type:varchar(64);primaryKey;not null"`
	PlayerID  string    `gorm:"type:varchar(64);primaryKey;not null"`
	CounterID string    `gorm:"type:varchar(64);primaryKey;not null"`
	Value     int64     `gorm:"not null;default:0;check:chk_player_counters_nonnegative,value >= 0"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (playerCounterStateRecord) TableName() string { return "player_counters" }

// CounterRepository persists immutable Counter definitions.
type CounterRepository struct{ db *gorm.DB }

func NewCounterRepository(db *gorm.DB) *CounterRepository { return &CounterRepository{db: db} }

func (r *CounterRepository) Save(ctx context.Context, value *counter.Definition) error {
	conditions, err := json.Marshal(value.Conditions())
	if err != nil {
		return fmt.Errorf("encode counter conditions: %w", err)
	}
	record := counterDefinitionRecord{
		ProjectID:  value.ProjectID(),
		ID:         value.ID(),
		Name:       value.Name(),
		EventType:  value.EventType(),
		Conditions: conditions,
		CreatedAt:  value.CreatedAt(),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mapped := mapPersistenceError(err)
		if isUniqueConstraint(mapped, "uq_counters_project_name") {
			return counter.ErrNameTaken
		}
		return fmt.Errorf("create counter: %w", mapped)
	}
	return nil
}

func (r *CounterRepository) ListByProject(ctx context.Context, projectID string) ([]*counter.Definition, error) {
	var records []counterDefinitionRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("name ASC, id ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query counters by project: %w", err)
	}
	return restoreCounterDefinitions(records)
}

func (r *CounterRepository) ListByEventType(ctx context.Context, projectID, eventType string) ([]*counter.Definition, error) {
	var records []counterDefinitionRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND event_type = ?", projectID, eventType).
		Order("id ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query counters by event type: %w", err)
	}
	return restoreCounterDefinitions(records)
}

// PlayerCounterRepository atomically increments and queries materialized progress.
type PlayerCounterRepository struct{ db *gorm.DB }

func NewPlayerCounterRepository(db *gorm.DB) *PlayerCounterRepository {
	return &PlayerCounterRepository{db: db}
}

func (r *PlayerCounterRepository) Increment(ctx context.Context, projectID, playerID, counterID string, updatedAt time.Time) (*counter.State, error) {
	if updatedAt.IsZero() {
		return nil, counter.ErrInvalidUpdatedAt
	}
	var record playerCounterStateRecord
	result := r.db.WithContext(ctx).Raw(
		"INSERT INTO player_counters (project_id, player_id, counter_id, value, updated_at) VALUES (?, ?, ?, 1, ?) "+
			"ON CONFLICT (project_id, player_id, counter_id) DO UPDATE SET "+
			"value = player_counters.value + 1, "+
			"updated_at = GREATEST(player_counters.updated_at, EXCLUDED.updated_at) "+
			"WHERE player_counters.value < ? "+
			"RETURNING project_id, player_id, counter_id, value, updated_at",
		projectID,
		playerID,
		counterID,
		updatedAt.UTC().Truncate(time.Microsecond),
		int64(math.MaxInt64),
	).Scan(&record)
	if result.Error != nil {
		return nil, fmt.Errorf("increment player counter: %w", mapPersistenceError(result.Error))
	}
	if result.RowsAffected == 0 {
		return nil, counter.ErrValueOverflow
	}
	return restorePlayerCounterState(record)
}

func (r *PlayerCounterRepository) ListByPlayer(ctx context.Context, projectID, playerID string) ([]*counter.State, error) {
	var records []playerCounterStateRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND player_id = ?", projectID, playerID).
		Order("counter_id ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("query player counters: %w", err)
	}
	values := make([]*counter.State, 0, len(records))
	for _, record := range records {
		value, err := restorePlayerCounterState(record)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func restoreCounterDefinitions(records []counterDefinitionRecord) ([]*counter.Definition, error) {
	values := make([]*counter.Definition, 0, len(records))
	for _, record := range records {
		var conditions map[string]any
		if len(record.Conditions) > 0 {
			decoder := json.NewDecoder(strings.NewReader(string(record.Conditions)))
			decoder.UseNumber()
			if err := decoder.Decode(&conditions); err != nil {
				return nil, fmt.Errorf("decode counter %s conditions: %w", record.ID, err)
			}
		}
		value, err := counter.Restore(record.ID, record.ProjectID, record.Name, record.EventType, conditions, record.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("restore counter %s: %w", record.ID, err)
		}
		values = append(values, value)
	}
	return values, nil
}

func restorePlayerCounterState(record playerCounterStateRecord) (*counter.State, error) {
	value, err := counter.RestoreState(record.ProjectID, record.PlayerID, record.CounterID, record.Value, record.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("restore player counter: %w", err)
	}
	return value, nil
}

var _ counter.Repository = (*CounterRepository)(nil)
var _ counter.StateRepository = (*PlayerCounterRepository)(nil)
