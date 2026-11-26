---
name: review-strict
description: Deep security review of a pull request labelled review:strict (auth, sessions, passwords, tenancy, data migrations) against a written checklist, posted as one pull request comment. Use with a pull request number, e.g. "/review-strict 55". Does not mark the pull request ready or merge it.
---

# Strict review

For pull requests whose issue is labelled `review:strict`. The checklist below is the review: it does
not depend on what any one session remembers, so every session reviews to the same standard.

Input: a pull request number.

## 1. Read
- `gh pr view <n>` (body, Review guide), `gh issue view <issue>`, and the ADRs the issue names.
- The diff: `gh pr diff <n>`. Read the code the Review guide points to first, then every non-generated file. Skip `gen/`, `mocks_*_test.go` and lock files.
- The tests that claim each acceptance criterion. Check that each one asserts the criterion, not just that it runs.

## 2. Checklist
Mark each item as pass, finding or not applicable. Every finding names `file:line`, says what goes wrong (input, then wrong result) and proposes a fix.

**Secrets and personal data**
- Passwords, tokens, token hashes, reset links and cookies never reach logs, span attributes, error messages, problem details or `last_error`. Look for a test that proves it.
- Only hashes of tokens are stored; tokens have at least 256 bits from `crypto/rand`.
- Emails and names are not logged; IDs are fine.

**Authentication and sessions (ADR 0006, 0011, 0013)**
- Cookie: `__Host-session`, `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`, no `Domain`; cleared with `Max-Age=0`.
- A new token is issued at sign-in, and an existing session cookie is revoked (session fixation).
- Every sign-in failure looks the same: same status, same body, comparable time (dummy hash for unknown and pending accounts).
- Rate limits count what the ADR says, by the client IP from the trusted-proxy middleware, never from a raw header.
- Password changes and resets revoke the sessions the ADR says, in the same unit of work.

**Authorization and tenancy (ADR 0007, 0008)**
- Organization data is read and written only inside `InScope`, with a scope that comes from the session and the membership, never from request input.
- A user ID taken from the request is checked against the session user.
- Row-level security is not bypassed; no query runs as the owner role from application code.
- A test shows that a user of organization A cannot read or change organization B's data, if the change touches organization data.

**Input and output**
- Input is validated by the contract (lengths, formats, `additionalProperties: false`); handlers do not trust anything else.
- Errors are RFC 9457 problems; `detail` never echoes a submitted value or leaks internals (SQL, stack, file paths).
- Responses carrying personal data send `Cache-Control: no-store`.

**Database**
- Queries are sqlc only; no SQL is built from strings.
- Races: single-use and limited actions (token use, refresh, revoke) are decided by one conditional `UPDATE`, not read-then-write.
- Migrations have `lock_timeout`/`statement_timeout`, are safe on a non-empty table where relevant, and the down migration works.

**Configuration and dependencies**
- Secure defaults: an empty or missing setting does not open anything (for example `TRUSTED_PROXIES` empty trusts nobody).
- New dependencies are pinned, at least 14 days old, and needed.

## 3. Report
Post one comment on the pull request starting with `<!-- orchestrate:review-strict -->`:
```
<!-- orchestrate:review-strict -->
## Strict review
**Verdict:** no blocking findings | blocking findings (fix before merge)

**Findings** (blocking first): file:line, failure scenario, proposed fix
**Checked and passing:** one line per checklist area
**Notes for later issues:** what belongs in another issue, and which one
```
Then:
- Put each "note for later issues" into that issue's body as an acceptance criterion (`gh issue edit <n> --body-file …`), marked "(strict review of #<pr>)". Create an issue if none fits. `/slice` does not read comments.
- Do not mark the pull request ready and do not merge it. Tell the developer the verdict, and that the decision is theirs.
