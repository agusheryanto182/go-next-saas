package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

func TestCheckSeedHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		ok   bool
	}{
		{"localhost", "postgres://saas_app:x@localhost:5432/saas?sslmode=disable", true},
		{"IPv4 loopback", "postgres://saas_app:x@127.0.0.1:5432/saas", true},
		{"IPv6 loopback", "postgres://saas_app:x@[::1]:5432/saas", true},
		{"Compose service", "postgres://saas_app:x@postgres:5432/saas", true},
		{"keyword form", "host=localhost user=saas_app dbname=saas", true},
		{"hosted database", "postgres://app:x@db.example.com:5432/saas?sslmode=require", false},
		{"private address", "postgres://app:x@10.0.0.5/saas", false},
		{"host query parameter overrides", "postgres://app:x@localhost/saas?host=db.example.com", false},
		{"hosted fallback", "postgres://app:x@localhost,db.example.com/saas", false},
		{"lookalike", "postgres://app:x@localhost.example.com/saas", false},
		{"unix socket", "postgres:///saas?host=/var/run/postgresql", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkSeedHost(tt.url)
			if tt.ok {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrSeedHostRefused)
		})
	}
}

func TestCheckSeedHostUnparsable(t *testing.T) {
	t.Parallel()
	err := checkSeedHost("postgres://saas_app:secret-pw@localhost:notaport/saas")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-pw", "the password never appears in an error")
}

// TestSeedData pins the dataset from ADR 0012, which end-to-end tests and the README rely on.
func TestSeedData(t *testing.T) {
	t.Parallel()

	type member struct {
		email  string
		role   entity.Role
		status entity.MembershipStatus
	}
	got := map[string][]member{}
	for _, in := range seedData() {
		got[in.Slug] = append(got[in.Slug], member{in.Owner.Email, entity.RoleOwner, ""})
		for _, m := range in.Members {
			got[in.Slug] = append(got[in.Slug], member{m.User.Email, m.Role, m.Status})
		}
		require.NoError(t, entity.ValidatePassword(in.Owner.Password))
	}

	assert.Equal(t, map[string][]member{
		"acme": {
			{"alice@acme.test", entity.RoleOwner, ""},
			{"dina@acme.test", entity.RoleAdmin, ""},
			{"edo@acme.test", entity.RoleMember, ""},
			{"fajar@acme.test", entity.RoleMember, entity.MembershipDeactivated},
			{"carol@example.test", entity.RoleMember, ""},
		},
		"globex": {
			{"bob@globex.test", entity.RoleOwner, ""},
			{"carol@example.test", entity.RoleMember, ""},
		},
	}, got)
}
