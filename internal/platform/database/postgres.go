package database

import (
	"errors"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// OpenPostgres creates the PostgreSQL persistence adapter.
// The returned *gorm.DB must remain inside infrastructure/persistence boundaries.
func OpenPostgres(dsn string) (*gorm.DB, error) {
	if dsn == "" {
		return nil, errors.New("database URL is required")
	}

	return gorm.Open(postgres.Open(dsn), &gorm.Config{})
}
