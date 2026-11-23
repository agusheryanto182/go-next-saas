#!/usr/bin/env bash
# PostToolUse hook: format the file an agent just edited. Never blocks; formatting problems surface in `make check`.
set -uo pipefail

file=$(jq -r '.tool_input.file_path // empty')
[[ -z "$file" || ! -f "$file" ]] && exit 0

root="${CLAUDE_PROJECT_DIR:-$(git rev-parse --show-toplevel)}"

case "$file" in
  "$root"/api/*.go)
    (cd "$root/api" && go tool gofumpt -w "$file") >/dev/null 2>&1
    ;;
  "$root"/web/*.ts | "$root"/web/*.tsx | "$root"/web/*.css | "$root"/web/*.json)
    prettier="$root/web/node_modules/.bin/prettier"
    [[ -x "$prettier" ]] && (cd "$root/web" && "$prettier" --write --log-level silent "$file") >/dev/null 2>&1
    ;;
esac
exit 0
