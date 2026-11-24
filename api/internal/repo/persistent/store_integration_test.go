//go:build integration

package persistent

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
	"github.com/agusheryanto182/go-next-saas/api/migrations"
)

// testDB is a real PostgreSQL with the same roles as development and production (ADR 0008).
type testDB struct {
	ownerURL, appURL string
}

func startPostgres(t *testing.T) testDB {
	t.Helper()
	ctx := t.Context()

	container, err := tcpostgres.Run(ctx, "postgres:18.6-alpine",
		tcpostgres.WithDatabase("saas"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.WithInitScripts(filepath.Join("..", "..", "..", "..", "dev", "postgres-init", "01-roles.sh")),
		testcontainers.WithEnv(map[string]string{
			"POSTGRES_OWNER_PASSWORD": "owner",
			"POSTGRES_APP_PASSWORD":   "app",
		}),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatal(err)
	}
	url := func(user, password string) string {
		return "postgres://" + user + ":" + password + "@" + host + ":" + port.Port() + "/saas?sslmode=disable"
	}
	return testDB{ownerURL: url("saas_owner", "owner"), appURL: url("saas_app", "app")}
}

func openPool(ctx context.Context, t *testing.T, url string, maxConns int32) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = maxConns
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestMigrationsUpDownUp(t *testing.T) {
	db := startPostgres(t)
	ctx := t.Context()
	log := slog.New(slog.DiscardHandler)

	if err := Migrate(ctx, db.ownerURL, log); err != nil {
		t.Fatalf("up: %v", err)
	}
	err := withProvider(db.ownerURL, migrations.FS, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	})
	if err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	if err := Migrate(ctx, db.ownerURL, log); err != nil {
		t.Fatalf("up again: %v", err)
	}
}

// tenants seeds two organizations through the app role and the unit of work, as the API will:
// alice owns Acme, bob owns Globex, and carol is a member of both.
type tenants struct {
	acme, globex      uuid.UUID
	alice, bob, carol uuid.UUID
}

func seed(t *testing.T, s *Store) tenants {
	t.Helper()
	ctx := t.Context()
	tn := tenants{
		acme: uuid.Must(uuid.NewV7()), globex: uuid.Must(uuid.NewV7()),
		alice: uuid.Must(uuid.NewV7()), bob: uuid.Must(uuid.NewV7()), carol: uuid.Must(uuid.NewV7()),
	}

	users := []gen.CreateUserParams{
		{ID: tn.alice, Email: "alice@acme.test", FullName: "Alice"},
		{ID: tn.bob, Email: "bob@globex.test", FullName: "Bob"},
		{ID: tn.carol, Email: "carol@both.test", FullName: "Carol"},
	}
	orgs := []struct {
		id, owner uuid.UUID
		name      string
		slug      string
	}{{tn.acme, tn.alice, "Acme", "acme"}, {tn.globex, tn.bob, "Globex", "globex"}}

	must(t, s.InScope(ctx, repo.Scope{}, func(q *gen.Queries) error {
		for _, u := range users {
			if _, err := q.CreateUser(ctx, u); err != nil {
				return err
			}
		}
		return nil
	}))
	for _, o := range orgs {
		must(t, s.InScope(ctx, repo.Scope{OrgID: o.id}, func(q *gen.Queries) error {
			if _, err := q.CreateOrganization(ctx, gen.CreateOrganizationParams{ID: o.id, Name: o.name, Slug: o.slug}); err != nil {
				return err
			}
			for _, m := range []gen.CreateMembershipParams{
				{OrganizationID: o.id, UserID: o.owner, Role: "owner"},
				{OrganizationID: o.id, UserID: tn.carol, Role: "member"},
			} {
				if _, err := q.CreateMembership(ctx, m); err != nil {
					return err
				}
			}
			return nil
		}))
	}
	return tn
}

func TestRowLevelSecurity(t *testing.T) {
	db := startPostgres(t)
	ctx := t.Context()
	must(t, Migrate(ctx, db.ownerURL, slog.New(slog.DiscardHandler)))

	// One connection, so every subtest reuses it and leaked settings would show.
	store := NewStore(openPool(ctx, t, db.appURL, 1))
	tn := seed(t, store)

	t.Run("no scope sees no tenant data", func(t *testing.T) {
		must(t, store.inTx(ctx, repo.Scope{}, func(tx pgx.Tx) error {
			assertCount(ctx, t, tx, "SELECT count(*) FROM organizations", 0)
			assertCount(ctx, t, tx, "SELECT count(*) FROM memberships", 0)
			return nil
		}))
	})

	t.Run("an organization scope sees only that organization", func(t *testing.T) {
		must(t, store.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(q *gen.Queries) error {
			acme, err := q.ListOrganizationMembers(ctx, tn.acme)
			if err != nil || len(acme) != 2 {
				t.Errorf("Acme members = %d (err %v), want 2", len(acme), err)
			}
			// Even asking for Globex explicitly returns nothing: the filter is in the database, not the query.
			globex, err := q.ListOrganizationMembers(ctx, tn.globex)
			if err != nil || len(globex) != 0 {
				t.Errorf("Globex members visible from Acme: %d (err %v)", len(globex), err)
			}
			if _, err := q.GetOrganizationBySlug(ctx, "globex"); !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("Globex found from Acme's scope: %v", err)
			}
			return nil
		}))
	})

	t.Run("writing into another organization is rejected", func(t *testing.T) {
		err := store.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(q *gen.Queries) error {
			_, err := q.CreateMembership(ctx, gen.CreateMembershipParams{OrganizationID: tn.globex, UserID: tn.alice, Role: "admin"})
			return err
		})
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" { // insufficient_privilege: RLS policy violation
			t.Fatalf("err = %v, want a row-level security violation", err)
		}
	})

	t.Run("a user scope lists only that user's own organizations", func(t *testing.T) {
		must(t, store.InScope(ctx, repo.Scope{UserID: tn.carol}, func(q *gen.Queries) error {
			orgs, err := q.ListUserOrganizations(ctx, tn.carol)
			if err != nil || len(orgs) != 2 {
				t.Errorf("Carol's organizations = %d (err %v), want 2", len(orgs), err)
			}
			others, err := q.ListUserOrganizations(ctx, tn.alice)
			if err != nil || len(others) != 0 {
				t.Errorf("Carol sees Alice's memberships: %d (err %v)", len(others), err)
			}
			return nil
		}))
	})

	t.Run("the scope ends with the transaction", func(t *testing.T) {
		must(t, store.InScope(ctx, repo.Scope{OrgID: tn.acme, UserID: tn.alice}, func(*gen.Queries) error { return nil }))

		var org, user string
		row := store.pool.QueryRow(ctx, "SELECT current_setting('app.org_id', true), current_setting('app.user_id', true)")
		must(t, row.Scan(&org, &user))
		if org != "" || user != "" {
			t.Errorf("scope leaked to the pooled connection: org=%q user=%q", org, user)
		}
	})

	t.Run("the app role cannot switch row-level security off", func(t *testing.T) {
		_, err := store.pool.Exec(ctx, "SET row_security = off; SELECT count(*) FROM organizations")
		if err == nil {
			t.Fatal("query ran with row security off")
		}
		_, _ = store.pool.Exec(ctx, "RESET row_security")
	})

	t.Run("the owner role is bound by the policies too", func(t *testing.T) {
		owner := openPool(ctx, t, db.ownerURL, 1)
		var n int
		must(t, owner.QueryRow(ctx, "SELECT count(*) FROM memberships").Scan(&n))
		if n != 0 {
			t.Errorf("owner sees %d memberships without a scope; FORCE ROW LEVEL SECURITY missing", n)
		}
	})

	t.Run("the app role cannot change the schema", func(t *testing.T) {
		if _, err := store.pool.Exec(ctx, "CREATE TABLE sneaky (id int)"); err == nil {
			t.Error("app role created a table")
		}
	})
}

func assertCount(ctx context.Context, t *testing.T, tx pgx.Tx, sql string, want int) {
	t.Helper()
	var n int
	if err := tx.QueryRow(ctx, sql).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	if n != want {
		t.Errorf("%s = %d, want %d", sql, n, want)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
