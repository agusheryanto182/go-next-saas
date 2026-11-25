// Package usecase holds the business rules, one sub-package per domain (ADR 0003).
//
// This file declares the use-case interfaces the controller layer depends on. Use cases import only
// the standard library, internal/entity and internal/repo contracts; lint rules enforce it.
// Tests for every domain live in this directory (package usecase_test) and use the generated mocks.
package usecase

import (
	"context"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

//go:generate go tool -modfile=../../tools/go.mod mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test

type (
	// Organization creates organizations. The development seed and `app create-org` use it (ADR 0012).
	Organization interface {
		// Create creates the organization, its owner and its other members in one unit of work. A user
		// whose email already exists is reused and keeps their password. A taken slug means the
		// organization already exists: nothing changes and the result has Created false. Invalid
		// input returns an entity.ErrInvalid* sentinel. Two runs creating the same new user at the
		// same moment can fail one of them (entity.ErrUserAlreadyExists), which then changes nothing.
		Create(ctx context.Context, in CreateOrganizationInput) (CreateOrganizationResult, error)
	}

	// NewUser is a person to add to an organization, matched to an existing user by email.
	NewUser struct {
		Email    string
		FullName string
		// Password is set only on a newly created user. Empty leaves it without a password.
		Password string
	}

	// NewMember is a member created together with the organization, besides the owner.
	NewMember struct {
		User   NewUser
		Role   entity.Role
		Status entity.MembershipStatus // empty means active
	}

	// CreateOrganizationInput -.
	CreateOrganizationInput struct {
		Name    string
		Slug    string
		Owner   NewUser
		Members []NewMember
	}

	// CreateOrganizationResult -.
	CreateOrganizationResult struct {
		// Organization is the new organization; zero when Created is false.
		Organization entity.Organization
		Created      bool
	}
)
