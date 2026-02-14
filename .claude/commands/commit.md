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
   - Include affected bean IDs
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

### Step 5: Update Homebrew Tap

After pushing tags, the GoReleaser GitHub Action builds the release. Wait for it, then update the tap:

1. Wait for the release to be available (poll every 15s, up to 5 minutes):
   ```bash
   VERSION=<tag just pushed, e.g. v0.8.2>
   until gh release view "$VERSION" --repo toba/todo &>/dev/null; do sleep 15; done
   ```

2. Download checksums and extract the SHA256:
   ```bash
   SHA=$(gh release download "$VERSION" --repo toba/todo --pattern checksums.txt -O - | grep 'todo_darwin_arm64.tar.gz' | awk '{print $1}')
   ```

3. Update `../homebrew-todo/Formula/todo.rb`:
   - Change the `url` line to use the new version tag
   - Change the `version` line to the new version (without 'v' prefix)
   - Set `sha256` to the value from step 2

4. Commit and push the homebrew tap:
   ```bash
   cd ../homebrew-todo
   git add Formula/todo.rb
   git commit -m "bump to <version>"
   git push
   cd ../todo
   ```
