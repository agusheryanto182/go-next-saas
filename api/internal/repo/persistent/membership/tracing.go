package membership

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/membership"

// tracedRepo wraps a MembershipRepo with OpenTelemetry spans.
type tracedRepo struct {
	next repo.MembershipRepo
}

func newTraced(next repo.MembershipRepo) repo.MembershipRepo {
	return &tracedRepo{next: next}
}

func (r *tracedRepo) Store(ctx context.Context, m *entity.Membership) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "MembershipRepo.Store",
		attribute.String("organization.id", m.OrganizationID), attribute.String("user.id", m.UserID))
	err := r.next.Store(ctx, m)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) ListMembers(ctx context.Context, orgID string) ([]entity.Member, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "MembershipRepo.ListMembers", attribute.String("organization.id", orgID))
	result, err := r.next.ListMembers(ctx, orgID)
	otel.EndSpan(span, err)
	return result, err
}

func (r *tracedRepo) ListUserOrganizations(ctx context.Context, userID string) ([]entity.UserOrganization, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "MembershipRepo.ListUserOrganizations", attribute.String("user.id", userID))
	result, err := r.next.ListUserOrganizations(ctx, userID)
	otel.EndSpan(span, err)
	return result, err
}
