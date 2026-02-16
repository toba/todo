---
# obh-s8c
title: Upload local images during sync
status: completed
type: feature
priority: normal
created_at: 2026-02-16T23:46:45Z
updated_at: 2026-02-16T23:50:50Z
---

Detect local image references in issue bodies (markdown and custom format), upload them to the sync target (ClickUp attachments or GitHub Contents API), and replace local paths with hosted URLs in both the synced copy and local issue file.

## Summary of Changes

- Created `internal/integration/syncutil/images.go` with shared image detection (`FindLocalImages`), replacement (`ReplaceImages`), and content hashing (`ContentHash`, `ImageFileName`) utilities
- Created `internal/integration/clickup/images.go` with `UploadAttachment` (multipart/form-data POST) and `UploadImages` orchestrator
- Created `internal/integration/github/images.go` with `GetContents`/`UploadContents` (Contents API) and `UploadImages` orchestrator using content-hashed filenames
- Integrated image upload into `clickup/sync.go` on both update and create paths
- Integrated image upload into `github/sync.go` before `buildIssueBody` (covers both update and create paths)
- Added comprehensive tests for all new code
