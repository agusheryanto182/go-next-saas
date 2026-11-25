package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

func TestLoggerAddsTraceIDs(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log := New(&buf, slog.LevelInfo).With("component", "test")

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1},
		SpanID:     trace.SpanID{2},
		TraceFlags: trace.FlagsSampled,
	})
	log.InfoContext(trace.ContextWithSpanContext(context.Background(), sc), "hello")

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got), "log line is not JSON: %s", buf.String())
	assert.Equal(t, sc.TraceID().String(), got["trace_id"])
	assert.Equal(t, sc.SpanID().String(), got["span_id"])
	assert.Equal(t, "test", got["component"])
}

func TestLoggerWithoutSpan(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	New(&buf, slog.LevelInfo).InfoContext(context.Background(), "hello")

	assert.NotContains(t, buf.String(), "trace_id")
}

func TestLoggerRespectsLevel(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	New(&buf, slog.LevelWarn).Info("hidden")

	assert.Zero(t, buf.Len(), "info logged at warn level: %s", buf.String())
}
