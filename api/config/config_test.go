package config

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setValidEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://saas_app:secret@localhost:5432/saas")
	t.Setenv("APP_BASE_URL", "http://localhost:3000")
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("MAIL_FROM", "go-next-saas <no-reply@example.com>")
}

//nolint:paralleltest // t.Setenv cannot be used in parallel tests
func TestNewDefaults(t *testing.T) {
	setValidEnv(t)

	cfg, err := New()
	require.NoError(t, err)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, 15*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, slog.LevelInfo, cfg.SlogLevel())
	assert.Equal(t, SMTP{
		Host: "smtp.example.com", Port: 587, TLS: "starttls",
		ConnectTimeout: 10 * time.Second, SendTimeout: 15 * time.Second,
	}, cfg.SMTP)
	assert.Empty(t, cfg.TrustedProxyPrefixes(), "no proxy is trusted by default")
}

func TestNewParsesTrustedProxies(t *testing.T) {
	setValidEnv(t)
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32,::1/128,172.16.0.0/12")

	cfg, err := New()
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{
		netip.MustParsePrefix("127.0.0.1/32"), netip.MustParsePrefix("::1/128"), netip.MustParsePrefix("172.16.0.0/12"),
	}, cfg.TrustedProxyPrefixes())
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
		{"missing SMTP host", map[string]string{"SMTP_HOST": ""}, "SMTP_HOST"},
		{"SMTP port out of range", map[string]string{"SMTP_PORT": "70000"}, "SMTP_PORT"},
		{"TLS as a boolean", map[string]string{"SMTP_TLS": "false"}, "SMTP_TLS"},
		{"username without password", map[string]string{"SMTP_USERNAME": "mailer"}, "SMTP_PASSWORD"},
		{"zero connect timeout", map[string]string{"SMTP_CONNECT_TIMEOUT": "0s"}, "SMTP_CONNECT_TIMEOUT"},
		{"zero send timeout", map[string]string{"SMTP_SEND_TIMEOUT": "0s"}, "SMTP_SEND_TIMEOUT"},
		{"send timeout longer than a tenth of the lease", map[string]string{"SMTP_SEND_TIMEOUT": "30s"}, "SMTP_SEND_TIMEOUT"},
		{"trusted proxy that is not a CIDR", map[string]string{"TRUSTED_PROXIES": "10.0.0.1"}, "TRUSTED_PROXIES"},
		{"trusted proxy with host bits", map[string]string{"TRUSTED_PROXIES": "10.0.0.1/8"}, "TRUSTED_PROXIES"},
		{"missing sender", map[string]string{"MAIL_FROM": ""}, "MAIL_FROM"},
		{"sender without address", map[string]string{"MAIL_FROM": "go-next-saas"}, "MAIL_FROM"},
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
