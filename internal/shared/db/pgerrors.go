package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const uniqueViolationSQLState = "23505"

// IsUniqueConstraintViolation returns true when err is a Postgres unique
// violation for the specified constraint.
func IsUniqueConstraintViolation(err error, constraint string) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == uniqueViolationSQLState && pgErr.ConstraintName == constraint
}
