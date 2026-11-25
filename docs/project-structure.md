# Project structure

What every folder and important file is for. Update this page in the same pull request that adds, moves or removes one.

Files marked **generated** are produced by a tool; change their source and regenerate instead of editing them.

## Root

```
go-next-saas/
├─ openapi/              API contract: the source of truth for routes and payloads
├─ api/                  Go API (its own module)
├─ web/                  Next.js web app
├─ docs/                 documentation and decision records
├─ dev/                  files used only by the local development stack
├─ .claude/              Claude Code rules, roles, skills and hooks
├─ e2e/                  Playwright end-to-end tests against the full Docker Compose stack
├─ .github/              CI and Claude workflows, issue and pull request templates
├─ AGENTS.md             rules for every coding agent (any tool)
├─ CLAUDE.md             Claude Code entry point; imports AGENTS.md
├─ Makefile              one entry point for people, agents and CI
├─ compose.yml           local stack: PostgreSQL, Mailpit; profile `app` adds migrate (one-shot), the API and web app
├─ .env.example          every environment variable, with local defaults; copy to `.env`
├─ .worktreeinclude      gitignored files copied into agent worktrees (`.env`)
├─ .editorconfig         indentation and line endings for all editors
├─ .gitignore
├─ LICENSE               MIT
└─ README.md             overview and getting started
```

`.gitattributes` marks generated code so GitHub collapses it in pull request diffs. Coming with later issues: `deploy/` (production config, M5). `.cache/` holds scratch output of `make check` (gitignored).

## `openapi/` — API contract

| Path | Contents |
|---|---|
| `openapi.yaml` | every endpoint, parameter, schema and error (OpenAPI 3.1, ADR 0005) |
| `redocly.yaml` | spec lint rules (`make check-spec`) |
| `package.json` | pins the Redocly CLI used for linting |

Changing the contract: edit `openapi.yaml`, run `make gen`, commit the spec and the generated code together (`/spec` skill).

## `api/` — Go API

Clean architecture with the [go-clean-template](https://github.com/evrone/go-clean-template) layout ([ADR 0003](decisions/0003-clean-architecture-layout.md)). Dependencies point inward: controller → usecase → repo. Lint enforces it.

```
api/
├─ cmd/app/main.go                       entry point: serve; `-migrate` applies migrations; `-healthcheck` for containers
├─ config/config.go                      environment variables → validated Config
├─ internal/
│  ├─ app/app.go                         wiring (manual dependency injection), start-up and graceful shutdown
│  ├─ controller/restapi/                HTTP layer
│  │  ├─ router.go                       routes, /healthz and /readyz, 404/405 as problem details
│  │  ├─ middleware/                     shared middleware (panic recovery; sessions and CSRF later)
│  │  ├─ problem/                        RFC 9457 error responses
│  │  ├─ contract_test.go                responses checked against openapi.yaml
│  │  └─ v1/                             /api/v1 endpoints
│  │     ├─ v1.go                        mounts the contract routes with request validation
│  │     ├─ handler.go                   implements the generated strict server interface
│  │     └─ gen/                         oapi-codegen config; api.gen.go is **generated**
│  ├─ entity/                            business models with their invariants; errors.go = domain errors
│  ├─ usecase/
│  │  ├─ contracts.go                    use-case interfaces; one sub-folder per domain will follow (M1)
│  │  └─ mocks_*_test.go                 **generated** gomock mocks of both contracts.go files
│  └─ repo/
│     ├─ contracts.go                    storage interfaces, the unit of work (`Transactor`) and its `Scope`
│     └─ persistent/                     PostgreSQL
│        ├─ transactor.go                unit of work: transaction + transaction-local scope for row-level security
│        ├─ convert.go                   ID parsing, NULL mapping, unique-violation check shared by repositories
│        ├─ <domain>/                    one repository per domain (user, organization, membership): <domain>.go + tracing.go
│        ├─ migrate.go                   applies the embedded migrations with goose
│        ├─ pgtest/                      PostgreSQL container with production roles, for integration tests
│        ├─ *_integration_test.go        migrations round trip; unit of work and tenant isolation through the repositories
│        └─ gen/                         **generated** by sqlc from queries/ and migrations/
├─ pkg/                                  generic infrastructure, no application imports
│  ├─ httpserver/                        http.Server with timeouts and shutdown
│  ├─ logger/                            slog JSON logger with trace IDs
│  ├─ otel/                              OpenTelemetry setup; span helpers for tracing decorators
│  └─ postgres/                          pgx connection pool with query tracing
├─ migrations/                           goose SQL migrations (embedded into the binary by embed.go)
├─ queries/                              SQL queries for sqlc, one file per domain
├─ sqlc.yaml                             sqlc settings: schema from migrations/, output to repo/persistent/gen
├─ .squawk.toml                          migration lint settings
├─ tools/go.mod                          pinned dev tools: golangci-lint, gofumpt, govulncheck, mockgen, oapi-codegen, oasdiff, sqlc
├─ .golangci.yml                         lint rules, including the layer rules (depguard)
├─ Dockerfile                            multi-stage build → distroless, non-root image
├─ AGENTS.md / CLAUDE.md                 API rules for agents: where code goes, conventions, checklists
├─ NOTICE                                attribution for patterns adapted from go-clean-template (MIT)
└─ go.mod / go.sum                       module and pinned dependencies
```

Tests sit next to the code as `*_test.go`. Tests that need a real PostgreSQL carry the `integration` build tag and start a container with testcontainers (`make test-integration`).

Changing the database: add a new file in `migrations/` (never edit an applied one), add or change queries in `queries/`, run `make gen`, and `make migrate` locally. Every migration starts with `SET LOCAL lock_timeout` and `SET LOCAL statement_timeout`; Squawk exceptions need a `-- squawk-ignore <rule>` line with a reason.

## `web/` — Next.js app

```
web/
├─ src/
│  ├─ app/                               App Router: one folder per route
│  │  ├─ layout.tsx                      root layout: font, metadata, providers
│  │  ├─ providers.tsx                   client providers: theme, TanStack Query, tooltips, toasts
│  │  ├─ page.tsx                        public landing page
│  │  ├─ globals.css                     Tailwind and the shadcn/ui theme (CSS variables, dark mode)
│  │  └─ icon.svg                        app icon
│  ├─ components/ui/                     shadcn/ui components (added with the shadcn CLI, then owned by us)
│  ├─ hooks/                             React hooks (use-mobile)
│  ├─ lib/
│  │  ├─ api/client.ts                   typed API client for the browser (same-origin /api)
│  │  ├─ api/server.ts                   typed API client for server components (forwards cookie and client IP)
│  │  ├─ api/schema.d.ts                 API types, **generated** from openapi.yaml (`npm run gen:api`)
│  │  └─ utils.ts                        `cn` class helper
│  └─ test/                              Vitest setup and the MSW mock server
├─ public/                               static files served as-is (logo)
├─ next.config.ts                        standalone output, dev-only /api rewrite to the Go API
├─ components.json                       shadcn/ui settings (style radix-nova)
├─ vitest.config.mts                     test runner
├─ eslint.config.mjs / .prettierrc.json  lint and format
├─ tsconfig.json                         TypeScript, `@/` → `src/`
├─ .npmrc                                exact versions, strict Node engine
├─ Dockerfile                            multi-stage build → standalone Node server, non-root
└─ AGENTS.md / CLAUDE.md                 Next.js-managed agent notes plus our web rules
```

**Generated:** `next-env.d.ts` and `.next/` (by Next.js, gitignored); `src/lib/api/schema.d.ts` (from the OpenAPI contract).

Tests sit next to the code as `*.test.ts(x)`.

## `docs/`

| Path | Contents |
|---|---|
| `README.md` | index of all documents and decision records |
| `architecture.md` | system overview, layers, request flow, tenancy |
| `project-structure.md` | this page |
| `agentic-workflow.md` | how issues become merged pull requests with coding agents |
| `decisions/` | Architecture Decision Records; new ones copy `template.md` |

## `dev/`

| Path | Contents |
|---|---|
| `postgres-init/01-roles.sh` | runs once when the local database volume is created: the `saas_owner` role (owns the schema, runs migrations) and the `saas_app` role (runtime, subject to row-level security). `make reset-db` runs it again. |

## `.claude/`

| Path | Contents |
|---|---|
| `settings.json` | allowed and denied commands for agents; hook registration |
| `hooks/format.sh` | after an agent edits a file: gofumpt for Go, Prettier for web files |
| `hooks/check-on-stop.sh` | before an agent finishes: `make check-fast` for the areas it changed; failures send it back to work |
| `agents/` | role agents: `go-implementer`, `web-implementer`, `reviewer` (read-only), `verifier` |
| `skills/` | `/slice` (issue → pull request), `/verify` (checks), `/spec` (contract change), `/adr` (decision record) |

Local-only and gitignored: `settings.local.json`, `worktrees/` (agent view worktrees).

## `.github/`

| Path | Contents |
|---|---|
| `ISSUE_TEMPLATE/task.md` | spec, acceptance criteria, out of scope, `depends_on` / `parallel` / `conflicts_with` |
| `pull_request_template.md` | linked issue, what changed, how it was tested, metrics |
| `workflows/ci.yml` | CI on every pull request and push to `main`: jobs `api`, `web`, `spec`, `e2e` run the same `make` targets as local development; `dependency-review` (pull requests only) fails when a pull request adds or changes a dependency with a known vulnerability, in any ecosystem. All five are required to merge. |
| `workflows/claude.yml` | `@claude` in issues and pull requests (Level 3, see `docs/agentic-workflow.md`) |
| `workflows/claude-review.yml` | automatic pull request review with the `code-review` plugin (not a required check) |
| `dependabot.yml` | weekly grouped dependency updates per ecosystem and directory, 14-day cooldown, `chore(deps)` commits |

The `spec` job fails on a breaking contract change (`make check-breaking`, oasdiff) unless the pull request has the `api:breaking` label; with the label the report is still shown. Adding the label does not start a new run: re-run the `spec` job afterwards. Breaking is fine while nothing outside this repository uses the API, but it should always be a deliberate choice.

## `e2e/` — end-to-end tests

| Path | Contents |
|---|---|
| `playwright.config.ts` | desktop and mobile (Pixel 7) projects; `E2E_BASE_URL` (web) and `E2E_API_URL` (API), which `make e2e` reads from the running stack |
| `tests/smoke.spec.ts` | the stack starts and its parts are wired together; replaced by user journeys from M1 |
| `package.json` | pins Playwright; `npm run typecheck` |

Setup once: `cd e2e && npm ci && npx playwright install chromium`. Then `make e2e` builds and starts the `app` profile and runs the tests; the stack keeps running until `make down`.

## Makefile targets

Run `make help` for the list. Targets for parts not built yet say which issue adds them and succeed.

| Target | What it does |
|---|---|
| `make dev` | start PostgreSQL and Mailpit |
| `make down` | stop the stack, keep data |
| `make reset-db` | delete local database data and recreate the roles |
| `make migrate` | apply pending migrations as the owner role (`MIGRATION_DATABASE_URL`) |
| `make fmt` | format Go code |
| `make check-fast AREAS="api web spec"` | lint and unit tests for the given areas (the stop hook passes only changed ones) |
| `make check` | everything CI runs except end-to-end tests: all areas, vulnerability scan, web production build |
| `make gen` | regenerate Go server code and TypeScript types from the contract, Go query code from SQL, and mocks from the `contracts.go` interfaces |
| `make check-spec` | lint the contract and fail if generated code is out of date |
| `make test-integration` | API tests against a real PostgreSQL in Docker (testcontainers) |
| `make check-breaking BASE=origin/main` | fail on contract changes that break existing clients (oasdiff), compared to `BASE` |
| `make vuln` | known vulnerabilities in the API and its dependencies (govulncheck) |
| `make e2e` | build and start the full stack, then run Playwright |
| `make seed` | sample organizations and users (M1) |
