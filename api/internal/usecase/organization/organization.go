// Package organization implements usecase.Organization.
package organization

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

// UseCase -.
type UseCase struct {
	tx      repo.Transactor
	users   repo.UserRepo
	orgs    repo.OrganizationRepo
	members repo.MembershipRepo
	hasher  repo.PasswordHasher
}

// New returns the Organization use case instrumented with OpenTelemetry tracing spans.
func New(
	tx repo.Transactor, users repo.UserRepo, orgs repo.OrganizationRepo, members repo.MembershipRepo,
	hasher repo.PasswordHasher,
) usecase.Organization {
	return newTraced(&UseCase{tx: tx, users: users, orgs: orgs, members: members, hasher: hasher})
}

// person is a validated member to add: the user to create if the email is new, and the membership.
type person struct {
	user     entity.User
	password string
	role     entity.Role
	status   entity.MembershipStatus
}

// Create -.
func (uc *UseCase) Create(ctx context.Context, in usecase.CreateOrganizationInput) (usecase.CreateOrganizationResult, error) {
	org := entity.Organization{ID: newID(), Name: strings.TrimSpace(in.Name), Slug: in.Slug}
	if err := org.Validate(); err != nil {
		return usecase.CreateOrganizationResult{}, err
	}
	people, err := validatePeople(in)
	if err != nil {
		return usecase.CreateOrganizationResult{}, err
	}

	// Hash before the unit of work: a hash must not hold a pooled connection (repo.PasswordHasher).
	// A password of a user who turns out to exist is hashed for nothing and never stored.
	for i := range people {
		if people[i].password == "" {
			continue
		}
		if people[i].user.PasswordHash, err = uc.hasher.Hash(ctx, people[i].password); err != nil {
			return usecase.CreateOrganizationResult{}, fmt.Errorf("OrganizationUseCase - Create - uc.hasher.Hash: %w", err)
		}
	}

	// The new organization's ID is the scope: the row-level security policy's WITH CHECK needs it.
	err = uc.tx.InScope(ctx, repo.Scope{OrgID: org.ID}, func(ctx context.Context) error {
		if err := uc.orgs.Store(ctx, &org); err != nil {
			return err
		}
		for _, p := range people {
			userID, err := uc.userID(ctx, p.user)
			if err != nil {
				return err
			}
			m := entity.Membership{OrganizationID: org.ID, UserID: userID, Role: p.role, Status: p.status}
			if err := uc.members.Store(ctx, &m); err != nil {
				return fmt.Errorf("uc.members.Store: %w", err)
			}
		}
		return nil
	})
	if errors.Is(err, entity.ErrOrganizationSlugTaken) {
		return usecase.CreateOrganizationResult{}, nil
	}
	if err != nil {
		return usecase.CreateOrganizationResult{}, fmt.Errorf("OrganizationUseCase - Create - %w", err)
	}
	return usecase.CreateOrganizationResult{Organization: org, Created: true}, nil
}

// userID returns the ID of the user with u's email, creating u if there is none. An existing user
// is left unchanged, password included.
func (uc *UseCase) userID(ctx context.Context, u entity.User) (string, error) {
	existing, err := uc.users.GetByEmail(ctx, u.Email)
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, entity.ErrUserNotFound) {
		return "", fmt.Errorf("uc.users.GetByEmail: %w", err)
	}
	u.ID = newID()
	if err := uc.users.Store(ctx, &u); err != nil {
		return "", fmt.Errorf("uc.users.Store: %w", err)
	}
	return u.ID, nil
}

// validatePeople normalizes and checks the owner and the members, owner first.
func validatePeople(in usecase.CreateOrganizationInput) ([]person, error) {
	all := make([]usecase.NewMember, 0, 1+len(in.Members))
	all = append(all, usecase.NewMember{User: in.Owner, Role: entity.RoleOwner})
	all = append(all, in.Members...)

	people := make([]person, 0, len(all))
	for _, m := range all {
		p := person{
			user:     entity.User{Email: entity.NormalizeEmail(m.User.Email), FullName: strings.TrimSpace(m.User.FullName)},
			password: m.User.Password,
			role:     m.Role,
			status:   m.Status,
		}
		if p.status == "" {
			p.status = entity.MembershipActive
		}
		if err := p.user.Validate(); err != nil {
			return nil, err
		}
		if p.password != "" {
			if err := entity.ValidatePassword(p.password); err != nil {
				return nil, err
			}
		}
		if !p.role.Valid() {
			return nil, entity.ErrInvalidRole
		}
		if !p.status.Valid() {
			return nil, entity.ErrInvalidStatus
		}
		people = append(people, p)
	}
	return people, nil
}

func newID() string { return uuid.Must(uuid.NewV7()).String() }
