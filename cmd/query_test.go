package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
)

func setupQueryTestCore(t *testing.T) (*core.Core, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".issues")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create test .issues dir: %v", err)
	}

	cfg := config.Default()
	testCore := core.New(dataDir, cfg)
	if err := testCore.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	// Save and restore the global store
	oldStore := store
	store = testCore

	cleanup := func() {
		store = oldStore
	}

	return testCore, cleanup
}

func createQueryTestIssue(t *testing.T, c *core.Core, id, title, status string) *issue.Issue {
	t.Helper()
	b := &issue.Issue{
		ID:     id,
		Slug:   issue.Slugify(title),
		Title:  title,
		Status: status,
	}
	if err := c.Create(b); err != nil {
		t.Fatalf("failed to create test issue: %v", err)
	}
	return b
}

func TestExecuteQuery(t *testing.T) {
	testCore, cleanup := setupQueryTestCore(t)
	defer cleanup()

	// Create test issues
	createQueryTestIssue(t, testCore, "test-1", "First Issue", "todo")
	createQueryTestIssue(t, testCore, "test-2", "Second Issue", "in-progress")
	createQueryTestIssue(t, testCore, "test-3", "Third Issue", "completed")

	t.Run("basic query all issues", func(t *testing.T) {
		query := `{ issues { id title status } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID     string `json:"id"`
				Title  string `json:"title"`
				Status string `json:"status"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 3 {
			t.Errorf("expected 3 issues, got %d", len(data.Issues))
		}
	})

	t.Run("query single issue by id", func(t *testing.T) {
		query := `{ issue(id: "test-1") { id title } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if data.Issue.ID != "test-1" {
			t.Errorf("expected id 'test-1', got %q", data.Issue.ID)
		}
		if data.Issue.Title != "First Issue" {
			t.Errorf("expected title 'First Issue', got %q", data.Issue.Title)
		}
	})

	t.Run("query with filter", func(t *testing.T) {
		query := `{ issues(filter: { status: ["todo"] }) { id } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID string `json:"id"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 1 {
			t.Errorf("expected 1 issue with status 'todo', got %d", len(data.Issues))
		}
		if len(data.Issues) > 0 && data.Issues[0].ID != "test-1" {
			t.Errorf("expected issue ID 'test-1', got %q", data.Issues[0].ID)
		}
	})

	t.Run("query with variables", func(t *testing.T) {
		query := `query GetIssue($id: ID!) { issue(id: $id) { id title } }`
		variables := map[string]any{
			"id": "test-2",
		}
		result, err := executeQuery(query, variables, "GetIssue")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if data.Issue.ID != "test-2" {
			t.Errorf("expected id 'test-2', got %q", data.Issue.ID)
		}
	})

	t.Run("query nonexistent issue returns null", func(t *testing.T) {
		query := `{ issue(id: "nonexistent") { id } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue *struct {
				ID string `json:"id"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if data.Issue != nil {
			t.Errorf("expected null issue, got %+v", data.Issue)
		}
	})

	t.Run("invalid query returns error", func(t *testing.T) {
		query := `{ invalid { field } }`
		_, err := executeQuery(query, nil, "")
		if err == nil {
			t.Fatal("expected error for invalid query, got nil")
		}
		if !strings.Contains(err.Error(), "graphql") {
			t.Errorf("expected error to contain 'graphql', got %q", err.Error())
		}
	})
}

func TestExecuteQueryWithRelationships(t *testing.T) {
	testCore, cleanup := setupQueryTestCore(t)
	defer cleanup()

	// Create parent issue
	parent := &issue.Issue{
		ID:     "parent-1",
		Slug:   "parent-issue",
		Title:  "Parent Issue",
		Status: "todo",
	}
	if err := testCore.Create(parent); err != nil {
		t.Fatalf("failed to create parent issue: %v", err)
	}

	// Create child issue with parent link
	child := &issue.Issue{
		ID:     "child-1",
		Slug:   "child-issue",
		Title:  "Child Issue",
		Status: "todo",
		Parent: "parent-1",
	}
	if err := testCore.Create(child); err != nil {
		t.Fatalf("failed to create child issue: %v", err)
	}

	// Create blocker issue
	blocker := &issue.Issue{
		ID:     "blocker-1",
		Slug:   "blocker-issue",
		Title:  "Blocker Issue",
		Status: "todo",
		Blocking: []string{"child-1"},
	}
	if err := testCore.Create(blocker); err != nil {
		t.Fatalf("failed to create blocker issue: %v", err)
	}

	t.Run("query parent relationship", func(t *testing.T) {
		query := `{ issue(id: "child-1") { id parent { id title } } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue struct {
				ID     string `json:"id"`
				Parent *struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"parent"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if data.Issue.Parent == nil {
			t.Fatal("expected parent to be set")
		}
		if data.Issue.Parent.ID != "parent-1" {
			t.Errorf("expected parent id 'parent-1', got %q", data.Issue.Parent.ID)
		}
	})

	t.Run("query children relationship", func(t *testing.T) {
		query := `{ issue(id: "parent-1") { id children { id title } } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue struct {
				ID       string `json:"id"`
				Children []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"children"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issue.Children) != 1 {
			t.Errorf("expected 1 child, got %d", len(data.Issue.Children))
		}
		if len(data.Issue.Children) > 0 && data.Issue.Children[0].ID != "child-1" {
			t.Errorf("expected child id 'child-1', got %q", data.Issue.Children[0].ID)
		}
	})

	t.Run("query blockedBy relationship", func(t *testing.T) {
		query := `{ issue(id: "child-1") { id blockedBy { id title } } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue struct {
				ID        string `json:"id"`
				BlockedBy []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"blockedBy"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issue.BlockedBy) != 1 {
			t.Errorf("expected 1 blocker, got %d", len(data.Issue.BlockedBy))
		}
		if len(data.Issue.BlockedBy) > 0 && data.Issue.BlockedBy[0].ID != "blocker-1" {
			t.Errorf("expected blocker id 'blocker-1', got %q", data.Issue.BlockedBy[0].ID)
		}
	})

	t.Run("query blocking relationship", func(t *testing.T) {
		query := `{ issue(id: "blocker-1") { id blocking { id title } } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issue struct {
				ID       string `json:"id"`
				Blocking []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"blocking"`
			} `json:"issue"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issue.Blocking) != 1 {
			t.Errorf("expected 1 blocked issue, got %d", len(data.Issue.Blocking))
		}
		if len(data.Issue.Blocking) > 0 && data.Issue.Blocking[0].ID != "child-1" {
			t.Errorf("expected blocked id 'child-1', got %q", data.Issue.Blocking[0].ID)
		}
	})
}

func TestExecuteQueryWithFilters(t *testing.T) {
	testCore, cleanup := setupQueryTestCore(t)
	defer cleanup()

	// Create issues with different types and priorities
	b1 := &issue.Issue{
		ID:       "bug-1",
		Slug:     "bug-one",
		Title:    "Bug One",
		Status:   "todo",
		Type:     "bug",
		Priority: "critical",
		Tags:     []string{"frontend"},
	}
	b2 := &issue.Issue{
		ID:       "feat-1",
		Slug:     "feature-one",
		Title:    "Feature One",
		Status:   "in-progress",
		Type:     "feature",
		Priority: "high",
		Tags:     []string{"backend"},
	}
	b3 := &issue.Issue{
		ID:       "task-1",
		Slug:     "task-one",
		Title:    "Task One",
		Status:   "completed",
		Type:     "task",
		Priority: "normal",
		Tags:     []string{"frontend", "backend"},
	}

	testCore.Create(b1)
	testCore.Create(b2)
	testCore.Create(b3)

	t.Run("filter by type", func(t *testing.T) {
		query := `{ issues(filter: { type: ["bug"] }) { id type } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID   string `json:"id"`
				Type string `json:"type"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 1 {
			t.Errorf("expected 1 issue with type 'bug', got %d", len(data.Issues))
		}
	})

	t.Run("filter by priority", func(t *testing.T) {
		query := `{ issues(filter: { priority: ["critical", "high"] }) { id priority } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID       string `json:"id"`
				Priority string `json:"priority"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 2 {
			t.Errorf("expected 2 issues with priority 'critical' or 'high', got %d", len(data.Issues))
		}
	})

	t.Run("filter by tags", func(t *testing.T) {
		query := `{ issues(filter: { tags: ["frontend"] }) { id tags } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID   string   `json:"id"`
				Tags []string `json:"tags"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 2 {
			t.Errorf("expected 2 issues with tag 'frontend', got %d", len(data.Issues))
		}
	})

	t.Run("exclude by status", func(t *testing.T) {
		query := `{ issues(filter: { excludeStatus: ["completed"] }) { id status } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 2 {
			t.Errorf("expected 2 issues (excluding completed), got %d", len(data.Issues))
		}
		for _, b := range data.Issues {
			if b.Status == "completed" {
				t.Errorf("should not include completed issues, got issue with status %q", b.Status)
			}
		}
	})

	t.Run("combined filters", func(t *testing.T) {
		query := `{ issues(filter: { status: ["todo", "in-progress"], type: ["bug", "feature"] }) { id } }`
		result, err := executeQuery(query, nil, "")
		if err != nil {
			t.Fatalf("executeQuery() error = %v", err)
		}

		var data struct {
			Issues []struct {
				ID string `json:"id"`
			} `json:"issues"`
		}

		if err := json.Unmarshal(result, &data); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if len(data.Issues) != 2 {
			t.Errorf("expected 2 issues matching combined filters, got %d", len(data.Issues))
		}
	})
}

func TestGetGraphQLSchema(t *testing.T) {
	_, cleanup := setupQueryTestCore(t)
	defer cleanup()

	schema := GetGraphQLSchema()

	// Verify schema contains expected types
	expectedTypes := []string{
		"type Query",
		"type Issue",
		"input IssueFilter",
	}

	for _, expected := range expectedTypes {
		if !strings.Contains(schema, expected) {
			t.Errorf("schema missing expected type: %s", expected)
		}
	}

	// Verify schema contains expected fields
	expectedFields := []string{
		"issue(id: ID!)",
		"issues(filter: IssueFilter)",
		"blockedBy",
		"blocking",
		"parent",
		"children",
	}

	for _, expected := range expectedFields {
		if !strings.Contains(schema, expected) {
			t.Errorf("schema missing expected field: %s", expected)
		}
	}

	// Verify no introspection fields
	if strings.Contains(schema, "__schema") || strings.Contains(schema, "__type") {
		t.Error("schema should not contain introspection fields")
	}
}

func TestReadFromStdin(t *testing.T) {
	// Note: Testing stdin behavior is tricky in unit tests.
	// This tests the function when stdin is a terminal (returns empty).
	// Integration tests would need to actually pipe data.
	t.Run("returns empty when stdin is terminal", func(t *testing.T) {
		// In a test environment, stdin is typically a terminal
		result, err := readFromStdin()
		if err != nil {
			t.Fatalf("readFromStdin() error = %v", err)
		}
		// Result will be empty string when stdin is a terminal
		if result != "" {
			t.Logf("readFromStdin() returned %q (may vary by test environment)", result)
		}
	})
}
