//go:build integration

package app

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/membership"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/pgtest"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/user"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
	"github.com/agusheryanto182/go-next-saas/api/pkg/password"
)

//nolint:paralleltest // the subtests share one database and build on each other's data
func TestOrganizationsIntegration(t *testing.T) {
	db := pgtest.Start(t)
	ctx := t.Context()
	require.NoError(t, persistent.Migrate(ctx, db.OwnerURL, slog.New(slog.DiscardHandler)))

	tx := persistent.NewTransactor(pgtest.Pool(ctx, t, db.AppURL, 2))
	users, members := user.New(), membership.New()

	getUser := func(t *testing.T, email string) entity.User {
		t.Helper()
		var u entity.User
		require.NoError(t, tx.InScope(ctx, repo.Scope{}, func(ctx context.Context) error {
			var err error
			u, err = users.GetByEmail(ctx, email)
			return err
		}))
		return u
	}
	userOrgs := func(t *testing.T, userID string) []entity.UserOrganization {
		t.Helper()
		var orgs []entity.UserOrganization
		require.NoError(t, tx.InScope(ctx, repo.Scope{UserID: userID}, func(ctx context.Context) error {
			var err error
			orgs, err = members.ListUserOrganizations(ctx, userID)
			return err
		}))
		return orgs
	}

	in := usecase.CreateOrganizationInput{
		Name: "Initech", Slug: "initech",
		Owner: usecase.NewUser{Email: "Peter@Initech.test", FullName: "Peter", Password: "first-password"},
	}

	t.Run("create twice gives one organization and one membership", func(t *testing.T) {
		first, err := CreateOrganization(ctx, db.AppURL, in)
		require.NoError(t, err)
		assert.True(t, first.Created)

		second, err := CreateOrganization(ctx, db.AppURL, in)
		require.NoError(t, err)
		assert.False(t, second.Created)

		owner := getUser(t, "peter@initech.test")
		orgs := userOrgs(t, owner.ID)
		require.Len(t, orgs, 1, "the owner lists their organization")
		assert.Equal(t, entity.UserOrganization{
			OrganizationID: first.Organization.ID, Name: "Initech", Slug: "initech",
			Role: entity.RoleOwner, Status: entity.MembershipActive,
		}, orgs[0])

		require.NoError(t, tx.InScope(ctx, repo.Scope{OrgID: first.Organization.ID}, func(ctx context.Context) error {
			list, err := members.ListMembers(ctx, first.Organization.ID)
			require.NoError(t, err)
			assert.Len(t, list, 1)
			return nil
		}))

		ok, err := password.New().Verify(ctx, owner.PasswordHash, "first-password")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("an existing owner keeps their password", func(t *testing.T) {
		before := getUser(t, "peter@initech.test")

		res, err := CreateOrganization(ctx, db.AppURL, usecase.CreateOrganizationInput{
			Name: "Initrode", Slug: "initrode",
			Owner: usecase.NewUser{Email: "peter@initech.test", FullName: "Someone Else", Password: "other-password"},
		})
		require.NoError(t, err)
		assert.True(t, res.Created)

		after := getUser(t, "peter@initech.test")
		assert.Equal(t, before, after, "the user is reused unchanged")
		assert.Len(t, userOrgs(t, after.ID), 2)
	})

	t.Run("seed twice gives the ADR 0012 dataset", func(t *testing.T) {
		log := slog.New(slog.DiscardHandler)
		require.NoError(t, Seed(ctx, db.AppURL, log))
		require.NoError(t, Seed(ctx, db.AppURL, log), "seed is idempotent")

		carol := userOrgs(t, getUser(t, "carol@example.test").ID)
		require.Len(t, carol, 2)
		assert.Equal(t, "acme", carol[0].Slug)
		assert.Equal(t, "globex", carol[1].Slug)

		fajar := userOrgs(t, getUser(t, "fajar@acme.test").ID)
		require.Len(t, fajar, 1)
		assert.Equal(t, entity.MembershipDeactivated, fajar[0].Status)

		dina := userOrgs(t, getUser(t, "dina@acme.test").ID)
		require.Len(t, dina, 1)
		assert.Equal(t, entity.RoleAdmin, dina[0].Role)

		bob := userOrgs(t, getUser(t, "bob@globex.test").ID)
		require.Len(t, bob, 1)
		assert.Equal(t, entity.RoleOwner, bob[0].Role)
	})
}
