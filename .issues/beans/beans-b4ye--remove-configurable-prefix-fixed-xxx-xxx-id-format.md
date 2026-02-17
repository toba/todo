---
# beans-b4ye
title: Remove configurable prefix, fixed xxx-xxx ID format, hash subfolders
status: completed
type: feature
priority: normal
created_at: 2026-02-14T18:21:07Z
updated_at: 2026-02-14T18:32:24Z
sync:
    github:
        issue_number: "12"
        synced_at: "2026-02-17T18:33:08Z"
---

Remove configurable prefix and IDLength from config. Change ID format to fixed xxx-xxx (3+hyphen+3). Store new beans in 1-char hash subfolders.

## Tasks
- [x] Update internal/issue/id.go: NewID() and add BuildPath()
- [x] Update internal/issue/id_test.go
- [x] Update internal/config/config.go: remove Prefix, IDLength, DefaultIDLength, DefaultWithPrefix
- [x] Update internal/config/config_test.go
- [x] Update internal/core/core.go: simplify Create/Get/Delete/NormalizeID, use BuildPath in saveToDisk, fix Unarchive
- [x] Update internal/core/core_test.go
- [x] Update internal/graph/schema.graphqls: remove prefix field
- [x] Run mise codegen
- [x] Update internal/graph/schema.resolvers.go: remove prefix handling
- [x] Update internal/graph/schema.resolvers_test.go
- [x] Update cmd/init.go
- [x] Update cmd/create.go
- [x] Update cmd/prompt.tmpl
- [x] Update .beans.yml
- [x] Update README.md
- [x] Run tests


## Summary of Changes

Removed configurable prefix and ID length from the config system. IDs now use a fixed `xxx-xxx` format (3 random chars + hyphen + 3 random chars). New beans are stored in 1-character hash-prefixed subfolders (e.g., `a/abc-def--my-slug.md`) for better filesystem organization. Existing `beans-xxxx` files remain loadable via recursive directory walking.

Key changes:
- `NewID()` takes no arguments, returns `xxx-xxx` format
- `BuildPath()` generates hash-prefixed relative paths
- Removed `Prefix`, `IDLength`, `DefaultIDLength`, `DefaultWithPrefix` from config
- Simplified `Get()`, `Delete()`, `NormalizeID()`, `findBeanLocked()` to exact match only
- `Unarchive()`/`LoadAndUnarchive()` now restore to hash subfolders
- Removed `prefix` field from GraphQL `CreateIssueInput`
- Removed `--prefix` CLI flag from `create` command
- Updated all tests
