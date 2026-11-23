@AGENTS.md

## Claude Code
- Workflow and parallel-work rules: `docs/agentic-workflow.md`.
- Skills: `/slice <issue>` implements an issue end to end, `/verify` runs the checks, `/spec` changes the contract, `/adr` records a decision.
- Role agents in `.claude/agents/`: `go-implementer`, `web-implementer`, `reviewer`, `verifier`.
- Hooks format edited files and run `make check-fast` before a session stops.
