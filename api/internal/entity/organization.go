package entity

import (
	"time"
	"unicode/utf8"
)

// Organization is a tenant. Its data is isolated by row-level security (ADR 0008).
type Organization struct {
	ID        string
	Name      string
	Slug      string // URL name, ^[a-z0-9-]{3,40}$
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks the name and slug as the database does (organizations_name_length,
// organizations_slug_format).
func (o Organization) Validate() error {
	if n := utf8.RuneCountInString(o.Name); n < 1 || n > 100 {
		return ErrInvalidOrganizationName
	}
	if !validSlug(o.Slug) {
		return ErrInvalidOrganizationSlug
	}
	return nil
}

func validSlug(s string) bool {
	if len(s) < 3 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return false
		}
	}
	return true
}
