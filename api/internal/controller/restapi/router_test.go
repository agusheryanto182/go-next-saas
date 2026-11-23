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

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/middleware"
	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/problem"
)

type fakeDB struct{ err error }

func (f fakeDB) Ping(context.Context) error { return f.err }

func serve(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))
	return rec
}

func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problem.Problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var p problem.Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	return p
}

func TestHealthAndReadiness(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
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
			rec := serve(t, NewRouter(Deps{Log: log, DB: tt.db}), http.MethodGet, tt.path)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusServiceUnavailable {
				if p := decodeProblem(t, rec); p.Status != tt.wantStatus || p.Title != "Service Unavailable" {
					t.Errorf("problem = %+v", p)
				}
			}
		})
	}
}

func TestUnknownRouteIsProblem(t *testing.T) {
	rec := serve(t, NewRouter(Deps{Log: slog.New(slog.DiscardHandler), DB: fakeDB{}}), http.MethodGet, "/nope")

	if p := decodeProblem(t, rec); rec.Code != http.StatusNotFound || p.Status != http.StatusNotFound {
		t.Errorf("status = %d, problem = %+v", rec.Code, p)
	}
}

func TestWrongMethodIsRejected(t *testing.T) {
	rec := serve(t, NewRouter(Deps{Log: slog.New(slog.DiscardHandler), DB: fakeDB{}}), http.MethodPost, "/healthz")

	if p := decodeProblem(t, rec); rec.Code != http.StatusMethodNotAllowed || p.Status != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, problem = %+v", rec.Code, p)
	}
	if allow := rec.Header().Get("Allow"); allow == "" {
		t.Error("Allow header missing")
	}
}

func TestRecoverTurnsPanicIntoProblem(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logs, nil))
	h := middleware.Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))

	rec := serve(t, h, http.MethodGet, "/")

	if p := decodeProblem(t, rec); p.Status != http.StatusInternalServerError {
		t.Errorf("problem = %+v", p)
	}
	if !bytes.Contains(logs.Bytes(), []byte("boom")) {
		t.Errorf("panic not logged: %s", logs.String())
	}
}
