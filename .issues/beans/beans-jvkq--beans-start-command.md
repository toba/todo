---
# beans-jvkq
title: beans start command
status: ready
type: task
priority: normal
created_at: 2025-12-27T21:44:04Z
updated_at: 2025-12-27T21:44:04Z
parent: beans-mmyp
sync:
    github:
        issue_number: "68"
        synced_at: "2026-02-17T18:33:10Z"
---

Add `beans start <id>` command.

## Behavior

- Sets status to `in-progress`
- Displays the bean details (like `beans show`) so you can see what you're working on
- Could optionally check if another bean is already in-progress and warn

## Example

```bash
beans start beans-abc
# Sets to in-progress and shows the bean
```
