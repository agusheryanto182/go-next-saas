#!/usr/bin/env bash
# Stop hook: before an agent finishes, run fast checks for the areas it changed.
# Exit 2 sends the failure back to the agent so it keeps working; exit 0 lets it stop.
set -uo pipefail

input=$(cat)
# Already continuing because of this hook: do not loop.
[[ "$(jq -r '.stop_hook_active // false' <<<"$input")" == "true" ]] && exit 0

root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"
cd "$root" || exit 0

changed=$( { git diff --name-only HEAD; git ls-files --others --exclude-standard; } 2>/dev/null | sort -u)
[[ -z "$changed" ]] && exit 0

areas=()
grep -q '^api/' <<<"$changed" && areas+=(api)
grep -q '^web/' <<<"$changed" && areas+=(web)
grep -q '^openapi/' <<<"$changed" && areas+=(spec)
[[ ${#areas[@]} -eq 0 ]] && exit 0

# The Makefile may not have this target yet (early in the project).
make -n check-fast >/dev/null 2>&1 || exit 0

if ! output=$(make check-fast AREAS="${areas[*]}" 2>&1); then
  echo "make check-fast failed for: ${areas[*]}. Fix it before finishing:" >&2
  tail -n 60 <<<"$output" >&2
  exit 2
fi
exit 0
