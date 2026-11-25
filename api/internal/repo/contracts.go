// Package repo declares the storage and external-service interfaces that use cases depend on (ADR 0003).
// Implementations live in persistent/<domain>/ (PostgreSQL via sqlc) and webapi/ (SMTP and other services).
package repo

import (
	"context"
	"time"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

//go:generate go tool -modfile=../../tools/go.mod mockgen -source=contracts.go -destination=../usecase/mocks_repo_test.go -package=usecase_test

type (
	// Scope is who a unit of work acts for. It becomes the transaction settings app.user_id and
	// app.org_id, which row-level security policies read (ADR 0008). An empty ID leaves that setting
	// empty, which matches no organization data.
	Scope struct {
		UserID string
		OrgID  string
	}

	// Transactor is the unit of work (the one addition to the go-clean-template shape, ADR 0003).
	// InScope runs fn in one transaction whose scope is set first. Repositories called with the ctx
	// that fn receives use that transaction; called with any other ctx they return an error, so no
	// query runs outside a unit of work. fn's error rolls the transaction back. InScope calls do not
	// nest: one transaction has one scope.
	Transactor interface {
		InScope(ctx context.Context, scope Scope, fn func(ctx context.Context) error) error
	}

	// PasswordHasher hashes and verifies passwords (ADR 0011, pkg/password). Its methods wait for a
	// free hashing slot and return ctx.Err() if ctx ends first. Never call them inside InScope: a hash
	// would hold a pooled connection for its whole duration.
	PasswordHasher interface {
		// Hash returns the encoded hash of password with a fresh salt.
		Hash(ctx context.Context, password string) (string, error)
		// Verify reports whether password matches encoded. A malformed encoded returns an error.
		Verify(ctx context.Context, encoded, password string) (bool, error)
		// VerifyDummy takes as long as a Verify against a current hash and discards the result. Run it
		// for unknown emails and accounts that cannot sign in, so timing does not reveal which exist.
		VerifyDummy(ctx context.Context, password string) error
		// NeedsRehash reports whether encoded was made with other parameters than Hash uses now.
		NeedsRehash(encoded string) bool
	}

	// OutboxRepo stores emails to send (ADR 0010). The table is global: use an empty scope.
	OutboxRepo interface {
		// Enqueue inserts e as pending and due now. Call it in the unit of work of the change that
		// causes the email, so the email exists if and only if that change commits.
		Enqueue(ctx context.Context, e entity.Email) error
		// ClaimDue leases up to 10 due emails for 5 minutes and counts the attempt. Commit the claim
		// before sending: rows claimed by an open transaction are skipped by other claimers.
		ClaimDue(ctx context.Context) ([]entity.Email, error)
		// MarkSent records the email as sent and clears its payload.
		MarkSent(ctx context.Context, id string) error
		// Reschedule records a failed attempt: the next one is due after 1, 2, 4… minutes, and the
		// email fails for good after its 8th attempt. lastError must hold no payload values.
		Reschedule(ctx context.Context, id, lastError string) error
		// ClearFailedPayloads clears the payload of failed emails created before createdBefore.
		ClearFailedPayloads(ctx context.Context, createdBefore time.Time) (int64, error)
		// PurgeSent deletes emails sent before sentBefore.
		PurgeSent(ctx context.Context, sentBefore time.Time) (int64, error)
	}

	// Mailer renders an email's template and sends it (repo/webapi/mail). It does no I/O on the
	// database and must be called outside InScope.
	Mailer interface {
		// Send delivers e once; its Message-ID is derived from e.ID.
		Send(ctx context.Context, e entity.Email) error
	}

	// UserRepo stores global user identities.
	UserRepo interface {
		// Store inserts u and sets its timestamps. A taken email returns entity.ErrUserAlreadyExists.
		Store(ctx context.Context, u *entity.User) error
	}

	// OrganizationRepo stores organizations. Visible rows depend on the scope.
	OrganizationRepo interface {
		// Store inserts o and sets its timestamps; the scope's OrgID must be o.ID.
		// A taken slug returns entity.ErrOrganizationSlugTaken.
		Store(ctx context.Context, o *entity.Organization) error
		// GetBySlug returns entity.ErrOrganizationNotFound if no visible organization has the slug.
		GetBySlug(ctx context.Context, slug string) (entity.Organization, error)
	}

	// MembershipRepo stores memberships. Visible rows depend on the scope.
	MembershipRepo interface {
		// Store inserts m as an active membership and sets its status and timestamps; the scope's OrgID
		// must be m.OrganizationID.
		// An existing membership returns entity.ErrMembershipExists.
		Store(ctx context.Context, m *entity.Membership) error
		// ListMembers returns the organization's members, newest first.
		ListMembers(ctx context.Context, orgID string) ([]entity.Member, error)
		// ListUserOrganizations returns the user's organizations by name.
		ListUserOrganizations(ctx context.Context, userID string) ([]entity.UserOrganization, error)
	}
)
