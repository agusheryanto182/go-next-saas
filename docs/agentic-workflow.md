# Agentic workflow

How work moves from an idea to a merged pull request when most code is written by coding agents. The reasoning is in [ADR 0017](decisions/0017-agentic-workflow.md).

## Levels

| Level | Where | Used for |
|---|---|---|
| 1. Pair programming | Interactive Claude Code session | Design, contract changes, foundations, reviews |
| 2. Local orchestration | Background sessions in agent view (`claude agents`) | One issue per session, implemented end to end |
| 3. Cloud | `claude-code-action` on GitHub | `@claude` on small issues, automatic pull request review |

## From issue to merge

1. **Spec.** Every piece of work is a GitHub issue from the task template: spec, acceptance criteria, area, and `depends_on` / `parallel` / `conflicts_with`. Issues that an agent can finish alone get the `agent-ready` label.
2. **Contract first.** If the API changes, a small pull request updates `openapi/openapi.yaml` (`/spec`). It is reviewed and merged before implementation.
3. **Dispatch.** In agent view, start one background session per issue with `/slice <issue>`. Each session works in its own worktree under `.claude/worktrees/` and ends with a draft pull request that says `Closes #n`.
4. **Verify.** Agents run `/verify` (`make check`) before finishing. A hook runs fast checks when an agent stops.
5. **Review.** The `reviewer` agent and the Claude review workflow comment first; then the developer reviews.
6. **Merge.** Squash merge when CI is green. `main` is protected: pull requests only, linear history.

## Rules

- **At most two agents in parallel.** Usage quota and review time are the limits.
- **One issue = one session = one worktree = one pull request.**
- **Kill rule:** an agent that fails three times on the same error stops and reports; the issue goes back to Level 1.
- **Pick parallel work by metadata:** only start issues together when both say `parallel: yes` and neither lists the other in `conflicts_with`.
- **Generated code is never edited by hand**: change the spec or the SQL and regenerate.
- **No secrets in prompts, issues or code.** Agents cannot read `.env` files.
- **Removing sessions:** use `claude rm <id>`, which keeps worktrees with unpushed work. Deleting a session in agent view with `Ctrl+X` twice also deletes its worktree, including uncommitted changes.
- **Issues from outside contributors** are read by a human before anyone mentions `@claude` on them, because issue text reaches the model.

## Roles

| Agent | Writes to | Purpose |
|---|---|---|
| `go-implementer` | `api/` | API slices: queries, use cases, controllers, tests |
| `web-implementer` | `web/` | Pages, components, client tests |
| `reviewer` | nothing (read-only) | Reviews a diff against the issue, the contract and the rules |
| `verifier` | nothing in source | Runs the full stack and end-to-end tests, reports results |

Workers run on a cost-efficient model; the interactive session uses the strongest model.

## Metrics

For every merged pull request, record in its description: time from issue to merge, CI failures before green, and manual corrections. These feed the case study in milestone 6.
