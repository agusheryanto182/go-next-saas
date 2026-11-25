package user

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/user"

// tracedRepo wraps a UserRepo with OpenTelemetry spans. Attributes carry IDs, never emails.
type tracedRepo struct {
	next repo.UserRepo
}

func newTraced(next repo.UserRepo) repo.UserRepo {
	return &tracedRepo{next: next}
}

func (r *tracedRepo) Store(ctx context.Context, u *entity.User) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "UserRepo.Store", attribute.String("user.id", u.ID))
	err := r.next.Store(ctx, u)
	otel.EndSpan(span, err)
	return err
}
