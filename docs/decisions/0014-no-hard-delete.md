# 0014. Deactivate memberships instead of deleting

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Carried over from employee-portal: deleting people destroys their login history and audit trail. With organizations, a person may be removed from one company but stay in another.

## Decision
Admins deactivate a **membership**, not a user. A deactivated member loses access to that organization at once and can be reactivated. Their account, sessions for other organizations and history remain. Admins cannot deactivate themselves or change their own role, and the last owner cannot be removed.

## Alternatives considered
- **Hard delete:** loses history and breaks references.
- **Deactivate the whole user:** would lock them out of unrelated organizations.

## Consequences
- History is preserved.
- Deleting personal data on request (for example under privacy laws) is a separate, later feature.
