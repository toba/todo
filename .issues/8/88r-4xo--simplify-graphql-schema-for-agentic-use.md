---
# 88r-4xo
title: Simplify GraphQL schema for agentic use
status: completed
type: task
priority: normal
created_at: 2026-02-16T23:18:45Z
updated_at: 2026-02-16T23:22:23Z
---

Remove 5 redundant standalone mutations (setParent, addBlocking, removeBlocking, addBlockedBy, removeBlockedBy) from GraphQL schema. Remove dead GraphQLSchema field from prime.go. Update tests.

## Summary of Changes

- Removed 5 redundant standalone mutations from GraphQL schema: setParent, addBlocking, removeBlocking, addBlockedBy, removeBlockedBy
- Removed corresponding resolver methods from schema.resolvers.go
- Updated TUI code (internal/tui/tui.go) to use updateIssue instead of standalone mutations
- Removed dead GraphQLSchema field from promptData struct in cmd/prime.go
- Removed tests for deleted standalone mutations from schema.resolvers_test.go
- Regenerated internal/graph/generated.go via mise codegen
