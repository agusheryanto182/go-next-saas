// Package middleware holds the HTTP middleware shared by every route.
package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi/problem"
)

// Recover turns a panic in a handler into a logged 500 problem response instead of a dropped connection.
func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func(ctx context.Context) {
				rec := recover()
				if rec == nil {
					return
				}
				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}
				log.ErrorContext(ctx, "panic in handler",
					slog.Any("panic", rec), slog.String("method", r.Method), slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())))
				problem.Write(w, problem.New(http.StatusInternalServerError, "Something went wrong. Please try again."))
			}(r.Context())
			next.ServeHTTP(w, r)
		})
	}
}
