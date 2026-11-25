# 0006. Server-side sessions in an HttpOnly cookie instead of JWTs

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
employee-portal kept a 15-minute access JWT in browser memory and a rotating refresh token in a cookie. With Next.js the server renders pages too, and it cannot see a token that lives only in browser memory. There is one backend, so JWT's main strength, verification without a database lookup across many services, brings nothing.

OWASP's session management guidance recommends random session IDs in `HttpOnly`, `Secure`, `SameSite` cookies, renewed on login and revocable on the server. The "backend for frontend" pattern for browser apps keeps every token out of JavaScript.

## Decision
- The Go API creates a session row in PostgreSQL and sets `__Host-session` (`HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, no `Domain`).
- The session token is 256 random bits; only its SHA-256 hash is stored.
- A new session is created on every login (no session fixation). Changing or resetting a password revokes sessions. Deactivating a membership does not: access to that organization ends at once through the per-request membership check, and sessions for the user's other organizations stay valid (ADR 0014).
- Idle timeout of 7 days, extended at most once an hour; absolute lifetime of 30 days.
- The browser calls `/api` on the same origin, so the cookie is sent automatically. During server rendering, Next.js forwards the cookie (and the client IP) to the API.
- Next.js only checks whether the cookie exists, for redirects. Authorization always happens in Go.
- Users can list their sessions and sign out other devices.

### Implementation details (M1 design, 2026-11-25)
- **Table `sessions`** (global identity data, no row-level security, ADR 0008): `id` (UUIDv7, the public handle for listing and revoking; never the token), `user_id`, `token_hash` (32-byte SHA-256, unique), `created_at`, `last_seen_at`, `idle_expires_at`, `absolute_expires_at`, `revoked_at`, `revoked_reason`, `ip` (`inet`), `user_agent` (at most 512 characters). Partial index on `user_id` for active sessions.
- **Token:** 32 bytes from `crypto/rand`, base64url without padding. A plain SHA-256 is enough: 256 bits of entropy cannot be brute-forced from a leaked hash, so no salt or pepper.
- **Cookie:** persistent, `Max-Age` = remaining absolute lifetime; no "remember me" option. The idle limit is enforced on the server.
- **Login** with a still-valid session cookie revokes that session before creating the new one.
- **Per request:** one lookup by token hash that also checks revocation and both expiries. The idle expiry is extended with `UPDATE … WHERE last_seen_at < now() - interval '1 hour'`, so at most one write an hour and safe under concurrent requests. The rule lives on the entity (`Session.NeedsRefresh(now)`); use cases take a clock through `New` for tests.
- **Which endpoints need a session is defined by the contract:** a middleware resolves the cookie (if any) into the request context; the OpenAPI validator's `AuthenticationFunc` rejects operations with `security: session` when the context has no session (401 problem). There is no second list of protected routes in Go.
- **Revocation reasons:** `logout` (this session), `password_changed` (all others), `password_reset` (all), `signed_out_elsewhere` (one or all others, from "my devices").
- **Retention:** expired or revoked sessions stay visible in "my devices" and are purged 90 days after they ended by a daily job in the worker loop (technical data, not covered by ADR 0014). Permanent sign-in records are in `login_history` (ADR 0013).
- **Client IP:** taken from `X-Forwarded-For` only when the request comes from a trusted proxy (`TRUSTED_PROXIES`, CIDRs: Caddy in production, Next.js during server rendering), using the rightmost address that is not a trusted proxy. Rate limiting (ADR 0013) uses the same value.
- **Secure cookies in development:** browsers treat `http://localhost` as a secure context, so `__Host-session` works locally without turning `Secure` off. End-to-end tests must use `localhost`, not `0.0.0.0` or another host.

## Alternatives considered
- **Keep JWT + refresh cookie:** server rendering cannot authenticate, and revocation waits for token expiry.
- **Next.js holds the Go tokens in an encrypted cookie:** refresh logic and an encryption key move into Next.js; authentication lives in two places.

## Consequences
- Simpler clients: no token refresh, no retry on 401.
- Revocation takes effect immediately.
- One indexed lookup per request.
- CSRF protection is needed for cookie-authenticated writes ([0015](0015-csrf-origin-checks.md)).
- A mobile app or other services can later use the same session token as a bearer token, or receive their own tokens from the API.
