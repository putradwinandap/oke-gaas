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
	return nil
}
