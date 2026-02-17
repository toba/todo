---
# su6-yv7
title: Add sync footer note to externally created issues
status: completed
type: feature
priority: normal
created_at: 2026-02-17T18:26:10Z
updated_at: 2026-02-17T18:27:12Z
---

Issues synced to GitHub and ClickUp should include a visible attribution footer so viewers know the issue is managed externally and edits will be overwritten.

## Summary of Changes

- Added shared `SyncFooter` constant in `internal/integration/syncutil/footer.go`
- Updated GitHub `buildIssueBody()` to insert the footer between the body and the hidden comment
- Updated ClickUp `syncIssue()` to append the footer to the description
- Updated `TestBuildIssueBody` test expectations to include the footer
