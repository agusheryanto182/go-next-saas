// Package persistent implements the repo contracts on PostgreSQL. Queries are written in api/queries
// and generated into gen/ by sqlc (ADR 0009).
package persistent

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

// Store runs queries as the API's database role, which row-level security applies to.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore wraps a pool connected as the runtime (app) role, never the owner role.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// InScope is the unit of work: it runs fn in one transaction whose first statement sets
// app.user_id and app.org_id with transaction-local scope (the SQL equivalent of SET LOCAL),
// so every query in fn sees only that tenant's rows. The settings end with the transaction,
// which keeps pooled connections clean and works with transaction-mode poolers.
// fn's error rolls the transaction back.
func (s *Store) InScope(ctx context.Context, scope repo.Scope, fn func(q *gen.Queries) error) error {
	return s.inTx(ctx, scope, func(tx pgx.Tx) error { return fn(gen.New(tx)) })
}

// inTx is InScope with the raw transaction, for code in this package that needs it.
func (s *Store) inTx(ctx context.Context, scope repo.Scope, fn func(tx pgx.Tx) error) (err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
		}
	}()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.user_id', $1, true), set_config('app.org_id', $2, true)`,
		settingValue(scope.UserID), settingValue(scope.OrgID)); err != nil {
		return fmt.Errorf("set scope: %w", err)
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// settingValue renders an ID for set_config; the zero UUID becomes empty, which policies read as NULL.
func settingValue(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
