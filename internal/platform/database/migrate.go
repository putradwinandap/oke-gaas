package database

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateCore creates or updates the persistence schema required by the current core modules.
func MigrateCore(db *gorm.DB) error {
	if err := db.AutoMigrate(&projectRecord{}, &playerRecord{}); err != nil {
		return fmt.Errorf("migrate core schema: %w", err)
	}
	return nil
}
