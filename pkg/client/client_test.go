package client

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// mockCmd returns a newCmd function that captures args and returns preset output.
func mockCmd(output string) (func(string, ...string) *exec.Cmd, *[][]string) {
	var calls [][]string
	fn := func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		return exec.Command("echo", output)
	}
	return fn, &calls
}

func TestNew_Defaults(t *testing.T) {
	c := New()
	if c.binPath != "todo" {
		t.Errorf("default binPath = %q, want %q", c.binPath, "todo")
	}
	if c.dataPath != "" {
		t.Errorf("default dataPath = %q, want empty", c.dataPath)
	}
}

func TestNew_WithOptions(t *testing.T) {
	c := New(WithBinPath("/usr/local/bin/todo"), WithDataPath("/tmp/test"))
	if c.binPath != "/usr/local/bin/todo" {
		t.Errorf("binPath = %q, want %q", c.binPath, "/usr/local/bin/todo")
	}
	if c.dataPath != "/tmp/test" {
		t.Errorf("dataPath = %q, want %q", c.dataPath, "/tmp/test")
	}
}

func TestQuery_BasicArgs(t *testing.T) {
	fn, calls := mockCmd(`{"todo":[]}`)
	c := New()
	c.newCmd = fn

	_, err := c.Query(`{ issues { id } }`, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(*calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(*calls))
	}
	args := (*calls)[0]

	if args[0] != "todo" {
		t.Errorf("binary = %q, want %q", args[0], "todo")
	}
	if args[1] != "graphql" || args[2] != "--json" {
		t.Errorf("args[1:3] = %v, want [graphql --json]", args[1:3])
	}
	if args[len(args)-1] != `{ issues { id } }` {
		t.Errorf("last arg = %q, want query string", args[len(args)-1])
	}
}

func TestQuery_NoVariables(t *testing.T) {
	fn, calls := mockCmd(`{}`)
	c := New()
	c.newCmd = fn

	_, _ = c.Query(`{ issues { id } }`, nil)

	args := (*calls)[0]
	for _, a := range args {
		if a == "-v" {
			t.Error("-v flag should not be present when variables are nil")
		}
	}
}

func TestQuery_WithDataPath(t *testing.T) {
	fn, calls := mockCmd(`{}`)
	c := New(WithDataPath("/tmp/test"))
	c.newCmd = fn

	_, _ = c.Query(`{ issues { id } }`, nil)

	args := (*calls)[0]
	found := false
	for i, a := range args {
		if a == "--data-path" && i+1 < len(args) && args[i+1] == "/tmp/test" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("--data-path /tmp/test not found in args: %v", args)
	}
}

func TestQuery_WithBinPath(t *testing.T) {
	fn, calls := mockCmd(`{}`)
	c := New(WithBinPath("/opt/todo"))
	c.newCmd = fn

	_, _ = c.Query(`{ issues { id } }`, nil)

	if (*calls)[0][0] != "/opt/todo" {
		t.Errorf("binary = %q, want %q", (*calls)[0][0], "/opt/todo")
	}
}

func TestQuery_WithVariables(t *testing.T) {
	fn, calls := mockCmd(`{"issue":{"id":"abc"}}`)
	c := New()
	c.newCmd = fn

	vars := map[string]any{"id": "abc", "name": "test"}
	_, err := c.Query(`query ($id: ID!) { issue(id: $id) { id } }`, vars)
	if err != nil {
		t.Fatal(err)
	}

	args := (*calls)[0]
	var varsJSON string
	for i, a := range args {
		if a == "-v" && i+1 < len(args) {
			varsJSON = args[i+1]
			break
		}
	}
	if varsJSON == "" {
		t.Fatalf("-v flag not found in args: %v", args)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(varsJSON), &parsed); err != nil {
		t.Fatalf("invalid variables JSON %q: %v", varsJSON, err)
	}
	if parsed["id"] != "abc" {
		t.Errorf("variables[id] = %v, want abc", parsed["id"])
	}
	if parsed["name"] != "test" {
		t.Errorf("variables[name] = %v, want test", parsed["name"])
	}
}

func TestQuery_ErrorHandling(t *testing.T) {
	c := New(WithBinPath("false")) // "false" exits with code 1
	_, err := c.Query(`{ issues { id } }`, nil)
	if err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestSetSyncData(t *testing.T) {
	fn, calls := mockCmd(`{"setSyncData":{"id":"test-abc"}}`)
	c := New()
	c.newCmd = fn

	err := c.SetSyncData("test-abc", "myext", map[string]any{"key": "val"})
	if err != nil {
		t.Fatal(err)
	}

	args := (*calls)[0]

	// Last arg should be the mutation query
	query := args[len(args)-1]
	if !strings.Contains(query, "setSyncData") {
		t.Errorf("query should contain setSyncData: %s", query)
	}
	if !strings.Contains(query, "$id") || !strings.Contains(query, "$name") || !strings.Contains(query, "$data") {
		t.Errorf("query should use GraphQL variables ($id, $name, $data): %s", query)
	}

	// Verify variables are correct
	vars := extractVars(t, args)
	if vars["id"] != "test-abc" {
		t.Errorf("vars[id] = %v, want test-abc", vars["id"])
	}
	if vars["name"] != "myext" {
		t.Errorf("vars[name] = %v, want myext", vars["name"])
	}
	data, ok := vars["data"].(map[string]any)
	if !ok {
		t.Fatalf("vars[data] type = %T, want map[string]any", vars["data"])
	}
	if data["key"] != "val" {
		t.Errorf("vars[data][key] = %v, want val", data["key"])
	}
}

func TestSetSyncDataBatch_Empty(t *testing.T) {
	err := New().SetSyncDataBatch(nil)
	if err != nil {
		t.Errorf("empty batch should return nil, got: %v", err)
	}
}

func TestSetSyncDataBatch(t *testing.T) {
	fn, calls := mockCmd(`{"op0":{"id":"a"},"op1":{"id":"b"}}`)
	c := New()
	c.newCmd = fn

	ops := []SyncDataOp{
		{ID: "a", Name: "ext1", Data: map[string]any{"k1": "v1"}},
		{ID: "b", Name: "ext2", Data: map[string]any{"k2": "v2"}},
	}
	err := c.SetSyncDataBatch(ops)
	if err != nil {
		t.Fatal(err)
	}

	args := (*calls)[0]
	query := args[len(args)-1]

	// Should have aliased operations
	if !strings.Contains(query, "op0: setSyncData") {
		t.Errorf("query missing op0 alias: %s", query)
	}
	if !strings.Contains(query, "op1: setSyncData") {
		t.Errorf("query missing op1 alias: %s", query)
	}
	// Should use numbered variables
	if !strings.Contains(query, "$id0") || !strings.Contains(query, "$id1") {
		t.Errorf("query should use numbered variables: %s", query)
	}
	if !strings.Contains(query, "$data0") || !strings.Contains(query, "$data1") {
		t.Errorf("query should use numbered data variables: %s", query)
	}

	// Verify variables
	vars := extractVars(t, args)
	if vars["id0"] != "a" || vars["id1"] != "b" {
		t.Errorf("id vars = %v/%v, want a/b", vars["id0"], vars["id1"])
	}
	if vars["name0"] != "ext1" || vars["name1"] != "ext2" {
		t.Errorf("name vars = %v/%v, want ext1/ext2", vars["name0"], vars["name1"])
	}
}

func TestRemoveSyncData(t *testing.T) {
	fn, calls := mockCmd(`{"removeSyncData":{"id":"test-abc"}}`)
	c := New()
	c.newCmd = fn

	err := c.RemoveSyncData("test-abc", "myext")
	if err != nil {
		t.Fatal(err)
	}

	args := (*calls)[0]
	query := args[len(args)-1]

	if !strings.Contains(query, "removeSyncData") {
		t.Errorf("query should contain removeSyncData: %s", query)
	}
	if !strings.Contains(query, "$id") || !strings.Contains(query, "$name") {
		t.Errorf("query should use GraphQL variables: %s", query)
	}
	// Should NOT reference $data
	if strings.Contains(query, "$data") {
		t.Errorf("removeSyncData query should not reference $data: %s", query)
	}

	vars := extractVars(t, args)
	if vars["id"] != "test-abc" {
		t.Errorf("vars[id] = %v, want test-abc", vars["id"])
	}
	if vars["name"] != "myext" {
		t.Errorf("vars[name] = %v, want myext", vars["name"])
	}
}

func TestIssue_JSONRoundtrip(t *testing.T) {
	// Verify the issue struct decodes CLI JSON output correctly.
	input := `{
		"id": "test-abc",
		"slug": "my-issue",
		"path": "test-abc--my-issue.md",
		"title": "My Issue",
		"status": "todo",
		"type": "task",
		"priority": "normal",
		"tags": ["cli", "test"],
		"created_at": "2025-01-01T00:00:00Z",
		"updated_at": "2025-01-02T00:00:00Z",
		"body": "Some body content",
		"parent": "test-xyz",
		"blocked_by": ["test-123"],
		"sync": {"myext": {"key": "val"}},
		"etag": "abcdef1234567890"
	}`

	var b Issue
	if err := json.Unmarshal([]byte(input), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if b.ID != "test-abc" {
		t.Errorf("ID = %q, want test-abc", b.ID)
	}
	if b.Title != "My Issue" {
		t.Errorf("Title = %q", b.Title)
	}
	if b.Status != "todo" {
		t.Errorf("Status = %q", b.Status)
	}
	if len(b.Tags) != 2 || b.Tags[0] != "cli" {
		t.Errorf("Tags = %v", b.Tags)
	}
	if b.Parent != "test-xyz" {
		t.Errorf("Parent = %q, want test-xyz", b.Parent)
	}
	if len(b.BlockedBy) != 1 || b.BlockedBy[0] != "test-123" {
		t.Errorf("BlockedBy = %v", b.BlockedBy)
	}
	if b.Sync["myext"]["key"] != "val" {
		t.Errorf("Sync = %v", b.Sync)
	}
	if b.ETag != "abcdef1234567890" {
		t.Errorf("ETag = %q", b.ETag)
	}
	if b.CreatedAt == nil {
		t.Error("CreatedAt should not be nil")
	}
}

// extractVars finds the -v flag value in args and parses it as JSON.
func extractVars(t *testing.T, args []string) map[string]any {
	t.Helper()
	for i, a := range args {
		if a == "-v" && i+1 < len(args) {
			var vars map[string]any
			if err := json.Unmarshal([]byte(args[i+1]), &vars); err != nil {
				t.Fatalf("invalid variables JSON: %v", err)
			}
			return vars
		}
	}
	t.Fatal("-v flag not found in args")
	return nil
}
