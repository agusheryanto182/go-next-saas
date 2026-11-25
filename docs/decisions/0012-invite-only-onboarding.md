# 0012. Invite-only onboarding and password resets

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
As in employee-portal, people do not register themselves: an admin adds them. With many organizations, one person may already have an account from another organization. Organizations themselves have to come from somewhere without a sign-up page or a super admin page.

## Decision
**Organizations**
- Development: `make seed` creates two organizations with owners, admins, members and one user in both, to exercise isolation.
- Production, including the first organization: the CLI command `app create-org -name -slug -owner-email -owner-name`, run over SSH. It invites the owner by email (M2). Until invitations exist, and as a fallback without email, `-owner-password-stdin` reads the operator's own password from standard input (no echo on a terminal), so it never appears in the environment, shell history or logs; it is meant for the operator's own account, not for setting someone else's password.

### Implementation details (M1 design, 2026-11-25)
- Seed and `create-org` call the same use case as the API (`organization.Create`), through the unit of work, as the **app** role (`DATABASE_URL`). With `FORCE ROW LEVEL SECURITY` the owner role has no advantage, so the least-privileged role is used.
- Idempotent by insert, not by lookup: a slug that already exists (`ErrOrganizationSlugTaken`) means the organization is there, and the run succeeds without changes. An existing user with the owner's email is reused; their password is not touched.
- The binary uses subcommands: `app` (serve), `app migrate`, `app healthcheck`, `app seed`, `app create-org`.
- Seed data: Acme (`acme`): alice owner, dina admin, edo member, fajar deactivated member; Globex (`globex`): bob owner; carol is a member of both. One development password for all, documented in the README. `seed` refuses to run unless the database host is `localhost`, `127.0.0.1`, `::1` or `postgres` (the Compose service), so it cannot touch a hosted database. A one-shot `seed` service in the Compose `app` profile runs after `migrate`, so end-to-end tests always have these accounts.
- The environment bootstrap (`BOOTSTRAP_*`) from the first version of this record is dropped: it kept the owner's password in the production environment and duplicated the CLI.

### Password reset details (M1 design, 2026-11-25)
- **Table `account_tokens`** (global, no row-level security): `id`, `user_id`, `type` (`password_reset`), `token_hash` (unique), `expires_at` (1 hour), `used_at`, `invalidated_at`, `created_at`.
- **Forgot password** always answers 204. A token and its email are created only for an existing user who has a password (pending accounts accept their invitation instead). The new token invalidates earlier unused ones; token, invalidation and outbox email are one transaction. At most 3 reset emails per user per hour; beyond that the answer is still 204 and nothing is sent.
- **The link carries the token in the fragment** (`/reset-password#token=…`), which browsers never send to a server, so it stays out of proxy and server logs; the page sets `Referrer-Policy: no-referrer`. The page checks a token with `POST /auth/tokens/inspect` (token in the body), replacing the planned `GET /auth/tokens/{token}`, which would put it in access logs.
- **Reset:** hash the new password first (outside any transaction), then one transaction consumes the token atomically (`UPDATE … WHERE used_at IS NULL AND invalidated_at IS NULL AND expires_at > now() RETURNING user_id`), sets the password, revokes every session (`password_reset`) and enqueues a "password changed" email. No automatic sign-in. An invalid, expired or used token gets one generic message: "This link is invalid or has expired."
- **Admin-triggered reset** uses the same use case and limit; the target must be an active member of the admin's organization (checked through the row-level-secured membership), and the caller an admin or owner. The response never contains the link.
- A "password changed" email is also sent after a change from the account page (M3).

**Members**
- An organization admin invites an email with a role. An invitation is a one-time token (only its SHA-256 hash is stored) valid for 7 days, emailed as a link. The admin also gets a "copy link" fallback.
- A new person sets their name and password when accepting. Someone with an account accepts and gets a membership.
- Admins can edit or revoke pending invitations. Once accepted, the profile belongs to the user, because it is shared with other organizations.

**Password resets**
- Self-service "forgot password" on the sign-in page.
- An admin can trigger the same reset email for a member. The link is never shown to the admin.
- Reset tokens are one-time, hashed and valid for 1 hour; a new token invalidates earlier ones.

## Alternatives considered
- **Self sign-up:** kept in the backlog until the product needs it.
- **Copy link for password resets (as in employee-portal):** an admin of one organization could take over an account that also belongs to another organization.
- **Admins edit any member's profile:** would change an identity other organizations rely on.

## Consequences
- No password is ever known to anyone but its owner (development seed accounts excepted).
- Accepting an invitation proves the email address, so no separate verification step is needed.
- A super admin page and self sign-up are later additions that do not change this model.
