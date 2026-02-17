---
# uwb-lbw
title: 'Fix GitHub sync: use native types, remove label abuse'
status: completed
type: task
priority: normal
created_at: 2026-02-15T05:06:38Z
updated_at: 2026-02-15T05:09:31Z
sync:
    github:
        issue_number: "29"
        synced_at: "2026-02-17T18:33:09Z"
---

The GitHub sync maps types, priorities, and statuses to labels. GitHub now has a native type field on issues via REST API — use that instead. Priority and status should not be labels at all; only tags should become labels.

Also fix stale "todo" status key → "ready" in DefaultStatusMapping.

## Tasks
- [x] Add Type field to CreateIssueRequest and UpdateIssueRequest in types.go
- [x] Add IssueType struct and Type field to Issue response struct
- [x] Add Type to hasChanges() check
- [x] Change DefaultTypeMapping values to GitHub native type names (Bug, Feature, Task)
- [x] Delete DefaultPriorityMapping
- [x] Simplify DefaultStatusMapping to map[string]string (status → open/closed)
- [x] Delete StatusTarget struct
- [x] Fix stale "todo" key → "ready"
- [x] Simplify computeLabels() to return only tags
- [x] Delete getStatusLabel()
- [x] Simplify getGitHubState() to use new mapping
- [x] Add getGitHubType() function
- [x] Update syncIssue() create path to set Type on request
- [x] Update buildUpdateRequest() to compare and set Type
- [x] Update all tests

## Summary of Changes

- Replaced `StatusTarget` struct with simple `map[string]string` for status→state mapping
- Fixed stale "todo" status key → "ready" in `DefaultStatusMapping`
- Deleted `DefaultPriorityMapping` — priorities no longer create labels
- Changed `DefaultTypeMapping` to map to GitHub native type names (Bug, Feature, Task)
- Added `IssueType` struct and `Type` field to `Issue` response struct
- Added `Type` field to `CreateIssueRequest` and `UpdateIssueRequest`
- Simplified `computeLabels()` to return only tags
- Deleted `getStatusLabel()`, added `getGitHubType()`
- Updated `buildUpdateRequest()` to diff and set type changes
- Updated all tests for new behavior
