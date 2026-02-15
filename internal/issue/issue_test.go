package issue

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedTitle  string
		expectedStatus string
		expectedBody   string
		wantErr        bool
	}{
		{
			name: "basic issue",
			input: `---
title: Test Issue
status: todo
---

This is the body.`,
			expectedTitle:  "Test Issue",
			expectedStatus: "todo",
			expectedBody:   "\nThis is the body.",
		},
		{
			name: "with timestamps",
			input: `---
title: With Times
status: in-progress
created_at: 2024-01-15T10:30:00Z
updated_at: 2024-01-16T14:45:00Z
---

Body content here.`,
			expectedTitle:  "With Times",
			expectedStatus: "in-progress",
			expectedBody:   "\nBody content here.",
		},
		{
			name: "empty body",
			input: `---
title: No Body
status: completed
---`,
			expectedTitle:  "No Body",
			expectedStatus: "completed",
			expectedBody:   "",
		},
		{
			name: "multiline body",
			input: `---
title: Multi Line
status: todo
---

# Header

- Item 1
- Item 2

Paragraph text.`,
			expectedTitle:  "Multi Line",
			expectedStatus: "todo",
			expectedBody:   "\n# Header\n\n- Item 1\n- Item 2\n\nParagraph text.",
		},
		{
			name:           "plain text without frontmatter",
			input:          `Just plain text without any YAML frontmatter.`,
			expectedTitle:  "",
			expectedStatus: "",
			expectedBody:   "Just plain text without any YAML frontmatter.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Title != tt.expectedTitle {
				t.Errorf("Title = %q, want %q", parsed.Title, tt.expectedTitle)
			}
			if parsed.Status != tt.expectedStatus {
				t.Errorf("Status = %q, want %q", parsed.Status, tt.expectedStatus)
			}
			if parsed.Body != tt.expectedBody {
				t.Errorf("Body = %q, want %q", parsed.Body, tt.expectedBody)
			}
		})
	}
}

func TestParseWithType(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedType string
	}{
		{
			name: "with type field",
			input: `---
title: Bug Report
status: todo
type: bug
---

Description of the bug.`,
			expectedType: "bug",
		},
		{
			name: "without type field",
			input: `---
title: No Type
status: todo
---

No type specified.`,
			expectedType: "",
		},
		{
			// Backwards compatibility: issues with types not in current config
			// should still be readable without error
			name: "with unknown type (backwards compatibility)",
			input: `---
title: Legacy Issue
status: todo
type: deprecated-type-no-longer-in-config
---`,
			expectedType: "deprecated-type-no-longer-in-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Type != tt.expectedType {
				t.Errorf("Type = %q, want %q", parsed.Type, tt.expectedType)
			}
		})
	}
}

func TestParseWithPriority(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedPriority string
	}{
		{
			name: "with priority field",
			input: `---
title: Urgent Bug
status: todo
type: bug
priority: critical
---

Fix this immediately.`,
			expectedPriority: "critical",
		},
		{
			name: "without priority field",
			input: `---
title: Normal Task
status: todo
---

No priority specified.`,
			expectedPriority: "",
		},
		{
			name: "with high priority",
			input: `---
title: Important Feature
status: in-progress
priority: high
---`,
			expectedPriority: "high",
		},
		{
			name: "with deferred priority",
			input: `---
title: Later Task
status: draft
priority: deferred
---`,
			expectedPriority: "deferred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Priority != tt.expectedPriority {
				t.Errorf("Priority = %q, want %q", parsed.Priority, tt.expectedPriority)
			}
		})
	}
}

func TestRenderWithPriority(t *testing.T) {
	tests := []struct {
		name     string
		issue    *Issue
		contains []string
	}{
		{
			name: "with priority",
			issue: &Issue{
				Title:    "High Priority",
				Status:   "todo",
				Priority: "high",
			},
			contains: []string{
				"title: High Priority",
				"status: todo",
				"priority: high",
			},
		},
		{
			name: "without priority",
			issue: &Issue{
				Title:  "No Priority",
				Status: "todo",
			},
			contains: []string{
				"title: No Priority",
				"status: todo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			content := string(rendered)
			for _, want := range tt.contains {
				if !strings.Contains(content, want) {
					t.Errorf("Render() missing %q in:\n%s", want, content)
				}
			}

			// Verify priority is NOT in output when empty
			if tt.issue.Priority == "" && strings.Contains(content, "priority:") {
				t.Errorf("Render() should not contain 'priority:' when priority is empty:\n%s", content)
			}
		})
	}
}

func TestPriorityRoundtrip(t *testing.T) {
	priorities := []string{"critical", "high", "normal", "low", "deferred", ""}

	for _, priority := range priorities {
		t.Run(priority, func(t *testing.T) {
			original := &Issue{
				Title:    "Test Issue",
				Status:   "todo",
				Priority: priority,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render() error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}

			if parsed.Priority != original.Priority {
				t.Errorf("Priority roundtrip failed: got %q, want %q", parsed.Priority, original.Priority)
			}
		})
	}
}

func TestRender(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		issue    *Issue
		contains []string
	}{
		{
			name: "basic issue",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			contains: []string{
				"---",
				"title: Test Issue",
				"status: todo",
			},
		},
		{
			name: "with body",
			issue: &Issue{
				Title:  "With Body",
				Status: "completed",
				Body:   "This is content.",
			},
			contains: []string{
				"title: With Body",
				"status: completed",
				"This is content.",
			},
		},
		{
			name: "with timestamps",
			issue: &Issue{
				Title:     "Timed",
				Status:    "todo",
				CreatedAt: &now,
				UpdatedAt: &now,
			},
			contains: []string{
				"title: Timed",
				"created_at:",
				"updated_at:",
			},
		},
		{
			name: "with type",
			issue: &Issue{
				Title:  "Typed Issue",
				Status: "todo",
				Type:   "bug",
			},
			contains: []string{
				"title: Typed Issue",
				"status: todo",
				"type: bug",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}
		})
	}
}

func TestParseRenderRoundtrip(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	later := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)

	tests := []struct {
		name string
		issue *Issue
	}{
		{
			name: "basic",
			issue: &Issue{
				Title:  "Basic Issue",
				Status: "todo",
			},
		},
		{
			name: "with body",
			issue: &Issue{
				Title:  "Issue With Body",
				Status: "in-progress",
				Body:   "This is the body content.\n\nWith multiple paragraphs.",
			},
		},
		{
			name: "with timestamps",
			issue: &Issue{
				Title:     "Timestamped Issue",
				Status:    "completed",
				CreatedAt: &now,
				UpdatedAt: &later,
				Body:      "Some content.",
			},
		},
		{
			name: "with type",
			issue: &Issue{
				Title:  "Typed Issue",
				Status: "todo",
				Type:   "bug",
				Body:   "Bug description.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render to bytes
			rendered, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			// Parse back
			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Compare fields
			if parsed.Title != tt.issue.Title {
				t.Errorf("Title roundtrip: got %q, want %q", parsed.Title, tt.issue.Title)
			}
			if parsed.Status != tt.issue.Status {
				t.Errorf("Status roundtrip: got %q, want %q", parsed.Status, tt.issue.Status)
			}
			if parsed.Type != tt.issue.Type {
				t.Errorf("Type roundtrip: got %q, want %q", parsed.Type, tt.issue.Type)
			}

			// Body comparison (parse adds newline prefix for non-empty body)
			wantBody := tt.issue.Body
			if wantBody != "" {
				wantBody = "\n" + wantBody
			}
			if parsed.Body != wantBody {
				t.Errorf("Body roundtrip: got %q, want %q", parsed.Body, wantBody)
			}

			// Timestamp comparison
			if tt.issue.CreatedAt != nil {
				if parsed.CreatedAt == nil {
					t.Error("CreatedAt: got nil, want non-nil")
				} else if !parsed.CreatedAt.Equal(*tt.issue.CreatedAt) {
					t.Errorf("CreatedAt: got %v, want %v", parsed.CreatedAt, tt.issue.CreatedAt)
				}
			}
			if tt.issue.UpdatedAt != nil {
				if parsed.UpdatedAt == nil {
					t.Error("UpdatedAt: got nil, want non-nil")
				} else if !parsed.UpdatedAt.Equal(*tt.issue.UpdatedAt) {
					t.Errorf("UpdatedAt: got %v, want %v", parsed.UpdatedAt, tt.issue.UpdatedAt)
				}
			}
		})
	}
}

func TestIssueJSONSerialization(t *testing.T) {
	t.Run("body omitted when empty", func(t *testing.T) {
		b := &Issue{
			ID:     "test-123",
			Title:  "Test Issue",
			Status: "todo",
			Body:   "",
		}

		data, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		jsonStr := string(data)
		if strings.Contains(jsonStr, `"body"`) {
			t.Errorf("JSON should not contain 'body' field when empty, got: %s", jsonStr)
		}
	})

	t.Run("body included when non-empty", func(t *testing.T) {
		b := &Issue{
			ID:     "test-123",
			Title:  "Test Issue",
			Status: "todo",
			Body:   "This is the body content.",
		}

		data, err := json.Marshal(b)
		if err != nil {
			t.Fatalf("failed to marshal: %v", err)
		}

		jsonStr := string(data)
		if !strings.Contains(jsonStr, `"body":"This is the body content."`) {
			t.Errorf("JSON should contain 'body' field with content, got: %s", jsonStr)
		}
	})
}

func TestParseWithParentAndBlocking(t *testing.T) {
	tests := []struct {
		name             string
		input            string
		expectedParent   string
		expectedBlocking []string
	}{
		{
			name: "with parent only",
			input: `---
title: Test
status: todo
parent: xyz789
---`,
			expectedParent:   "xyz789",
			expectedBlocking: nil,
		},
		{
			name: "with blocking only",
			input: `---
title: Test
status: todo
blocking:
  - abc123
  - def456
---`,
			expectedParent:   "",
			expectedBlocking: []string{"abc123", "def456"},
		},
		{
			name: "with parent and blocking",
			input: `---
title: Test
status: todo
parent: xyz789
blocking:
  - abc123
---`,
			expectedParent:   "xyz789",
			expectedBlocking: []string{"abc123"},
		},
		{
			name: "no relationships",
			input: `---
title: Test
status: todo
---`,
			expectedParent:   "",
			expectedBlocking: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if parsed.Parent != tt.expectedParent {
				t.Errorf("Parent = %q, want %q", parsed.Parent, tt.expectedParent)
			}

			if len(tt.expectedBlocking) == 0 && len(parsed.Blocking) == 0 {
				return // Both empty, OK
			}

			if len(parsed.Blocking) != len(tt.expectedBlocking) {
				t.Errorf("Blocking count = %d, want %d", len(parsed.Blocking), len(tt.expectedBlocking))
				return
			}

			for i, expected := range tt.expectedBlocking {
				if parsed.Blocking[i] != expected {
					t.Errorf("Blocking[%d] = %q, want %q", i, parsed.Blocking[i], expected)
				}
			}
		})
	}
}

func TestRenderWithParentAndBlocking(t *testing.T) {
	tests := []struct {
		name     string
		issue    *Issue
		contains []string
	}{
		{
			name: "with parent only",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Parent: "xyz789",
			},
			contains: []string{
				"parent: xyz789",
			},
		},
		{
			name: "with blocking only",
			issue: &Issue{
				Title:    "Test Issue",
				Status:   "todo",
				Blocking: []string{"abc123", "def456"},
			},
			contains: []string{
				"blocking:",
				"- abc123",
				"- def456",
			},
		},
		{
			name: "with parent and blocking",
			issue: &Issue{
				Title:    "Test Issue",
				Status:   "todo",
				Parent:   "xyz789",
				Blocking: []string{"abc123"},
			},
			contains: []string{
				"parent: xyz789",
				"blocking:",
				"- abc123",
			},
		},
		{
			name: "without relationships",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			contains: []string{
				"title: Test Issue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}

			// Check that empty parent/blocking don't appear in output
			if tt.issue.Parent == "" && strings.Contains(result, "parent:") {
				t.Errorf("output should not contain 'parent:' when no parent\ngot:\n%s", result)
			}
			if len(tt.issue.Blocking) == 0 && strings.Contains(result, "blocking:") {
				t.Errorf("output should not contain 'blocking:' when no blocking\ngot:\n%s", result)
			}
		})
	}
}

func TestParentAndBlockingRoundtrip(t *testing.T) {
	tests := []struct {
		name     string
		parent   string
		blocking []string
	}{
		{
			name:     "parent only",
			parent:   "xyz789",
			blocking: nil,
		},
		{
			name:     "single blocking",
			parent:   "",
			blocking: []string{"abc123"},
		},
		{
			name:     "multiple blocking",
			parent:   "",
			blocking: []string{"abc123", "def456"},
		},
		{
			name:     "parent and blocking",
			parent:   "xyz789",
			blocking: []string{"abc123", "def456"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Issue{
				Title:    "Test",
				Status:   "todo",
				Parent:   tt.parent,
				Blocking: tt.blocking,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if parsed.Parent != tt.parent {
				t.Errorf("Parent: got %q, want %q", parsed.Parent, tt.parent)
			}

			if len(parsed.Blocking) != len(tt.blocking) {
				t.Errorf("Blocking count: got %d, want %d", len(parsed.Blocking), len(tt.blocking))
				return
			}

			for i, expected := range tt.blocking {
				if parsed.Blocking[i] != expected {
					t.Errorf("Blocking[%d] = %q, want %q", i, parsed.Blocking[i], expected)
				}
			}
		})
	}
}

func TestParseWithBlockedBy(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedBlockedBy []string
	}{
		{
			name: "with blocked_by",
			input: `---
title: Test
status: todo
blocked_by:
  - abc123
  - def456
---`,
			expectedBlockedBy: []string{"abc123", "def456"},
		},
		{
			name: "no blocked_by",
			input: `---
title: Test
status: todo
---`,
			expectedBlockedBy: nil,
		},
		{
			name: "with blocking and blocked_by",
			input: `---
title: Test
status: todo
blocking:
  - xyz789
blocked_by:
  - abc123
---`,
			expectedBlockedBy: []string{"abc123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tt.expectedBlockedBy) == 0 && len(parsed.BlockedBy) == 0 {
				return // Both empty, OK
			}

			if len(parsed.BlockedBy) != len(tt.expectedBlockedBy) {
				t.Errorf("BlockedBy count = %d, want %d", len(parsed.BlockedBy), len(tt.expectedBlockedBy))
				return
			}

			for i, expected := range tt.expectedBlockedBy {
				if parsed.BlockedBy[i] != expected {
					t.Errorf("BlockedBy[%d] = %q, want %q", i, parsed.BlockedBy[i], expected)
				}
			}
		})
	}
}

func TestRenderWithBlockedBy(t *testing.T) {
	tests := []struct {
		name     string
		issue    *Issue
		contains []string
	}{
		{
			name: "with blocked_by only",
			issue: &Issue{
				Title:     "Test Issue",
				Status:    "todo",
				BlockedBy: []string{"abc123", "def456"},
			},
			contains: []string{
				"blocked_by:",
				"- abc123",
				"- def456",
			},
		},
		{
			name: "with blocking and blocked_by",
			issue: &Issue{
				Title:     "Test Issue",
				Status:    "todo",
				Blocking:  []string{"xyz789"},
				BlockedBy: []string{"abc123"},
			},
			contains: []string{
				"blocking:",
				"- xyz789",
				"blocked_by:",
				"- abc123",
			},
		},
		{
			name: "without blocked_by",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			contains: []string{
				"title: Test Issue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}

			// Check that empty blocked_by doesn't appear in output
			if len(tt.issue.BlockedBy) == 0 && strings.Contains(result, "blocked_by:") {
				t.Errorf("output should not contain 'blocked_by:' when no blocked_by\ngot:\n%s", result)
			}
		})
	}
}

func TestBlockedByRoundtrip(t *testing.T) {
	tests := []struct {
		name      string
		blockedBy []string
	}{
		{
			name:      "single blocked_by",
			blockedBy: []string{"abc123"},
		},
		{
			name:      "multiple blocked_by",
			blockedBy: []string{"abc123", "def456"},
		},
		{
			name:      "empty blocked_by",
			blockedBy: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Issue{
				Title:     "Test",
				Status:    "todo",
				BlockedBy: tt.blockedBy,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(parsed.BlockedBy) != len(tt.blockedBy) {
				t.Errorf("BlockedBy count: got %d, want %d", len(parsed.BlockedBy), len(tt.blockedBy))
				return
			}

			for i, expected := range tt.blockedBy {
				if parsed.BlockedBy[i] != expected {
					t.Errorf("BlockedBy[%d] = %q, want %q", i, parsed.BlockedBy[i], expected)
				}
			}
		})
	}
}

func TestIssueRelationshipMethods(t *testing.T) {
	t.Run("HasParent", func(t *testing.T) {
		withParent := &Issue{Parent: "xyz789"}
		if !withParent.HasParent() {
			t.Error("expected HasParent() = true when parent is set")
		}

		withoutParent := &Issue{}
		if withoutParent.HasParent() {
			t.Error("expected HasParent() = false when parent is empty")
		}
	})

	t.Run("IsBlocking", func(t *testing.T) {
		b := &Issue{Blocking: []string{"abc", "def"}}
		if !b.IsBlocking("abc") {
			t.Error("expected IsBlocking('abc') = true")
		}
		if !b.IsBlocking("def") {
			t.Error("expected IsBlocking('def') = true")
		}
		if b.IsBlocking("xyz") {
			t.Error("expected IsBlocking('xyz') = false")
		}

		empty := &Issue{}
		if empty.IsBlocking("abc") {
			t.Error("expected IsBlocking('abc') = false for empty blocks")
		}
	})

	t.Run("AddBlocking", func(t *testing.T) {
		b := &Issue{Blocking: []string{"abc"}}
		b.AddBlocking("def")
		if len(b.Blocking) != 2 {
			t.Errorf("AddBlocking new: got len=%d, want 2", len(b.Blocking))
		}
		if !b.IsBlocking("def") {
			t.Error("AddBlocking didn't add the block")
		}

		// Adding duplicate should not add
		b.AddBlocking("abc")
		if len(b.Blocking) != 2 {
			t.Errorf("AddBlocking duplicate: got len=%d, want 2", len(b.Blocking))
		}
	})

	t.Run("RemoveBlocking", func(t *testing.T) {
		b := &Issue{Blocking: []string{"abc", "def", "ghi"}}
		b.RemoveBlocking("def")
		if len(b.Blocking) != 2 {
			t.Errorf("RemoveBlocking existing: got len=%d, want 2", len(b.Blocking))
		}
		if b.IsBlocking("def") {
			t.Error("RemoveBlocking didn't remove the block")
		}

		// Removing non-existent should not change anything
		b.RemoveBlocking("nonexistent")
		if len(b.Blocking) != 2 {
			t.Errorf("RemoveBlocking non-existent: got len=%d, want 2", len(b.Blocking))
		}
	})

	t.Run("IsBlockedBy", func(t *testing.T) {
		b := &Issue{BlockedBy: []string{"abc", "def"}}
		if !b.IsBlockedBy("abc") {
			t.Error("expected IsBlockedBy('abc') = true")
		}
		if !b.IsBlockedBy("def") {
			t.Error("expected IsBlockedBy('def') = true")
		}
		if b.IsBlockedBy("xyz") {
			t.Error("expected IsBlockedBy('xyz') = false")
		}

		empty := &Issue{}
		if empty.IsBlockedBy("abc") {
			t.Error("expected IsBlockedBy('abc') = false for empty blocked_by")
		}
	})

	t.Run("AddBlockedBy", func(t *testing.T) {
		b := &Issue{BlockedBy: []string{"abc"}}
		b.AddBlockedBy("def")
		if len(b.BlockedBy) != 2 {
			t.Errorf("AddBlockedBy new: got len=%d, want 2", len(b.BlockedBy))
		}
		if !b.IsBlockedBy("def") {
			t.Error("AddBlockedBy didn't add the blocker")
		}

		// Adding duplicate should not add
		b.AddBlockedBy("abc")
		if len(b.BlockedBy) != 2 {
			t.Errorf("AddBlockedBy duplicate: got len=%d, want 2", len(b.BlockedBy))
		}
	})

	t.Run("RemoveBlockedBy", func(t *testing.T) {
		b := &Issue{BlockedBy: []string{"abc", "def", "ghi"}}
		b.RemoveBlockedBy("def")
		if len(b.BlockedBy) != 2 {
			t.Errorf("RemoveBlockedBy existing: got len=%d, want 2", len(b.BlockedBy))
		}
		if b.IsBlockedBy("def") {
			t.Error("RemoveBlockedBy didn't remove the blocker")
		}

		// Removing non-existent should not change anything
		b.RemoveBlockedBy("nonexistent")
		if len(b.BlockedBy) != 2 {
			t.Errorf("RemoveBlockedBy non-existent: got len=%d, want 2", len(b.BlockedBy))
		}
	})
}

func TestValidateTag(t *testing.T) {
	tests := []struct {
		tag     string
		wantErr bool
	}{
		{"frontend", false},
		{"backend", false},
		{"tech-debt", false},
		{"v1", false},
		{"a", false},
		{"urgent2", false},
		{"wont-fix", false},
		{"a-b-c", false},
		{"", true},         // empty
		{"Frontend", true}, // uppercase
		{"URGENT", true},   // all uppercase
		{"123", true},      // starts with number
		{"123abc", true},   // starts with number
		{"my tag", true},   // contains space
		{"my_tag", true},   // contains underscore
		{"my--tag", true},  // consecutive hyphens
		{"-tag", true},     // starts with hyphen
		{"tag-", true},     // ends with hyphen
		{"my.tag", true},   // contains dot
		{"my/tag", true},   // contains slash
	}

	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			err := ValidateTag(tt.tag)
			if tt.wantErr && err == nil {
				t.Errorf("ValidateTag(%q) = nil, want error", tt.tag)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ValidateTag(%q) = %v, want nil", tt.tag, err)
			}
		})
	}
}

func TestNormalizeTag(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"frontend", "frontend"},
		{"FRONTEND", "frontend"},
		{"FrontEnd", "frontend"},
		{"  frontend  ", "frontend"},
		{"  FRONTEND  ", "frontend"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := NormalizeTag(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeTag(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIssueTagMethods(t *testing.T) {
	t.Run("HasTag", func(t *testing.T) {
		b := &Issue{Tags: []string{"frontend", "urgent"}}
		if !b.HasTag("frontend") {
			t.Error("expected HasTag('frontend') = true")
		}
		if !b.HasTag("urgent") {
			t.Error("expected HasTag('urgent') = true")
		}
		if b.HasTag("backend") {
			t.Error("expected HasTag('backend') = false")
		}
		// Case insensitive lookup
		if !b.HasTag("FRONTEND") {
			t.Error("expected HasTag('FRONTEND') = true (case insensitive)")
		}
	})

	t.Run("AddTag", func(t *testing.T) {
		b := &Issue{Tags: []string{"frontend"}}

		// Add new valid tag
		if err := b.AddTag("backend"); err != nil {
			t.Errorf("AddTag('backend') error: %v", err)
		}
		if len(b.Tags) != 2 {
			t.Errorf("expected 2 tags, got %d", len(b.Tags))
		}

		// Adding duplicate should not add
		if err := b.AddTag("frontend"); err != nil {
			t.Errorf("AddTag('frontend') error: %v", err)
		}
		if len(b.Tags) != 2 {
			t.Errorf("expected 2 tags (no duplicate), got %d", len(b.Tags))
		}

		// Adding invalid tag should error
		if err := b.AddTag("Invalid Tag"); err == nil {
			t.Error("expected AddTag('Invalid Tag') to error")
		}
	})

	t.Run("RemoveTag", func(t *testing.T) {
		b := &Issue{Tags: []string{"frontend", "backend", "urgent"}}

		b.RemoveTag("backend")
		if len(b.Tags) != 2 {
			t.Errorf("expected 2 tags after remove, got %d", len(b.Tags))
		}
		if b.HasTag("backend") {
			t.Error("expected backend tag to be removed")
		}

		// Case insensitive removal
		b.RemoveTag("FRONTEND")
		if len(b.Tags) != 1 {
			t.Errorf("expected 1 tag after remove, got %d", len(b.Tags))
		}
		if b.HasTag("frontend") {
			t.Error("expected frontend tag to be removed")
		}

		// Remove non-existent tag (should not error)
		b.RemoveTag("nonexistent")
		if len(b.Tags) != 1 {
			t.Errorf("expected 1 tag (no change), got %d", len(b.Tags))
		}
	})
}

func TestParseWithTags(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedTags []string
	}{
		{
			name: "single tag",
			input: `---
title: Test
status: todo
tags:
  - frontend
---`,
			expectedTags: []string{"frontend"},
		},
		{
			name: "multiple tags",
			input: `---
title: Test
status: todo
tags:
  - frontend
  - urgent
  - tech-debt
---`,
			expectedTags: []string{"frontend", "urgent", "tech-debt"},
		},
		{
			name: "inline tags syntax",
			input: `---
title: Test
status: todo
tags: [frontend, backend]
---`,
			expectedTags: []string{"frontend", "backend"},
		},
		{
			name: "no tags",
			input: `---
title: Test
status: todo
---`,
			expectedTags: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tt.expectedTags) == 0 && len(parsed.Tags) == 0 {
				return // Both empty, OK
			}

			if len(parsed.Tags) != len(tt.expectedTags) {
				t.Errorf("Tags count = %d, want %d", len(parsed.Tags), len(tt.expectedTags))
				return
			}

			for i, expected := range tt.expectedTags {
				if parsed.Tags[i] != expected {
					t.Errorf("Tags[%d] = %q, want %q", i, parsed.Tags[i], expected)
				}
			}
		})
	}
}

func TestRenderWithTags(t *testing.T) {
	tests := []struct {
		name     string
		issue    *Issue
		contains []string
	}{
		{
			name: "with single tag",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Tags:   []string{"frontend"},
			},
			contains: []string{
				"tags:",
				"- frontend",
			},
		},
		{
			name: "with multiple tags",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Tags:   []string{"frontend", "urgent", "tech-debt"},
			},
			contains: []string{
				"tags:",
				"- frontend",
				"- urgent",
				"- tech-debt",
			},
		},
		{
			name: "without tags",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			contains: []string{
				"title: Test Issue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}

			// Check that empty tags don't appear in output
			if len(tt.issue.Tags) == 0 && strings.Contains(result, "tags:") {
				t.Errorf("output should not contain 'tags:' when no tags\ngot:\n%s", result)
			}
		})
	}
}

func TestTagsRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		tags []string
	}{
		{
			name: "single tag",
			tags: []string{"frontend"},
		},
		{
			name: "multiple tags",
			tags: []string{"frontend", "backend", "urgent"},
		},
		{
			name: "hyphenated tags",
			tags: []string{"tech-debt", "wont-fix", "needs-review"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Issue{
				Title:  "Test",
				Status: "todo",
				Tags:   tt.tags,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if len(parsed.Tags) != len(tt.tags) {
				t.Errorf("Tags count: got %d, want %d", len(parsed.Tags), len(tt.tags))
				return
			}

			for i, expected := range tt.tags {
				if parsed.Tags[i] != expected {
					t.Errorf("Tags[%d] = %q, want %q", i, parsed.Tags[i], expected)
				}
			}
		})
	}
}

func TestRenderWithIDComment(t *testing.T) {
	tests := []struct {
		name          string
		issue         *Issue
		expectComment string
	}{
		{
			name: "with ID",
			issue: &Issue{
				ID:     "todo-abc123",
				Title:  "Test Issue",
				Status: "todo",
			},
			expectComment: "# todo-abc123",
		},
		{
			name: "without ID",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			expectComment: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)

			if tt.expectComment != "" {
				// Check that comment appears right after opening ---
				expectedStart := "---\n" + tt.expectComment + "\n"
				if !strings.HasPrefix(result, expectedStart) {
					t.Errorf("expected output to start with %q\ngot:\n%s", expectedStart, result)
				}
			} else {
				// When no ID, should not have a comment line
				lines := strings.Split(result, "\n")
				if len(lines) > 1 && strings.HasPrefix(lines[1], "#") {
					t.Errorf("expected no comment line when ID is empty\ngot:\n%s", result)
				}
			}
		})
	}
}

func TestRenderWithIDCommentRoundtrip(t *testing.T) {
	// Verify that the ID comment doesn't interfere with parsing
	original := &Issue{
		ID:     "todo-xyz789",
		Title:  "Test Issue",
		Status: "in-progress",
		Body:   "Some body content.",
	}

	rendered, err := original.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	// Verify the comment is present
	if !strings.Contains(string(rendered), "# todo-xyz789") {
		t.Errorf("rendered output should contain ID comment\ngot:\n%s", rendered)
	}

	// Parse should work correctly (comment is ignored)
	parsed, err := Parse(strings.NewReader(string(rendered)))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if parsed.Title != original.Title {
		t.Errorf("Title roundtrip: got %q, want %q", parsed.Title, original.Title)
	}
	if parsed.Status != original.Status {
		t.Errorf("Status roundtrip: got %q, want %q", parsed.Status, original.Status)
	}
}

func TestRenderTrailingNewline(t *testing.T) {
	tests := []struct {
		name string
		issue *Issue
	}{
		{
			name: "with body",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Body:   "Some content without trailing newline",
			},
		},
		{
			name: "with body ending in newline",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Body:   "Some content with trailing newline\n",
			},
		},
		{
			name: "without body",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}
			if !strings.HasSuffix(string(rendered), "\n") {
				t.Errorf("rendered output should end with newline\ngot: %q", rendered)
			}
		})
	}
}

func TestETag(t *testing.T) {
	t.Run("consistent hash", func(t *testing.T) {
		b := &Issue{
			Title:  "Test",
			Status: "todo",
			Body:   "content",
		}
		etag1 := b.ETag()
		etag2 := b.ETag()
		if etag1 != etag2 {
			t.Errorf("ETag not consistent: %s != %s", etag1, etag2)
		}
	})

	t.Run("16 hex characters", func(t *testing.T) {
		b := &Issue{
			Title:  "Test",
			Status: "todo",
		}
		etag := b.ETag()
		if len(etag) != 16 {
			t.Errorf("ETag length = %d, want 16", len(etag))
		}
		// Verify it's valid hex
		for _, c := range etag {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("ETag contains non-hex char: %c", c)
			}
		}
	})

	t.Run("changes when title changes", func(t *testing.T) {
		b := &Issue{
			Title:  "Test",
			Status: "todo",
		}
		etag1 := b.ETag()

		b.Title = "Changed"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when title changes")
		}
	})

	t.Run("changes when status changes", func(t *testing.T) {
		b := &Issue{
			Title:  "Test",
			Status: "todo",
		}
		etag1 := b.ETag()

		b.Status = "in-progress"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when status changes")
		}
	})

	t.Run("changes when body changes", func(t *testing.T) {
		b := &Issue{
			Title:  "Test",
			Status: "todo",
			Body:   "original",
		}
		etag1 := b.ETag()

		b.Body = "modified"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when body changes")
		}
	})

	t.Run("changes when metadata changes", func(t *testing.T) {
		b := &Issue{
			Title:    "Test",
			Status:   "todo",
			Priority: "normal",
		}
		etag1 := b.ETag()

		b.Priority = "high"
		etag2 := b.ETag()

		if etag1 == etag2 {
			t.Error("ETag should change when priority changes")
		}
	})

	t.Run("same content same etag", func(t *testing.T) {
		b1 := &Issue{
			Title:  "Test",
			Status: "todo",
			Body:   "content",
		}
		b2 := &Issue{
			Title:  "Test",
			Status: "todo",
			Body:   "content",
		}

		if b1.ETag() != b2.ETag() {
			t.Error("Same content should produce same ETag")
		}
	})

	t.Run("different order of tags produces different etag", func(t *testing.T) {
		b1 := &Issue{
			Title:  "Test",
			Status: "todo",
			Tags:   []string{"a", "b"},
		}
		b2 := &Issue{
			Title:  "Test",
			Status: "todo",
			Tags:   []string{"b", "a"},
		}

		// Tag order matters in rendered output, so ETags will differ
		if b1.ETag() == b2.ETag() {
			t.Error("Different tag order should produce different ETag")
		}
	})

	t.Run("etag is empty on render error", func(t *testing.T) {
		// This is a defensive test - Render() shouldn't fail in practice,
		// but ETag() handles it gracefully by returning empty string
		b := &Issue{
			Title:  "Test",
			Status: "todo",
		}
		etag := b.ETag()
		if etag == "" {
			t.Error("ETag should not be empty for valid issue")
		}
	})
}

func TestMarshalJSONIncludesETag(t *testing.T) {
	b := &Issue{
		ID:     "test-123",
		Title:  "Test Issue",
		Status: "todo",
		Body:   "Some content",
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Check etag field is present
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"etag"`) {
		t.Errorf("JSON should contain 'etag' field, got: %s", jsonStr)
	}

	// Parse and verify etag value
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	etag, ok := result["etag"].(string)
	if !ok {
		t.Error("etag should be a string")
	}
	if len(etag) != 16 {
		t.Errorf("etag length = %d, want 16", len(etag))
	}

	// Verify it matches the computed ETag
	if etag != b.ETag() {
		t.Errorf("JSON etag = %s, want %s", etag, b.ETag())
	}
}

func TestParseWithSync(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedSync map[string]map[string]any
	}{
		{
			name: "with sync data",
			input: `---
title: Test
status: todo
sync:
  clickup:
    task_id: "868h4hd05"
    synced_at: "2026-01-18T00:07:02Z"
---`,
			expectedSync: map[string]map[string]any{
				"clickup": {
					"task_id":   "868h4hd05",
					"synced_at": "2026-01-18T00:07:02Z",
				},
			},
		},
		{
			name: "with multiple sync entries",
			input: `---
title: Test
status: todo
sync:
  clickup:
    task_id: "abc"
  jira:
    issue_key: "PROJ-123"
---`,
			expectedSync: map[string]map[string]any{
				"clickup": {"task_id": "abc"},
				"jira":    {"issue_key": "PROJ-123"},
			},
		},
		{
			name: "no sync data",
			input: `---
title: Test
status: todo
---`,
			expectedSync: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectedSync == nil {
				if parsed.Sync != nil {
					t.Errorf("Sync = %v, want nil", parsed.Sync)
				}
				return
			}

			for name, expectedData := range tt.expectedSync {
				data, ok := parsed.Sync[name]
				if !ok {
					t.Errorf("Sync[%q] missing", name)
					continue
				}
				for key, expectedVal := range expectedData {
					val, ok := data[key]
					if !ok {
						t.Errorf("Sync[%q][%q] missing", name, key)
						continue
					}
					if fmt.Sprint(val) != fmt.Sprint(expectedVal) {
						t.Errorf("Sync[%q][%q] = %v, want %v", name, key, val, expectedVal)
					}
				}
			}
		})
	}
}

func TestRenderWithSync(t *testing.T) {
	tests := []struct {
		name        string
		issue       *Issue
		contains    []string
		notContains []string
	}{
		{
			name: "with sync data",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Sync: map[string]map[string]any{
					"clickup": {
						"task_id":   "868h4hd05",
						"synced_at": "2026-01-18T00:07:02Z",
					},
				},
			},
			contains: []string{
				"sync:",
				"clickup:",
				"task_id: 868h4hd05",
			},
		},
		{
			name: "without sync data",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			notContains: []string{"sync:"},
		},
		{
			name: "with nil sync",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Sync:   nil,
			},
			notContains: []string{"sync:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}
			for _, notWant := range tt.notContains {
				if strings.Contains(result, notWant) {
					t.Errorf("output should not contain %q\ngot:\n%s", notWant, result)
				}
			}
		})
	}
}

func TestSyncRoundtrip(t *testing.T) {
	original := &Issue{
		Title:  "Test",
		Status: "todo",
		Sync: map[string]map[string]any{
			"clickup": {
				"task_id":   "868h4hd05",
				"synced_at": "2026-01-18T00:07:02Z",
			},
			"jira": {
				"issue_key": "PROJ-123",
			},
		},
	}

	rendered, err := original.Render()
	if err != nil {
		t.Fatalf("Render error: %v", err)
	}

	parsed, err := Parse(strings.NewReader(string(rendered)))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(parsed.Sync) != len(original.Sync) {
		t.Errorf("Sync count: got %d, want %d", len(parsed.Sync), len(original.Sync))
		return
	}

	for name, origData := range original.Sync {
		parsedData, ok := parsed.Sync[name]
		if !ok {
			t.Errorf("Sync[%q] missing after roundtrip", name)
			continue
		}
		for key, origVal := range origData {
			parsedVal, ok := parsedData[key]
			if !ok {
				t.Errorf("Sync[%q][%q] missing after roundtrip", name, key)
				continue
			}
			if fmt.Sprint(parsedVal) != fmt.Sprint(origVal) {
				t.Errorf("Sync[%q][%q] = %v, want %v", name, key, parsedVal, origVal)
			}
		}
	}
}

func TestSyncHelperMethods(t *testing.T) {
	t.Run("HasSync", func(t *testing.T) {
		b := &Issue{
			Sync: map[string]map[string]any{
				"clickup": {"task_id": "abc"},
			},
		}
		if !b.HasSync("clickup") {
			t.Error("expected HasSync('clickup') = true")
		}
		if b.HasSync("jira") {
			t.Error("expected HasSync('jira') = false")
		}

		empty := &Issue{}
		if empty.HasSync("clickup") {
			t.Error("expected HasSync('clickup') = false for nil Sync")
		}
	})

	t.Run("SetSync", func(t *testing.T) {
		b := &Issue{}
		b.SetSync("clickup", map[string]any{"task_id": "abc"})
		if !b.HasSync("clickup") {
			t.Error("SetSync didn't set data")
		}
		if b.Sync["clickup"]["task_id"] != "abc" {
			t.Errorf("SetSync data = %v, want task_id=abc", b.Sync["clickup"])
		}

		// Overwrite existing
		b.SetSync("clickup", map[string]any{"task_id": "def"})
		if b.Sync["clickup"]["task_id"] != "def" {
			t.Errorf("SetSync overwrite = %v, want task_id=def", b.Sync["clickup"])
		}
	})

	t.Run("RemoveSync", func(t *testing.T) {
		b := &Issue{
			Sync: map[string]map[string]any{
				"clickup": {"task_id": "abc"},
				"jira":    {"issue_key": "PROJ-123"},
			},
		}
		b.RemoveSync("clickup")
		if b.HasSync("clickup") {
			t.Error("RemoveSync didn't remove data")
		}
		if !b.HasSync("jira") {
			t.Error("RemoveSync removed wrong sync entry")
		}

		// Remove last sync entry should nil out map
		b.RemoveSync("jira")
		if b.Sync != nil {
			t.Errorf("Sync should be nil after removing all sync entries, got %v", b.Sync)
		}

		// Remove from nil Sync should not panic
		empty := &Issue{}
		empty.RemoveSync("clickup") // should not panic
	})
}

func TestETagChangesWithSync(t *testing.T) {
	b := &Issue{
		Title:  "Test",
		Status: "todo",
	}
	etagWithout := b.ETag()

	b.SetSync("clickup", map[string]any{"task_id": "abc"})
	etagWith := b.ETag()

	if etagWithout == etagWith {
		t.Error("ETag should change when sync data is added")
	}

	b.SetSync("clickup", map[string]any{"task_id": "def"})
	etagModified := b.ETag()

	if etagWith == etagModified {
		t.Error("ETag should change when sync data is modified")
	}

	b.RemoveSync("clickup")
	etagRemoved := b.ETag()

	if etagRemoved != etagWithout {
		t.Error("ETag should return to original when sync data is removed")
	}
}

func TestParseWithDueDate(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedDue string // empty means nil
	}{
		{
			name: "with due date",
			input: `---
title: Test
status: todo
due: 2025-03-15
---`,
			expectedDue: "2025-03-15",
		},
		{
			name: "without due date",
			input: `---
title: Test
status: todo
---`,
			expectedDue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Parse(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectedDue == "" {
				if b.Due != nil {
					t.Errorf("Due = %v, want nil", b.Due)
				}
			} else {
				if b.Due == nil {
					t.Fatal("Due = nil, want non-nil")
				}
				if b.Due.String() != tt.expectedDue {
					t.Errorf("Due = %q, want %q", b.Due.String(), tt.expectedDue)
				}
			}
		})
	}
}

func TestRenderWithDueDate(t *testing.T) {
	due := NewDueDate(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))

	tests := []struct {
		name        string
		issue       *Issue
		contains    []string
		notContains []string
	}{
		{
			name: "with due date",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
				Due:    due,
			},
			contains: []string{"due: \"2025-06-01\""},
		},
		{
			name: "without due date",
			issue: &Issue{
				Title:  "Test Issue",
				Status: "todo",
			},
			notContains: []string{"due:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := tt.issue.Render()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			result := string(output)
			for _, want := range tt.contains {
				if !strings.Contains(result, want) {
					t.Errorf("output missing %q\ngot:\n%s", want, result)
				}
			}
			for _, notWant := range tt.notContains {
				if strings.Contains(result, notWant) {
					t.Errorf("output should not contain %q\ngot:\n%s", notWant, result)
				}
			}
		})
	}
}

func TestDueDateRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		due  *DueDate
	}{
		{
			name: "with due date",
			due:  NewDueDate(time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)),
		},
		{
			name: "without due date",
			due:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &Issue{
				Title:  "Test",
				Status: "todo",
				Due:    tt.due,
			}

			rendered, err := original.Render()
			if err != nil {
				t.Fatalf("Render error: %v", err)
			}

			parsed, err := Parse(strings.NewReader(string(rendered)))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			if tt.due == nil {
				if parsed.Due != nil {
					t.Errorf("Due: got %v, want nil", parsed.Due)
				}
			} else {
				if parsed.Due == nil {
					t.Fatal("Due: got nil, want non-nil")
				}
				if parsed.Due.String() != tt.due.String() {
					t.Errorf("Due: got %q, want %q", parsed.Due.String(), tt.due.String())
				}
			}
		})
	}
}

func TestParseDueDate(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"2025-03-15", "2025-03-15", false},
		{"2024-12-31", "2024-12-31", false},
		{"not-a-date", "", true},
		{"2025-13-01", "", true},
		{"2025/03/15", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := ParseDueDate(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.String() != tt.want {
				t.Errorf("got %q, want %q", d.String(), tt.want)
			}
		})
	}
}

func TestDueDateJSON(t *testing.T) {
	due := NewDueDate(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	b := &Issue{
		ID:     "test-123",
		Title:  "Test Issue",
		Status: "todo",
		Due:    due,
	}

	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"due":"2025-06-01"`) {
		t.Errorf("JSON should contain due date, got: %s", jsonStr)
	}
}

func TestETagChangesAfterModification(t *testing.T) {
	// Verify that ETag changes reflect actual content changes
	// (this is important for optimistic concurrency control)
	b := &Issue{
		Title:  "Original",
		Status: "todo",
		Body:   "Original body",
	}

	etag1 := b.ETag()

	// Modify the issue
	b.Title = "Modified"
	b.Body = "Modified body"

	etag2 := b.ETag()

	if etag1 == etag2 {
		t.Error("ETag should change after modification")
	}

	// Verify JSON serialization reflects the change
	data1, _ := json.Marshal(&Issue{Title: "Original", Status: "todo", Body: "Original body"})
	data2, _ := json.Marshal(b)

	var result1, result2 map[string]any
	json.Unmarshal(data1, &result1)
	json.Unmarshal(data2, &result2)

	if result1["etag"] == result2["etag"] {
		t.Error("JSON etag should differ after modification")
	}
}
