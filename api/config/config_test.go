package config

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://saas_app:secret@localhost:5432/saas")
	t.Setenv("APP_BASE_URL", "http://localhost:3000")
}

func TestNewDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.ShutdownTimeout != 15*time.Second || cfg.SlogLevel() != slog.LevelInfo {
		t.Errorf("unexpected defaults: %+v", cfg)
	}
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
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("New() error = %v, want it to mention %s", err, tt.wantErr)
			}
		})
	}
}
