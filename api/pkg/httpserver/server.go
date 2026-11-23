// Package httpserver wraps http.Server with production timeouts and graceful shutdown.
package httpserver

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Server is an HTTP server that reports a failed listen on Notify.
type Server struct {
	srv    *http.Server
	notify chan error
}

// New creates a server with timeouts that protect against slow clients.
func New(addr string, handler http.Handler) *Server {
	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       15 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       60 * time.Second,
		},
		notify: make(chan error, 1),
	}
}

// Start listens in the background. Listen errors arrive on Notify.
func (s *Server) Start() {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.notify <- err
		}
		close(s.notify)
	}()
}

// Notify receives an error if the server stops unexpectedly.
func (s *Server) Notify() <-chan error { return s.notify }

// Shutdown stops accepting connections and waits for in-flight requests until ctx expires.
func (s *Server) Shutdown(ctx context.Context) error { return s.srv.Shutdown(ctx) }
