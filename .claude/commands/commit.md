---
description: Stage all changes and commit with a descriptive message
---

## Active Codebase Expectations

This is an active codebase with multiple agents and people making changes concurrently. Do NOT waste time investigating unexpected git status:
- If a file you edited shows no changes, someone else likely already committed it - move on
- If files you didn't touch appear modified, another agent may have changed them - include or exclude as appropriate
- Focus on what IS changed, not what ISN'T

## Stage and Commit

Run `./scripts/commit.sh $ARGUMENTS`

### If script exits with code 2 (gitignore candidates found)

Ask the user whether to:
1. Add the files to .gitignore
2. Proceed with committing them anyway
3. Cancel

### If script succeeds (staged changes shown)

1. Commit ALL staged changes - never unstage or filter files
2. Create a commit with a concise, descriptive message:
   - Lowercase, imperative mood (e.g., "add feature" not "Added feature")
   - Focus on "why" not just "what"
   - No need to check git log for style
   - Include affected issue IDs
3. Run `git status` to confirm the commit succeeded
4. If output contains "PUSH_AFTER_COMMIT":
   a. Tag a version bump using `mise release:<level>` (see Version Bumps below)
   b. Run `git push && git push --tags`

## Version Bumps

Every push includes a version bump. Choose the level based on the commit(s) being pushed:

- **patch** (`mise release:patch`): Bug fixes, docs, refactors, tests — no behavior change
- **minor** (`mise release:minor`): New features, non-breaking additions, breaking changes while pre-1.0
- **major** (`mise release:major`): Breaking changes (post-1.0 only)

Look at the conventional commit prefixes to decide:
- `fix:`, `docs:`, `chore:`, `refactor:`, `test:` → **patch**
- `feat:` → **minor**
- `feat!:` or any `!:` → **minor** (pre-1.0) / **major** (post-1.0)

