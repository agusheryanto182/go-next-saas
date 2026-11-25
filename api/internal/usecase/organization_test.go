package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase"
	"github.com/agusheryanto182/go-next-saas/api/internal/usecase/organization"
)

type orgMocks struct {
	tx      *MockTransactor
	users   *MockUserRepo
	orgs    *MockOrganizationRepo
	members *MockMembershipRepo
	hasher  *MockPasswordHasher
	// fnErr is the error the unit of work's fn returned, which makes the real Transactor roll back.
	fnErr *error
}

func newOrganization(t *testing.T) (usecase.Organization, orgMocks) {
	t.Helper()
	ctrl := gomock.NewController(t)
	m := orgMocks{
		tx: NewMockTransactor(ctrl), users: NewMockUserRepo(ctrl), orgs: NewMockOrganizationRepo(ctrl),
		members: NewMockMembershipRepo(ctrl), hasher: NewMockPasswordHasher(ctrl), fnErr: new(error),
	}
	return organization.New(m.tx, m.users, m.orgs, m.members, m.hasher), m
}

// expectUnitOfWork runs fn like the real Transactor, checking that the scope is the new organization
// (captured from the org Store call) and recording fn's error.
func (m orgMocks) expectUnitOfWork(orgID *string) {
	m.tx.EXPECT().InScope(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, scope repo.Scope, fn func(context.Context) error) error {
			err := fn(ctx)
			*m.fnErr = err
			if orgID != nil && *orgID != "" && scope.OrgID != *orgID {
				return errors.New("scope is not the new organization")
			}
			return err
		})
}

func ownerInput() usecase.CreateOrganizationInput {
	return usecase.CreateOrganizationInput{
		Name: " Acme ", Slug: "acme",
		Owner: usecase.NewUser{Email: " Alice@Acme.Test ", FullName: "Alice", Password: "correct horse"},
	}
}

func TestOrganizationCreateNewOwner(t *testing.T) {
	t.Parallel()
	uc, m := newOrganization(t)
	ctx := t.Context()
	var orgID string

	m.hasher.EXPECT().Hash(gomock.Any(), "correct horse").Return("hash", nil)
	m.expectUnitOfWork(&orgID)
	m.orgs.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, o *entity.Organization) error {
		orgID = o.ID
		assert.Equal(t, "Acme", o.Name, "name is trimmed")
		assert.Equal(t, "acme", o.Slug)
		return nil
	})
	m.users.EXPECT().GetByEmail(gomock.Any(), "alice@acme.test").Return(entity.User{}, entity.ErrUserNotFound)
	var userID string
	m.users.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u *entity.User) error {
		userID = u.ID
		assert.NotEmpty(t, u.ID)
		assert.Equal(t, "alice@acme.test", u.Email, "email is normalized")
		assert.Equal(t, "hash", u.PasswordHash)
		return nil
	})
	m.members.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ms *entity.Membership) error {
		assert.Equal(t, entity.Membership{
			OrganizationID: orgID, UserID: userID, Role: entity.RoleOwner, Status: entity.MembershipActive,
		}, *ms)
		return nil
	})

	res, err := uc.Create(ctx, ownerInput())

	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.Equal(t, orgID, res.Organization.ID)
	assert.Equal(t, "acme", res.Organization.Slug)
}

func TestOrganizationCreateReusesExistingUser(t *testing.T) {
	t.Parallel()
	uc, m := newOrganization(t)
	var orgID string

	m.hasher.EXPECT().Hash(gomock.Any(), gomock.Any()).Return("new-hash", nil)
	m.expectUnitOfWork(&orgID)
	m.orgs.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, o *entity.Organization) error {
		orgID = o.ID
		return nil
	})
	m.users.EXPECT().GetByEmail(gomock.Any(), "alice@acme.test").
		Return(entity.User{ID: "existing", Email: "alice@acme.test", PasswordHash: "old-hash"}, nil)
	// No users.Store: the existing user, and their password, stay as they are.
	m.members.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ms *entity.Membership) error {
		assert.Equal(t, "existing", ms.UserID)
		return nil
	})

	res, err := uc.Create(t.Context(), ownerInput())

	require.NoError(t, err)
	assert.True(t, res.Created)
}

func TestOrganizationCreateWithMembers(t *testing.T) {
	t.Parallel()
	uc, m := newOrganization(t)

	in := ownerInput()
	in.Owner.Password = ""
	in.Members = []usecase.NewMember{
		{User: usecase.NewUser{Email: "dina@acme.test", FullName: "Dina"}, Role: entity.RoleAdmin},
		{User: usecase.NewUser{Email: "fajar@acme.test", FullName: "Fajar"}, Role: entity.RoleMember, Status: entity.MembershipDeactivated},
	}

	// No passwords: no hashing.
	m.expectUnitOfWork(nil)
	m.orgs.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
	m.users.EXPECT().GetByEmail(gomock.Any(), gomock.Any()).Return(entity.User{}, entity.ErrUserNotFound).Times(3)
	m.users.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, u *entity.User) error {
		assert.Empty(t, u.PasswordHash)
		return nil
	}).Times(3)
	var got []entity.Membership
	m.members.EXPECT().Store(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, ms *entity.Membership) error {
		got = append(got, *ms)
		return nil
	}).Times(3)

	_, err := uc.Create(t.Context(), in)

	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, entity.RoleOwner, got[0].Role)
	assert.Equal(t, entity.RoleAdmin, got[1].Role)
	assert.Equal(t, entity.MembershipActive, got[1].Status, "empty status means active")
	assert.Equal(t, entity.MembershipDeactivated, got[2].Status)
}

func TestOrganizationCreateSlugTakenChangesNothing(t *testing.T) {
	t.Parallel()
	uc, m := newOrganization(t)

	m.hasher.EXPECT().Hash(gomock.Any(), gomock.Any()).Return("hash", nil)
	m.expectUnitOfWork(nil)
	m.orgs.EXPECT().Store(gomock.Any(), gomock.Any()).Return(entity.ErrOrganizationSlugTaken)
	// No user or membership calls: the organization's insert comes first.

	res, err := uc.Create(t.Context(), ownerInput())

	require.NoError(t, err)
	assert.False(t, res.Created)
	assert.Equal(t, entity.Organization{}, res.Organization)
	require.ErrorIs(t, *m.fnErr, entity.ErrOrganizationSlugTaken, "the unit of work is rolled back")
}

func TestOrganizationCreateInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		modify func(*usecase.CreateOrganizationInput)
		want   error
	}{
		{"blank name", func(in *usecase.CreateOrganizationInput) { in.Name = "   " }, entity.ErrInvalidOrganizationName},
		{"long name", func(in *usecase.CreateOrganizationInput) { in.Name = strings.Repeat("a", 101) }, entity.ErrInvalidOrganizationName},
		{"uppercase slug", func(in *usecase.CreateOrganizationInput) { in.Slug = "Acme" }, entity.ErrInvalidOrganizationSlug},
		{"short slug", func(in *usecase.CreateOrganizationInput) { in.Slug = "ac" }, entity.ErrInvalidOrganizationSlug},
		{"owner email", func(in *usecase.CreateOrganizationInput) { in.Owner.Email = "alice" }, entity.ErrInvalidEmail},
		{"owner name", func(in *usecase.CreateOrganizationInput) { in.Owner.FullName = " " }, entity.ErrInvalidFullName},
		{"short password", func(in *usecase.CreateOrganizationInput) { in.Owner.Password = "short" }, entity.ErrInvalidPassword},
		{"member role", func(in *usecase.CreateOrganizationInput) {
			in.Members = []usecase.NewMember{{User: usecase.NewUser{Email: "d@acme.test", FullName: "D"}, Role: "boss"}}
		}, entity.ErrInvalidRole},
		{"member status", func(in *usecase.CreateOrganizationInput) {
			in.Members = []usecase.NewMember{{User: usecase.NewUser{Email: "d@acme.test", FullName: "D"}, Role: entity.RoleMember, Status: "gone"}}
		}, entity.ErrInvalidStatus},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			uc, _ := newOrganization(t) // no calls expected: validation comes before any work

			in := ownerInput()
			tt.modify(&in)
			_, err := uc.Create(t.Context(), in)

			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestOrganizationCreateRepositoryErrorRollsBack(t *testing.T) {
	t.Parallel()
	uc, m := newOrganization(t)
	errDB := errors.New("connection lost")

	m.hasher.EXPECT().Hash(gomock.Any(), gomock.Any()).Return("hash", nil)
	m.expectUnitOfWork(nil)
	m.orgs.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
	m.users.EXPECT().GetByEmail(gomock.Any(), gomock.Any()).Return(entity.User{}, entity.ErrUserNotFound)
	m.users.EXPECT().Store(gomock.Any(), gomock.Any()).Return(nil)
	m.members.EXPECT().Store(gomock.Any(), gomock.Any()).Return(errDB)

	res, err := uc.Create(t.Context(), ownerInput())

	require.ErrorIs(t, err, errDB)
	assert.False(t, res.Created)
	require.ErrorIs(t, *m.fnErr, errDB, "fn's error is what makes the unit of work roll back")
}

func TestOrganizationCreateHashError(t *testing.T) {
	t.Parallel()
	uc, m := newOrganization(t)

	m.hasher.EXPECT().Hash(gomock.Any(), gomock.Any()).Return("", context.Canceled)
	// No unit of work starts.

	_, err := uc.Create(t.Context(), ownerInput())

	require.ErrorIs(t, err, context.Canceled)
}
