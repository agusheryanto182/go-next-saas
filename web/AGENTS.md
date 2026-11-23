<!-- BEGIN:nextjs-agent-rules -->

## This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Web agent guide

Applies to `web/`. Root rules in `../AGENTS.md` still apply. The block above is maintained by Next.js; keep it.

## Rules (ADR 0018)
- The Go API owns business rules and authorization. Next.js renders and calls the API; no server actions, no business logic, no database access.
- Data a layout needs: server component with `createServerClient()` (`src/lib/api/server.ts`), which forwards the cookie and client IP. Lists and writes: TanStack Query with `createBrowserClient()` (same-origin `/api`).
- `proxy.ts` may only redirect based on whether the session cookie exists.
- UI: shadcn/ui components in `src/components/ui` (add new ones with `npx shadcn@4.23.0 add <name>`), Tailwind classes, lucide icons, `sonner` toasts. Keep components accessible: labelled inputs, alerts with roles, keyboard reachable.
- Tests: Vitest + Testing Library + MSW (`src/test/server.ts`). Query by role and label. Unhandled requests fail the test.

## Commands (from the repository root)
- `make check-web` — lint, typecheck, unit tests, format check
- `make check` — adds the production build
- In `web/`: `npm run dev` (needs the API on `API_INTERNAL_URL`), `npm test`, `npm run format`
