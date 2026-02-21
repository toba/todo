---
# het-15f
title: 'Rename .todo.yml to .toba.yaml with todo: section nesting'
status: completed
type: task
priority: normal
created_at: 2026-02-20T23:41:42Z
updated_at: 2026-02-20T23:47:33Z
sync:
    github:
        issue_number: "79"
        synced_at: "2026-02-21T00:25:14Z"
---

- [x] Update config.go core loading/saving with TobaConfig wrapper and legacy migration
- [x] Update CLI help text references across cmd/*.go
- [x] Update prompt.tmpl references
- [x] Remove migrate package (obsolete beans→issues migration)
- [x] Update project's own config file (.todo.yml → .toba.yaml)
- [x] Update README.md
- [x] Update tests
- [x] Verify all tests pass


## Summary of Changes

Renamed config from .todo.yml to .toba.yaml with a `todo:` top-level wrapper key. Added TobaConfig wrapper struct, auto-migration from legacy format in FindConfig, and dual-format Load support. Removed the obsolete beans→issues migrate command and package.
