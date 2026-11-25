package persistent

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// ParseID parses an entity ID (a UUID string).
func ParseID(id string) (uuid.UUID, error) {
	u, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w %q", ErrInvalidID, id)
	}
	return u, nil
}

// TextOrNull maps an empty optional entity field to SQL NULL.
func TextOrNull(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// TextOrEmpty maps SQL NULL to an empty optional entity field.
func TextOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// IsUniqueViolation reports whether err is a unique_violation (23505) on the named constraint.
func IsUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
