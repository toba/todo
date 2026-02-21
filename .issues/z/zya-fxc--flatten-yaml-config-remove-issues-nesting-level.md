---
# zya-fxc
title: 'Flatten YAML config: remove issues: nesting level'
status: completed
type: task
priority: normal
created_at: 2026-02-21T00:32:35Z
updated_at: 2026-02-21T00:36:42Z
---

- [x] Flatten Config struct in config.go
- [x] Update all references to cfg.Issues.X across codebase
- [x] Handle legacy migration
- [x] Update test YAML strings
- [x] Update .toba.yaml
- [x] Verify tests pass


## Summary of Changes

- Moved all `IssuesConfig` fields (`Path`, `DefaultStatus`, `DefaultType`, `DefaultSort`, `Editor`, `RequireIfMatch`) directly into `Config` struct
- Removed `IssuesConfig` struct and `Issues` field
- Added `legacyConfig` struct for parsing old `.todo.yml` format (which had `issues:` top-level key)
- Updated all references across 8 files: config.go, config_test.go, refry.go, tui_test.go, core.go, core_test.go, resolver.go, schema.resolvers_test.go
- Updated all test YAML strings to use flat format
- Updated `.toba.yaml` to new flat format
- `todo init` now creates flat format config
