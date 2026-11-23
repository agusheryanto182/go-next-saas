# 0015. CSRF protection with SameSite cookies and Origin checks

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
With cookie sessions ([0006](0006-server-side-sessions.md)), a malicious site could try to make the browser send state-changing requests with the user's cookie.

## Decision
- The session cookie is `SameSite=Lax`.
- The API wraps state-changing routes with Go's `http.CrossOriginProtection`, which rejects cross-origin requests based on `Sec-Fetch-Site` and `Origin`.
- `GET` requests never change state.

## Alternatives considered
- **Synchronizer or double-submit CSRF tokens:** extra state and client code; modern browsers send the headers this check relies on.

## Consequences
- No CSRF tokens in forms or clients.
- Very old browsers without these headers are not protected by the header check; `SameSite=Lax` still applies.
