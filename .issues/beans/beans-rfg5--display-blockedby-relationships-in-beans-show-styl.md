---
# beans-rfg5
title: Display blockedBy relationships in beans show styled output
status: completed
type: bug
priority: normal
created_at: 2026-02-09T16:40:50Z
updated_at: 2026-02-09T16:40:55Z
sync:
    github:
        issue_number: "36"
        synced_at: "2026-02-17T18:33:09Z"
---

The beans show command's styled (human-readable) output renders parent and blocking relationships but completely ignores blockedBy. Only the JSON output works correctly. This caused agents to miss dependency information when inspecting beans.

## Summary of Changes\n\n- Expanded guard condition in `showStyledBean` to include `BlockedBy`\n- Added `blocked by:` rendering loop in `formatRelationships()`\n- Added unit tests in `cmd/show_test.go` covering all relationship combinations
