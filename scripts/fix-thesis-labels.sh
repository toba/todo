#!/usr/bin/env bash
# Fix GitHub issues in toba/thesis:
# 1. Set native issue types derived from type:* labels
# 2. Remove type:*, priority:*, status:* labels
# 3. Preserve all other labels
#
# Usage:
#   bash scripts/fix-thesis-labels.sh          # dry-run (default)
#   bash scripts/fix-thesis-labels.sh --apply  # actually make changes

set -euo pipefail

REPO="toba/thesis"
APPLY=false
if [[ "${1:-}" == "--apply" ]]; then
    APPLY=true
fi

# Map type:* label to native GitHub issue type
map_type() {
    case "$1" in
        type:bug)       echo "Bug" ;;
        type:feature)   echo "Feature" ;;
        type:task|type:epic|type:milestone) echo "Task" ;;
        *)              echo "" ;;
    esac
}

echo "Fetching all issues from $REPO (open + closed)..."

# Paginate through all issues, filter out PRs
# Write to temp file to avoid shell quoting issues
TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

gh api "repos/$REPO/issues?state=all&per_page=100" --paginate --jq '
    .[] | select(.pull_request == null) | {
        number: .number,
        labels: [.labels[].name],
        has_type: (.type != null)
    }
' > "$TMPFILE"

TOTAL=$(jq -s 'length' < "$TMPFILE")
echo "Found $TOTAL issues (excluding PRs)"
echo ""

FIXED=0
SKIPPED=0
ERRORS=0

# Process each issue line-by-line (one JSON object per line from jq)
while IFS= read -r ROW; do
    NUMBER=$(echo "$ROW" | jq -r '.number')
    LABELS=$(echo "$ROW" | jq -c '.labels')

    # Find the type:* label
    TYPE_LABEL=$(echo "$LABELS" | jq -r '[.[] | select(startswith("type:"))] | first // empty')

    # Compute native type
    NATIVE_TYPE=""
    if [[ -n "$TYPE_LABEL" ]]; then
        NATIVE_TYPE=$(map_type "$TYPE_LABEL")
    fi

    # Filter out bad labels (type:*, priority:*, status:*)
    CLEAN_LABELS=$(echo "$LABELS" | jq -c '[.[] | select(test("^(type|priority|status):") | not)]')

    # Check if any changes needed
    BAD_LABELS=$(echo "$LABELS" | jq -c '[.[] | select(test("^(type|priority|status):"))]')
    BAD_COUNT=$(echo "$BAD_LABELS" | jq 'length')

    if [[ "$BAD_COUNT" -eq 0 && -z "$NATIVE_TYPE" ]]; then
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    # Build the PATCH payload
    PATCH_BODY=$(jq -n --argjson labels "$CLEAN_LABELS" '{labels: $labels}')
    if [[ -n "$NATIVE_TYPE" ]]; then
        PATCH_BODY=$(echo "$PATCH_BODY" | jq --arg t "$NATIVE_TYPE" '. + {type: $t}')
    fi

    REMOVED=$(echo "$BAD_LABELS" | jq -r 'join(", ")')
    KEPT=$(echo "$CLEAN_LABELS" | jq -r 'if length > 0 then join(", ") else "(none)" end')

    if $APPLY; then
        if gh api "repos/$REPO/issues/$NUMBER" -X PATCH --input - <<< "$PATCH_BODY" > /dev/null 2>&1; then
            echo "#$NUMBER: type=${NATIVE_TYPE:-(skip)} | removed=[$REMOVED] | kept=[$KEPT]"
            FIXED=$((FIXED + 1))
        else
            echo "#$NUMBER: ERROR"
            ERRORS=$((ERRORS + 1))
        fi
        sleep 0.5
    else
        echo "[dry-run] #$NUMBER: type=${NATIVE_TYPE:-(skip)} | remove=[$REMOVED] | keep=[$KEPT]"
        FIXED=$((FIXED + 1))
    fi
done < <(jq -c '.' < "$TMPFILE")

echo ""
echo "=== Summary ==="
if $APPLY; then
    echo "Fixed:   $FIXED"
else
    echo "Would fix: $FIXED"
fi
echo "Skipped: $SKIPPED (already clean)"
echo "Errors:  $ERRORS"

if ! $APPLY && [[ $FIXED -gt 0 ]]; then
    echo ""
    echo "Run with --apply to execute changes:"
    echo "  bash scripts/fix-thesis-labels.sh --apply"
fi
