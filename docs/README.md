# Documentation

What is being built, how, and why. Docs change in the same pull request as the decision or code they describe.

| Document | Contents |
|---|---|
| [architecture.md](architecture.md) | System overview, layers, request flow, tenancy |
| [project-structure.md](project-structure.md) | What every folder and important file is for, Makefile targets |
| [agentic-workflow.md](agentic-workflow.md) | How work moves from issue to merged pull request with coding agents |
| [decisions/](decisions/) | Architecture Decision Records |

Database, authentication flows, testing and deployment docs are added with the milestones that build them.

## Decision records

| # | Decision |
|---|---|
| [0001](decisions/0001-origin-and-stack.md) | Rewrite employee-portal as a Go API and a Next.js web app |
| [0002](decisions/0002-monorepo-layout.md) | One repository with api/, web/, openapi/ and e2e/ |
| [0003](decisions/0003-clean-architecture-layout.md) | Clean architecture with the go-clean-template layout |
| [0004](decisions/0004-net-http.md) | Standard library net/http instead of a web framework |
| [0005](decisions/0005-spec-first-openapi.md) | Spec-first OpenAPI contract with generated server and client |
| [0006](decisions/0006-server-side-sessions.md) | Server-side sessions in an HttpOnly cookie instead of JWTs |
| [0007](decisions/0007-multi-org-tenancy.md) | Organizations, memberships and the organization in the URL |
| [0008](decisions/0008-row-level-security.md) | PostgreSQL row-level security as a second line of tenant isolation |
| [0009](decisions/0009-pgx-sqlc-goose-squawk.md) | pgx with sqlc, goose migrations linted by Squawk |
| [0010](decisions/0010-email-outbox.md) | Transactional outbox for emails |
| [0011](decisions/0011-argon2id.md) | argon2id for password hashing |
| [0012](decisions/0012-invite-only-onboarding.md) | Invite-only onboarding and password resets |
| [0013](decisions/0013-login-history-and-rate-limit.md) | Login history records failures and drives the rate limit |
| [0014](decisions/0014-no-hard-delete.md) | Deactivate memberships instead of deleting |
| [0015](decisions/0015-csrf-origin-checks.md) | CSRF protection with SameSite cookies and Origin checks |
| [0016](decisions/0016-pwa-shell-only.md) | PWA: cache the app shell, never the API |
| [0017](decisions/0017-agentic-workflow.md) | Agentic workflow on stable Claude Code features |
| [0018](decisions/0018-nextjs-app-router.md) | Next.js App Router with the API as the only backend |
| [0019](decisions/0019-session-independent-orchestration.md) | Session-independent orchestration of agent work |

New ADRs copy [decisions/template.md](decisions/template.md).
