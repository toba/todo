---
# rmw-6tw
title: Fix sync to update parent/subtask relationships on existing ClickUp tasks
status: completed
type: bug
priority: normal
created_at: 2026-02-16T18:18:39Z
updated_at: 2026-02-16T18:19:40Z
---

When syncing issues to ClickUp, the parent-child relationship is only set during task creation. If an issue gains a parent after the ClickUp task already exists, the update path doesn't reparent it. Need to add parent comparison in buildUpdateRequest.

## Todo

- [x] Add parent comparison logic in `buildUpdateRequest`
- [x] Handle reparenting (setting parent) on updates
- [x] Handle unparenting (removing parent) on updates
- [x] Write tests
- [ ] Manual verification

## Summary of Changes

Added parent-child relationship syncing on task updates. Previously, parent was only set during task creation in ClickUp. Now `buildUpdateRequest` compares the current ClickUp parent with the expected parent (resolved via `issueToTaskID` map) and includes a parent update when they differ. Added `stringPtrEqual` helper and table-driven tests.
