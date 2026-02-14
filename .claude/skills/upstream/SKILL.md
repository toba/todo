---
name: upstream
description: >
  Review upstream hmans/beans repository for changes worth incorporating into
  the toba/todo fork. Use when: (1) user asks to check upstream for changes,
  (2) user says "upstream", "sync", or "cherry-pick" in context of the beans
  project, (3) user asks what's new in the original beans repo, (4) user wants
  to compare fork against upstream, (5) user says "/upstream".
---

# Upstream Review

Review https://github.com/hmans/beans for changes to incorporate into the toba/todo fork.

## Context

- **Upstream**: `hmans/beans` — the original beans repository
- **Fork**: `toba/todo` — this project's fork, with its own features (extensions, client package, TUI improvements, etc.)
- **Module rename**: The fork renamed the Go module from `github.com/hmans/beans` to `github.com/toba/todo` (commit `ec71d08`). Any upstream Go code needs import path adjustment.

## Workflow

### 1. Fetch upstream changes

```bash
bash .claude/skills/upstream/scripts/fetch_upstream.sh
```

Use `--since "2 weeks ago"` to limit to recent changes. Use `--oneline` for a compact view.

### 2. Review the diff summary

```bash
bash .claude/skills/upstream/scripts/diff_upstream.sh
```

Use `--path "internal/bean/"` to focus on a specific package. Use `--full` for the complete diff.

### 3. Analyze and categorize changes

For each upstream commit, classify as:

- **High value**: Bug fixes, performance improvements, new features aligned with fork goals
- **Medium value**: Refactors that improve code quality, test improvements
- **Low value**: Style changes, documentation-only changes
- **Skip**: Changes conflicting with fork-specific features (extensions, client package, module rename)

### 4. Check for conflicts

Before recommending a cherry-pick, check if the upstream change touches files that the fork has modified:

```bash
# Files modified in the fork since divergence
git diff --name-only $(git merge-base HEAD upstream/main)..HEAD

# Files modified in the upstream commit
git diff --name-only <commit>~1..<commit>
```

If overlap exists, note the conflict risk in the recommendation.

### 5. Present recommendations

For each recommended change, provide:

- **Commit hash and message** from upstream
- **Category** (bug fix, feature, refactor, etc.)
- **Value assessment** (high/medium/low)
- **Conflict risk** (none, low, high) with affected files
- **Import path note** if Go files are involved (needs `github.com/hmans/beans` → `github.com/toba/todo`)
- **Cherry-pick command**: `git cherry-pick <hash>` (or range)

### 6. Optionally apply changes

If the user wants to apply changes:

```bash
# Cherry-pick a single commit
git cherry-pick <hash>

# After cherry-pick, fix import paths if needed
grep -rl "github.com/hmans/beans" --include="*.go" . | xargs sed -i '' 's|github.com/hmans/beans|github.com/toba/todo|g'

# Run tests to verify
mise test
```

Create a bean to track the incorporation work when applying changes.

## Important considerations

- Never auto-apply changes without user confirmation
- Always run `mise test` after applying upstream changes
- The fork's `.issues/` directory and `CLAUDE.md` are fork-specific — ignore upstream changes to these
- The fork has features upstream doesn't: extensions system, client package, TUI enhancements, `/commit` skill integration. Protect these.
- Watch for upstream changes to `go.mod` / `go.sum` — these may need careful merging
