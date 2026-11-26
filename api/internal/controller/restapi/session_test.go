package restapi

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/middleware"
)

func request(t *testing.T, method, path, token string) *http.Request {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
	if token != "" {
		req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: token})
	}
	return req
}

func TestSessionGuardsProtectedOperations(t *testing.T) {
	t.Parallel()
	contract := contractRouter(t)

	tests := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"without a cookie", "", http.StatusUnauthorized},
		{"with an unknown token", "unknown-token", http.StatusUnauthorized},
		// GetMe answers 501 until #39: reaching it is what counts.
		{"with a valid session the handler is reached", validToken, http.StatusNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := request(t, http.MethodGet, "/api/v1/me", tt.token)
			rec := httptest.NewRecorder()
			newRouter(t, fakeDB{}).ServeHTTP(rec, req)

			p := decodeProblemKeepBody(t, rec)
			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantStatus, p.Status)
			if tt.wantStatus == http.StatusUnauthorized {
				assert.Equal(t, "Unauthorized", p.Title)
				assert.Equal(t, "Sign in to continue.", p.Detail)
			}
			assertMatchesContract(t, contract, req, rec)
		})
	}
}

func TestPublicOperationsIgnoreAStaleCookie(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	newRouter(t, fakeDB{}).ServeHTTP(rec, request(t, http.MethodPost, "/api/v1/auth/logout", "revoked-token"))

	// Logout answers 501 until #38; it is not refused for the stale cookie.
	assert.Equal(t, http.StatusNotImplemented, rec.Code)
}

func TestSessionLookupFailureIsAServerError(t *testing.T) {
	t.Parallel()
	h := newRouterWith(t, Deps{DB: fakeDB{}, Sessions: fakeSessions{err: errors.New("database unavailable")}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, request(t, http.MethodGet, "/api/v1/me", validToken))

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "not 401: the client must not think it was signed out")
	assert.Equal(t, http.StatusInternalServerError, decodeProblem(t, rec).Status)
}

func TestCrossOriginWritesAreRejected(t *testing.T) {
	t.Parallel()
	contract := contractRouter(t)
	const body = `{"email":"a@example.com","password":"x"}`

	tests := []struct {
		name       string
		headers    map[string]string
		wantStatus int
	}{
		{"from another site", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, http.StatusForbidden},
		{"from another origin, older browser", map[string]string{"Origin": "https://evil.example"}, http.StatusForbidden},
		// Login answers 501 until #38: passing the check is what counts.
		{"from the same origin", map[string]string{"Sec-Fetch-Site": "same-origin", "Origin": "http://example.com"}, http.StatusNotImplemented},
		{"without browser headers (server rendering, curl)", nil, http.StatusNotImplemented},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()
			newRouter(t, fakeDB{}).ServeHTTP(rec, req)

			p := decodeProblemKeepBody(t, rec)
			require.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantStatus, p.Status)
			assertMatchesContract(t, contract, req, rec)
		})
	}

	t.Run("reads from another site are allowed", func(t *testing.T) {
		t.Parallel()
		req := request(t, http.MethodGet, "/api/v1/me", validToken)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rec := httptest.NewRecorder()
		newRouter(t, fakeDB{}).ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotImplemented, rec.Code)
	})
}

// Logs and spans leave the process; the session token must appear in neither (ADR 0006).
//
//nolint:paralleltest // replaces the global tracer provider for the duration of the test
func TestSessionTokenNeverInLogsOrSpans(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)))
	t.Cleanup(func() { otel.SetTracerProvider(previous) })

	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ok := newRouterWith(t, Deps{Log: log, DB: fakeDB{}, Sessions: fakeSessions{}})
	failing := newRouterWith(t, Deps{Log: log, DB: fakeDB{}, Sessions: fakeSessions{err: errors.New("database unavailable")}})

	for _, h := range []http.Handler{ok, failing} {
		for _, token := range []string{validToken, "unknown-token-value"} {
			req := request(t, http.MethodGet, "/api/v1/me", token)
			req.Header.Set("User-Agent", "test")
			h.ServeHTTP(httptest.NewRecorder(), req)

			assert.NotContains(t, logs.String(), token)
		}
	}
	assert.Contains(t, logs.String(), "session lookup failed", "the failure path was exercised")

	spans := recorder.Ended()
	require.NotEmpty(t, spans)
	for _, span := range spans {
		for _, attr := range span.Attributes() {
			for _, token := range []string{validToken, "unknown-token-value"} {
				assert.NotContains(t, attr.Value.String(), token, "span %s attribute %s", span.Name(), attr.Key)
			}
		}
	}
}
