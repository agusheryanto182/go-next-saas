# API agent guide

Applies to `api/`. Root rules in `../AGENTS.md` still apply.

## Layout (ADR 0003)
- `cmd/app` entry point · `config` environment · `internal/app` wiring and lifecycle
- `internal/controller/restapi` HTTP: middleware, handlers, `problem` errors
- `internal/usecase/<domain>` business rules; interfaces in `internal/usecase/contracts.go`
- `internal/repo` interfaces in `contracts.go`; `persistent/` PostgreSQL, `webapi/` SMTP and other services
- `internal/entity` business models · `pkg/` generic infrastructure (no application imports)

Layer direction is enforced by depguard (`.golangci.yml`). Do not add `//nolint:depguard`; move the code instead.

## Commands (from the repository root)
- `make check-api` — format check, lint, unit tests
- `make check` — adds vulnerability scan
- Tools are pinned in `api/tools/go.mod`; run them with `go tool -modfile=tools/go.mod <tool>` from `api/`.

## Conventions
- Errors: return wrapped errors (`fmt.Errorf("...: %w", err)`); handlers turn them into problem details. Never leak internal error text to clients.
- Context: every function doing I/O takes `ctx context.Context` first.
- Logging: `log/slog` with the request context (`log.InfoContext(ctx, ...)`) so trace IDs are attached. Never log secrets, tokens or passwords.
- Tests: table-driven, standard `testing` package, `httptest` for handlers.
