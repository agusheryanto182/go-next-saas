# 0012. Invite-only onboarding and password resets

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
As in employee-portal, people do not register themselves: an admin adds them. With many organizations, one person may already have an account from another organization. Organizations themselves have to come from somewhere without a sign-up page or a super admin page.

## Decision
**Organizations**
- Development: `make seed` creates two organizations with owners, admins, members and one user in both, to exercise isolation.
- Production: an idempotent bootstrap from environment variables creates the first organization and owner.
- Further organizations: a CLI command in the API binary (`create-org --name --slug --owner-email`) run over SSH, which invites the owner.

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
- No password is ever known to anyone but its owner.
- Accepting an invitation proves the email address, so no separate verification step is needed.
- A super admin page and self sign-up are later additions that do not change this model.
