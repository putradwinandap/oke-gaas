package database

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	eventdomain "github.com/putradwinandap/oke-gaas/internal/event"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type eventRecord struct {
	ProjectID  string          `gorm:"type:varchar(64);primaryKey;not null"`
	ID         string          `gorm:"type:varchar(255);primaryKey;not null"`
	PlayerID   string          `gorm:"type:varchar(64);not null;index"`
	EventType  string          `gorm:"column:event_type;type:varchar(255);not null;index"`
	OccurredAt time.Time       `gorm:"not null"`
	ReceivedAt time.Time       `gorm:"not null"`
	Properties json.RawMessage `gorm:"type:jsonb;not null"`
}

func (eventRecord) TableName() string { return "events" }

// EventRepository is the GORM/PostgreSQL adapter for event.Repository.
type EventRepository struct{ db *gorm.DB }

// NewEventRepository creates a GORM-backed Event repository.
func NewEventRepository(db *gorm.DB) *EventRepository { return &EventRepository{db: db} }

// Save persists an Event without aborting an enclosing PostgreSQL transaction on an idempotent collision.
func (r *EventRepository) Save(ctx context.Context, value *eventdomain.Event) error {
	record := eventRecord{
		ProjectID:  value.ProjectID(),
		ID:         value.ID(),
		PlayerID:   value.PlayerID(),
		EventType:  value.Type(),
		OccurredAt: value.OccurredAt(),
		ReceivedAt: value.ReceivedAt(),
		Properties: value.PropertiesJSON(),
	}

	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}, {Name: "id"}},
			DoNothing: true,
		}).
		Create(&record)
	if result.Error != nil {
		return fmt.Errorf("create event: %w", mapPersistenceError(result.Error))
	}
	if result.RowsAffected == 0 {
		return eventdomain.ErrAlreadyExists
	}
	return nil
}

// GetByID retrieves an Event only within the supplied Project scope.
func (r *EventRepository) GetByID(ctx context.Context, projectID, eventID string) (*eventdomain.Event, error) {
	var record eventRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND id = ?", projectID, eventID).
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, eventdomain.ErrNotFound
		}
		return nil, fmt.Errorf("query event by id: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(record.Properties))
	decoder.UseNumber()
	var properties map[string]any
	if err := decoder.Decode(&properties); err != nil {
		return nil, fmt.Errorf("decode event properties: %w", err)
	}

	value, err := eventdomain.Restore(
		record.ID,
		record.ProjectID,
		record.PlayerID,
		record.EventType,
		record.OccurredAt,
		record.ReceivedAt,
		properties,
	)
	if err != nil {
		return nil, fmt.Errorf("restore event: %w", err)
	}
	return value, nil
}

var _ eventdomain.Repository = (*EventRepository)(nil)
