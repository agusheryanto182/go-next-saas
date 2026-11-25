# API agent guide

Applies to `api/`. Root rules in `../AGENTS.md` still apply. The structure and conventions follow
[evrone/go-clean-template](https://github.com/evrone/go-clean-template) (ADR 0003); where this file
differs, it says why.

## Commands — use the Makefile target, not the tool

Run from the repository root. Targets carry flags and pinned tool versions (`api/tools/go.mod`);
running a tool directly runs it with other settings than CI.

| Task | Use this | Not this |
|---|---|---|
| Regenerate code (OpenAPI server, sqlc queries, mocks) | `make gen` | `oapi-codegen`, `sqlc generate`, `mockgen`, `go generate` |
| Format | `make fmt` | `gofmt`, `gofumpt` |
| Lint, generated code in sync, migration lint, unit tests | `make check-api` | `golangci-lint run`, `go test ./...` |
| Everything CI runs except end-to-end | `make check` | running the steps by hand |
| Integration tests (real PostgreSQL in Docker) | `make test-integration` | `go test -tags integration` |
| Vulnerability scan | `make vuln` | `govulncheck ./...` |

Never claim a change is done without `make check` passing.

## Dependency rule

```
cmd/app → internal/app → internal/controller/restapi ─┐
                       → internal/repo/*             ─┤→ internal/usecase (interfaces)
                                                      └→ internal/entity
```

- `internal/entity`: domain types, their invariants, and sentinel errors. Standard library only.
- `internal/usecase/contracts.go`: the interfaces controllers call. `internal/usecase/<domain>/` implements them.
- `internal/repo/contracts.go`: the interfaces use cases call, including the unit of work (`Transactor`).
  `internal/repo/persistent/<domain>/` (PostgreSQL) and `internal/repo/webapi/` (SMTP and other services) implement them.
- `internal/controller/restapi/v1/`: HTTP handlers for the generated strict server.
- `pkg/`: infrastructure with no application knowledge (server, logger, postgres pool, OpenTelemetry).

Inner layers never import outer ones; depguard (`.golangci.yml`) fails the build otherwise. Do not
add `//nolint:depguard`; move the code instead. Use cases may import the standard library, `entity`,
the `repo` and `usecase` contracts, `github.com/google/uuid` (to create IDs) and, in `tracing.go`
only, OpenTelemetry. Wiring happens once, in `internal/app/app.go`.

## Where code goes

| The code knows about… | It belongs in | Shape |
|---|---|---|
| nothing but the domain | `internal/entity/<name>.go` | struct + methods (`Role.Valid`) |
| a business rule, possibly across repositories | `internal/usecase/<domain>/<domain>.go` | method on `UseCase` |
| SQL, a table, a PostgreSQL error code | `queries/<domain>.sql` + `internal/repo/persistent/<domain>/<domain>.go` | sqlc query + method on `Repo` |
| an outbound service (SMTP, HTTP API) | `internal/repo/webapi/<name>.go` | method on the webapi struct |
| an HTTP status, a header, a cookie | `internal/controller/restapi/v1/<domain>.go` | strict-server handler method |
| a server, pool or client with no domain knowledge | `pkg/<name>/` | reusable package |

Layout rules:
- One file per domain per layer, named after the domain, plus a `tracing.go` in every
  `usecase/<domain>/` and `repo/persistent/<domain>/` package. Package name = directory name.
- Constructors are called `New`, take their dependencies as arguments, and return the **interface**
  from `contracts.go`, wrapped in the tracing decorator: `return newTraced(&UseCase{...})`.
- No package-level state (`gochecknoglobals`, `gochecknoinits`); sentinel errors are the exception.
- Unexported package constants start with an underscore (`_tracerName`, `_defaultTimeout`).

## Unit of work and tenant isolation (ADR 0008)

The one addition to the template, which has no transactions: `repo.Transactor`.

```go
err := uc.tx.InScope(ctx, repo.Scope{UserID: userID, OrgID: orgID}, func(ctx context.Context) error {
    org, err := uc.orgs.GetBySlug(ctx, slug)   // repositories use the ctx that fn receives
    ...
    return uc.members.Store(ctx, &m)          // an error rolls everything back
})
```

- `InScope` opens one transaction and sets `app.user_id` / `app.org_id` (transaction-local) first.
  Row-level security policies read them, so a query only sees the scoped organization's rows.
- Repositories take their transaction from `ctx` (`persistent.Queries(ctx)`). Called outside
  `InScope` they return `persistent.ErrNoUnitOfWork`; they never fall back to the pool.
- `InScope` does not nest (`persistent.ErrNestedUnitOfWork`): one transaction has one scope. Work
  that must survive a rollback (login history) uses its own `InScope` call.
- Creating an organization uses its new ID as the scope's `OrgID` (the policy's `WITH CHECK`).
- Tables: organization data (`organizations`, `memberships`, later `invitations` and business tables)
  has `ENABLE` + `FORCE ROW LEVEL SECURITY`. Global identity tables (`users`, `sessions`,
  `account_tokens`, `login_history`, `outbox_emails`) do not: they are read before a scope exists.
  Every query still runs inside `InScope`, with an empty scope where none applies.

## The handler contract

The strict server (`v1/gen`) has already decoded the request and validated it against
`openapi/openapi.yaml`. Every handler method then does the same four things, in this order:

1. **Get the caller** from the request context (session middleware, M1). Missing = 401, not 500.
2. **Call the use case** with the request `ctx`, converting generated types to entity values.
3. **Map the error:** `errors.Is` against `entity.Err*` → a problem response from the contract.
   Log the wrapped error; never return its text to the client.
4. **Return the generated response type**, converting from entities. Never expose an entity directly.

## Conventions that will bite you

- **Traced decorators.** Adding a method to a contract means implementing it three times: the
  interface, the struct, and the `traced*` decorator in `tracing.go`. A missing decorator method is
  a compile error that reads like an unrelated interface mismatch. Span attributes carry IDs and
  slugs, **never emails, names or tokens**: traces leave the system. Repositories use
  `pkg/otel.StartSpan`/`EndSpan`; use cases cannot import `pkg/`, so their `tracing.go` keeps two
  small local helpers on `go.opentelemetry.io/otel`, as in the template.
- **Mocks are generated, not written.** `internal/usecase/mocks_*_test.go` come from the
  `//go:generate` lines in both `contracts.go` files. After changing a contract, run `make gen`;
  `make check-api` fails while they are stale.
- **Errors.** Domain failures are sentinels in `internal/entity/errors.go`. Everything else is
  wrapped with the layer path: `fmt.Errorf("OrganizationRepo - Store - q.CreateOrganization: %w", err)`.
  `err113` rejects new dynamic errors (`errors.New` inside a function, `fmt.Errorf` without `%w`)
  outside tests: add a sentinel instead. Only controllers turn a sentinel into an HTTP status.
  Sign-in failures always say "Invalid email or password." (root rule 4).
- **IDs** are UUIDv7 strings in entities, created in use cases with `uuid.Must(uuid.NewV7()).String()`
  and parsed in repositories (`persistent.ParseID`). Optional text fields are `""` in entities and
  `NULL` in the database (`persistent.TextOrNull` / `TextOrEmpty`).
- **Config is environment-only** (`config/config.go`, caarlos0/env, validated at start-up). A new
  setting needs the field, a validation rule if it has a format, an entry in `../.env.example`, and,
  if the container needs it, `../compose.yml`.
- **Lint is strict.** Beyond the defaults: `exhaustive` (a `switch` over `Role` or a status needs every
  case or a `default`), `paralleltest`/`tparallel`, `err113`, `cyclop` (15), `gocognit` (20),
  `unparam`, `revive`, and `nolintlint` (every `//nolint` names the linter and says why).
- **Context** is the first parameter of every function that does I/O or crosses a layer.
- **Logging:** `log/slog` with the request context (`log.InfoContext(ctx, ...)`) so trace IDs are
  attached. Never log secrets, tokens or passwords.

## Generated files — do not hand-edit

- `internal/controller/restapi/v1/gen/api.gen.go` ← `make gen` from `../openapi/openapi.yaml`
- `internal/repo/persistent/gen/*.go` ← `make gen` from `queries/` and `migrations/`
- `internal/usecase/mocks_*_test.go` ← `make gen` from the `contracts.go` files

## Adding a use case to an existing domain

1. Add the method to `internal/usecase/contracts.go`.
2. Implement it in `internal/usecase/<domain>/<domain>.go` and mirror it in that package's `tracing.go`.
3. If it needs new persistence: the query in `queries/<domain>.sql`, the method in
   `internal/repo/contracts.go`, `internal/repo/persistent/<domain>/<domain>.go` and its `tracing.go`.
4. `make gen`, then extend `internal/usecase/<domain>_test.go` (table-driven, gomock, testify, `t.Parallel()`).
5. If the API changes: the contract first (`/spec`), then the handler in `internal/controller/restapi/v1/`.

## Adding a whole domain

Work outward from the entity; each step has a counterpart in the `organization` domain.

1. **Entity:** `internal/entity/<domain>.go` with the type and its invariants; sentinels in `errors.go`.
2. **Migration:** a new file in `migrations/` starting with `SET LOCAL lock_timeout` and
   `SET LOCAL statement_timeout`, with a working `-- +goose Down`. Organization data gets
   `ENABLE` + `FORCE ROW LEVEL SECURITY` and a policy on `app_org_id()`. Never edit an applied migration.
3. **Queries:** `queries/<domain>.sql`, then `make gen`.
4. **Repo contract:** the interface in `internal/repo/contracts.go`.
5. **Repo implementation:** `internal/repo/persistent/<domain>/<domain>.go` (`package <domain>`,
   `persistent.Queries(ctx)`, PostgreSQL errors mapped to sentinels) plus `tracing.go`, and `New`
   returning the interface wrapped in `newTraced`.
6. **Use-case contract and implementation:** `internal/usecase/contracts.go`,
   `internal/usecase/<domain>/<domain>.go` + `tracing.go`; dependencies (repos, `repo.Transactor`) through `New`.
7. **`make gen`**, then `internal/usecase/<domain>_test.go` against the mocks.
8. **Wiring:** construct the repositories and the use case in `internal/app/app.go`.
9. **Transport:** contract change (`/spec`), handler, contract tests in `internal/controller/restapi/`.
10. **Integration tests:** repositories against PostgreSQL in `internal/repo/persistent/`, including a
    tenant-isolation test for organization data.

## Tests

- **Unit:** next to the code or, for use cases, in `internal/usecase/<domain>_test.go`
  (`package usecase_test`). Table-driven, testify (`require` to stop, `assert` to continue), gomock
  for dependencies, `t.Parallel()` everywhere except tests that use `t.Setenv`. Fake the unit of work
  with the `Transactor` mock:
  `tx.EXPECT().InScope(gomock.Any(), scope, gomock.Any()).DoAndReturn(func(ctx context.Context, _ repo.Scope, fn func(context.Context) error) error { return fn(ctx) })`.
- **Integration:** build tag `integration`, real PostgreSQL from `persistent/pgtest` (same roles as
  production), through `persistent.NewTransactor` and the repositories. `make test-integration`.
- **HTTP:** `httptest` against `restapi.NewRouter`; responses are checked against the contract (`contract_test.go`).
