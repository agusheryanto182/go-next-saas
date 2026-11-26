package outbox

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/outbox"

// tracedRepo wraps an OutboxRepo with OpenTelemetry spans. Attributes carry IDs and kinds, never
// recipients, payloads or errors that could hold them.
type tracedRepo struct {
	next repo.OutboxRepo
}

func newTraced(next repo.OutboxRepo) repo.OutboxRepo {
	return &tracedRepo{next: next}
}

func (r *tracedRepo) Enqueue(ctx context.Context, e entity.Email) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.Enqueue",
		attribute.String("email.id", e.ID), attribute.String("email.kind", string(e.Kind)))
	err := r.next.Enqueue(ctx, e)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) ClaimDue(ctx context.Context) ([]entity.Email, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.ClaimDue")
	emails, err := r.next.ClaimDue(ctx)
	span.SetAttributes(attribute.Int("email.claimed", len(emails)))
	otel.EndSpan(span, err)
	return emails, err
}

func (r *tracedRepo) Release(ctx context.Context, ids []string) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.Release", attribute.Int("email.released", len(ids)))
	err := r.next.Release(ctx, ids)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) MarkSent(ctx context.Context, id string) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.MarkSent", attribute.String("email.id", id))
	err := r.next.MarkSent(ctx, id)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) Reschedule(ctx context.Context, id, lastError string) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.Reschedule", attribute.String("email.id", id))
	err := r.next.Reschedule(ctx, id, lastError)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) ClearFailedPayloads(ctx context.Context, createdBefore time.Time) (int64, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.ClearFailedPayloads")
	n, err := r.next.ClearFailedPayloads(ctx, createdBefore)
	span.SetAttributes(attribute.Int64("email.cleared", n))
	otel.EndSpan(span, err)
	return n, err
}

func (r *tracedRepo) PurgeSent(ctx context.Context, sentBefore time.Time) (int64, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.PurgeSent")
	n, err := r.next.PurgeSent(ctx, sentBefore)
	span.SetAttributes(attribute.Int64("email.purged", n))
	otel.EndSpan(span, err)
	return n, err
}

func (r *tracedRepo) PurgeFailed(ctx context.Context, createdBefore time.Time) (int64, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "OutboxRepo.PurgeFailed")
	n, err := r.next.PurgeFailed(ctx, createdBefore)
	span.SetAttributes(attribute.Int64("email.purged", n))
	otel.EndSpan(span, err)
	return n, err
}
