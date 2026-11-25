//go:build integration

// Package pgtest starts a real PostgreSQL for integration tests, with the same roles as development
// and production (ADR 0008): an owner role for migrations and an app role bound by row-level security.
package pgtest

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	_image          = "postgres:18.6-alpine"
	_startupTimeout = 60 * time.Second
)

// DB is a running PostgreSQL container.
type DB struct {
	OwnerURL string // migrations, schema owner
	AppURL   string // runtime role, subject to row-level security
}

// Start runs a container for the test (removed at cleanup). Apply migrations with persistent.Migrate
// and DB.OwnerURL.
func Start(t *testing.T) DB {
	t.Helper()
	ctx := t.Context()

	container, err := tcpostgres.Run(ctx, _image,
		tcpostgres.WithDatabase("saas"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.WithInitScripts(rolesScript(t)),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_OWNER_PASSWORD": "owner",
			"POSTGRES_APP_PASSWORD":   "app",
		}),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(_startupTimeout)),
	)
	testcontainers.CleanupContainer(t, container)
	require.NoError(t, err, "start postgres")

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	url := func(user, password string) string {
		return "postgres://" + user + ":" + password + "@" + host + ":" + port.Port() + "/saas?sslmode=disable"
	}
	return DB{OwnerURL: url("saas_owner", "owner"), AppURL: url("saas_app", "app")}
}

// Pool opens a pool of at most maxConns connections, closed at cleanup.
func Pool(ctx context.Context, t *testing.T, url string, maxConns int32) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	cfg.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

// rolesScript is the development role setup (dev/postgres-init/01-roles.sh), found from this file.
func rolesScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "..", "dev", "postgres-init", "01-roles.sh")
}
