package database

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenPostgresRejectsEmptyDSN(t *testing.T) {
	db, err := OpenPostgres("")

	require.Error(t, err)
	require.Nil(t, db)
}
