# 0003. Clean architecture with the go-clean-template layout

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
The API will grow over many domains and much of it will be written by coding agents. Without enforced boundaries, HTTP details leak into business logic and queries spread everywhere.

[evrone/go-clean-template](https://github.com/evrone/go-clean-template) is a widely used, maintained layout for clean architecture in Go. Its libraries (Fiber, Squirrel, Swaggo, JWT, message brokers) do not match our other decisions.

## Decision
Follow the template's **layout and rules**, without cloning it:

```
api/
├─ cmd/app/                  entry point
├─ config/                   environment configuration
├─ internal/app/             wiring (manual DI), start and shutdown
├─ internal/controller/restapi/{middleware,v1}
├─ internal/entity/          business models
├─ internal/usecase/         contracts.go + one folder per domain
├─ internal/repo/            contracts.go + persistent/ (PostgreSQL) + webapi/ (SMTP and other services)
└─ pkg/                      generic infrastructure, no business logic
```

Rules:
- Dependencies point inward: controller → usecase → repo. Use cases import only the standard library, `entity` and contracts.
- Layers talk through interfaces in `contracts.go`; dependencies are passed through constructors.
- Data crosses layers as `entity` types.
- Layer direction is enforced by `depguard` in golangci-lint, so a violation fails CI.

## Alternatives considered
- **Package by feature (as in employee-portal):** fine for a small app; less explicit boundaries as domains multiply.
- **Clone the template:** most of its libraries would be removed again, leaving leftovers that confuse agents.
- **Flat `internal/` packages:** simplest, but no protection against layer leaks.

## Consequences
- Business logic is unit-testable with mocks (`go.uber.org/mock`).
- More files and interfaces than a flat layout; the template's own advice applies: add layers only when needed.
