package otel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// StartSpan starts a span from the tracer named tracerName (by convention, the caller's package path).
// Attributes must not carry personal data such as email addresses: traces leave the system.
func StartSpan(ctx context.Context, tracerName, spanName string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, spanName, trace.WithAttributes(attrs...))
}

// EndSpan records err (if any) on span and ends it.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
