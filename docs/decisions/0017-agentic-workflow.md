# 0017. Agentic workflow on stable Claude Code features

- **Status:** Accepted; dispatching and coordinating Level 2 work superseded by [0019](0019-session-independent-orchestration.md)
- **Date:** 2026-11-23

## Context
The project is built by one developer with coding agents on a Claude Pro subscription, so review time and usage quota are the limits, not typing speed. Third-party orchestrators exist (for example CCPM, Claude Squad, desktop workspaces), but they add their own conventions or are not available on Linux.

## Decision
Build on Claude Code's own features plus a thin layer in the repository:
- **Level 1, pair programming:** design, foundations and reviews in an interactive session.
- **Level 2, local orchestration:** one GitHub issue per background session in agent view, each in its own worktree, ending in a draft pull request. At most two in parallel.
- **Level 3, cloud:** `claude-code-action` for `@claude` on issues and automatic pull request review.
- **Rails:** layered `CLAUDE.md`, permission allow and deny lists, hooks (format on edit, fast checks before stopping), role agents (Go implementer, web implementer, read-only reviewer, verifier), skills (`/slice`, `/verify`, `/spec`, `/adr`), issue metadata `depends_on`, `parallel`, `conflicts_with` (borrowed from CCPM).
- The developer reviews and merges every pull request.

Agent view is a research preview. If it changes, the same flow runs with `claude --worktree` sessions.

## Alternatives considered
- **CCPM:** a good fit, but one worktree per epic shared by several agents, and its own PRD and epic files.
- **Agent teams:** experimental.
- **BMAD and other heavy spec frameworks:** spend a large share of tokens on documents, costly on a Pro plan.

## Consequences
- The workflow uses tools that track Claude Code releases.
- Parallelism is limited on purpose; quality is guarded by CI, the reviewer agent and human review.
- See [agentic-workflow.md](../agentic-workflow.md) for the day-to-day rules.
