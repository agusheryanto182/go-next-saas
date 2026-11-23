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

## Status

Milestone 0 (foundation) is in progress. Decisions are recorded in [`docs/decisions`](docs/decisions).

## License

[MIT](LICENSE)
