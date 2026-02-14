// Package client provides a Go API for interacting with the todo issue tracker.
//
// The client wraps the todo CLI, executing GraphQL queries via the
// "todo graphql" subcommand. It uses GraphQL variables for all parameterized
// queries, avoiding bugs caused by manually interpolating values into query
// strings (e.g., JSON quoted keys vs GraphQL input object literal syntax).
//
// Usage:
//
//	c := client.New(client.WithDataPath("/path/to/.todo"))
//	err := c.SetSyncData("todo-abc", "myext", map[string]any{"key": "val"})
package client

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Issue represents an issue as returned by the todo CLI JSON output.
// This struct matches the format used by "todo show --json" and "todo list --json".
type Issue struct {
	ID         string                    `json:"id"`
	Slug       string                    `json:"slug,omitempty"`
	Path       string                    `json:"path"`
	Title      string                    `json:"title"`
	Status     string                    `json:"status"`
	Type       string                    `json:"type,omitempty"`
	Priority   string                    `json:"priority,omitempty"`
	Tags       []string                  `json:"tags,omitempty"`
	CreatedAt  *time.Time                `json:"created_at,omitempty"`
	UpdatedAt  *time.Time                `json:"updated_at,omitempty"`
	Body       string                    `json:"body,omitempty"`
	Parent     string                    `json:"parent,omitempty"`
	Blocking   []string                  `json:"blocking,omitempty"`
	BlockedBy  []string                  `json:"blocked_by,omitempty"`
	Sync map[string]map[string]any `json:"sync,omitempty"`
	ETag       string                    `json:"etag"`
}

// SyncDataOp represents a single set-sync-data operation for batch updates.
type SyncDataOp struct {
	ID   string
	Name string
	Data map[string]any
}

// Client provides typed access to the todo issue tracker via its CLI.
//
// All parameterized queries use GraphQL variables (-v flag) to avoid
// formatting issues with literal values in query strings.
type Client struct {
	dataPath string
	binPath  string

	// newCmd is overridable for testing. When nil, exec.Command is used.
	newCmd func(name string, args ...string) *exec.Cmd
}

// Option configures a Client.
type Option func(*Client)

// WithDataPath sets the --data-path flag for all CLI invocations.
func WithDataPath(path string) Option {
	return func(c *Client) { c.dataPath = path }
}

// WithBinPath overrides the todo binary path (default: "todo").
func WithBinPath(path string) Option {
	return func(c *Client) { c.binPath = path }
}

// New creates a new Client.
func New(opts ...Option) *Client {
	c := &Client{binPath: "todo"}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Query executes a GraphQL query against the issues data store and returns
// the raw JSON response (the data portion, without a "data" wrapper).
//
// Variables are passed via the -v flag as JSON, which lets the GraphQL
// engine handle value formatting correctly. This avoids bugs caused by
// manually interpolating values into query strings.
func (c *Client) Query(query string, variables map[string]any) ([]byte, error) {
	args := []string{"graphql", "--json"}
	if c.dataPath != "" {
		args = append(args, "--data-path", c.dataPath)
	}
	if len(variables) > 0 {
		v, err := json.Marshal(variables)
		if err != nil {
			return nil, fmt.Errorf("marshaling variables: %w", err)
		}
		args = append(args, "-v", string(v))
	}
	args = append(args, query)

	mkCmd := exec.Command
	if c.newCmd != nil {
		mkCmd = c.newCmd
	}
	cmd := mkCmd(c.binPath, args...)

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("todo query: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("todo query: %w", err)
	}
	return out, nil
}

// SetSyncData sets sync data for an issue. The data map fully
// replaces any existing sync data for the given name.
func (c *Client) SetSyncData(id, name string, data map[string]any) error {
	const query = `mutation ($id: ID!, $name: String!, $data: Map!) {
  setSyncData(id: $id, name: $name, data: $data) { id }
}`
	_, err := c.Query(query, map[string]any{
		"id":   id,
		"name": name,
		"data": data,
	})
	return err
}

// SetSyncDataBatch sets sync data for multiple issues in a single
// GraphQL request using aliased mutations and numbered variables.
func (c *Client) SetSyncDataBatch(ops []SyncDataOp) error {
	if len(ops) == 0 {
		return nil
	}

	var defs []string
	var fields []string
	vars := make(map[string]any, len(ops)*3)

	for i, op := range ops {
		defs = append(defs,
			fmt.Sprintf("$id%d: ID!", i),
			fmt.Sprintf("$name%d: String!", i),
			fmt.Sprintf("$data%d: Map!", i),
		)
		fields = append(fields, fmt.Sprintf(
			"op%d: setSyncData(id: $id%d, name: $name%d, data: $data%d) { id }",
			i, i, i, i,
		))
		vars[fmt.Sprintf("id%d", i)] = op.ID
		vars[fmt.Sprintf("name%d", i)] = op.Name
		vars[fmt.Sprintf("data%d", i)] = op.Data
	}

	query := fmt.Sprintf("mutation (%s) {\n  %s\n}",
		strings.Join(defs, ", "),
		strings.Join(fields, "\n  "),
	)

	_, err := c.Query(query, vars)
	return err
}

// RemoveSyncData removes sync data for a named extension from an issue.
func (c *Client) RemoveSyncData(id, name string) error {
	const query = `mutation ($id: ID!, $name: String!) {
  removeSyncData(id: $id, name: $name) { id }
}`
	_, err := c.Query(query, map[string]any{
		"id":   id,
		"name": name,
	})
	return err
}
