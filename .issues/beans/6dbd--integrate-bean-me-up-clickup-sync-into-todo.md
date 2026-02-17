---
# 6dbd
title: Integrate bean-me-up ClickUp sync into todo
status: completed
type: feature
priority: normal
created_at: 2026-02-14T19:28:07Z
updated_at: 2026-02-14T19:36:40Z
sync:
    github:
        issue_number: "47"
        synced_at: "2026-02-17T18:33:09Z"
---

Port the ClickUp sync functionality from toba/bean-me-up into toba/todo as a built-in integration.

## Summary of Changes

Ported the ClickUp sync functionality from toba/bean-me-up into toba/todo as a built-in integration.

### New files created:

**Phase 1 - Foundation:**
- `internal/integration/clickup/types.go` - ClickUp API data structures (verbatim port)
- `internal/integration/clickup/client.go` - REST API client with retry logic (verbatim port)
- `internal/integration/clickup/config.go` - Config parsing from `map[string]any` (rewritten)
- `internal/integration/clickup/config_test.go` - Config parsing tests

**Phase 2 - Core sync logic:**
- `internal/integration/clickup/sync_state.go` - Sync state via `core.SaveExtensionOnly()` (adapted)
- `internal/integration/clickup/sync.go` - Sync engine using `*issue.Issue` directly (adapted)
- `internal/integration/clickup/sync_test.go` - Sync tests (adapted)
- `internal/integration/integration.go` - Integration interface + `Detect()`
- `internal/integration/clickup_adapter.go` - ClickUp adapter implementing Integration interface

**Phase 3 - CLI commands:**
- `cmd/sync.go` - Parent command + main sync RunE
- `cmd/sync_link.go` - Link subcommand
- `cmd/sync_unlink.go` - Unlink subcommand
- `cmd/sync_check.go` - Check subcommand

### Key adaptations from bean-me-up:
- `[]beans.Bean` → `[]*issue.Issue`
- `b.Due *string` → `b.Due *issue.DueDate` (use `.Time` field directly)
- Subprocess-based extension writes → `core.SaveExtensionOnly()` direct calls
- `config.Load()` from YAML → `ParseConfig(map[string]any)` from `cfg.ExtensionConfig()`
- `fatih/color` → `internal/ui` lipgloss styles
- Status default mapping updated: `ready` → `to do` (was `todo` → `to do`)

### No existing files modified. No new dependencies added.
