#!/usr/bin/env bash
# Show file-level diff summary between this fork and upstream.
# Usage: diff_upstream.sh [--stat] [--path FILTER] [--repo-path PATH]

set -euo pipefail

UPSTREAM_REMOTE="upstream"
REPO_PATH="."
STAT_ONLY=true
PATH_FILTER=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --full) STAT_ONLY=false; shift ;;
    --stat) STAT_ONLY=true; shift ;;
    --path) PATH_FILTER="$2"; shift 2 ;;
    --repo-path) REPO_PATH="$2"; shift 2 ;;
    *) echo "Unknown arg: $1" >&2; exit 1 ;;
  esac
done

cd "$REPO_PATH"

# Ensure upstream is fetched
if ! git remote get-url "$UPSTREAM_REMOTE" &>/dev/null; then
  echo "ERROR: upstream remote not configured. Run fetch_upstream.sh first." >&2
  exit 1
fi

MERGE_BASE=$(git merge-base HEAD "${UPSTREAM_REMOTE}/main" 2>/dev/null || echo "")
if [[ -z "$MERGE_BASE" ]]; then
  echo "ERROR: No common ancestor found." >&2
  exit 1
fi

DIFF_ARGS=("${MERGE_BASE}..${UPSTREAM_REMOTE}/main")

if [[ -n "$PATH_FILTER" ]]; then
  DIFF_ARGS+=("--" "$PATH_FILTER")
fi

if $STAT_ONLY; then
  echo "Files changed in upstream since fork diverged:"
  echo ""
  git diff --stat "${DIFF_ARGS[@]}"
else
  git diff "${DIFF_ARGS[@]}"
fi
