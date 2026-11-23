# 0007. Organizations, memberships and the organization in the URL

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
The product is meant to become a SaaS for businesses and possibly consumers. Adding a tenant column to every table later is a large, risky migration. An employee portal is already "people who belong to a company"; employee-portal only had one implicit company.

## Decision
- **Users are global** (one account per email). **Organizations** hold the data. **Memberships** link a user to an organization with a role (`owner`, `admin`, `member`) and a status (`active`, `deactivated`).
- A user can belong to many organizations.
- The active organization comes from the URL: `/o/{slug}/...`. Every request that touches organization data checks the caller's membership.
- Every organization-scoped table has `organization_id NOT NULL`.
- An organization keeps at least one owner.
- Login history belongs to the user, because signing in happens before choosing an organization.

## Alternatives considered
- **One organization per user:** simpler, but a consultant, or someone invited by two companies, needs two accounts. Changing this later is expensive.
- **Active organization in the session:** two browser tabs would switch each other's organization.
- **Subdomain per organization:** attractive, but needs wildcard DNS and certificates and complicates cookies. It can be added on top of URL slugs later.
- **Database or schema per tenant:** strong isolation, heavy operations for many small tenants.

## Consequences
- Links are shareable and tabs are independent.
- Every organization query must be scoped; row-level security backs this up ([0008](0008-row-level-security.md)).
- Personal workspaces for consumers can later be organizations of one.
