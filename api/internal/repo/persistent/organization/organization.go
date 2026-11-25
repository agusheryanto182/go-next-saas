// Package organization implements repo.OrganizationRepo on PostgreSQL.
package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

// Repo -.
type Repo struct{}

// New returns an Organization repository instrumented with OpenTelemetry tracing spans.
func New() repo.OrganizationRepo {
	return newTraced(&Repo{})
}

// Store -.
func (r *Repo) Store(ctx context.Context, o *entity.Organization) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("OrganizationRepo - Store - persistent.Queries: %w", err)
	}
	id, err := persistent.ParseID(o.ID)
	if err != nil {
		return fmt.Errorf("OrganizationRepo - Store - persistent.ParseID: %w", err)
	}

	row, err := q.CreateOrganization(ctx, gen.CreateOrganizationParams{ID: id, Name: o.Name, Slug: o.Slug})
	if err != nil {
		if persistent.IsUniqueViolation(err, "organizations_slug_key") {
			return entity.ErrOrganizationSlugTaken
		}
		return fmt.Errorf("OrganizationRepo - Store - q.CreateOrganization: %w", err)
	}

	o.CreatedAt, o.UpdatedAt = row.CreatedAt, row.UpdatedAt
	return nil
}

// GetBySlug -.
func (r *Repo) GetBySlug(ctx context.Context, slug string) (entity.Organization, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return entity.Organization{}, fmt.Errorf("OrganizationRepo - GetBySlug - persistent.Queries: %w", err)
	}

	row, err := q.GetOrganizationBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Organization{}, entity.ErrOrganizationNotFound
		}
		return entity.Organization{}, fmt.Errorf("OrganizationRepo - GetBySlug - q.GetOrganizationBySlug: %w", err)
	}

	return toEntity(row), nil
}

func toEntity(row gen.Organization) entity.Organization {
	return entity.Organization{
		ID:        row.ID.String(),
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
