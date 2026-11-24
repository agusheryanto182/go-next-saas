// Command app runs the API.
//
//	app               serve HTTP (configuration from the environment, see .env.example)
//	app -migrate      apply pending database migrations as the owner role (MIGRATION_DATABASE_URL), then exit
//	app -healthcheck  exit 0 if the local server answers /healthz (for container health checks)
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/agusheryanto182/go-next-saas/api/config"
	"github.com/agusheryanto182/go-next-saas/api/internal/app"
	"github.com/agusheryanto182/go-next-saas/api/pkg/logger"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local server's /healthz and exit")
	migrate := flag.Bool("migrate", false, "apply pending migrations using MIGRATION_DATABASE_URL and exit")
	flag.Parse()

	switch {
	case *healthcheck:
		os.Exit(checkHealth())
	case *migrate:
		os.Exit(runMigrations())
	default:
		os.Exit(serve())
	}
}

func runMigrations() int {
	log := logger.New(os.Stdout, slog.LevelInfo)
	url := os.Getenv("MIGRATION_DATABASE_URL")
	if url == "" {
		log.Error("MIGRATION_DATABASE_URL is not set (the owner role's connection URL)")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Migrate(ctx, url, log); err != nil {
		log.Error("migration failed", slog.Any("error", err))
		return 1
	}
	log.Info("database is up to date")
	return 0
}

func serve() int {
	cfg, err := config.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	log := logger.New(os.Stdout, cfg.SlogLevel())
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, cfg, log); err != nil {
		log.Error("api failed", slog.Any("error", err))
		return 1
	}
	return 0
}

// checkHealth calls /healthz on the port from HTTP_ADDR. The runtime image has no shell or curl.
func checkHealth() int {
	port := 8080
	if _, p, err := net.SplitHostPort(os.Getenv("HTTP_ADDR")); err == nil {
		if n, err := strconv.Atoi(p); err == nil && n > 0 && n < 65536 {
			port = n
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // loopback host, numeric port
	if err != nil {
		return 1
	}
	res, err := http.DefaultClient.Do(req) //nolint:gosec // loopback host, numeric port
	if err != nil {
		return 1
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
