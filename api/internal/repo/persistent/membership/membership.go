// Package membership implements repo.MembershipRepo on PostgreSQL.
package membership

import (
	"context"
	"fmt"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

// Repo -.
type Repo struct{}

// New returns a Membership repository instrumented with OpenTelemetry tracing spans.
func New() repo.MembershipRepo {
	return newTraced(&Repo{})
}

// Store -.
func (r *Repo) Store(ctx context.Context, m *entity.Membership) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("MembershipRepo - Store - persistent.Queries: %w", err)
	}
	orgID, err := persistent.ParseID(m.OrganizationID)
	if err != nil {
		return fmt.Errorf("MembershipRepo - Store - persistent.ParseID organization: %w", err)
	}
	userID, err := persistent.ParseID(m.UserID)
	if err != nil {
		return fmt.Errorf("MembershipRepo - Store - persistent.ParseID user: %w", err)
	}

	row, err := q.CreateMembership(ctx, gen.CreateMembershipParams{OrganizationID: orgID, UserID: userID, Role: string(m.Role)})
	if err != nil {
		if persistent.IsUniqueViolation(err, "memberships_pkey") {
			return entity.ErrMembershipExists
		}
		return fmt.Errorf("MembershipRepo - Store - q.CreateMembership: %w", err)
	}

	m.Status = entity.MembershipStatus(row.Status)
	m.CreatedAt, m.UpdatedAt = row.CreatedAt, row.UpdatedAt
	return nil
}

// ListMembers -.
func (r *Repo) ListMembers(ctx context.Context, orgID string) ([]entity.Member, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return nil, fmt.Errorf("MembershipRepo - ListMembers - persistent.Queries: %w", err)
	}
	id, err := persistent.ParseID(orgID)
	if err != nil {
		return nil, fmt.Errorf("MembershipRepo - ListMembers - persistent.ParseID: %w", err)
	}

	rows, err := q.ListOrganizationMembers(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MembershipRepo - ListMembers - q.ListOrganizationMembers: %w", err)
	}

	members := make([]entity.Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, entity.Member{
			UserID:   row.UserID.String(),
			Email:    row.Email,
			FullName: row.FullName,
			Phone:    persistent.TextOrEmpty(row.Phone),
			Role:     entity.Role(row.Role),
			Status:   entity.MembershipStatus(row.Status),
			JoinedAt: row.JoinedAt,
		})
	}
	return members, nil
}

// ListUserOrganizations -.
func (r *Repo) ListUserOrganizations(ctx context.Context, userID string) ([]entity.UserOrganization, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return nil, fmt.Errorf("MembershipRepo - ListUserOrganizations - persistent.Queries: %w", err)
	}
	id, err := persistent.ParseID(userID)
	if err != nil {
		return nil, fmt.Errorf("MembershipRepo - ListUserOrganizations - persistent.ParseID: %w", err)
	}

	rows, err := q.ListUserOrganizations(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("MembershipRepo - ListUserOrganizations - q.ListUserOrganizations: %w", err)
	}

	orgs := make([]entity.UserOrganization, 0, len(rows))
	for _, row := range rows {
		orgs = append(orgs, entity.UserOrganization{
			OrganizationID: row.ID.String(),
			Name:           row.Name,
			Slug:           row.Slug,
			Role:           entity.Role(row.Role),
			Status:         entity.MembershipStatus(row.Status),
		})
	}
	return orgs, nil
}
