# 0018. Next.js App Router with the API as the only backend

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Next.js can act as a backend (server actions, route handlers), but the Go API is the single place for business rules and authorization, and other clients will use the same API.

## Decision
- App Router. Server components load the data a layout needs (current user, membership) by calling the Go API with the forwarded cookie.
- Interactive lists and every write use TanStack Query with `openapi-fetch` against same-origin `/api`.
- No server actions and no business logic in Next.js.
- `proxy.ts` only redirects when the session cookie is missing.
- UI: Tailwind CSS, shadcn/ui (sidebar, data tables with TanStack Table, charts), dark mode, a command palette.

## Alternatives considered
- **Server actions for writes:** a second path into the backend outside the contract.
- **Keep the Vite SPA:** no server rendering for future public pages.

## Consequences
- One contract serves the web app, tests and future clients.
- Some data is fetched on the server and some on the client; the rule above decides which.
