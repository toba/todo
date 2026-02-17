---
# beans-jigt
title: Fix deep search pointer invalidation bug
status: completed
type: bug
priority: normal
created_at: 2026-02-05T19:51:02Z
updated_at: 2026-02-05T19:55:21Z
sync:
    github:
        issue_number: "5"
        synced_at: "2026-02-17T18:33:08Z"
---

The deep search feature doesn't work because beanItem holds `*bool` pointing to `&m.deepSearch` where `m` is the listModel value receiver. Since Bubble Tea uses value receivers, each Update() call gets a new copy of `m`, so toggling `m.deepSearch` doesn't affect the pointer stored in the items (which points to the old copy's field).

Fix: change `deepSearch` from a plain `bool` to a `*bool` (heap-allocated pointer) so all copies share the same underlying bool.

## Checklist
- [ ] Write a test that demonstrates the bug (pointer invalidation across value copies)
- [ ] Fix the bug by using a heap-allocated `*bool` for deepSearch in listModel
- [ ] Verify existing tests still pass
- [ ] Verify the new test passes
