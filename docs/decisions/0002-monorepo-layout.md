# 0002. One repository with api/, web/, openapi/ and e2e/

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
The API, the web app and the contract between them change together. A single developer works with coding agents that need the whole picture in one checkout.

## Decision
One repository:
- `api/`: the Go module.
- `web/`: the Next.js app.
- `openapi/`: the API contract both sides are generated from.
- `e2e/`: Playwright tests against the full stack.
- `docs/`, `deploy/`, `.claude/`, `.github/` at the root.

There is no `apps/` or `packages/` split and no workspace tool until there is a second app.

## Alternatives considered
- **Separate repositories for API and web:** contract changes need coordinated pull requests in two places.
- **A full monorepo toolchain (Turborepo, Nx):** useful with several apps and shared packages, which do not exist yet.

## Consequences
- A single pull request can change the contract, the API and the web app together.
- CI runs per area, so a web-only change does not wait for API tests longer than needed.
- If a marketing site or another app appears, the layout moves to `apps/`.
