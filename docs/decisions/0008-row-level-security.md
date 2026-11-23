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

## Alternatives considered
- **Application checks only:** simpler, but one forgotten filter is a data leak.
- **Database per tenant:** strongest isolation, too heavy for many small organizations.

## Consequences
- A forgotten filter returns no rows instead of another tenant's rows.
- Every organization query must run inside the unit of work; queries outside it see nothing.
- `SET LOCAL` only lasts for the transaction, which works with transaction-mode connection poolers. This is verified against the production pooler before deployment.
- Two database roles to manage.
