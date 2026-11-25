package organization

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/organization"

// tracedRepo wraps an OrganizationRepo with OpenTelemetry spans.
type tracedRepo struct {
	next repo.OrganizationRepo
}

func newTraced(next repo.OrganizationRepo) repo.OrganizationRepo {
	return &tracedRepo{next: next}
}

func (r *tracedRepo) Store(ctx context.Context, o *entity.Organization) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OrganizationRepo.Store", attribute.String("organization.id", o.ID))
	err := r.next.Store(ctx, o)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) GetBySlug(ctx context.Context, slug string) (entity.Organization, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OrganizationRepo.GetBySlug", attribute.String("organization.slug", slug))
	result, err := r.next.GetBySlug(ctx, slug)
	otel.EndSpan(span, err)
	return result, err
}
