# Agent guide

Rules for every coding agent working in this repository. Area-specific rules live in `api/AGENTS.md` and `web/AGENTS.md`.

## Project
A multi-tenant SaaS foundation: Go API (`api/`), Next.js web app (`web/`), OpenAPI contract (`openapi/`), Playwright tests (`e2e/`). Read `docs/architecture.md` first; decisions are in `docs/decisions/`.

## Commands
- `make dev` — start PostgreSQL and Mailpit
- `make gen` — regenerate code from `openapi/openapi.yaml` and `api/queries/`
- `make check-fast` — format, lint and unit tests for changed areas
- `make check` — everything CI runs except end-to-end tests
- `make test-integration` — API tests against a real PostgreSQL
- `make e2e` — full stack + Playwright

Run `make check` before saying work is done. If a command is missing, say so instead of working around it.

## Rules
1. **Contract first.** API changes start in `openapi/openapi.yaml`, then `make gen`. Never hand-edit generated code (`api/internal/controller/restapi/v1/gen/`, `api/internal/repo/persistent/gen/`, `web/src/lib/api/schema.d.ts`).
2. **Tenant isolation.** Every query on organization data runs inside the unit of work, which sets `app.user_id` and `app.org_id`. Never bypass row-level security, never connect as the owner role from application code.
3. **Layers.** In `api/`: controller → usecase → repo, with interfaces in `contracts.go`. Use cases import only the standard library, `entity` and contracts. Lint enforces this.
4. **Errors.** API errors are RFC 9457 problem details. Sign-in failures always say "Invalid email or password."
5. **Secrets.** Never read `.env` files, print secrets, or put credentials in code, tests, issues or pull requests.
6. **Scope.** Do what the issue asks. Note anything else you notice in the pull request instead of fixing it.
7. **Tests.** New behaviour comes with tests at the right level (unit, integration, component, end-to-end).
8. **Git.** Work on a branch, never push to `main`. Conventional commit messages. Pull request body says `Closes #n`.
9. **Docs.** A changed decision updates its ADR in the same pull request.
10. **Dependencies.** Use the newest stable release that is at least 14 days old, pinned exactly. Security fixes are the exception: take them as soon as `govulncheck` or an advisory reports them.

## Getting stuck
If the same error comes back three times, stop and report what you tried. Do not disable checks, skip tests or loosen lint rules to get green.
