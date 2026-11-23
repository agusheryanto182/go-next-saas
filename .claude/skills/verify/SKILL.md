---
name: verify
description: Run the repository checks (the same ones CI runs) and report results. Use before finishing any change.
---

# Verify

1. `make check`. It runs, for the whole repository: formatting, lint (including layer rules), vulnerability scan, unit tests, type checks, web tests and build, spec lint, generated-code sync and migration lint.
2. If the change touches the database or an endpoint, also `make test-integration`.
3. If the change affects a user journey, also `make e2e` (or ask the `verifier` agent).

Report each command as passed or failed. For failures, show the relevant output, not the whole log. Never skip, disable or weaken a check to make it pass.
