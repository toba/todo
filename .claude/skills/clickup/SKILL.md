---
name: clickup
description: ClickUp API reference for discovery and validation. Use when (1) inspecting ClickUp task state via API, (2) verifying sync results, (3) debugging parent/subtask relationships, (4) checking custom fields or statuses, (5) looking up workspace members. Triggers on ClickUp, task verification, sync validation.
---

# ClickUp API Reference

## Workspace

- **Team ID**: `9011518645`
- **List ID**: See project's `todo.yaml` sync config for the target list

## Authentication

API token from `CLICKUP_TOKEN` env var (personal token, starts with `pk_`).

**IMPORTANT:** The env var may contain a trailing newline. Always trim it:
```bash
TOKEN=$(printenv CLICKUP_TOKEN | tr -d '\n')
curl -s "https://api.clickup.com/api/v2/..." -H "Authorization: $TOKEN"
```

Base URL: `https://api.clickup.com/api/v2`

## Quick Reference

| Operation | Endpoint | Method |
|-----------|----------|--------|
| Get current user | `/user` | GET |
| List workspace members | `/team` | GET |
| Get task | `/task/{taskID}` | GET |
| Get task with subtasks | `/task/{taskID}?include_subtasks=true` | GET |
| Create task | `/list/{listID}/task` | POST |
| Update task | `/task/{taskID}` | PUT |
| Add comment | `/task/{taskID}/comment` | POST |
| Add dependency | `/task/{taskID}/dependency` | POST |
| List statuses | `/list/{listID}` | GET |
| List custom fields | `/list/{listID}/field` | GET |
| List task types | `/team/{teamID}/custom_item` | GET |

## Common Validation Commands

### Verify a task's parent
```bash
curl -s "https://api.clickup.com/api/v2/task/{TASK_ID}" \
  -H "Authorization: $CLICKUP_TOKEN" | jq '{id, name, parent, status: .status.status}'
```

### Verify subtasks
```bash
curl -s "https://api.clickup.com/api/v2/task/{TASK_ID}?include_subtasks=true" \
  -H "Authorization: $CLICKUP_TOKEN" | jq '.subtasks[] | {id, name, status: .status.status}'
```

### Get task details
```bash
curl -s "https://api.clickup.com/api/v2/task/{TASK_ID}" \
  -H "Authorization: $CLICKUP_TOKEN" | jq '{id, name, status: .status.status, url, parent}'
```

### List workspace members
```bash
curl -s "https://api.clickup.com/api/v2/team" \
  -H "Authorization: $CLICKUP_TOKEN" | jq '[.teams[].members[].user | {id, username, email}] | unique_by(.id)'
```

### List statuses for a list
```bash
curl -s "https://api.clickup.com/api/v2/list/{LIST_ID}" \
  -H "Authorization: $CLICKUP_TOKEN" | jq '.statuses[] | {status, orderindex}'
```

### List custom fields for a list
```bash
curl -s "https://api.clickup.com/api/v2/list/{LIST_ID}/field" \
  -H "Authorization: $CLICKUP_TOKEN" | jq '.fields[] | {id, name, type}'
```

## Priority Mapping

| Priority | ClickUp Value |
|----------|---------------|
| Urgent   | 1             |
| High     | 2             |
| Normal   | 3             |
| Low      | 4             |

## Subtasks

### Create subtask
Same as create task, but include `parent` field:
```bash
curl -s "https://api.clickup.com/api/v2/list/{LIST_ID}/task" \
  -H "Authorization: $CLICKUP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Subtask name",
    "parent": "PARENT_TASK_ID"
  }'
```

## Dependencies

### Add dependency (A blocked by B)
```bash
curl -s "https://api.clickup.com/api/v2/task/{TASK_A_ID}/dependency" \
  -H "Authorization: $CLICKUP_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"depends_on": "{TASK_B_ID}"}'
```

### Remove dependency
```bash
curl -s -X DELETE "https://api.clickup.com/api/v2/task/{TASK_A_ID}/dependency?depends_on={TASK_B_ID}" \
  -H "Authorization: $CLICKUP_TOKEN"
```

## Error Handling

| HTTP | Code | Meaning |
|------|------|---------|
| 401 | OAUTH_019 | Invalid/expired token |
| 404 | - | Task/list not found |
| 429 | APP_002 | Rate limited |
| 500+ | - | Transient server error |

## Rate Limits

Track via response headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1705315260
```

For 429 responses, use exponential backoff: 1s, 2s, 4s, 8s, 16s (max 5 retries).

## Todo Sync CLI

Use `todo sync` to sync issues to ClickUp:
```bash
todo sync                    # Sync modified issues
todo sync <id> [id...]       # Sync specific issues
todo sync --force <id>       # Force sync even if unchanged
todo sync link <id> <taskID> # Link issue to existing task
todo sync unlink <id>        # Unlink issue from task
todo sync check              # Validate sync configuration
```
