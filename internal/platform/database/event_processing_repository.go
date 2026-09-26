package database

import (
	"context"
	"fmt"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/progression"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type eventProcessingRecord struct {
	ProjectID   string    `gorm:"type:varchar(64);primaryKey;not null"`
	EventID     string    `gorm:"type:varchar(255);primaryKey;not null"`
	ProcessedAt time.Time `gorm:"not null"`
}

func (eventProcessingRecord) TableName() string { return "event_processing" }

type EventProcessingRepository struct{ db *gorm.DB }

func NewEventProcessingRepository(db *gorm.DB) *EventProcessingRepository {
	return &EventProcessingRepository{db: db}
}

// Claim atomically claims an Event for processing.
// The claim is rolled back automatically when the surrounding transaction fails.
func (r *EventProcessingRepository) Claim(
	ctx context.Context,
	projectID, eventID string,
	processedAt time.Time,
) (bool, error) {
	if processedAt.IsZero() {
		return false, fmt.Errorf("processing timestamp is required")
	}

	record := eventProcessingRecord{
		ProjectID:   projectID,
		EventID:     eventID,
		ProcessedAt: processedAt.UTC().Truncate(time.Microsecond),
	}
	result := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "project_id"}, {Name: "event_id"}},
			DoNothing: true,
		}).
		Create(&record)
	if result.Error != nil {
		return false, fmt.Errorf("claim event processing: %w", mapPersistenceError(result.Error))
	}
	return result.RowsAffected == 1, nil
}

var _ progression.ProcessingClaims = (*EventProcessingRepository)(nil)
