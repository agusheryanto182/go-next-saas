# Project structure

What every folder and important file is for. Update this page in the same pull request that adds, moves or removes one.

Files marked **generated** are produced by a tool; change their source and regenerate instead of editing them.

## Root

```
go-next-saas/
├─ api/                  Go API (its own module)
├─ web/                  Next.js web app
├─ docs/                 documentation and decision records
├─ dev/                  files used only by the local development stack
├─ .claude/              Claude Code rules, roles, skills and hooks
├─ .github/              issue and pull request templates (CI workflows arrive with #8)
├─ AGENTS.md             rules for every coding agent (any tool)
├─ CLAUDE.md             Claude Code entry point; imports AGENTS.md
├─ Makefile              one entry point for people, agents and CI
├─ compose.yml           local stack: PostgreSQL, Mailpit; profile `app` adds the API and web app
├─ .env.example          every environment variable, with local defaults; copy to `.env`
├─ .worktreeinclude      gitignored files copied into agent worktrees (`.env`)
├─ .editorconfig         indentation and line endings for all editors
├─ .gitignore
├─ LICENSE               MIT
└─ README.md             overview and getting started
```

Coming with later issues: `openapi/` (API contract, #5), `e2e/` (Playwright, #8), `deploy/` (production config, M5).

## `api/` — Go API

Clean architecture with the [go-clean-template](https://github.com/evrone/go-clean-template) layout ([ADR 0003](decisions/0003-clean-architecture-layout.md)). Dependencies point inward: controller → usecase → repo. Lint enforces it.

```
api/
├─ cmd/app/main.go                       entry point: load config, start the app; `-healthcheck` for containers
├─ config/config.go                      environment variables → validated Config
├─ internal/
│  ├─ app/app.go                         wiring (manual dependency injection), start-up and graceful shutdown
│  ├─ controller/restapi/                HTTP layer
│  │  ├─ router.go                       routes, /healthz and /readyz, 404/405 as problem details
│  │  ├─ middleware/                     shared middleware (panic recovery; sessions and CSRF later)
│  │  └─ problem/                        RFC 9457 error responses
│  ├─ entity/                            business models used by every layer
│  ├─ usecase/contracts.go               use-case interfaces; one sub-folder per domain will follow
│  └─ repo/contracts.go                  storage and external-service interfaces;
│                                        persistent/ (PostgreSQL) and webapi/ (SMTP) will follow
├─ pkg/                                  generic infrastructure, no application imports
│  ├─ httpserver/                        http.Server with timeouts and shutdown
│  ├─ logger/                            slog JSON logger with trace IDs
│  ├─ otel/                              OpenTelemetry setup
│  └─ postgres/                          pgx connection pool
├─ tools/go.mod                          pinned dev tools: golangci-lint, gofumpt, govulncheck
├─ .golangci.yml                         lint rules, including the layer rules (depguard)
├─ Dockerfile                            multi-stage build → distroless, non-root image
├─ AGENTS.md / CLAUDE.md                 API rules for agents
└─ go.mod / go.sum                       module and pinned dependencies
```

Coming next: `migrations/` and `queries/` (SQL, #6), `internal/repo/persistent/gen/` and `internal/controller/restapi/v1/gen/` (**generated**, #5 and #6).

Tests sit next to the code as `*_test.go`.

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

**Generated:** `next-env.d.ts` and `.next/` (by Next.js, gitignored); `src/lib/api/schema.d.ts` (from the OpenAPI contract, #5).

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

## Makefile targets

Run `make help` for the list. Targets for parts not built yet say which issue adds them and succeed.

| Target | What it does |
|---|---|
| `make dev` | start PostgreSQL and Mailpit |
| `make down` | stop the stack, keep data |
| `make reset-db` | delete local database data and recreate the roles |
| `make fmt` | format Go code |
| `make check-fast AREAS="api web spec"` | lint and unit tests for the given areas (the stop hook passes only changed ones) |
| `make check` | everything CI runs except end-to-end tests: all areas, vulnerability scan, web production build |
| `make gen` | regenerate code from the contract and SQL (#5, #6) |
| `make test-integration` | API tests against a real PostgreSQL (#6) |
| `make e2e` | full stack + Playwright (#8) |
| `make seed` | sample organizations and users (M1) |
