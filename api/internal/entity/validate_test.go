package entity_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

func TestOrganizationValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		org  entity.Organization
		want error
	}{
		{"valid", entity.Organization{Name: "Acme", Slug: "acme"}, nil},
		{"slug with digits and dashes", entity.Organization{Name: "A", Slug: "acme-2-eu"}, nil},
		{"longest name", entity.Organization{Name: strings.Repeat("é", 100), Slug: "acme"}, nil},
		{"longest slug", entity.Organization{Name: "A", Slug: strings.Repeat("a", 40)}, nil},
		{"empty name", entity.Organization{Name: "", Slug: "acme"}, entity.ErrInvalidOrganizationName},
		{"long name", entity.Organization{Name: strings.Repeat("a", 101), Slug: "acme"}, entity.ErrInvalidOrganizationName},
		{"short slug", entity.Organization{Name: "A", Slug: "ac"}, entity.ErrInvalidOrganizationSlug},
		{"long slug", entity.Organization{Name: "A", Slug: strings.Repeat("a", 41)}, entity.ErrInvalidOrganizationSlug},
		{"uppercase slug", entity.Organization{Name: "A", Slug: "Acme"}, entity.ErrInvalidOrganizationSlug},
		{"slug with space", entity.Organization{Name: "A", Slug: "ac me"}, entity.ErrInvalidOrganizationSlug},
		{"slug with non-ASCII", entity.Organization{Name: "A", Slug: "acmé"}, entity.ErrInvalidOrganizationSlug},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.org.Validate()
			if tt.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "alice@acme.test", entity.NormalizeEmail("  Alice@ACME.test\n"))
}

func TestUserValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user entity.User
		want error
	}{
		{"valid", entity.User{Email: "a@b", FullName: "A"}, nil},
		{"no at", entity.User{Email: "alice", FullName: "A"}, entity.ErrInvalidEmail},
		{"short email", entity.User{Email: "@b", FullName: "A"}, entity.ErrInvalidEmail},
		{"long email", entity.User{Email: strings.Repeat("a", 250) + "@b.test", FullName: "A"}, entity.ErrInvalidEmail},
		{"not normalized", entity.User{Email: "Alice@acme.test", FullName: "A"}, entity.ErrInvalidEmail},
		{"empty name", entity.User{Email: "a@b", FullName: ""}, entity.ErrInvalidFullName},
		{"long name", entity.User{Email: "a@b", FullName: strings.Repeat("a", 151)}, entity.ErrInvalidFullName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.user.Validate()
			if tt.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.want)
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	assert.NoError(t, entity.ValidatePassword("12345678"))
	assert.NoError(t, entity.ValidatePassword(strings.Repeat("é", 128)))
	assert.ErrorIs(t, entity.ValidatePassword("1234567"), entity.ErrInvalidPassword)
	assert.ErrorIs(t, entity.ValidatePassword(strings.Repeat("a", 129)), entity.ErrInvalidPassword)
}
