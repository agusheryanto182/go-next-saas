package session

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/session"

// tracedRepo wraps a SessionRepo with OpenTelemetry spans. Attributes carry session and user IDs,
// never tokens, their hashes, IP addresses or user agents.
type tracedRepo struct {
	next repo.SessionRepo
}

func newTraced(next repo.SessionRepo) repo.SessionRepo {
	return &tracedRepo{next: next}
}

func (r *tracedRepo) Store(ctx context.Context, s entity.Session, tokenHash []byte) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "SessionRepo.Store",
		attribute.String("session.id", s.ID), attribute.String("user.id", s.UserID))
	err := r.next.Store(ctx, s, tokenHash)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) GetActiveByTokenHash(ctx context.Context, tokenHash []byte, now time.Time) (entity.Session, entity.User, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "SessionRepo.GetActiveByTokenHash")
	s, u, err := r.next.GetActiveByTokenHash(ctx, tokenHash, now)
	if err == nil {
		span.SetAttributes(attribute.String("session.id", s.ID), attribute.String("user.id", u.ID))
	}
	otel.EndSpan(span, err)
	return s, u, err
}

func (r *tracedRepo) Touch(ctx context.Context, id string, now time.Time) (bool, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "SessionRepo.Touch", attribute.String("session.id", id))
	touched, err := r.next.Touch(ctx, id, now)
	span.SetAttributes(attribute.Bool("session.touched", touched))
	otel.EndSpan(span, err)
	return touched, err
}

func (r *tracedRepo) Revoke(ctx context.Context, userID, id string, reason entity.RevokeReason, now time.Time) error {
	ctx, span := otel.StartSpan(ctx, _tracerName, "SessionRepo.Revoke",
		attribute.String("user.id", userID), attribute.String("session.id", id),
		attribute.String("session.revoked_reason", string(reason)))
	err := r.next.Revoke(ctx, userID, id, reason, now)
	otel.EndSpan(span, err)
	return err
}

func (r *tracedRepo) RevokeOthers(ctx context.Context, userID, exceptID string, reason entity.RevokeReason, now time.Time) (int64, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "SessionRepo.RevokeOthers",
		attribute.String("user.id", userID), attribute.String("session.id", exceptID),
		attribute.String("session.revoked_reason", string(reason)))
	n, err := r.next.RevokeOthers(ctx, userID, exceptID, reason, now)
	span.SetAttributes(attribute.Int64("session.revoked", n))
	otel.EndSpan(span, err)
	return n, err
}

func (r *tracedRepo) PurgeEnded(ctx context.Context, endedBefore time.Time) (int64, error) {
	ctx, span := otel.StartSpan(ctx, _tracerName, "SessionRepo.PurgeEnded")
	n, err := r.next.PurgeEnded(ctx, endedBefore)
	span.SetAttributes(attribute.Int64("session.purged", n))
	otel.EndSpan(span, err)
	return n, err
}
