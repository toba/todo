---
# beans-7sca
title: Add external integration metadata to beans
status: completed
type: feature
priority: normal
created_at: 2026-02-08T21:34:34Z
updated_at: 2026-02-08T21:34:46Z
sync:
    github:
        issue_number: "50"
        synced_at: "2026-02-17T18:33:09Z"
---

Add an `external` field to bean frontmatter that plugins can use to store arbitrary metadata (external IDs, sync timestamps, etc.), along with dedicated GraphQL mutations and filters to support sync workflows.

## Summary of Changes

- **`internal/bean/bean.go`**: Added `External map[string]map[string]any` field to `Bean`, `frontMatter`, and `renderFrontMatter` structs. Propagated through `Parse()` and `Render()`. Added helper methods: `HasExternal(plugin)`, `SetExternal(plugin, data)`, `RemoveExternal(plugin)`.
- **`internal/bean/bean_test.go`**: Added tests for parse/render/roundtrip with external data, helper methods, and ETag changes.
- **`internal/graph/schema.graphqls`**: Added `Map` scalar, `ExternalEntry` type, `external` field on `Bean`, `setExternalData`/`removeExternalData` mutations, and filter fields (`hasExternal`, `noExternal`, `externalStale`, `changedSince`).
- **`gqlgen.yml`**: Added `Map` scalar model mapping.
- **`internal/graph/generated.go`** and **`internal/graph/model/models_gen.go`**: Regenerated via codegen.
- **`internal/graph/schema.resolvers.go`**: Implemented `External` field resolver (sorted by plugin name), `SetExternalData` and `RemoveExternalData` mutations.
- **`internal/graph/filters.go`**: Added `filterByHasExternal`, `filterByNoExternal`, `filterByExternalStale`, `filterByChangedSince` functions wired into `ApplyFilter()`.
- **`internal/graph/filters_test.go`**: Unit tests for all four new filter functions.
- **`internal/graph/schema.resolvers_test.go`**: Integration tests for external resolver, mutations, and query filters.
