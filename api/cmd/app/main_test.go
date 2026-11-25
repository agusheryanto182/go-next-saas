package main

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunUsage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string // substring; "" means stdout must be empty
		wantStderr string // substring; "" means stderr must be empty
	}{
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Usage: app"},
		{name: "-h", args: []string{"-h"}, wantCode: 0, wantStdout: "Usage: app"},
		{name: "--help", args: []string{"--help"}, wantCode: 0, wantStdout: "Usage: app"},
		{name: "subcommand -h", args: []string{"migrate", "-h"}, wantCode: 0, wantStdout: "Usage: app"},
		{name: "unknown subcommand", args: []string{"frobnicate"}, wantCode: 2, wantStderr: `unknown command "frobnicate"`},
		{name: "old -migrate flag", args: []string{"-migrate"}, wantCode: 2, wantStderr: "flag provided but not defined: -migrate"},
		{name: "old -healthcheck flag", args: []string{"-healthcheck"}, wantCode: 2, wantStderr: "flag provided but not defined: -healthcheck"},
		{name: "bad subcommand flag", args: []string{"migrate", "-x"}, wantCode: 2, wantStderr: "flag provided but not defined: -x"},
		{name: "help with argument", args: []string{"help", "migrate"}, wantCode: 2, wantStderr: `unexpected argument "migrate"`},
		{name: "extra argument", args: []string{"healthcheck", "now"}, wantCode: 2, wantStderr: `unexpected argument "now"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer

			code := run(tt.args, &stdout, &stderr)

			assert.Equal(t, tt.wantCode, code)
			assertOutput(t, "stdout", tt.wantStdout, stdout.String())
			assertOutput(t, "stderr", tt.wantStderr, stderr.String())
			if tt.wantCode == 2 {
				assert.Contains(t, stderr.String(), "Usage: app", "a usage error prints usage to stderr")
			}
		})
	}
}

// TestRunDispatch checks that each subcommand reaches its implementation. The environment is set so
// that none of them needs a database: serve and migrate stop at missing configuration. Not parallel:
// it uses t.Setenv.
func TestRunDispatch(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		healthStatus int // when non-zero, a local /healthz answering with this status; HTTP_ADDR points at it
		wantCode     int
		wantStdout   string
		wantStderr   string
	}{
		{name: "no subcommand serves", args: nil, wantCode: 1, wantStderr: "DATABASE_URL"},
		{name: "serve", args: []string{"serve"}, wantCode: 1, wantStderr: "DATABASE_URL"},
		{name: "migrate", args: []string{"migrate"}, wantCode: 1, wantStdout: "MIGRATION_DATABASE_URL is not set"},
		{name: "seed", args: []string{"seed"}, wantCode: 1, wantStdout: "DATABASE_URL is not set"},
		{
			name:       "create-org without a password source",
			args:       []string{"create-org", "-name", "Acme", "-slug", "acme", "-owner-email", "a@acme.test", "-owner-name", "A"},
			wantCode:   2,
			wantStderr: "invitations arrive in M2; use -owner-password-stdin",
		},
		{
			name:       "create-org missing a flag",
			args:       []string{"create-org", "-name", "Acme", "-owner-email", "a@acme.test", "-owner-name", "A", "-owner-password-stdin"},
			wantCode:   2,
			wantStderr: "-slug is required",
		},
		{
			name:       "create-org without a database",
			args:       []string{"create-org", "-name", "Acme", "-slug", "acme", "-owner-email", "a@acme.test", "-owner-name", "A", "-owner-password-stdin"},
			wantCode:   1,
			wantStderr: "DATABASE_URL is not set",
		},
		{name: "healthcheck healthy", args: []string{"healthcheck"}, healthStatus: http.StatusOK, wantCode: 0},
		{name: "healthcheck unhealthy", args: []string{"healthcheck"}, healthStatus: http.StatusServiceUnavailable, wantCode: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "")
			t.Setenv("MIGRATION_DATABASE_URL", "")
			if tt.healthStatus != 0 {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/healthz" {
						http.NotFound(w, r)
						return
					}
					w.WriteHeader(tt.healthStatus)
				}))
				t.Cleanup(srv.Close)
				u, err := url.Parse(srv.URL)
				assert.NoError(t, err)
				_, port, _ := net.SplitHostPort(u.Host)
				t.Setenv("HTTP_ADDR", ":"+port)
			}
			var stdout, stderr bytes.Buffer

			code := run(tt.args, &stdout, &stderr)

			assert.Equal(t, tt.wantCode, code)
			assertOutput(t, "stdout", tt.wantStdout, stdout.String())
			assertOutput(t, "stderr", tt.wantStderr, stderr.String())
		})
	}
}

func assertOutput(t *testing.T, stream, want, got string) {
	t.Helper()
	if want == "" {
		assert.Empty(t, got, stream)
		return
	}
	assert.Contains(t, got, want, stream)
}
