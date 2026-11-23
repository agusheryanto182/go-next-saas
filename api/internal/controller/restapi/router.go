// Package restapi is the HTTP layer: routing, middleware and handlers. Handlers translate HTTP to
// use-case calls and back; they hold no business rules.
package restapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/middleware"
	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/problem"
)

// Pinger reports whether a dependency (the database) is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Deps are what the router needs from the rest of the application.
type Deps struct {
	Log *slog.Logger
	DB  Pinger
}

// NewRouter builds the complete HTTP handler.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Liveness: the process is up. Readiness: it can serve traffic (the database answers).
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := d.DB.Ping(ctx); err != nil {
			d.Log.WarnContext(r.Context(), "readiness check failed", slog.Any("error", err))
			problem.Write(w, problem.New(http.StatusServiceUnavailable, "The database is not reachable."))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	h := problemOnNoRoute(mux)
	h = middleware.Recover(d.Log)(h)
	h = otelhttp.NewHandler(h, "api",
		otelhttp.WithFilter(func(r *http.Request) bool { return r.URL.Path != "/healthz" && r.URL.Path != "/readyz" }))
	return h
}

// problemOnNoRoute keeps ServeMux's 404 and 405 decisions (including the Allow header)
// but answers them as problem details, like every other API error.
func problemOnNoRoute(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}
		status := &statusRecorder{header: w.Header()}
		mux.ServeHTTP(status, r)
		detail := "No such endpoint."
		if status.code == http.StatusMethodNotAllowed {
			detail = "This endpoint does not support " + r.Method + "."
		}
		problem.Write(w, problem.New(status.code, detail))
	})
}

// statusRecorder captures the status ServeMux chooses and discards its plain-text body.
type statusRecorder struct {
	header http.Header
	code   int
}

func (s *statusRecorder) Header() http.Header         { return s.header }
func (s *statusRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (s *statusRecorder) WriteHeader(code int)        { s.code = code }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
