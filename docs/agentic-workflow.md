# Agentic workflow

How work moves from an idea to a merged pull request when most code is written by coding agents. The reasoning is in [ADR 0017](decisions/0017-agentic-workflow.md) and, for orchestration, [ADR 0019](decisions/0019-session-independent-orchestration.md).

## Levels

| Level | Where | Used for |
|---|---|---|
| 1. Pair programming | Interactive Claude Code session | Design, contract changes, foundations, reviews |
| 2. Local orchestration | Background sessions in agent view (`claude agents`), dispatched by `/orchestrate` | One issue per session, implemented end to end |
| 3. Cloud | `claude-code-action` on GitHub | `@claude` on small issues, automatic pull request review |

## From issue to merge

1. **Spec.** Every piece of work is a GitHub issue from the task template: spec, acceptance criteria, area, and `depends_on` / `parallel` / `conflicts_with`. Issues that an agent can finish alone get the `agent-ready` label.
2. **Contract and backend together.** If the API changes, one pull request updates `openapi/openapi.yaml` (`/spec`), regenerates code and implements the endpoint with contract and integration tests. The contract is merged only once it is proven implementable (ADR 0005). Web work follows in its own pull request; it may start early against MSW mocks from the spec on the backend branch.
3. **Dispatch.** `/orchestrate` starts one background session per issue (`claude --bg … "/slice <n>"`), picking issues whose dependencies are closed and that do not conflict with work in flight. Each session works in its own worktree under `.claude/worktrees/` and ends with a draft pull request that says `Closes #n` and carries a Review guide. Starting `/slice <n>` by hand in agent view still works.
4. **Verify.** Agents run `/verify` (`make check`) before finishing. A hook runs fast checks when an agent stops.
5. **Review.** Four layers:
   1. the `reviewer` agent inside `/slice`;
   2. the orchestrator's gate (criteria → tests present, scope, Review guide, CI), which then marks the pull request ready;
   3. the Claude review workflow;
   4. the developer, reading by risk with the Review guide.

   Findings, CI failures and conflicts go back to the agent's session as fix rounds, at most 2. `review:strict` pull requests also get `/review-strict` and are never marked ready automatically.
6. **Merge.** The developer squash merges when CI is green. `main` is protected: pull requests only, linear history, and the CI jobs `api`, `web`, `spec`, `e2e` and `dependency-review` must pass. A breaking contract change needs the `api:breaking` label.

## Rules

- **At most two pieces of work in flight** (running sessions plus open pull requests). Usage quota, review time and the laptop are the limits. `make e2e` runs one at a time per machine.
- **Orchestration** (ADR 0019): `/orchestrate` keeps no state of its own and can run from any session. `/orchestrate dry-run` shows the plan without acting, and `/loop /orchestrate` repeats it every 20 to 30 minutes. It claims issues with the `agent:running` label, never merges, never force-pushes, and acts only on comments from the owner and the review bots. Its report lists what is ready to merge and what needs the developer.
- **Notes for later issues go into the issue body**, not only a comment: agents read issues with `gh issue view`, which shows no comments.
- **One issue = one session = one worktree = one pull request.**
- **Kill rule:** an agent that fails three times on the same error stops and reports; the issue goes back to Level 1.
- **Pick parallel work by metadata:** only start issues together when both say `parallel: yes` and neither lists the other in `conflicts_with`.
- **Generated code is never edited by hand**: change the spec or the SQL and regenerate.
- **No secrets in prompts, issues or code.** Agents cannot read `.env` files.
- **Removing sessions:** use `claude rm <id>`, which keeps worktrees with unpushed work. After a squash merge it may refuse because the branch's commits are not on `main`. The work is merged, so the developer may run the `--discard-unpushed` command it prints; the orchestrator never does. Deleting a session in agent view with `Ctrl+X` twice also deletes its worktree, including uncommitted changes.
- **Issues from outside contributors** are read by a human before anyone mentions `@claude` on them, because issue text reaches the model.

## Level 3 on GitHub

Two workflows run `claude-code-action` with the repository owner's Claude subscription (secret `CLAUDE_CODE_OAUTH_TOKEN`, created with `claude setup-token`; the Claude GitHub App must be installed on the repository).

| Workflow | Trigger | What Claude may do |
|---|---|---|
| `claude.yml` | `@claude` in an issue, a pull request comment or a review | edit files, run the `make` checks (including `make check` and `make vuln`) and `go test`, read issues, pull requests and git history, commit and push to its own branch or the pull request |
| `claude-review.yml` | a pull request opens ready for review, leaves draft, or reopens | review with the official `code-review` plugin and post one comment; read-only otherwise |

Safety settings:
- Only users with write access start a run; pull requests from forks are skipped (action defaults, not overridden).
- `--permission-mode acceptEdits` with an explicit `--allowedTools` list: anything not on the list is denied, because nobody is there to approve it. The review runs in `default` mode with only the plugin's tools.
- `--max-turns 30` and job timeouts bound usage; one run per issue or pull request at a time.
- `show_full_output: false`: logs of a public repository are public, so Claude's transcript is never printed.
- Claude's configuration (`.claude/`, `CLAUDE.md`, `AGENTS.md`) comes from the base branch on pull requests, so a pull request cannot change the rules it is checked against.

Usage notes:
- The review runs once per pull request to save quota; comment `@claude review` for another pass after larger changes.
- Small, well-specified issues only: `@claude` on an issue implements it and opens a pull request that goes through the same CI and review as any other.
- A run uses the same subscription limits as local sessions; skip `@claude` when the quota is needed for agent view.

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
