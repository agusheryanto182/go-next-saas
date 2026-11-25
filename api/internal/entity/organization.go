package entity

import "time"

// Organization is a tenant. Its data is isolated by row-level security (ADR 0008).
type Organization struct {
	ID        string
	Name      string
	Slug      string // URL name, ^[a-z0-9-]{3,40}$
	CreatedAt time.Time
	UpdatedAt time.Time
}
