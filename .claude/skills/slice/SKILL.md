---
name: slice
description: Implement one GitHub issue end to end on its own branch and open a draft pull request. Use with an issue number, e.g. "/slice 42", typically in a background session.
---

# Slice an issue

Input: an issue number.

1. `gh issue view <n>`. Read the spec, acceptance criteria and the `depends_on` line. If a dependency is still open, stop and report it.
2. Make sure you are on a fresh branch from the latest `main`, named `<type>/<n>-<short-slug>` (`feat/`, `fix/`, `chore/`, `docs/`). Background sessions already run in their own worktree.
3. If the issue needs an API change, start with `/spec` on this same branch: the contract, the generated code and the backend implementation ship in one pull request (ADR 0005). If the backend shows the contract does not work, change the contract in the same branch.
4. Implement with the role that fits: `go-implementer` for `api/`, `web-implementer` for `web/`. Web work for a new endpoint is a separate issue and pull request after the backend one; it may start earlier against MSW mocks from the spec.
5. Run `/verify`. Fix until it passes, within the three-attempts rule in `AGENTS.md`.
6. Ask the `reviewer` agent to review the branch. Fix important findings.
7. Commit with conventional messages, push the branch, and open a **draft** pull request:
   - title in conventional-commit form (it becomes the squash commit),
   - body: `Closes #<n>`, what changed, how it was tested, a **Review guide** (below), open questions,
   - metrics: CI or check failures before green, and anything a human had to correct.
8. Report the pull request URL.

## Review guide
The developer reviews by risk, not line by line. Write the guide from what you know of the code, scaled to the risk:

- **Low risk** (docs, chore, tooling, dependencies): one line, `Read: <file>`.
- **Feature:**
  - **Read in this order:** 2 to 4 `file:line-range` entries with what to check there: interfaces and contracts, migrations, the core logic.
  - **Criteria → tests:** every acceptance criterion with the test name that proves it.
  - **Skip:** generated code, mocks and fixtures.
- **New or changed endpoint or flow:** add a Mermaid `sequenceDiagram` of the request through controller → use case → repositories, with the error branches.
- **`review:strict` issues** (auth, sessions, passwords, tenancy, data migrations): also name what an attacker would try and where the code stops it.

When a fix round changes code the guide points to, update the guide in the pull request body (`gh pr edit <n> --body-file …`).

Fix rounds from `/orchestrate` arrive in this same session: fix on the same branch, run `/verify`, push normally (never force-push), then say what changed.
