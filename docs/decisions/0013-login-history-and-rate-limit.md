# 0013. Login history records failures and drives the rate limit

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Carried over from employee-portal: users and admins should see every sign-in attempt, including failures, while attackers learn nothing from error messages. employee-portal kept its rate limit in memory, which breaks with more than one API instance.

## Decision
- Every attempt writes a `login_history` row: user (if known), attempted email, success, failure reason (`INVALID_CREDENTIALS`, `ACCOUNT_PENDING`, `RATE_LIMITED`), IP, user agent, time. The row is written even when the request fails.
- Clients always get the same message: "Invalid email or password."
- Users see their own history. Organization admins see the history of their members.
- **Rate limit:** count failed attempts for the same IP and email in the last 15 minutes from `login_history`; after 5, answer 429 with `Retry-After`. No extra store is needed and it works across instances.

### Implementation details (M1 design, 2026-11-25)
- **Sign-in order:** normalize the email (trim, lowercase) → rate limit → read the user (unit of work) → verify the password outside any transaction (a dummy hash for an unknown email or a pending account) → in a second unit of work, revoke the session from a still-valid cookie, create the new session, record the success and rehash if needed. A failure is recorded in its own unit of work, so it is kept.
- **Responses:** every failure is 401 "Invalid email or password."; only the rate limit answers 429, which reveals nothing about whether the email exists.
- **Two limits**, counting only `INVALID_CREDENTIALS` and `ACCOUNT_PENDING` (attempts already refused as `RATE_LIMITED` do not extend the window):
  - same IP and email: 5 failures in 15 minutes (brute force on one account);
  - same IP, any email: 30 failures in 15 minutes (credential stuffing).
  There is no per-email limit across all IPs: anyone could use it to lock a victim out. `Retry-After` is when the oldest counted failure leaves the window. The IP comes from the trusted-proxy rule in ADR 0006.
- **Table `login_history`** (global identity data, no row-level security, ADR 0008): `id`, `user_id` (nullable), `email_attempted`, `success`, `failure_reason` (checked against the three values), `ip`, `user_agent`, `created_at`. Indexes: `(user_id, created_at DESC)`; `(email_attempted, ip, created_at) WHERE NOT success`; `(ip, created_at) WHERE NOT success`. An organization admin's view joins `memberships`, so it only shows that organization's members; attempts on unknown emails belong to no organization.
- A user without any active membership can still sign in (the account is global and may accept a new invitation); the organization picker says they have no organizations.
- **Sign-out** revokes the current session and always clears the cookie (`Max-Age=0`), even when the session is no longer valid. Sign-in is a state-changing request and goes through the cross-origin check (ADR 0015).
- **Retention:** rows are purged after 365 days by the daily job that also purges sessions (ADR 0006). The table holds personal data (email, IP), so it does not grow forever.

## Alternatives considered
- **In-memory limiter:** per instance, lost on restart.
- **Redis:** a new service for one feature.

## Consequences
- Security-relevant history for users and admins.
- One indexed count query per login attempt.
- History is kept for 365 days.
