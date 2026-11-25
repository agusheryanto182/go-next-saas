package organization

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

const _tracerName = "github.com/agusheryanto182/go-next-saas/api/internal/usecase/organization"

// tracedUseCase wraps an Organization use case with OpenTelemetry spans. Attributes carry IDs and
// slugs, never emails, names or passwords.
type tracedUseCase struct {
	next usecase.Organization
}

func newTraced(next usecase.Organization) usecase.Organization {
	return &tracedUseCase{next: next}
}

func (t *tracedUseCase) Create(ctx context.Context, in usecase.CreateOrganizationInput) (usecase.CreateOrganizationResult, error) {
	ctx, span := startSpan(ctx, "OrganizationUseCase.Create", attribute.String("organization.slug", in.Slug))
	res, err := t.next.Create(ctx, in)
	span.SetAttributes(attribute.Bool("organization.created", res.Created))
	if res.Created {
		span.SetAttributes(attribute.String("organization.id", res.Organization.ID))
	}
	endSpan(span, err)
	return res, err
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
