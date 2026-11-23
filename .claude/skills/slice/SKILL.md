---
name: slice
description: Implement one GitHub issue end to end on its own branch and open a draft pull request. Use with an issue number, e.g. "/slice 42", typically in a background session.
---

# Slice an issue

Input: an issue number.

1. `gh issue view <n>`. Read the spec, acceptance criteria and the `depends_on` line. If a dependency is still open, stop and report it.
2. Make sure you are on a fresh branch from the latest `main`, named `<type>/<n>-<short-slug>` (`feat/`, `fix/`, `chore/`, `docs/`). Background sessions already run in their own worktree.
3. If the issue needs an API change that is not in `openapi/openapi.yaml` yet, stop: the contract is changed first with `/spec`.
4. Implement with the role that fits: `go-implementer` for `api/`, `web-implementer` for `web/`. An issue touching both does API first, then web.
5. Run `/verify`. Fix until it passes, within the three-attempts rule in `AGENTS.md`.
6. Ask the `reviewer` agent to review the branch. Fix important findings.
7. Commit with conventional messages, push the branch, and open a **draft** pull request:
   - title in conventional-commit form (it becomes the squash commit),
   - body: `Closes #<n>`, what changed, how it was tested, open questions,
   - metrics: CI or check failures before green, and anything a human had to correct.
8. Report the pull request URL.
