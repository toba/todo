---
# beans-e28s
title: Support editor config field from .beans.yml
status: completed
type: feature
priority: normal
created_at: 2026-02-05T22:04:41Z
updated_at: 2026-02-05T22:05:59Z
sync:
    github:
        issue_number: "7"
        synced_at: "2026-02-17T18:33:09Z"
---

The editor field in .beans.yml is ignored. Add Editor field to BeansConfig, update getEditor() to use config as first priority, handle multi-word commands and relative paths, and update help text.
