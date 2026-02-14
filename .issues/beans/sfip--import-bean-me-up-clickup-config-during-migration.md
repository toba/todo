---
# sfip
title: Import bean-me-up ClickUp config during migration
status: completed
type: feature
priority: normal
created_at: 2026-02-14T20:08:30Z
updated_at: 2026-02-14T20:10:35Z
---

Make `beans migrate` automatically detect and import ClickUp config from either `.beans.clickup.yml` (standalone) or `.beans.yml` (inline extensions.clickup section).

## Tasks
- [x] Add `ClickUpImported` to `Result` struct
- [x] Add `ImportClickUpConfig` function and helpers
- [x] Update `Run()` to call `ImportClickUpConfig`
- [x] Update `cmd/migrate.go` output
- [x] Add unit tests in `migrate_test.go`
- [x] Add integration test in `cmd/migrate_test.go`
- [x] All tests pass


## Summary of Changes

- Added `ClickUpImported` field to `Result` struct
- Added `ImportClickUpConfig()` function that searches for ClickUp config in `.beans.clickup.yml` (standalone) and `.beans.yml` (inline), with standalone taking priority
- Added helpers: `extractClickUpFromStandalone`, `extractClickUpFromExtensions`, `convertStatusMappingKeys`
- Updated `Run()` to call `ImportClickUpConfig()` after `MigrateConfig()`
- Updated `cmd/migrate.go` to display ClickUp import status in both human-readable and JSON output
- Added comprehensive unit tests for all new functions and edge cases
- Added integration test in `cmd/migrate_test.go`
