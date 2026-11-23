---
name: adr
description: Record an architecture decision as a new ADR in docs/decisions and add it to the index. Use when a decision is made or an earlier one changes.
---

# Record a decision

1. Find the next number in `docs/decisions/` (four digits).
2. Copy `docs/decisions/template.md` to `docs/decisions/NNNN-short-title.md`. Fill in Context, Decision, Alternatives considered and Consequences. Plain English, short sentences, today's date.
3. If it replaces an earlier ADR, set the old one's status to `Superseded by NNNN` and link both ways.
4. Add a row to the decision table in `docs/README.md`.
5. Update any doc that now describes the old decision (`architecture.md`, `AGENTS.md`, ...).
