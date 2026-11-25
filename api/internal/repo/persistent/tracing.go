package persistent

import (
	"context"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"

// tracedTransactor wraps a Transactor with an OpenTelemetry span around the whole unit of work.
type tracedTransactor struct {
	next repo.Transactor
}

func newTraced(next repo.Transactor) repo.Transactor {
	return &tracedTransactor{next: next}
}

func (t *tracedTransactor) InScope(ctx context.Context, scope repo.Scope, fn func(ctx context.Context) error) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "Transactor.InScope",
		attribute.String("scope.user_id", scope.UserID), attribute.String("scope.org_id", scope.OrgID))
	err := t.next.InScope(ctx, scope, fn)
	otel.EndSpan(span, err)
	return err
}
