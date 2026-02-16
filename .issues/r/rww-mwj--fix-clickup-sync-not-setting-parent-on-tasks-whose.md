---
# rww-mwj
title: Fix ClickUp sync not setting parent on tasks whose parent isn't in the sync batch
status: completed
type: bug
priority: normal
created_at: 2026-02-16T18:34:46Z
updated_at: 2026-02-16T18:57:42Z
---

When syncing a subset of issues to ClickUp, parent relationships are not set if the parent issue is not included in the sync batch. The issueToTaskID map is only pre-populated for issues in the current batch, so parents that were previously synced but aren't in the current batch are never resolved.

## Summary of Changes\n\nAdded parent pre-population in `SyncIssues` so that when a child issue's parent has been previously synced but isn't in the current batch, the parent's task ID is still resolved from the sync store. This ensures parent relationships are always set on ClickUp tasks.\n\n### Files modified\n- `internal/integration/clickup/sync.go` — resolve parent task IDs by loading parent issue from core store (not sync store, which only contains batch issues)\n- `internal/integration/clickup/sync_test.go` — added `TestSyncIssues_ParentNotInBatch` test
