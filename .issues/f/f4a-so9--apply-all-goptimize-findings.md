---
# f4a-so9
title: Apply all goptimize findings
status: completed
type: task
priority: normal
created_at: 2026-02-16T03:18:06Z
updated_at: 2026-02-16T03:23:41Z
sync:
    github:
        issue_number: "46"
        synced_at: "2026-02-17T18:33:09Z"
---

Implement all 15 optimization findings from goptimize analysis:
- [x] Fix //go:fix inline misuse + consolidate ptr helpers into generic ptr[T] (N/A — new(expr) is valid Go 1.26 syntax; go fix auto-manages this)
- [x] Replace new(false) with ptr(false) in clickup/sync.go (N/A — valid Go 1.26)
- [~] Replace magic string literals with constants in sync files (skipped — import cycle prevents clickup/github from importing integration package)
- [x] Fix magic string "normal" in sort.go → config.PriorityNormal
- [x] Add field constants for created_at/updated_at → issue.FieldCreatedAt/FieldUpdatedAt
- [x] Add SortDefault constant → config.SortDefault
- [x] Remove dead SortByCreatedAt/SortByUpdatedAt wrappers → callers use SortByEffectiveDate directly
- [x] Consolidate config Names/IsValid/Get methods with generics → configNames/configFind/configIsValid/configList
- [~] Extract shared sync orchestration into syncutil (deferred — significant refactor with low ROI)
- [~] Unify sort logic between cmd/list.go and internal/issue/sort.go (deferred — CLI sort has different tie-breaking than TUI)
- [x] Remove duplicate Default*Names() functions → now delegate to configNames generic


## Summary of Changes

- Added generic config helpers (configNames, configFind, configIsValid, configList) consolidating 12+ methods into 4 generic functions
- Added constants: config.SortDefault, issue.FieldCreatedAt, issue.FieldUpdatedAt
- Replaced magic string "normal" with config.PriorityNormal in sort.go
- Removed dead SortByCreatedAt/SortByUpdatedAt wrappers; callers use SortByEffectiveDate directly
- Updated TUI and tests to use new constants and direct function calls
- Confirmed //go:fix inline + new(expr) is valid Go 1.26 syntax (editor diagnostics were false positives)
