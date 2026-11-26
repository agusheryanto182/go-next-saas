package session

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/usecase/session"

// tracedUseCase wraps the Session use case with OpenTelemetry spans. Attributes carry session and
// user IDs only: never the token, the IP address or the user agent. Use cases cannot import
// pkg/otel, so the two helpers below are local (as in go-clean-template).
type tracedUseCase struct {
	next usecase.Session
}

func newTraced(next usecase.Session) usecase.Session {
	return &tracedUseCase{next: next}
}

func (t *tracedUseCase) Create(ctx context.Context, userID, ip, userAgent string) (string, entity.Session, error) {
	ctx, span := startSpan(ctx, "Session.Create", attribute.String("user.id", userID))
	token, s, err := t.next.Create(ctx, userID, ip, userAgent)
	if err == nil {
		span.SetAttributes(attribute.String("session.id", s.ID))
	}
	endSpan(span, err)
	return token, s, err
}

func (t *tracedUseCase) Authenticate(ctx context.Context, token string) (entity.Session, entity.User, error) {
	ctx, span := startSpan(ctx, "Session.Authenticate")
	s, u, err := t.next.Authenticate(ctx, token)
	if err == nil {
		span.SetAttributes(attribute.String("session.id", s.ID), attribute.String("user.id", u.ID))
	}
	endSpan(span, err)
	return s, u, err
}

func (t *tracedUseCase) Revoke(ctx context.Context, userID, sessionID string, reason entity.RevokeReason) error {
	ctx, span := startSpan(ctx, "Session.Revoke", attribute.String("user.id", userID),
		attribute.String("session.id", sessionID), attribute.String("session.revoked_reason", string(reason)))
	err := t.next.Revoke(ctx, userID, sessionID, reason)
	endSpan(span, err)
	return err
}

func (t *tracedUseCase) RevokeOthers(ctx context.Context, userID, exceptSessionID string, reason entity.RevokeReason) error {
	ctx, span := startSpan(ctx, "Session.RevokeOthers", attribute.String("user.id", userID),
		attribute.String("session.id", exceptSessionID), attribute.String("session.revoked_reason", string(reason)))
	err := t.next.RevokeOthers(ctx, userID, exceptSessionID, reason)
	endSpan(span, err)
	return err
}

func (t *tracedUseCase) Purge(ctx context.Context) error {
	ctx, span := startSpan(ctx, "Session.Purge")
	err := t.next.Purge(ctx)
	endSpan(span, err)
	return err
}

func startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(_tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
