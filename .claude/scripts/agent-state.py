#!/usr/bin/env python3
"""Prints the state of agent work as JSON for /orchestrate (ADR 0019).

Everything is read from GitHub (`gh`) and `claude agents`, never from a session's memory, so any
session can run it and get the same answer. The plan is computed here, deterministically:

- which issues may be dispatched: open, labelled agent-ready, every `depends_on` closed, no claim,
  session or pull request yet, no `conflicts_with` overlap (either direction) with work in flight or
  with each other, within the parallel cap; the issues that unblock the most others go first;
- the state of each open pull request: CI, Claude review, conflicts, fix rounds, strict review;
- stale claims (agent:running with no live session and no pull request) and sessions to remove.

Usage: agent-state.py [--max N]   (default 2 parallel)
Run with `python3 -I` from anywhere inside the repository.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import re
import subprocess
import sys
from datetime import datetime, timezone

REQUIRED_CHECKS = {"api", "web", "spec", "e2e", "dependency-review"}
REVIEW_BOTS = {"claude", "claude[bot]", "github-actions", "github-actions[bot]"}
CLAIM_LABEL = "agent:running"
STRICT_LABEL = "review:strict"
READY_LABEL = "agent-ready"
# Comments the orchestrator posts carry this marker, so it never reads its own words as findings.
MARKER = "<!-- orchestrate"
FIX_ROUND_MARKER = "<!-- orchestrate:fix-round"
STRICT_REVIEW_MARKER = "<!-- orchestrate:review-strict"


def run(*args: str) -> str:
    return subprocess.run(args, check=True, capture_output=True, text=True).stdout


def gh_json(*args: str):
    return json.loads(run("gh", *args))


def repo_owner() -> str:
    return gh_json("repo", "view", "--json", "owner")["owner"]["login"]


def issue_refs(text: str) -> list[int]:
    return [int(n) for n in re.findall(r"#(\d+)", text or "")]


def meta(body: str) -> dict:
    """Parses the `## Meta` block of an issue (task template)."""
    out = {"depends_on": [], "conflicts_with": [], "level": "", "parallel": True}
    block = body.split("## Meta", 1)[1] if "## Meta" in (body or "") else ""
    for line in block.splitlines():
        key, _, value = line.partition(":")
        key, value = key.strip().lower(), value.strip()
        if key == "depends_on":
            out["depends_on"] = issue_refs(value)
        elif key == "conflicts_with":
            out["conflicts_with"] = issue_refs(value)
        elif key == "level":
            out["level"] = value
        elif key == "parallel":
            out["parallel"] = value.lower().startswith("yes")
    return out


def closes(body: str) -> int | None:
    m = re.search(r"(?i)\b(?:closes|fixes|resolves)\s+#(\d+)", body or "")
    return int(m.group(1)) if m else None


def session_issue(s: dict) -> int | None:
    name, cwd = s.get("name") or "", s.get("cwd") or ""
    m = re.search(r"(?i)\bslice(?:\s+issue)?[\s-]+(\d+)", name) or re.search(
        r"/(?:feat|fix|chore|docs|slice)[+-](\d+)(?:-|$)", cwd
    )
    return int(m.group(1)) if m else None


def ci_state(rollup: list[dict]) -> tuple[str, list[str]]:
    seen, failing, pending = set(), [], []
    for c in rollup or []:
        name = c.get("name") or c.get("context") or ""
        if name not in REQUIRED_CHECKS:
            continue
        seen.add(name)
        conclusion = (c.get("conclusion") or c.get("state") or "").upper()
        status = (c.get("status") or "").upper()
        if conclusion in {"FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED"}:
            failing.append(name)
        elif conclusion not in {"SUCCESS", "SKIPPED", "NEUTRAL"} or status not in {"", "COMPLETED"}:
            pending.append(name)
    if failing:
        return "failing", sorted(failing)
    if pending or seen != REQUIRED_CHECKS:
        return "pending", sorted(pending or REQUIRED_CHECKS - seen)
    return "green", []


def review_state(pr: dict, owner: str) -> tuple[str, int, bool]:
    """Claude review result (latest bot review comment), fix rounds used, strict review posted."""
    review, rounds, strict_done = "none", 0, False
    for c in pr.get("comments") or []:
        author = (c.get("author") or {}).get("login", "")
        body = c.get("body") or ""
        if author == owner and body.lstrip().startswith(FIX_ROUND_MARKER):
            rounds += 1
        if author == owner and body.lstrip().startswith(STRICT_REVIEW_MARKER):
            strict_done = True
        if author in REVIEW_BOTS and "## Code review" in body:
            review = "clean" if "No issues found" in body else "findings"
    return review, rounds, strict_done


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--max", type=int, default=2, help="parallel cap (sessions plus open pull requests)")
    args = ap.parse_args()

    owner = repo_owner()
    issues = gh_json("issue", "list", "--state", "open", "--limit", "200",
                     "--json", "number,title,labels,body,author")
    prs = gh_json("pr", "list", "--state", "open", "--limit", "100", "--json",
                  "number,title,isDraft,headRefName,body,mergeable,statusCheckRollup,author,comments,url")
    try:
        sessions = json.loads(run("claude", "agents", "--json", "--all"))
    except (subprocess.CalledProcessError, FileNotFoundError, json.JSONDecodeError):
        sessions = []
    sessions = [s for s in sessions if s.get("kind") == "background"]

    open_issues = {i["number"]: i for i in issues}
    labels = {n: {lab["name"] for lab in i["labels"]} for n, i in open_issues.items()}
    metas = {n: meta(i["body"]) for n, i in open_issues.items()}

    def strict(n: int | None) -> bool:
        return n in open_issues and (STRICT_LABEL in labels[n] or "strict" in metas[n]["level"].lower())

    # Sessions by issue. Live means it can still produce work or answer a fix round.
    sess_out = []
    live_by_issue: dict[int, dict] = {}
    now_ms = datetime.now(timezone.utc).timestamp() * 1000
    for s in sessions:
        n = session_issue(s)
        started = s.get("startedAt")
        entry = {"id": s.get("id"), "name": s.get("name"), "issue": n, "state": s.get("state"),
                 "status": s.get("status"), "cwd": s.get("cwd"),
                 "age_minutes": int((now_ms - started) / 60000) if isinstance(started, (int, float)) else None}
        sess_out.append(entry)
        if n is not None and s.get("state") in {"working", "done"}:
            live_by_issue.setdefault(n, entry)

    pr_out = []
    pr_by_issue: dict[int, dict] = {}
    for p in prs:
        if (p.get("author") or {}).get("login", "").startswith("dependabot"):
            pr_out.append({"number": p["number"], "title": p["title"], "dependabot": True, "url": p["url"]})
            continue
        n = closes(p["body"])
        ci, failing = ci_state(p.get("statusCheckRollup"))
        review, rounds, strict_done = review_state(p, owner)
        entry = {
            "number": p["number"], "title": p["title"], "issue": n, "draft": p["isDraft"],
            "branch": p["headRefName"], "ci": ci, "failing_checks": failing,
            "mergeable": p.get("mergeable"), "review": review, "fix_rounds": rounds,
            "strict": strict(n), "strict_review_posted": strict_done,
            "has_review_guide": "## Review guide" in (p["body"] or ""),
            "session": (live_by_issue.get(n) or {}).get("id") if n else None, "url": p["url"],
        }
        pr_out.append(entry)
        if n is not None:
            pr_by_issue[n] = entry

    working = {n for n, s in live_by_issue.items() if s["state"] == "working"}
    claimed = {n for n in open_issues if CLAIM_LABEL in labels[n]}
    stale_claims = sorted(n for n in claimed if n not in live_by_issue and n not in pr_by_issue)
    in_flight = (working | set(pr_by_issue) | claimed) - set(stale_claims)
    in_flight &= set(open_issues)

    dependents = {n: 0 for n in open_issues}
    for n, m in metas.items():
        for d in m["depends_on"]:
            if d in dependents:
                dependents[d] += 1

    def conflicts(a: int, b: int) -> bool:
        return b in metas.get(a, {}).get("conflicts_with", []) or a in metas.get(b, {}).get("conflicts_with", [])

    blocked, not_ready, candidates = [], [], []
    for n in sorted(open_issues):
        if n in in_flight:
            continue
        waiting = [d for d in metas[n]["depends_on"] if d in open_issues]
        if waiting:
            blocked.append({"issue": n, "waiting_on": waiting})
        elif READY_LABEL not in labels[n] or metas[n]["level"].strip().startswith("1"):
            not_ready.append({"issue": n, "title": open_issues[n]["title"]})
        else:
            candidates.append(n)
    candidates.sort(key=lambda n: (-dependents[n], n))

    slots = max(args.max - len(in_flight), 0)
    chosen: list[int] = []
    held_back = []
    for n in candidates:
        clash = [o for o in sorted(in_flight) + chosen if conflicts(n, o)]
        if clash:
            # In flight, or dispatched earlier in this same plan.
            held_back.append({"issue": n, "conflicts_with": clash})
        elif len(chosen) < slots:
            chosen.append(n)
        else:
            held_back.append({"issue": n, "reason": "parallel cap"})

    cleanup = [s for s in sess_out if s["issue"] is not None and s["issue"] not in open_issues]
    failed = [s for s in sess_out if s["state"] == "failed" and s["issue"] in open_issues]

    state = {
        "max_parallel": args.max,
        "in_flight": sorted(in_flight),
        "dispatch": [{"issue": n, "title": open_issues[n]["title"], "strict": strict(n),
                      "unblocks": dependents[n]} for n in chosen],
        "held_back": held_back,
        "blocked": blocked,
        "not_agent_ready": not_ready,
        "pull_requests": pr_out,
        "sessions": sess_out,
        "stale_claims": stale_claims,
        "failed_sessions": failed,
        "cleanup_sessions": cleanup,
    }
    # Ages change every minute; leave them out so an unchanged state keeps its fingerprint.
    stable = json.loads(json.dumps(state), object_hook=lambda o: {k: v for k, v in o.items() if k != "age_minutes"})
    state["fingerprint"] = hashlib.sha256(json.dumps(stable, sort_keys=True).encode()).hexdigest()[:12]
    state["generated_at"] = datetime.now(timezone.utc).isoformat(timespec="seconds")
    json.dump(state, sys.stdout, indent=2)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
