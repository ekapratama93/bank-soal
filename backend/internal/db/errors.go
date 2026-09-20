package db

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	UniqueViolation     = "23505"
	ForeignKeyViolation = "23503"
)

// IsUniqueViolation reports whether err is a Postgres unique-constraint
// violation (e.g. a race between two inserts for the same (quiz_id,
// client_id) attempt).
func IsUniqueViolation(err error) bool {
	return hasCode(err, UniqueViolation)
}

// IsForeignKeyViolation reports whether err is a Postgres foreign-key
// violation (e.g. deleting a subject/exam type still referenced by rows).
func IsForeignKeyViolation(err error) bool {
	return hasCode(err, ForeignKeyViolation)
}

func hasCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == code
	}
	return false
}
