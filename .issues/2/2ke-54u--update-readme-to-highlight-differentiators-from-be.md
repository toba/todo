---
# 2ke-54u
title: Update README to highlight differentiators from beans
status: completed
type: task
priority: normal
created_at: 2026-02-16T23:10:16Z
updated_at: 2026-02-16T23:11:08Z
sync:
    github:
        issue_number: "35"
        synced_at: "2026-02-17T18:33:08Z"
---

Update the README to clearly distinguish todo from upstream beans:

- [x] Add "what's different" section after "why" (before Installation)
- [x] Fix stale bean/beans GraphQL references (bean→issue, beans→issues)
- [x] Consolidate the two overlapping Sync sections into one
- [x] Trim sample config to minimal (let sync section own sync YAML)
- [x] Fix Homebrew install command
- [x] Remove remaining bean/beans terminology in non-attribution contexts

## Summary of Changes

- Added "what's different" section listing external sync, due dates, and TUI improvements
- Fixed GraphQL examples: bean→issue, beans→issues
- Consolidated two overlapping Sync sections into one
- Trimmed sample config to just the issues block
- Fixed Homebrew install command: `brew install toba/todo/todo`
- Renamed `bean_id` to `issue_id` in ClickUp custom_fields example
