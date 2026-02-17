---
# beans-0elf
title: Add --blocked and --ready flags to beans list
status: completed
type: feature
priority: normal
created_at: 2025-12-28T11:44:10Z
updated_at: 2025-12-28T11:46:51Z
sync:
    github:
        issue_number: "23"
        synced_at: "2026-02-17T18:33:09Z"
---

Add --ready convenience flag to beans list command:
- --ready: actionable beans (not blocked, excludes completed/scrapped/draft)

This avoids adding new top-level commands while providing easy workflow shortcuts.

Refs: beans-7kb7, beans-8q44
