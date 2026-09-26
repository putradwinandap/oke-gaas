package database

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

const postgresUniqueViolationCode = "23505"

// UniqueConstraintError preserves a PostgreSQL uniqueness violation without
// collapsing distinct constraints into one domain error.
type UniqueConstraintError struct {
	Constraint string
	Err        error
}

func (e *UniqueConstraintError) Error() string {
	return fmt.Sprintf("unique constraint %q violated: %v", e.Constraint, e.Err)
}

// Unwrap exposes the original PostgreSQL error for diagnostics.
func (e *UniqueConstraintError) Unwrap() error {
	return e.Err
}

func mapPersistenceError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == postgresUniqueViolationCode {
		return &UniqueConstraintError{
			Constraint: pgErr.ConstraintName,
			Err:        err,
		}
	}

	return err
}

func isUniqueConstraint(err error, constraint string) bool {
	var uniqueErr *UniqueConstraintError
	return errors.As(err, &uniqueErr) && uniqueErr.Constraint == constraint
}
