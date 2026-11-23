# 0009. pgx with sqlc, goose migrations linted by Squawk

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Row-level security, explicit transactions and reviewable queries all favour writing SQL directly. Migrations on a live database can lock tables or lose data.

## Decision
- **Driver:** `pgx` v5 with its connection pool.
- **Queries:** written as SQL in `api/queries/`; `sqlc` generates typed Go code into `internal/repo/persistent`.
- **Migrations:** `goose` with plain SQL files in `api/migrations/`, run as the owner role before the new version starts.
- **Migration lint:** Squawk checks every migration in CI for operations that lock tables or break running code.

## Alternatives considered
- **An ORM (GORM, ent):** less SQL to write, but queries are harder to review and row-level security plus transactions are less explicit.
- **A query builder (Squirrel):** dynamic queries are easier, but no compile-time check of columns and types.
- **golang-migrate or Atlas:** both capable; goose keeps plain SQL with optional Go migrations and is simple to embed.

## Consequences
- Every query is visible SQL with generated types.
- Dynamic filters (search with optional status and role) are written with `sqlc.narg` and nullable parameters.
- Squawk adds friction to risky migrations on purpose.
