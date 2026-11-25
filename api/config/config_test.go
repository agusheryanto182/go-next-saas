package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://saas_app:secret@localhost:5432/saas")
	t.Setenv("APP_BASE_URL", "http://localhost:3000")
}

//nolint:paralleltest // t.Setenv cannot be used in parallel tests
func TestNewDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := New()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, slog.LevelInfo, cfg.SlogLevel())
}

func TestNewRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{"missing database URL", map[string]string{"DATABASE_URL": ""}, "DATABASE_URL"},
		{"relative app URL", map[string]string{"APP_BASE_URL": "localhost:3000"}, "APP_BASE_URL"},
		{"unknown log level", map[string]string{"LOG_LEVEL": "loud"}, "LOG_LEVEL"},
		{"bad OTLP endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "collector:4318"}, "OTEL_EXPORTER_OTLP_ENDPOINT"},
		{"zero shutdown timeout", map[string]string{"SHUTDOWN_TIMEOUT": "0s"}, "SHUTDOWN_TIMEOUT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setValidEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}

			_, err := New()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
