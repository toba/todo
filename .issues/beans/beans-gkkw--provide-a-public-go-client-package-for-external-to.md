---
# beans-gkkw
title: Provide a public Go client package for external tools
status: completed
type: feature
priority: normal
created_at: 2026-02-08T23:16:38Z
updated_at: 2026-02-08T23:25:13Z
---

## Problem

External tools like bean-me-up that integrate with beans currently have to:
1. Shell out to the `beans` CLI via `exec.Command`
2. Construct raw GraphQL query strings with `fmt.Sprintf`
3. Manually handle GraphQL syntax (e.g. unquoted keys in input objects vs JSON quoted keys)

This led to a bug where `json.Marshal` was used to format a `Map!` argument, producing JSON with quoted keys (`{"task_id": "123"}`) instead of GraphQL input object literal syntax (`{task_id: "123"}`), causing a parse error.

Everything in beans is currently under `internal/`, so there's nothing for external Go tools to import.

## Proposal

Add a public Go client package (e.g. `pkg/client`) that wraps CLI invocation and provides typed methods. External tools import it instead of constructing raw GraphQL strings.

### Suggested API surface

```go
package client

type Client struct { ... }

func New(opts ...Option) *Client
// Options: WithBeansPath(string)

// Extension data
func (c *Client) SetExtensionData(id, name string, data map[string]any) error
func (c *Client) SetExtensionDataBatch(ops []ExtensionDataOp) error
func (c *Client) RemoveExtensionData(id, name string) error

// Raw GraphQL (escape hatch)
func (c *Client) Query(query string, variables map[string]any) ([]byte, error)
```

### Notes

- The client should use GraphQL variables (`-v` flag) internally to avoid literal formatting issues entirely
- Consider also exporting the Bean struct so consumers don't need to redefine it
- Keep the package minimal — only expose what external tools actually need today


## Summary of Changes

Added `pkg/client` package — the first public Go package in the beans module. External tools can now import `github.com/toba/todo/pkg/client` instead of constructing raw GraphQL strings or shelling out manually.

### API surface

- `client.New(opts ...Option) *Client` — create a client with optional `WithBeansPath` and `WithBinPath`
- `Client.Query(query, variables) ([]byte, error)` — raw GraphQL escape hatch using variables via `-v` flag
- `Client.SetExtensionData(id, name, data) error` — set extension data with proper variable passing
- `Client.SetExtensionDataBatch(ops) error` — batch set via aliased mutations with numbered variables
- `Client.RemoveExtensionData(id, name) error` — remove extension data
- `client.Bean` struct — matches CLI JSON output format for easy deserialization

All parameterized queries use GraphQL variables, eliminating the class of bugs where `json.Marshal` produces JSON quoted keys instead of GraphQL input object literal syntax.
