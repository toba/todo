---
# beans-ip6h
title: 'Optimize codebase: update to Go 1.26 and apply goptimize analysis'
status: completed
type: task
priority: normal
created_at: 2026-02-12T19:05:02Z
updated_at: 2026-02-12T19:16:47Z
sync:
    github:
        issue_number: "45"
        synced_at: "2026-02-17T18:33:09Z"
---

Update go.mod to Go 1.26, run go fix, analyze all six goptimize dimensions, and apply fixes.


## Summary of Changes

### Commit 1: `go fix` modernizers (14 files, -37 lines)
- Updated go.mod from Go 1.24.6 to Go 1.26
- Auto-applied: slices.Contains, range-over-int, wg.Go, new(expr), any, strings.Builder

### Commit 2: Manual goptimize findings (13 files, -154 lines net)
- **Modern Idioms**: sync.OnceValue, errors.AsType[T], slices.DeleteFunc, cmp.Or
- **Function Extraction**: validateETagLocked, CompareByStatusPriorityAndType, SortByEffectiveDate, Default*Names helpers
- **Constants/Enums**: DefaultIDLength, config.TypeTask/PriorityNormal/StatusTodo, bean.LinkType* constants, maxDescriptionLen
- **Deduplication**: Removed duplicate ETag error types from graph/resolver.go
