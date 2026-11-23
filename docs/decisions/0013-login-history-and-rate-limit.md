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

## Alternatives considered
- **In-memory limiter:** per instance, lost on restart.
- **Redis:** a new service for one feature.

## Consequences
- Security-relevant history for users and admins.
- One indexed count query per login attempt.
- History grows forever; a retention policy can come later.
