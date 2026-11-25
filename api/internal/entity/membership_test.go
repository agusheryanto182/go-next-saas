package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

func TestRoleValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		role entity.Role
		want bool
	}{
		{entity.RoleOwner, true},
		{entity.RoleAdmin, true},
		{entity.RoleMember, true},
		{"", false},
		{"Owner", false},
		{"superadmin", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.role.Valid())
		})
	}
}

func TestMembershipStatusValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status entity.MembershipStatus
		want   bool
	}{
		{entity.MembershipActive, true},
		{entity.MembershipDeactivated, true},
		{"", false},
		{"pending", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.status.Valid())
		})
	}
}
