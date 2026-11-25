package entity

import "errors"

// Domain errors. Repositories and use cases return these; only controllers turn them into
// transport responses. Other failures are wrapped errors with the layer path.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")

	ErrOrganizationNotFound  = errors.New("organization not found")
	ErrOrganizationSlugTaken = errors.New("organization slug already taken")

	ErrMembershipExists = errors.New("user is already a member of the organization")
)
