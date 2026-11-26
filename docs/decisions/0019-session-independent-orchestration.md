# 0019. Session-independent orchestration of agent work

- **Status:** Accepted
- **Date:** 2026-11-26

## Context
[ADR 0017](0017-agentic-workflow.md) made Level 2 one background session per issue, started by hand in agent view. After milestone 1's first waves the agents were reliable, but the developer was still the dispatcher. They picked issues by their metadata, typed `/slice`, relayed pull request numbers to a review session, marked pull requests ready, removed sessions and started the next wave. Keeping that coordination in one interactive session does not work either: when the session ends or its context fills, the plan is gone.

Two Claude Code features make a different split possible. `claude --bg` starts an agent view session from the command line, and that session outlives the one that started it. `claude agents --json` lists the sessions and their state.

## Decision
- **State lives only in GitHub and `claude agents`.** `.claude/scripts/agent-state.py` reads open issues and their `depends_on` / `conflicts_with` / `level` metadata, open pull requests with CI and Claude review status, and background sessions. From these it computes the plan deterministically: what to dispatch, stale claims, and sessions to remove. Ordering: the issues that unblock the most others go first, within a parallel cap of 2 (sessions plus open pull requests).
- **`/orchestrate` is one idempotent pass.** It cleans up, gates and fixes pull requests, dispatches the next issues with `claude --bg`, and reports. Any session can run it; `/loop /orchestrate` repeats it. `dry-run` shows the plan without acting.
- **Claims** use the `agent:running` label plus an issue comment naming the session. A claim with no live session and no pull request is released on the next pass.
- **Review layers:**
  1. the `reviewer` agent inside `/slice`;
  2. the orchestrator's gate: criteria → tests present, scope, a Review guide, CI;
  3. the Claude review workflow;
  4. the developer, guided by the risk-scaled Review guide in each pull request.
- **Fix rounds:** CI failures, review findings, gate findings and merge conflicts are sent back to the agent's own session (`claude --bg --resume`). There are at most 2 rounds; after that the pull request needs a human. Conflicts are solved by merging `main`, never by force-pushing.
- **`review:strict`** (auth, sessions, passwords, tenancy, data migrations) adds `/review-strict`, a written security checklist that every session applies the same way. These pull requests are never marked ready automatically.
- **Hard limits:** the developer merges (`gh pr merge` stays denied). The orchestrator never force-pushes, never discards unpushed work, and never edits code itself. It acts only on comments from the owner and the review bots, and ignores its own (marked `<!-- orchestrate`). It dispatches only `agent-ready` issues.
- **Notes for later issues go into their bodies**, because `/slice` reads issues with `gh issue view`, which shows no comments.

## Alternatives considered
- **The interactive session as orchestrator, with in-process subagents:** lost when the session ends; subagents do not appear in agent view.
- **Level 3 (agents in GitHub Actions on `agent-ready`):** survives a closed laptop, but the slices need Docker and the local toolchain, give less control, and spend the same quota.
- **Auto-merge on green:** removes the one human checkpoint of ADR 0017.

## Consequences
- The developer's part becomes merging and deciding what the report lists under "Needs you".
- The loop runs on the developer's machine: if it sleeps, the loop stops. A new `/loop /orchestrate` continues from GitHub's state.
- `claude --bg` and agent view are research previews; if they change, the skill changes, not the workflow.
- Permission rules match command prefixes, so dispatch (`claude --bg`) and `claude rm` are not allow-listed. They run under the developer's permission mode, and the skill fixes the exact commands.
- Ordering by unblocked issues and the cap of 2 replace the manual "waves".
