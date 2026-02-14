---
# msf-b2p
title: Integrate bean-hub GitHub sync into todo
status: completed
type: feature
priority: normal
created_at: 2026-02-14T19:48:19Z
updated_at: 2026-02-14T19:56:04Z
---

Port GitHub Issues sync from toba/bean-hub into internal/integration/github/. Refactor shared integration layer to be provider-agnostic.


## Summary of Changes

### Phase 1: Refactored shared integration layer
- Replaced `SyncResult` type alias (tied to `clickup`) with standalone struct using generic `ExternalID`/`ExternalURL` fields
- Defined standalone `ProgressFunc` type in `integration.go`, removing the `clickup` import
- Changed `Link()` to return `*LinkResult` (action: "linked" or "already_linked") and `Unlink()` to return `*UnlinkResult` (action: "unlinked" or "not_linked")
- Updated `clickup_adapter.go` with conversion functions between `clickup.SyncResult` and `integration.SyncResult`
- Extracted `detectClickUp()` helper from `Detect()` for cleaner dispatch

### Phase 2: cmd layer updates
- `cmd/sync.go`: JSON fields renamed `task_id`→`external_id`, `task_url`→`external_url`; text output uses `ExternalURL`
- `cmd/sync_link.go`: Removed `clickup` import; delegates "already linked" check to `integ.Link()` result
- `cmd/sync_unlink.go`: Removed `clickup` import; delegates "not linked" check to `integ.Unlink()` result
- `cmd/sync_check.go`: Generic "no integration" message mentions both clickup and github

### Phase 3: GitHub integration package
- `internal/integration/github/types.go`: GitHub API types (Issue, Label, User, Repo, requests), plus `SyncResult`/`ProgressFunc`/`SyncOptions`
- `internal/integration/github/client.go`: REST client with retry logic, label cache, sub-issue API support
- `internal/integration/github/config.go`: Config parsing from `extensions.github.repo`, default status/priority/type label mappings
- `internal/integration/github/sync_state.go`: `ExtensionSyncProvider` using `core.SaveExtensionOnly()`, extension helpers (`GetExtensionString`, `GetExtensionInt`, `GetExtensionTime`)
- `internal/integration/github/sync.go`: Multi-pass syncer (parents→children→relationships) using `*issue.Issue` directly

### Phase 4: GitHub adapter + detection
- `internal/integration/github_adapter.go`: Implements `Integration` interface with `Sync`, `Link`, `Unlink`, `Check`
- `Detect()` now checks for `extensions.github` with `repo` key

### Tests
- `config_test.go`: ParseConfig, ParseRepo, DefaultStatusMapping validation
- `sync_test.go`: computeLabels, buildIssueBody, getGitHubState, create/update/dry-run sync, FilterIssuesNeedingSync
