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
)

const eventProjectIdentityConstraint = "events_pkey"

type eventRecord struct {
	ProjectID string `gorm:"type:varchar(64);primaryKey;not null"`
	ID string `gorm:"type:varchar(255);primaryKey;not null"`
	PlayerID string `gorm:"type:varchar(64);not null;index"`
	EventType string `gorm:"column:event_type;type:varchar(255);not null;index"`
	OccurredAt time.Time `gorm:"not null"`
	ReceivedAt time.Time `gorm:"not null"`
	Properties json.RawMessage `gorm:"type:jsonb;not null"`
}
func (eventRecord) TableName() string { return "events" }

type EventRepository struct { db *gorm.DB }
func NewEventRepository(db *gorm.DB) *EventRepository { return &EventRepository{db:db} }

func (r *EventRepository) Save(ctx context.Context, value *eventdomain.Event) error {
	record := eventRecord{
		ProjectID:value.ProjectID(), ID:value.ID(), PlayerID:value.PlayerID(), EventType:value.Type(),
		OccurredAt:value.OccurredAt(), ReceivedAt:value.ReceivedAt(), Properties:value.PropertiesJSON(),
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		mappedErr := mapPersistenceError(err)
		if isUniqueConstraint(mappedErr, eventProjectIdentityConstraint) { return eventdomain.ErrAlreadyExists }
		return fmt.Errorf("create event: %w", mappedErr)
	}
	return nil
}

func (r *EventRepository) GetByID(ctx context.Context, projectID, eventID string) (*eventdomain.Event, error) {
	var record eventRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND id = ?",projectID,eventID).First(&record).Error; err != nil {
		if errors.Is(err,gorm.ErrRecordNotFound) { return nil,eventdomain.ErrNotFound }
		return nil,fmt.Errorf("query event by id: %w",err)
	}
	decoder := json.NewDecoder(bytes.NewReader(record.Properties))
	decoder.UseNumber()
	var properties map[string]any
	if err := decoder.Decode(&properties); err != nil { return nil,fmt.Errorf("decode event properties: %w",err) }

	value, err := eventdomain.Restore(
		record.ID, record.ProjectID, record.PlayerID, record.EventType,
		record.OccurredAt, record.ReceivedAt, properties,
	)
	if err != nil { return nil,fmt.Errorf("restore event: %w",err) }
	return value,nil
}

var _ eventdomain.Repository = (*EventRepository)(nil)
