# 0008. PostgreSQL row-level security as a second line of tenant isolation

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
In a shared database, one missing `WHERE organization_id = ...` leaks one company's data to another. Application checks and tests catch most mistakes, but code is increasingly written by agents and reviewed by one person.

## Decision
Isolation is enforced twice:
1. **Application:** repository functions for organization data require the organization ID; the organization context exists only after the membership check.
2. **Database:** organization-scoped tables use `ENABLE` and `FORCE ROW LEVEL SECURITY` with policies on `current_setting('app.org_id')` (and `app.user_id` where a user reads their own rows across organizations).

Each request that touches organization data runs in a transaction that starts with `SET LOCAL app.user_id = ...` and `SET LOCAL app.org_id = ...` (a unit of work). The API connects as an `app` role that does not own the tables and has no `BYPASSRLS`; migrations run as a separate owner role.

Integration tests prove that a query without the setting returns nothing and that one organization cannot read or write another's rows.

### Which tables (2026-11-25)
- **Row-level security:** organization data, i.e. `organizations`, `memberships`, `invitations` (M2) and every later business table.
- **No row-level security:** global identity tables, i.e. `users`, `sessions`, `account_tokens`, `login_history`, `outbox_emails`. They belong to no organization and are read before any scope exists (a user by email at sign-in, a session by token hash). Access is guarded by the application and by storing only token hashes. An organization admin's view of login history joins `memberships`, so its policy still limits the rows.
- Every query, on either kind of table, still runs inside the unit of work (`repo.Transactor.InScope`, ADR 0003): repositories refuse to run without one, with an empty scope where none applies.
- Invitations are looked up by token before sign-in (M2). That will add a token-hash setting to the scope and a policy clause on it, so holding the token is what grants the read; no `SECURITY DEFINER` function is needed (and with `FORCE`, one owned by the owner role would be filtered too).

### Spike result (2026-11-24, issue #6)
Proven against PostgreSQL 18.6 with the same role script as development (`dev/postgres-init/01-roles.sh`):
- The unit of work (`persistent.Store.InScope` at the time, now `repo.Transactor`, see ADR 0003) starts each transaction with `set_config('app.user_id', …, true)` and `set_config('app.org_id', …, true)` — `SET LOCAL` with bind parameters. Policies read them through `app_user_id()` / `app_org_id()`, which turn an unset value into `NULL`.
- Without a scope, the app role sees **0** organizations and memberships.
- Scoped to one organization, even an explicit query for another organization returns nothing; inserting a row for another organization fails with `42501`.
- A user scope lists only that user's own memberships across organizations ("my organizations").
- On a single pooled connection, the settings are gone after commit: nothing leaks to the next request.
- The app role cannot turn `row_security` off or create tables; with `FORCE`, even the owner role sees no rows without a scope.
- Consequence of `FORCE`: seeding, bootstrap and the `create-org` CLI write through the unit of work with a scope, like the API, instead of bypassing the policies.

## Alternatives considered
- **Application checks only:** simpler, but one forgotten filter is a data leak.
- **Database per tenant:** strongest isolation, too heavy for many small organizations.

## Consequences
- A forgotten filter returns no rows instead of another tenant's rows.
- Every organization query must run inside the unit of work; queries outside it see nothing.
- `SET LOCAL` only lasts for the transaction, which works with transaction-mode connection poolers. This is verified against the production pooler before deployment.
- Two database roles to manage.
