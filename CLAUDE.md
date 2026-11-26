@AGENTS.md

## Claude Code
- Workflow and parallel-work rules: `docs/agentic-workflow.md`.
- Skills: `/slice <issue>` implements an issue end to end, `/verify` runs the checks, `/spec` changes the contract, `/adr` records a decision, `/orchestrate [dry-run]` runs one pass of dispatch, gates, fix rounds and report (ADR 0019; `/loop /orchestrate` to repeat), `/review-strict <pr>` reviews a `review:strict` pull request against the security checklist.
- Role agents in `.claude/agents/`: `go-implementer`, `web-implementer`, `reviewer`, `verifier`.
- Hooks format edited files and run `make check-fast` before a session stops.
