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
	if err := db.Exec(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_events_player_project'
			) THEN
				ALTER TABLE events
				ADD CONSTRAINT fk_events_player_project
				FOREIGN KEY (project_id, player_id)
				REFERENCES players(project_id, id)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT;
			END IF;
		END
		$$;
	`).Error; err != nil {
		return fmt.Errorf("create development event player project constraint: %w", err)
	}
	return nil
}
