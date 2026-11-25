// Package app wires the application together (manual dependency injection) and runs it until the
// context is cancelled, then shuts down gracefully.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/agusheryanto182/go-next-saas/api/config"
	"github.com/agusheryanto182/go-next-saas/api/internal/controller/restapi"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/outbox"
	webmail "github.com/agusheryanto182/go-next-saas/api/internal/repo/webapi/mail"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase/mail"
	"github.com/agusheryanto182/go-next-saas/api/pkg/httpserver"
	"github.com/agusheryanto182/go-next-saas/api/pkg/otel"
	"github.com/agusheryanto182/go-next-saas/api/pkg/password"
	"github.com/agusheryanto182/go-next-saas/api/pkg/postgres"
)

// pkg/password implements the use cases' hashing contract; the sign-in use cases take it from here.
var _ repo.PasswordHasher = (*password.Hasher)(nil)

// Migrate applies pending database migrations. url must connect as the owner role.
func Migrate(ctx context.Context, url string, log *slog.Logger) error {
	return persistent.Migrate(ctx, url, log)
}

// Run starts the API and blocks until ctx is cancelled or the server fails.
func Run(ctx context.Context, cfg config.Config, log *slog.Logger) (err error) {
	shutdownTracing, err := otel.Setup(ctx, "go-next-saas-api", cfg.OTLPEndpoint)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, shutdownTracing(context.WithoutCancel(ctx))) }()

	pool, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	mailer, err := webmail.New(webmail.Config{
		Host:           cfg.SMTP.Host,
		Port:           cfg.SMTP.Port,
		Username:       cfg.SMTP.Username,
		Password:       cfg.SMTP.Password,
		TLS:            webmail.TLSMode(cfg.SMTP.TLS),
		ConnectTimeout: cfg.SMTP.ConnectTimeout,
		SendTimeout:    cfg.SMTP.SendTimeout,
		From:           cfg.MailFrom,
		AppBaseURL:     cfg.AppBaseURL,
	})
	if err != nil {
		return err
	}
	tx := persistent.NewTransactor(pool)
	worker := newMailWorker(mail.New(tx, outbox.New(), mailer, log), log)

	// The worker stops before the pool closes (defers run in reverse), after its email in flight.
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker.Run(workerCtx)
	}()
	defer func() {
		stopWorker()
		<-workerDone
	}()

	router, err := restapi.NewRouter(restapi.Deps{Log: log, DB: pool})
	if err != nil {
		return err
	}
	server := httpserver.New(cfg.HTTPAddr, router)
	server.Start()
	log.Info("api started", slog.String("addr", cfg.HTTPAddr))

	select {
	case <-ctx.Done():
		log.Info("shutting down")
	case err := <-server.Notify():
		return fmt.Errorf("http server: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	log.Info("stopped")
	return nil
}
