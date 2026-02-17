---
# beans-gna6
title: Cherry-pick upstream atomic relationship updates (6beaf6a)
status: completed
type: task
priority: normal
created_at: 2026-02-10T18:03:30Z
updated_at: 2026-02-10T18:05:40Z
sync:
    github:
        issue_number: "22"
        synced_at: "2026-02-17T18:33:09Z"
---

Cherry-pick commit 6beaf6a from hmans/beans which adds atomic relationship updates to updateBean mutation. Resolve conflicts and fix import paths.

## Summary of Changes

Cherry-picked upstream commit 6beaf6a (feat: atomic relationship updates in updateBean) from hmans/beans.

- Adds `parent`, `addBlocking`, `removeBlocking`, `addBlockedBy`, `removeBlockedBy` fields to `UpdateBeanInput` GraphQL schema
- Simplifies CLI `cmd/update.go` to use a single atomic mutation for all relationship changes
- Adds validation helpers in `resolver.go` for parent, blocking, and blocked-by relationships
- Adds 16 relationship test cases in `schema.resolvers_test.go`
- Resolved 2 merge conflicts (kept fork's extension helpers/tests alongside upstream's relationship helpers/tests)
- All import paths already correct (toba/todo)
