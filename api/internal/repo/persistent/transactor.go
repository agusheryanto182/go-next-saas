// Package persistent implements the repo contracts on PostgreSQL. Queries are written in api/queries
// and generated into gen/ by sqlc (ADR 0009). This package holds the unit of work (Transactor) and the
// helpers the per-domain repositories in its sub-packages share.
package persistent

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agusheryanto182/go-next-saas/api/internal/repo"
	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/gen"
)

var (
	// ErrNoUnitOfWork is returned by every repository called outside Transactor.InScope.
	ErrNoUnitOfWork = errors.New("repository called outside a unit of work (Transactor.InScope)")
	// ErrNestedUnitOfWork is returned by InScope when ctx already carries a transaction.
	ErrNestedUnitOfWork = errors.New("unit of work already in progress: InScope calls do not nest")
	// ErrInvalidID is returned for an ID that is not a UUID.
	ErrInvalidID = errors.New("invalid ID")
)

// Transactor runs units of work as the API's database role, which row-level security applies to.
type Transactor struct {
	pool *pgxpool.Pool
}

// NewTransactor wraps a pool connected as the runtime (app) role, never the owner role.
func NewTransactor(pool *pgxpool.Pool) repo.Transactor {
	return newTraced(&Transactor{pool: pool})
}

type txKey struct{}

// InScope starts a transaction whose first statement sets app.user_id and app.org_id with
// transaction-local scope (the SQL equivalent of SET LOCAL), so every query in fn sees only that
// tenant's rows. The settings end with the transaction, which keeps pooled connections clean and
// works with transaction-mode poolers.
func (t *Transactor) InScope(ctx context.Context, scope repo.Scope, fn func(ctx context.Context) error) (err error) {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return ErrNestedUnitOfWork
	}
	userID, err := settingValue(scope.UserID)
	if err != nil {
		return fmt.Errorf("Transactor - InScope - user: %w", err)
	}
	orgID, err := settingValue(scope.OrgID)
	if err != nil {
		return fmt.Errorf("Transactor - InScope - organization: %w", err)
	}

	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("Transactor - InScope - t.pool.Begin: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
			err = errors.Join(err, fmt.Errorf("Transactor - InScope - tx.Rollback: %w", rbErr))
		}
	}()

	if _, err := tx.Exec(ctx, `SELECT set_config('app.user_id', $1, true), set_config('app.org_id', $2, true)`,
		userID, orgID); err != nil {
		return fmt.Errorf("Transactor - InScope - set scope: %w", err)
	}
	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("Transactor - InScope - tx.Commit: %w", err)
	}
	return nil
}

// Tx returns the unit of work's transaction carried by ctx, or ErrNoUnitOfWork.
func Tx(ctx context.Context) (pgx.Tx, error) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	if !ok {
		return nil, ErrNoUnitOfWork
	}
	return tx, nil
}

// Queries returns the generated queries bound to the unit of work's transaction, or ErrNoUnitOfWork.
// Repositories never use the pool: this is what keeps every query inside a scope.
func Queries(ctx context.Context) (*gen.Queries, error) {
	tx, err := Tx(ctx)
	if err != nil {
		return nil, err
	}
	return gen.New(tx), nil
}

// settingValue renders an ID for set_config; empty stays empty, which policies read as NULL.
func settingValue(id string) (string, error) {
	if id == "" {
		return "", nil
	}
	u, err := ParseID(id)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
