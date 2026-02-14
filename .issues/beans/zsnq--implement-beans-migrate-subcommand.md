---
# zsnq
title: Implement beans migrate subcommand
status: completed
type: feature
priority: normal
created_at: 2026-02-14T19:00:25Z
updated_at: 2026-02-14T19:04:51Z
---

Implement the `beans migrate` CLI command to convert old-format beans projects to the new structure.


## Summary of Changes

- Added `internal/migrate/migrate.go` with core migration logic:
  - Config rewriting (`beans:` → `issues:`, drop `prefix`/`id_length`, `default_status: todo` → `ready`, `path: .beans` → `.issues`)
  - Bean file copying with targeted frontmatter `status: todo` → `status: ready` rewrite
  - Archive directory migration
  - Safety check to prevent overwriting existing data
- Added `cmd/migrate.go` CLI command with `--source` and `--json` flags
- Added `internal/migrate/migrate_test.go` with comprehensive unit tests
- Added `cmd/migrate_test.go` with end-to-end integration test
- Modified `cmd/root.go` to skip core initialization for migrate command
