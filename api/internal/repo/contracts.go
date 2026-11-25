// Package repo declares the storage and external-service interfaces that use cases depend on (ADR 0003).
// Implementations live in persistent/<domain>/ (PostgreSQL via sqlc) and webapi/ (SMTP and other services).
package repo

import (
	"context"

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
