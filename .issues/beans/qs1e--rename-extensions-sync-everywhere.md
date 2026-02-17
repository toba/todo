---
# qs1e
title: Rename extensions → sync everywhere
status: completed
type: task
priority: normal
created_at: 2026-02-14T20:27:16Z
updated_at: 2026-02-14T20:37:21Z
sync:
    github:
        issue_number: "2"
        synced_at: "2026-02-17T18:33:08Z"
---

Mechanical rename of extensions to sync across all layers: YAML keys, Go struct fields/methods, GraphQL schema, and the migrate command.


## Summary of Changes

Mechanical rename of `extensions` → `sync` across all layers:

- **Config**: `Config.Extensions` → `Config.Sync`, `ExtensionConfig()` → `SyncConfig()`, YAML tag `extensions` → `sync`
- **Issue**: `Issue.Extensions` → `Issue.Sync`, `HasExtension/SetExtension/RemoveExtension` → `HasSync/SetSync/RemoveSync`
- **Core**: `SaveExtensionOnly()` → `SaveSyncOnly()`
- **GraphQL schema**: `extensions` field → `sync`, `ExtensionEntry` → `SyncEntry`, `setExtensionData/removeExtensionData` → `setSyncData/removeSyncData`, filters `hasExtension/noExtension/extensionStale` → `hasSync/noSync/syncStale`
- **Graph resolvers + filters**: all renamed to match schema
- **Integration layer**: `ExtensionName` → `SyncName`, `ExtKey*` → `SyncKey*`, `ExtensionSyncProvider` → `SyncStateStore`, `GetExtensionString/Time/Int` → `GetSyncString/Time/Int`
- **Client package**: `ExtensionDataOp` → `SyncDataOp`, `SetExtensionData/Batch/RemoveExtensionData` → `SetSyncData/Batch/RemoveSyncData`
- **Cmd layer**: all `cfg.Extensions` → `cfg.Sync`, user-facing messages updated
- **Migration**: `MigrateConfig` now renames `extensions:` → `sync:` in config YAML; new `rewriteExtensionsKey()` renames `extensions:` → `sync:` in issue frontmatter during file migration; `ImportClickUpConfig` writes to `sync.clickup` instead of `extensions.clickup`
- **README**: all YAML examples and prose updated
- **Tests**: all test functions, fixtures, and assertions updated
