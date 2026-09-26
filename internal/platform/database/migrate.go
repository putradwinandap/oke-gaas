package database

import (
	"fmt"

	"gorm.io/gorm"
)

// AutoMigrateCoreForDevelopment creates or updates the current core schema for
// local development and tests only. Production schema evolution must use the
// versioned SQL migrations under /migrations.
func AutoMigrateCoreForDevelopment(db *gorm.DB) error {
	if err := db.AutoMigrate(&projectRecord{}, &playerRecord{}, &eventRecord{}); err != nil {
		return fmt.Errorf("auto-migrate core development schema: %w", err)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_players_project_id_unique
		ON players(project_id, id)
	`).Error; err != nil {
		return fmt.Errorf("create development player project identity index: %w", err)
	}
	if err := ensureDevelopmentForeignKey(
		db,
		"events",
		"fk_events_project",
		`
			ALTER TABLE events
			ADD CONSTRAINT fk_events_project
			FOREIGN KEY (project_id)
			REFERENCES projects(id)
			ON UPDATE RESTRICT
			ON DELETE RESTRICT
		`,
	); err != nil {
		return fmt.Errorf("create development event project constraint: %w", err)
	}
	if err := ensureDevelopmentForeignKey(
		db,
		"events",
		"fk_events_player_project",
		`
			ALTER TABLE events
			ADD CONSTRAINT fk_events_player_project
			FOREIGN KEY (project_id, player_id)
			REFERENCES players(project_id, id)
			ON UPDATE RESTRICT
			ON DELETE RESTRICT
		`,
	); err != nil {
		return fmt.Errorf("create development event player project constraint: %w", err)
	}
	return nil
}

func ensureDevelopmentForeignKey(db *gorm.DB, tableName, constraintName, statement string) error {
	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conname = ?
			  AND conrelid = ?::regclass
			  AND contype = 'f'
		)
	`, constraintName, tableName).Scan(&exists).Error; err != nil {
		return fmt.Errorf("check constraint %s on %s: %w", constraintName, tableName, err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("add constraint %s on %s: %w", constraintName, tableName, err)
	}
	return nil
}
