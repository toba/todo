---
# sxi-689
title: Fix TUI detail view selection resets on file watcher refresh
status: completed
type: bug
priority: normal
created_at: 2026-02-17T19:36:30Z
updated_at: 2026-02-17T19:38:20Z
sync:
    github:
        issue_number: "78"
        synced_at: "2026-02-17T19:44:07Z"
---

When viewing an issue's detail page and navigating the child/linked issues list, the cursor bounces back to the top item whenever the file watcher (or periodic tick) triggers a refresh.

## Tasks

- [x] Carry changed issue IDs in issuesChangedMsg
- [x] Add visibleIssueIDs() to detailModel
- [x] Add refreshIssue() to detailModel
- [x] Update issuesChangedMsg handler to filter by relevance
- [x] Update tickMsg handler to use refreshIssue()
- [x] Build and test


## Summary of Changes

Fixed cursor position resetting in TUI detail view on file watcher refresh and periodic tick. Changed `issuesChangedMsg` to carry specific changed issue IDs, added `visibleIssueIDs()` and `refreshIssue()` methods to `detailModel`, and updated all refresh paths to preserve cursor position and focus state.
