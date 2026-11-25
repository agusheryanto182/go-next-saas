//go:build integration

package persistent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/membership"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/organization"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/pgtest"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/user"
)

type repos struct {
	tx          repo.Transactor
	users       repo.UserRepo
	orgs        repo.OrganizationRepo
	memberships repo.MembershipRepo
}

// tenants are two organizations seeded through the app role and the unit of work, as the API does:
// alice owns Acme, bob owns Globex, and carol is a member of both.
type tenants struct {
	acme, globex      string
	alice, bob, carol string
}

func newID() string { return uuid.Must(uuid.NewV7()).String() }

func seed(t *testing.T, r repos) tenants {
	t.Helper()
	ctx := t.Context()
	tn := tenants{acme: newID(), globex: newID(), alice: newID(), bob: newID(), carol: newID()}

	require.NoError(t, r.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
		for _, u := range []entity.User{
			{ID: tn.alice, Email: "alice@acme.test", FullName: "Alice"},
			{ID: tn.bob, Email: "bob@globex.test", FullName: "Bob"},
			{ID: tn.carol, Email: "carol@both.test", FullName: "Carol"},
		} {
			if err := r.users.Store(ctx, &u); err != nil {
				return err
			}
		}
		return nil
	}))

	for _, o := range []struct{ id, owner, name, slug string }{
		{tn.acme, tn.alice, "Acme", "acme"},
		{tn.globex, tn.bob, "Globex", "globex"},
	} {
		// Creating an organization needs its own ID as the scope (the policy's WITH CHECK).
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{OrgID: o.id}, func(ctx context.Context) error {
			if err := r.orgs.Store(ctx, &entity.Organization{ID: o.id, Name: o.name, Slug: o.slug}); err != nil {
				return err
			}
			for _, m := range []entity.Membership{
				{OrganizationID: o.id, UserID: o.owner, Role: entity.RoleOwner},
				{OrganizationID: o.id, UserID: tn.carol, Role: entity.RoleMember},
			} {
				if err := r.memberships.Store(ctx, &m); err != nil {
					return err
				}
			}
			return nil
		}))
	}
	return tn
}

func count(ctx context.Context, t *testing.T, sql string) int {
	t.Helper()
	tx, err := persistent.Tx(ctx)
	require.NoError(t, err)
	var n int
	require.NoError(t, tx.QueryRow(ctx, sql).Scan(&n), sql)
	return n
}

//nolint:paralleltest // the subtests share one database connection on purpose, so leaked settings would show
func TestPersistent(t *testing.T) {
	db := pgtest.Start(t)
	ctx := t.Context()
	require.NoError(t, persistent.Migrate(ctx, db.OwnerURL, slog.New(slog.DiscardHandler)))

	pool := pgtest.Pool(ctx, t, db.AppURL, 1)
	r := repos{tx: persistent.NewTransactor(pool), users: user.New(), orgs: organization.New(), memberships: membership.New()}
	tn := seed(t, r)

	t.Run("no scope sees no tenant data", func(t *testing.T) {
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			assert.Equal(t, 0, count(ctx, t, "SELECT count(*) FROM organizations"))
			assert.Equal(t, 0, count(ctx, t, "SELECT count(*) FROM memberships"))
			return nil
		}))
	})

	t.Run("an organization scope sees only that organization", func(t *testing.T) {
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(ctx context.Context) error {
			acme, err := r.memberships.ListMembers(ctx, tn.acme)
			require.NoError(t, err)
			assert.Len(t, acme, 2)

			// Even asking for Globex explicitly returns nothing: the filter is in the database, not the query.
			globex, err := r.memberships.ListMembers(ctx, tn.globex)
			require.NoError(t, err)
			assert.Empty(t, globex)

			_, err = r.orgs.GetBySlug(ctx, "globex")
			assert.ErrorIs(t, err, entity.ErrOrganizationNotFound)

			got, err := r.orgs.GetBySlug(ctx, "acme")
			require.NoError(t, err)
			assert.Equal(t, tn.acme, got.ID)
			return nil
		}))
	})

	t.Run("writing into another organization is rejected", func(t *testing.T) {
		err := r.tx.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(ctx context.Context) error {
			return r.memberships.Store(ctx, &entity.Membership{OrganizationID: tn.globex, UserID: tn.alice, Role: entity.RoleAdmin})
		})
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, "42501", pgErr.Code, "insufficient_privilege: row-level security policy violation")
	})

	t.Run("a user scope lists only that user's own organizations", func(t *testing.T) {
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{UserID: tn.carol}, func(ctx context.Context) error {
			orgs, err := r.memberships.ListUserOrganizations(ctx, tn.carol)
			require.NoError(t, err)
			assert.Len(t, orgs, 2)

			others, err := r.memberships.ListUserOrganizations(ctx, tn.alice)
			require.NoError(t, err)
			assert.Empty(t, others, "Carol sees Alice's memberships")
			return nil
		}))
	})

	t.Run("the scope ends with the transaction", func(t *testing.T) {
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{OrgID: tn.acme, UserID: tn.alice}, func(context.Context) error { return nil }))

		var org, usr string
		require.NoError(t, pool.QueryRow(ctx, "SELECT current_setting('app.org_id', true), current_setting('app.user_id', true)").Scan(&org, &usr))
		assert.Empty(t, org, "organization scope leaked to the pooled connection")
		assert.Empty(t, usr, "user scope leaked to the pooled connection")
	})

	t.Run("a repository outside a unit of work is refused", func(t *testing.T) {
		_, err := r.orgs.GetBySlug(ctx, "acme")
		require.ErrorIs(t, err, persistent.ErrNoUnitOfWork)
	})

	t.Run("units of work do not nest", func(t *testing.T) {
		err := r.tx.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(ctx context.Context) error {
			return r.tx.InScope(ctx, repo.Scope{OrgID: tn.globex}, func(context.Context) error { return nil })
		})
		require.ErrorIs(t, err, persistent.ErrNestedUnitOfWork)
	})

	t.Run("a scope ID that is not a UUID is refused", func(t *testing.T) {
		err := r.tx.InScope(ctx, repo.Scope{OrgID: "acme"}, func(context.Context) error { return nil })
		require.ErrorIs(t, err, persistent.ErrInvalidID)
	})

	t.Run("an error rolls the unit of work back", func(t *testing.T) {
		errStop := errors.New("stop")
		u := entity.User{ID: newID(), Email: "rollback@test.test", FullName: "Rollback"}
		err := r.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			require.NoError(t, r.users.Store(ctx, &u))
			return errStop
		})
		require.ErrorIs(t, err, errStop)

		// The email is free again: the first insert was rolled back.
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			return r.users.Store(ctx, &entity.User{ID: newID(), Email: u.Email, FullName: u.FullName})
		}))
	})

	t.Run("unique violations become domain errors", func(t *testing.T) {
		err := r.tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			return r.users.Store(ctx, &entity.User{ID: newID(), Email: "alice@acme.test", FullName: "Alice again"})
		})
		require.ErrorIs(t, err, entity.ErrUserAlreadyExists)

		id := newID()
		err = r.tx.InScope(ctx, repo.Scope{OrgID: id}, func(ctx context.Context) error {
			return r.orgs.Store(ctx, &entity.Organization{ID: id, Name: "Acme again", Slug: "acme"})
		})
		require.ErrorIs(t, err, entity.ErrOrganizationSlugTaken)

		err = r.tx.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(ctx context.Context) error {
			return r.memberships.Store(ctx, &entity.Membership{OrganizationID: tn.acme, UserID: tn.carol, Role: entity.RoleAdmin})
		})
		require.ErrorIs(t, err, entity.ErrMembershipExists)
	})

	t.Run("stored rows come back with database timestamps and defaults", func(t *testing.T) {
		require.NoError(t, r.tx.InScope(ctx, repo.Scope{OrgID: tn.acme}, func(ctx context.Context) error {
			members, err := r.memberships.ListMembers(ctx, tn.acme)
			require.NoError(t, err)
			for _, m := range members {
				assert.Equal(t, entity.MembershipActive, m.Status)
				assert.False(t, m.JoinedAt.IsZero())
			}
			return nil
		}))
	})

	t.Run("the app role cannot switch row-level security off", func(t *testing.T) {
		_, err := pool.Exec(ctx, "SET row_security = off; SELECT count(*) FROM organizations")
		require.Error(t, err, "query ran with row security off")
		_, _ = pool.Exec(ctx, "RESET row_security")
	})

	t.Run("the owner role is bound by the policies too", func(t *testing.T) {
		owner := pgtest.Pool(ctx, t, db.OwnerURL, 1)
		var n int
		require.NoError(t, owner.QueryRow(ctx, "SELECT count(*) FROM memberships").Scan(&n))
		assert.Zero(t, n, "owner sees memberships without a scope; FORCE ROW LEVEL SECURITY missing")
	})

	t.Run("the app role cannot change the schema", func(t *testing.T) {
		_, err := pool.Exec(ctx, "CREATE TABLE sneaky (id int)")
		require.Error(t, err, "app role created a table")
	})
}
