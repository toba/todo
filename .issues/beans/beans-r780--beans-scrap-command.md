---
# beans-r780
title: beans scrap command
status: ready
type: task
priority: normal
created_at: 2025-12-27T21:44:04Z
updated_at: 2025-12-27T21:44:04Z
parent: beans-mmyp
sync:
    github:
        issue_number: "67"
        synced_at: "2026-02-17T18:33:10Z"
---

Add `beans scrap <id> --reason <text>` command.

## Behavior

- Sets status to `scrapped`
- **Required** `--reason` flag to document why the bean was scrapped
- Adds a `## Reason for Scrapping` section to the bean body (preserves project memory)
- Shows confirmation message

## Example

```bash
beans scrap beans-abc --reason "Superseded by beans-xyz approach"
```
