// Package repo declares the storage and external-service interfaces that use cases depend on (ADR 0003).
// Implementations live in persistent/ (PostgreSQL via sqlc) and webapi/ (SMTP and other services).
package repo

import "github.com/google/uuid"

// Scope is who a unit of work acts for. It becomes the transaction settings app.user_id and app.org_id,
// which row-level security policies read (ADR 0008). A zero ID leaves that setting empty, which
// matches no organization data.
type Scope struct {
	UserID uuid.UUID
	OrgID  uuid.UUID
}
