package mail

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/usecase/mail"

// tracedUseCase wraps the Mail use case with OpenTelemetry spans. Use cases cannot import pkg/otel,
// so the two helpers below are local (as in go-clean-template).
type tracedUseCase struct {
	next usecase.Mail
}

func newTraced(next usecase.Mail) usecase.Mail {
	return &tracedUseCase{next: next}
}

func (t *tracedUseCase) SendDue(ctx context.Context) (int, error) {
	ctx, span := startSpan(ctx, "Mail.SendDue")
	n, err := t.next.SendDue(ctx)
	span.SetAttributes(attribute.Int("email.claimed", n))
	endSpan(span, err)
	return n, err
}

func (t *tracedUseCase) Maintain(ctx context.Context) error {
	ctx, span := startSpan(ctx, "Mail.Maintain")
	err := t.next.Maintain(ctx)
	endSpan(span, err)
	return err
}

func startSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return otel.Tracer(_tracerName).Start(ctx, name)
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
