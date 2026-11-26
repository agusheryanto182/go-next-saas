#!/usr/bin/env bash
# Runs the end-to-end tests against the full stack (`make e2e`).
#
# The stack runs in its own Compose project with random host ports (compose.e2e.yml), so it never
# touches the development stack (`make dev`, fixed ports) or a run from another worktree. One run at a
# time per machine: the lock lives in the git directory that every worktree shares. The stack and its
# volumes are removed afterwards; on failure its logs are printed first. E2E_KEEP=1 keeps it running.
set -euo pipefail

root=$(git rev-parse --show-toplevel)
cd "$root"

project="go-next-saas-e2e-$(basename "$root" | tr -c 'a-z0-9\n' '-')"
compose=(docker compose -p "$project" --project-directory "$root" -f "$root/compose.yml" -f "$root/compose.e2e.yml" --profile app)
lock="$(git rev-parse --path-format=absolute --git-common-dir)/e2e.lock"

exec 9>"$lock"
if ! flock -n 9; then
  echo "Another make e2e is running on this machine; waiting for it to finish..."
  flock 9
fi

cleanup() {
  local status=$?
  if [ "$status" -ne 0 ]; then
    "${compose[@]}" logs --no-color || true
  fi
  if [ -n "${E2E_KEEP:-}" ]; then
    echo "Stack kept. Remove it with: ${compose[*]} down -v"
  else
    "${compose[@]}" down --volumes --remove-orphans >/dev/null || echo "Could not remove the e2e stack: ${compose[*]} down -v" >&2
  fi
  exit "$status"
}
trap cleanup EXIT

"${compose[@]}" up -d --build --wait
web=$("${compose[@]}" port web 3000)
api=$("${compose[@]}" port api 8080)
mail=$("${compose[@]}" port mailpit 8025)
echo "e2e stack $project: web http://$web, API http://$api, Mailpit http://$mail"

(
  cd e2e
  E2E_BASE_URL="http://$web" E2E_API_URL="http://$api" E2E_MAILPIT_URL="http://$mail" \
    npx --no-install playwright test
)
