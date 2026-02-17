---
# beans-y8so
title: Add due field and due-date sorting
status: completed
type: feature
priority: normal
created_at: 2026-02-12T19:27:29Z
updated_at: 2026-02-12T19:32:35Z
sync:
    github:
        issue_number: "59"
        synced_at: "2026-02-17T18:33:09Z"
---

Add optional due date field to beans with YYYY-MM-DD format, sort by due date, CLI flags, and TUI support.

## Summary of Changes

- Added `DueDate` type with YAML/JSON marshaling (date-only `YYYY-MM-DD` format)
- Added `Due` field to `Bean`, `frontMatter`, and `renderFrontMatter` structs
- Added `SortByDueDate` function (soonest first, nil last, ties broken by title)
- Added `due` field to GraphQL schema (Bean type, CreateBeanInput, UpdateBeanInput)
- Implemented `Due` resolver and due date handling in Create/Update mutations
- Added `--due` CLI flag to `create` and `update` commands
- Added `due` sort option to `list --sort` and TUI sort picker
- Added due date display in `show` command
- Added comprehensive tests for parsing, rendering, roundtrip, sorting, and GraphQL
