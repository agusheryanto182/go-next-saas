# 0006. Server-side sessions in an HttpOnly cookie instead of JWTs

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
employee-portal kept a 15-minute access JWT in browser memory and a rotating refresh token in a cookie. With Next.js the server renders pages too, and it cannot see a token that lives only in browser memory. There is one backend, so JWT's main strength, verification without a database lookup across many services, brings nothing.

OWASP's session management guidance recommends random session IDs in `HttpOnly`, `Secure`, `SameSite` cookies, renewed on login and revocable on the server. The "backend for frontend" pattern for browser apps keeps every token out of JavaScript.

## Decision
- The Go API creates a session row in PostgreSQL and sets `__Host-session` (`HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, no `Domain`).
- The session token is 256 random bits; only its SHA-256 hash is stored.
- A new session is created on every login (no session fixation). Changing or resetting a password and deactivation revoke sessions.
- Idle timeout of 7 days, extended at most once an hour; absolute lifetime of 30 days.
- The browser calls `/api` on the same origin, so the cookie is sent automatically. During server rendering, Next.js forwards the cookie (and the client IP) to the API.
- Next.js only checks whether the cookie exists, for redirects. Authorization always happens in Go.
- Users can list their sessions and sign out other devices.

## Alternatives considered
- **Keep JWT + refresh cookie:** server rendering cannot authenticate, and revocation waits for token expiry.
- **Next.js holds the Go tokens in an encrypted cookie:** refresh logic and an encryption key move into Next.js; authentication lives in two places.

## Consequences
- Simpler clients: no token refresh, no retry on 401.
- Revocation takes effect immediately.
- One indexed lookup per request.
- CSRF protection is needed for cookie-authenticated writes ([0015](0015-csrf-origin-checks.md)).
- A mobile app or other services can later use the same session token as a bearer token, or receive their own tokens from the API.
