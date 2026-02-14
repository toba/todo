#!/usr/bin/env bash
# Fetch upstream commits and show what's new since fork diverged.
# Usage: fetch_upstream.sh [--since COMMIT_OR_DATE] [--repo-path PATH]
#
# Requires: git, gh CLI
# Environment: UPSTREAM_REPO defaults to hmans/beans

set -euo pipefail

UPSTREAM_REPO="${UPSTREAM_REPO:-hmans/beans}"
UPSTREAM_REMOTE="upstream"
REPO_PATH="."
SINCE=""
FORMAT="medium"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --since) SINCE="$2"; shift 2 ;;
    --repo-path) REPO_PATH="$2"; shift 2 ;;
    --oneline) FORMAT="oneline"; shift ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

cd "$REPO_PATH"

# Ensure upstream remote exists
if ! git remote get-url "$UPSTREAM_REMOTE" &>/dev/null; then
  echo "Adding upstream remote: https://github.com/${UPSTREAM_REPO}.git"
  git remote add "$UPSTREAM_REMOTE" "https://github.com/${UPSTREAM_REPO}.git"
fi

echo "Fetching upstream..."
git fetch "$UPSTREAM_REMOTE" --quiet

# Find the merge base (where fork diverged)
MERGE_BASE=$(git merge-base HEAD "${UPSTREAM_REMOTE}/main" 2>/dev/null || echo "")

if [[ -z "$MERGE_BASE" ]]; then
  echo "ERROR: No common ancestor found between HEAD and ${UPSTREAM_REMOTE}/main."
  echo "The repositories may have diverged completely."
  exit 1
fi

echo "Fork diverged at: $(git log --oneline -1 "$MERGE_BASE")"
echo ""

# Build log command
LOG_ARGS=("${UPSTREAM_REMOTE}/main" "--not" "HEAD")
if [[ -n "$SINCE" ]]; then
  LOG_ARGS+=("--since=$SINCE")
fi

COMMIT_COUNT=$(git rev-list "${LOG_ARGS[@]}" 2>/dev/null | wc -l | tr -d ' ')
echo "Upstream has ${COMMIT_COUNT} commits not in this fork."
echo ""

if [[ "$COMMIT_COUNT" -eq 0 ]]; then
  echo "You are up to date with upstream."
  exit 0
fi

if [[ "$FORMAT" == "oneline" ]]; then
  git log --oneline "${LOG_ARGS[@]}"
else
  git log --format="### %h %s%n%n**Author:** %an | **Date:** %ad%n%n%b%n---" --date=short "${LOG_ARGS[@]}"
fi
