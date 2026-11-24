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

### Spike result (2026-11-24, issue #6)
- **sqlc** v1.31.1 (pinned in `api/tools`) reads the schema straight from the goose migrations; UUIDs map to `github.com/google/uuid`, timestamps to `time.Time`, nullable columns to pointers. `sqlc diff` in `make check-api` fails when generated code is stale.
- **goose** v3.30.0 runs from the API binary: migrations are embedded (`api/migrations/embed.go`) and applied with `app -migrate` using `MIGRATION_DATABASE_URL` (owner role). In Docker Compose a one-shot `migrate` service runs before the API starts. Rerunning is a no-op.
- The up → down → up round trip is tested; it caught a down migration that dropped `memberships` before the policy that reads it.
- **Squawk** 2.72.0 with `assume_in_transaction` (goose wraps each migration in a transaction). It required `lock_timeout` and `statement_timeout` at the top of a migration, which is now the convention. Exceptions are written next to the statement with a reason (`-- squawk-ignore <rule>`), e.g. a plain index on a table created empty in the same migration, and drops in Down sections. A risky probe (`ADD COLUMN … NOT NULL` without default, index without `CONCURRENTLY`) is rejected.
- **Integration tests** live next to the code with the `integration` build tag and start PostgreSQL with testcontainers (`make test-integration`).
- Query tracing: `otelpgx` on the pool.

## Alternatives considered
- **An ORM (GORM, ent):** less SQL to write, but queries are harder to review and row-level security plus transactions are less explicit.
- **A query builder (Squirrel):** dynamic queries are easier, but no compile-time check of columns and types.
- **golang-migrate or Atlas:** both capable; goose keeps plain SQL with optional Go migrations and is simple to embed.

## Consequences
- Every query is visible SQL with generated types.
- Dynamic filters (search with optional status and role) are written with `sqlc.narg` and nullable parameters.
- Squawk adds friction to risky migrations on purpose.
