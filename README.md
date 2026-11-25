# go-next-saas

A multi-tenant SaaS foundation with a Go API and a Next.js web app, built with an agentic coding workflow.

It continues [employee-portal](https://github.com/agusheryanto182/employee-portal) (Spring Boot + React): the same invite-only onboarding, login history and backoffice, rebuilt for many organizations.

## Stack

| Part | Choice |
|---|---|
| API | Go, `net/http`, spec-first OpenAPI (`oapi-codegen`), `pgx` + `sqlc`, `goose` |
| Web | Next.js (App Router), Tailwind CSS, shadcn/ui, TanStack Query |
| Data | PostgreSQL with row-level security per organization |
| Auth | Server-side sessions in an HttpOnly cookie |

## Getting started

**Requirements:** Docker Engine with Compose v2, GNU Make, Go 1.27 (newer toolchains download automatically), Node.js 26, and `jq` (used by the agent hooks).

```bash
git clone git@github.com:agusheryanto182/go-next-saas.git
cd go-next-saas
cp .env.example .env
```

**Everything in Docker**

```bash
docker compose --profile app up --build
```

| What | URL |
|---|---|
| Web app | http://localhost:3000 |
| API health | http://localhost:8080/healthz |
| Mailpit inbox (local emails) | http://localhost:8025 |

**Day to day: dependencies in Docker, code on your machine**

```bash
make dev                                              # PostgreSQL + Mailpit
set -a; . ./.env; set +a                              # load .env into this shell
(cd api && go run ./cmd/app)                          # API on :8080
(cd web && npm ci && npm run dev)                     # web on :3000, /api forwarded to the API
```

If a port is taken (for example by another project's PostgreSQL), change `POSTGRES_PORT`, `MAILPIT_*_PORT`, `API_PORT` or `WEB_PORT` in `.env` and the matching URLs.

**Development accounts**

`docker compose --profile app up` creates them automatically; with the day-to-day setup, run `make migrate` and `make seed` (both are safe to repeat). Seeding only runs against a local database. Every account's password is **`dev-password`**.

| Email | Acme (`acme`) | Globex (`globex`) |
|---|---|---|
| alice@acme.test | owner | |
| dina@acme.test | admin | |
| edo@acme.test | member | |
| fajar@acme.test | member, deactivated | |
| bob@globex.test | | owner |
| carol@example.test | member | member |

**Creating an organization (production)**

There is no sign-up. An operator creates each organization and its owner with the API binary, as the app role (`DATABASE_URL`); the password is read from standard input without echo, so it stays out of the shell history and the environment (ADR 0012):

```bash
app create-org -name "Acme" -slug acme -owner-email you@example.com -owner-name "Your Name" -owner-password-stdin
```

An existing account with that email is reused and keeps its password; an existing slug changes nothing.

**Checks**

```bash
make check-fast   # lint and unit tests
make check        # everything CI runs except end-to-end tests
make e2e          # full stack + Playwright (once: cd e2e && npm ci && npx playwright install chromium)
make help         # all targets
```

## Documentation

- [Project structure](docs/project-structure.md): what every folder and file is for
- [Architecture](docs/architecture.md): system, layers, request flow, tenancy
- [Agentic workflow](docs/agentic-workflow.md): how work goes from issue to merged pull request
- [Decision records](docs/README.md#decision-records)
- API reference: `make docs` builds it from [`openapi/openapi.yaml`](openapi/openapi.yaml) into `.cache/docs/api.html`

## Status

Milestone 0 (foundation) is in progress.

## License

[MIT](LICENSE)
