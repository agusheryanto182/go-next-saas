package entity

import "time"

// User is a global identity; it can belong to several organizations through memberships (ADR 0007).
type User struct {
	ID       string
	Email    string // lowercase
	FullName string
	Phone    string // empty when not given
	// PasswordHash is empty until the user accepts an invitation and sets a password.
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
