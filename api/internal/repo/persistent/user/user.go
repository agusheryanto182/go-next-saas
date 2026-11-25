// Package user implements repo.UserRepo on PostgreSQL.
package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

// Repo -.
type Repo struct{}

// New returns a User repository instrumented with OpenTelemetry tracing spans.
func New() repo.UserRepo {
	return newTraced(&Repo{})
}

// Store -.
func (r *Repo) Store(ctx context.Context, u *entity.User) error {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return fmt.Errorf("UserRepo - Store - persistent.Queries: %w", err)
	}
	id, err := persistent.ParseID(u.ID)
	if err != nil {
		return fmt.Errorf("UserRepo - Store - persistent.ParseID: %w", err)
	}

	row, err := q.CreateUser(ctx, gen.CreateUserParams{
		ID:           id,
		Email:        u.Email,
		FullName:     u.FullName,
		Phone:        persistent.TextOrNull(u.Phone),
		PasswordHash: persistent.TextOrNull(u.PasswordHash),
	})
	if err != nil {
		if persistent.IsUniqueViolation(err, "users_email_key") {
			return entity.ErrUserAlreadyExists
		}
		return fmt.Errorf("UserRepo - Store - q.CreateUser: %w", err)
	}

	u.CreatedAt, u.UpdatedAt = row.CreatedAt, row.UpdatedAt
	return nil
}

// GetByEmail -.
func (r *Repo) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	q, err := persistent.Queries(ctx)
	if err != nil {
		return entity.User{}, fmt.Errorf("UserRepo - GetByEmail - persistent.Queries: %w", err)
	}

	row, err := q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.User{}, entity.ErrUserNotFound
		}
		return entity.User{}, fmt.Errorf("UserRepo - GetByEmail - q.GetUserByEmail: %w", err)
	}

	return entity.User{
		ID:           row.ID.String(),
		Email:        row.Email,
		FullName:     row.FullName,
		Phone:        persistent.TextOrEmpty(row.Phone),
		PasswordHash: persistent.TextOrEmpty(row.PasswordHash),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}, nil
}
