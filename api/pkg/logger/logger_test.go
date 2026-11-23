package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLoggerAddsTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo).With("component", "test")

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	log.InfoContext(trace.ContextWithSpanContext(context.Background(), sc), "hello")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("log line is not JSON: %v: %s", err, buf.String())
	}
	if got["trace_id"] != sc.TraceID().String() || got["span_id"] != sc.SpanID().String() || got["component"] != "test" {
		t.Errorf("missing fields in %v", got)
	}
}

func TestLoggerWithoutSpan(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, slog.LevelInfo).InfoContext(context.Background(), "hello")

	if bytes.Contains(buf.Bytes(), []byte("trace_id")) {
		t.Errorf("unexpected trace_id without a span: %s", buf.String())
	}
}

func TestLoggerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	New(&buf, slog.LevelWarn).Info("hidden")

	if buf.Len() != 0 {
		t.Errorf("info logged at warn level: %s", buf.String())
	}
}
