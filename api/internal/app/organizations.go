package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/membership"
	orgrepo "github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/organization"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/user"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase/organization"
	"github.com/agusheryanto182/go-next-saas/api/pkg/password"
	"github.com/agusheryanto182/go-next-saas/api/pkg/postgres"
)

// ErrSeedHostRefused is returned by Seed for a database that is not on this machine or in Compose.
var ErrSeedHostRefused = errors.New("seed only runs against a local database (localhost, 127.0.0.1, ::1 or postgres)")

// _seedPassword is the password of every development account, documented in the README (ADR 0012).
const _seedPassword = "dev-password"

// CreateOrganization creates an organization and its owner through the use case, as the app role
// (databaseURL is DATABASE_URL). It changes nothing if the slug exists (ADR 0012).
func CreateOrganization(ctx context.Context, databaseURL string, in usecase.CreateOrganizationInput) (usecase.CreateOrganizationResult, error) {
	var res usecase.CreateOrganizationResult
	err := withOrganizations(ctx, databaseURL, func(orgs usecase.Organization) error {
		var err error
		res, err = orgs.Create(ctx, in)
		return err
	})
	return res, err
}

// Seed creates the development dataset (ADR 0012) as the app role. Running it again changes nothing.
// It refuses databases other than local ones, so it cannot touch a hosted database.
func Seed(ctx context.Context, databaseURL string, log *slog.Logger) error {
	if err := checkSeedHost(databaseURL); err != nil {
		return err
	}
	return withOrganizations(ctx, databaseURL, func(orgs usecase.Organization) error {
		for _, in := range seedData() {
			res, err := orgs.Create(ctx, in)
			if err != nil {
				return fmt.Errorf("seed %s: %w", in.Slug, err)
			}
			if res.Created {
				log.InfoContext(ctx, "organization created", slog.String("slug", in.Slug), slog.Int("members", 1+len(in.Members)))
			} else {
				log.InfoContext(ctx, "organization already exists, unchanged", slog.String("slug", in.Slug))
			}
		}
		return nil
	})
}

// checkSeedHost refuses a URL whose resolved hosts (including fallbacks and a host query parameter)
// are not all local.
func checkSeedHost(databaseURL string) error {
	cfg, err := pgconn.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("seed - parse DATABASE_URL: %w", err)
	}
	allowed := []string{"localhost", "127.0.0.1", "::1", "postgres"}
	hosts := []string{cfg.Host}
	for _, fb := range cfg.Fallbacks {
		hosts = append(hosts, fb.Host)
	}
	for _, h := range hosts {
		if !slices.Contains(allowed, h) {
			return ErrSeedHostRefused
		}
	}
	return nil
}

func seedData() []usecase.CreateOrganizationInput {
	person := func(name, email string) usecase.NewUser {
		return usecase.NewUser{Email: email, FullName: name, Password: _seedPassword}
	}
	carol := person("Carol", "carol@example.test")
	return []usecase.CreateOrganizationInput{
		{
			Name: "Acme", Slug: "acme", Owner: person("Alice", "alice@acme.test"),
			Members: []usecase.NewMember{
				{User: person("Dina", "dina@acme.test"), Role: entity.RoleAdmin},
				{User: person("Edo", "edo@acme.test"), Role: entity.RoleMember},
				{User: person("Fajar", "fajar@acme.test"), Role: entity.RoleMember, Status: entity.MembershipDeactivated},
				{User: carol, Role: entity.RoleMember},
			},
		},
		{
			Name: "Globex", Slug: "globex", Owner: person("Bob", "bob@globex.test"),
			Members: []usecase.NewMember{{User: carol, Role: entity.RoleMember}},
		},
	}
}

// withOrganizations opens a pool on databaseURL (the app role) and runs fn with the Organization
// use case wired to it.
func withOrganizations(ctx context.Context, databaseURL string, fn func(usecase.Organization) error) error {
	pool, err := postgres.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	orgs := organization.New(persistent.NewTransactor(pool), user.New(), orgrepo.New(), membership.New(), password.New())
	return fn(orgs)
}
