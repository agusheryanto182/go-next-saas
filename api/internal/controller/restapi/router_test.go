package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/middleware"
	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/problem"
	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

// validToken is the one token fakeSessions accepts.
const validToken = "valid-token-0123456789-abcdefghijklmnopqrst"

// fakeSessions authenticates validToken as Alice, or fails every lookup with err.
type fakeSessions struct {
	usecase.Session // only Authenticate is called by the router
	err             error
}

func (f fakeSessions) Authenticate(_ context.Context, token string) (entity.Session, entity.User, error) {
	switch {
	case f.err != nil:
		return entity.Session{}, entity.User{}, f.err
	case token == validToken:
		return entity.Session{ID: "0193b5a4-6c2e-7d4a-9f1e-2b3c4d5e6f71", UserID: "0193b5a4-6c2e-7d4a-9f1e-2b3c4d5e6f70"},
			entity.User{ID: "0193b5a4-6c2e-7d4a-9f1e-2b3c4d5e6f70", Email: "alice@acme.test", FullName: "Alice"}, nil
	default:
		return entity.Session{}, entity.User{}, entity.ErrSessionNotFound
	}
}

func newRouter(t *testing.T, db fakeDB) http.Handler {
	t.Helper()
	return newRouterWith(t, Deps{DB: db, Sessions: fakeSessions{}})
}

func newRouterWith(t *testing.T, d Deps) http.Handler {
	t.Helper()
	if d.Log == nil {
		d.Log = slog.New(slog.DiscardHandler)
	}
	h, err := NewRouter(d)
	require.NoError(t, err)
	return h
}

func signedIn(req *http.Request) *http.Request {
	req.AddCookie(&http.Cookie{Name: middleware.SessionCookieName, Value: validToken})
	return req
}

func serve(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	return rec
}

// decodeProblemKeepBody decodes without consuming the recorder body, so it can be validated afterwards.
func decodeProblemKeepBody(t *testing.T, rec *httptest.ResponseRecorder) problem.Problem {
	t.Helper()
	copyRec := httptest.NewRecorder()
	copyRec.Header().Set("Content-Type", rec.Header().Get("Content-Type"))
	copyRec.Body.Write(rec.Body.Bytes())
	return decodeProblem(t, copyRec)
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problem.Problem {
	t.Helper()
	require.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))
	var p problem.Problem
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&p), "decode problem")
	return p
}

func TestHealthAndReadiness(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		db         fakeDB
		path       string
		wantStatus int
	}{
		{"liveness ignores the database", fakeDB{errors.New("down")}, "/healthz", http.StatusOK},
		{"ready when the database answers", fakeDB{}, "/readyz", http.StatusOK},
		{"not ready when the database is down", fakeDB{errors.New("down")}, "/readyz", http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rec := serve(t, newRouter(t, tt.db), http.MethodGet, tt.path)
			require.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusServiceUnavailable {
				p := decodeProblem(t, rec)
				assert.Equal(t, tt.wantStatus, p.Status)
				assert.Equal(t, "Service Unavailable", p.Title)
			}
		})
	}
}

func TestUnknownRouteIsProblem(t *testing.T) {
	t.Parallel()
	rec := serve(t, newRouter(t, fakeDB{}), http.MethodGet, "/nope")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, http.StatusNotFound, decodeProblem(t, rec).Status)
}

func TestWrongMethodIsRejected(t *testing.T) {
	t.Parallel()
	rec := serve(t, newRouter(t, fakeDB{}), http.MethodPost, "/healthz")

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.StatusMethodNotAllowed, decodeProblem(t, rec).Status)
	assert.NotEmpty(t, rec.Header().Get("Allow"), "Allow header missing")
}

func TestRecoverTurnsPanicIntoProblem(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := middleware.Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))

	rec := serve(t, h, http.MethodGet, "/")

	assert.Equal(t, http.StatusInternalServerError, decodeProblem(t, rec).Status)
	assert.Contains(t, logs.String(), "boom", "panic not logged")
}
