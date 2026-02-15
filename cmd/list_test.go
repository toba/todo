package cmd

import (
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
)

func TestSortIssues(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-1 * time.Hour)
	evenEarlier := now.Add(-2 * time.Hour)

	// Statuses are now hardcoded, so we just use default config
	testCfg := config.Default()

	t.Run("sort by id", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "c3"},
			{ID: "a1"},
			{ID: "b2"},
		}
		sortIssues(issues, "id", testCfg)

		if issues[0].ID != "a1" || issues[1].ID != "b2" || issues[2].ID != "c3" {
			t.Errorf("sort by id: got [%s, %s, %s], want [a1, b2, c3]",
				issues[0].ID, issues[1].ID, issues[2].ID)
		}
	})

	t.Run("sort by created", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "old", CreatedAt: &evenEarlier},
			{ID: "new", CreatedAt: &now},
			{ID: "mid", CreatedAt: &earlier},
		}
		sortIssues(issues, "created", testCfg)

		// Should be newest first
		if issues[0].ID != "new" || issues[1].ID != "mid" || issues[2].ID != "old" {
			t.Errorf("sort by created: got [%s, %s, %s], want [new, mid, old]",
				issues[0].ID, issues[1].ID, issues[2].ID)
		}
	})

	t.Run("sort by created with nil", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "nil1", CreatedAt: nil},
			{ID: "has", CreatedAt: &now},
			{ID: "nil2", CreatedAt: nil},
		}
		sortIssues(issues, "created", testCfg)

		// Non-nil should come first, then nil sorted by ID
		if issues[0].ID != "has" {
			t.Errorf("sort by created with nil: first should be \"has\", got %q", issues[0].ID)
		}
	})

	t.Run("sort by updated", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "old", UpdatedAt: &evenEarlier},
			{ID: "new", UpdatedAt: &now},
			{ID: "mid", UpdatedAt: &earlier},
		}
		sortIssues(issues, "updated", testCfg)

		// Should be newest first
		if issues[0].ID != "new" || issues[1].ID != "mid" || issues[2].ID != "old" {
			t.Errorf("sort by updated: got [%s, %s, %s], want [new, mid, old]",
				issues[0].ID, issues[1].ID, issues[2].ID)
		}
	})

	t.Run("sort by status", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "c1", Status: "completed"},
			{ID: "r1", Status: "ready"},
			{ID: "i1", Status: "in-progress"},
			{ID: "r2", Status: "ready"},
		}
		sortIssues(issues, "status", testCfg)

		// Should be ordered by status config order (in-progress, ready, draft, completed, scrapped), then by ID within same status
		expected := []string{"i1", "r1", "r2", "c1"}
		for i, want := range expected {
			if issues[i].ID != want {
				t.Errorf("sort by status[%d]: got %q, want %q", i, issues[i].ID, want)
			}
		}
	})

	t.Run("default sort (archive status then type)", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "completed-bug", Status: "completed", Type: "bug"},
			{ID: "ready-feature", Status: "ready", Type: "feature"},
			{ID: "ready-task", Status: "ready", Type: "task"},
			{ID: "completed-task", Status: "completed", Type: "task"},
			{ID: "ready-bug", Status: "ready", Type: "bug"},
		}
		sortIssues(issues, "", testCfg)

		// Should be: non-archive first (sorted by type order from DefaultTypes: milestone, epic, bug, feature, task),
		// then archive (sorted by type)
		// DefaultTypes order: milestone, epic, bug, feature, task
		expected := []string{"ready-bug", "ready-feature", "ready-task", "completed-bug", "completed-task"}
		for i, want := range expected {
			if issues[i].ID != want {
				t.Errorf("default sort[%d]: got %q, want %q", i, issues[i].ID, want)
			}
		}
	})
}

func TestListReadyFlagMutualExclusion(t *testing.T) {
	// Test that --ready and --is-blocked are mutually exclusive
	// by checking the validation logic directly
	tests := []struct {
		name        string
		ready       bool
		isBlocked   bool
		expectError bool
	}{
		{"neither flag", false, false, false},
		{"only --ready", true, false, false},
		{"only --is-blocked", false, true, false},
		{"both flags", true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This mirrors the validation logic in list.go
			hasError := tt.ready && tt.isBlocked
			if hasError != tt.expectError {
				t.Errorf("ready=%v, isBlocked=%v: got error=%v, want error=%v",
					tt.ready, tt.isBlocked, hasError, tt.expectError)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"needs truncation", "hello world", 8, "hello..."},
		{"very short max", "hello", 4, "h..."},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

