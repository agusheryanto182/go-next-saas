package entity

import "time"

// Role is what a member may do in an organization.
type Role string

// Roles, from most to least privileged.
const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember:
		return true
	default:
		return false
	}
}

// MembershipStatus is whether a member can currently use the organization.
type MembershipStatus string

// Membership statuses.
const (
	MembershipActive      MembershipStatus = "active"
	MembershipDeactivated MembershipStatus = "deactivated"
)

// Valid reports whether s is a known status.
func (s MembershipStatus) Valid() bool {
	switch s {
	case MembershipActive, MembershipDeactivated:
		return true
	default:
		return false
	}
}

// Membership links a user to an organization with a role.
type Membership struct {
	OrganizationID string
	UserID         string
	Role           Role
	Status         MembershipStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Member is a user as seen from one organization.
type Member struct {
	UserID   string
	Email    string
	FullName string
	Phone    string
	Role     Role
	Status   MembershipStatus
	JoinedAt time.Time
}

// UserOrganization is an organization as seen by one of its members.
type UserOrganization struct {
	OrganizationID string
	Name           string
	Slug           string
	Role           Role
	Status         MembershipStatus
}
