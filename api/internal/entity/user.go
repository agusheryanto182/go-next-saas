package entity

import (
	"strings"
	"time"
	"unicode/utf8"
)

// User is a global identity; it can belong to several organizations through memberships (ADR 0007).
type User struct {
	ID       string
	Email    string // stored form, see NormalizeEmail
	FullName string
	Phone    string // empty when not given
	// PasswordHash is empty until the user accepts an invitation and sets a password.
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NormalizeEmail returns the stored form of an email address: trimmed and lowercase. Users are
// stored and looked up by it.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// Validate checks the email and full name as the database does (users_email_lowercase,
// users_email_length, users_full_name_length), and that the email has an @. The email must already
// be normalized.
func (u User) Validate() error {
	if n := utf8.RuneCountInString(u.Email); n < 3 || n > 255 || !strings.Contains(u.Email, "@") ||
		u.Email != NormalizeEmail(u.Email) {
		return ErrInvalidEmail
	}
	if n := utf8.RuneCountInString(u.FullName); n < 1 || n > 150 {
		return ErrInvalidFullName
	}
	return nil
}

// ValidatePassword checks a new password against the contract's rule (NewPassword: 8 to 128
// characters). Sign-in does not apply it, so every wrong password gets the same answer.
func ValidatePassword(password string) error {
	if n := utf8.RuneCountInString(password); n < 8 || n > 128 {
		return ErrInvalidPassword
	}
	return nil
}
