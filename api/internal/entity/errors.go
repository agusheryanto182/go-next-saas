package entity

import "errors"

// Domain errors. Repositories and use cases return these; only controllers turn them into
// transport responses. Other failures are wrapped errors with the layer path.
var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidEmail      = errors.New("email must contain @ and be 3 to 255 characters")
	ErrInvalidFullName   = errors.New("full name must be 1 to 150 characters")
	ErrInvalidPassword   = errors.New("password must be 8 to 128 characters")

	ErrOrganizationNotFound    = errors.New("organization not found")
	ErrOrganizationSlugTaken   = errors.New("organization slug already taken")
	ErrInvalidOrganizationName = errors.New("organization name must be 1 to 100 characters")
	ErrInvalidOrganizationSlug = errors.New("organization slug must be 3 to 40 characters of a-z, 0-9 and -")

	ErrMembershipExists = errors.New("user is already a member of the organization")
	ErrInvalidRole      = errors.New("invalid role")
	ErrInvalidStatus    = errors.New("invalid membership status")
)
