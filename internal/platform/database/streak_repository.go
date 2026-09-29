package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/putradwinandap/oke-gaas/internal/streak"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type streakDefinitionRecord struct {
	ProjectID  string    `gorm:"type:varchar(64);primaryKey;not null;uniqueIndex:uq_streaks_project_name,priority:1"`
	ID         string    `gorm:"type:varchar(64);primaryKey;not null"`
	Name       string    `gorm:"type:varchar(255);not null;uniqueIndex:uq_streaks_project_name,priority:2"`
	EventType  string    `gorm:"type:varchar(255);not null;index"`
	Conditions []byte    `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt  time.Time `gorm:"not null"`
}

func (streakDefinitionRecord) TableName() string { return "streak_definitions" }

type streakDayRecord struct {
	ProjectID string    `gorm:"type:varchar(64);primaryKey;not null"`
	PlayerID  string    `gorm:"type:varchar(64);primaryKey;not null"`
	StreakID  string    `gorm:"type:varchar(64);primaryKey;not null"`
	Day       time.Time `gorm:"type:date;primaryKey;column:qualified_day;not null"`
}

func (streakDayRecord) TableName() string { return "streak_days" }

type streakEventClaimRecord struct {
	ProjectID   string    `gorm:"type:varchar(64);primaryKey;not null"`
	EventID     string    `gorm:"type:varchar(255);primaryKey;not null"`
	StreakID    string    `gorm:"type:varchar(64);primaryKey;not null"`
	PlayerID    string    `gorm:"type:varchar(64);not null"`
	Day         time.Time `gorm:"type:date;column:qualified_day;not null"`
	QualifiedAt time.Time `gorm:"not null"`
}

func (streakEventClaimRecord) TableName() string { return "streak_event_claims" }

type StreakRepository struct{ db *gorm.DB }

func NewStreakRepository(db *gorm.DB) *StreakRepository { return &StreakRepository{db: db} }
func (r *StreakRepository) Save(ctx context.Context, d *streak.Definition) error {
	b, err := json.Marshal(d.Conditions())
	if err != nil {
		return fmt.Errorf("encode streak conditions: %w", err)
	}
	rec := streakDefinitionRecord{ProjectID: d.ProjectID(), ID: d.ID(), Name: d.Name(), EventType: d.EventType(), Conditions: b, CreatedAt: d.CreatedAt()}
	if err := r.db.WithContext(ctx).Create(&rec).Error; err != nil {
		mapped := mapPersistenceError(err)
		if isUniqueConstraint(mapped, "uq_streaks_project_name") {
			return streak.ErrNameTaken
		}
		return fmt.Errorf("create streak: %w", mapped)
	}
	return nil
}
func (r *StreakRepository) ListByProject(ctx context.Context, p string) ([]*streak.Definition, error) {
	var rows []streakDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ?", p).Order("name ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query streaks by project: %w", err)
	}
	return restoreStreakDefinitions(rows)
}
func (r *StreakRepository) ListByEventType(ctx context.Context, p, e string) ([]*streak.Definition, error) {
	var rows []streakDefinitionRecord
	if err := r.db.WithContext(ctx).Where("project_id = ? AND event_type = ?", p, e).Order("id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query streaks by event type: %w", err)
	}
	return restoreStreakDefinitions(rows)
}
func restoreStreakDefinitions(rows []streakDefinitionRecord) ([]*streak.Definition, error) {
	out := make([]*streak.Definition, 0, len(rows))
	for _, row := range rows {
		var cond map[string]any
		if len(row.Conditions) > 0 {
			dec := json.NewDecoder(strings.NewReader(string(row.Conditions)))
			dec.UseNumber()
			if err := dec.Decode(&cond); err != nil {
				return nil, fmt.Errorf("decode streak %s conditions: %w", row.ID, err)
			}
		}
		v, err := streak.Restore(row.ID, row.ProjectID, row.Name, row.EventType, cond, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

type StreakDayRepository struct{ db *gorm.DB }

func NewStreakDayRepository(db *gorm.DB) *StreakDayRepository { return &StreakDayRepository{db: db} }
func (r *StreakDayRepository) Claim(ctx context.Context, q *streak.QualifiedDay) (bool, error) {
	eventClaim := streakEventClaimRecord{ProjectID: q.ProjectID(), PlayerID: q.PlayerID(), StreakID: q.StreakID(), EventID: q.EventID(), Day: q.Day(), QualifiedAt: q.QualifiedAt()}
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&eventClaim)
	if result.Error != nil {
		return false, fmt.Errorf("record streak event claim: %w", mapPersistenceError(result.Error))
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	day := streakDayRecord{ProjectID: q.ProjectID(), PlayerID: q.PlayerID(), StreakID: q.StreakID(), Day: q.Day()}
	result = r.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&day)
	if result.Error != nil {
		return false, fmt.Errorf("claim streak day: %w", mapPersistenceError(result.Error))
	}
	return result.RowsAffected == 1, nil
}
func (r *StreakDayRepository) DaysByPlayer(ctx context.Context, p, pl string) (map[string][]time.Time, error) {
	var rows []streakDayRecord
	if err := r.db.WithContext(ctx).Table("streak_days").Where("project_id = ? AND player_id = ?", p, pl).Order("qualified_day ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query player streak days: %w", err)
	}
	out := make(map[string][]time.Time)
	for _, row := range rows {
		out[row.StreakID] = append(out[row.StreakID], row.Day.UTC())
	}
	return out, nil
}

var _ streak.Repository = (*StreakRepository)(nil)
var _ streak.ClaimRepository = (*StreakDayRepository)(nil)
