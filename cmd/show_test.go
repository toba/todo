package cmd

import (
	"strings"
	"testing"

	"github.com/toba/todo/internal/issue"
)

func TestFormatRelationships(t *testing.T) {
	t.Run("blocked by only", func(t *testing.T) {
		b := &issue.Issue{
			BlockedBy: []string{"issue-abc"},
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "blocked by:") {
			t.Error("expected 'blocked by:' label in output")
		}
		if !strings.Contains(result, "issue-abc") {
			t.Error("expected blocker ID 'issue-abc' in output")
		}
		if strings.Contains(result, "parent:") {
			t.Error("unexpected 'parent:' label in output")
		}
		if strings.Contains(result, "blocking:") {
			t.Error("unexpected 'blocking:' label in output")
		}
	})

	t.Run("blocking and blocked by together", func(t *testing.T) {
		b := &issue.Issue{
			Blocking:  []string{"issue-def"},
			BlockedBy: []string{"issue-ghi"},
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "blocking:") {
			t.Error("expected 'blocking:' label in output")
		}
		if !strings.Contains(result, "issue-def") {
			t.Error("expected blocking ID 'issue-def' in output")
		}
		if !strings.Contains(result, "blocked by:") {
			t.Error("expected 'blocked by:' label in output")
		}
		if !strings.Contains(result, "issue-ghi") {
			t.Error("expected blocker ID 'issue-ghi' in output")
		}
	})

	t.Run("parent only", func(t *testing.T) {
		b := &issue.Issue{
			Parent: "issue-parent",
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "parent:") {
			t.Error("expected 'parent:' label in output")
		}
		if !strings.Contains(result, "issue-parent") {
			t.Error("expected parent ID 'issue-parent' in output")
		}
		if strings.Contains(result, "blocking:") {
			t.Error("unexpected 'blocking:' label in output")
		}
		if strings.Contains(result, "blocked by:") {
			t.Error("unexpected 'blocked by:' label in output")
		}
	})

	t.Run("all three relationships", func(t *testing.T) {
		b := &issue.Issue{
			Parent:    "issue-parent",
			Blocking:  []string{"issue-child"},
			BlockedBy: []string{"issue-dep1", "issue-dep2"},
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "parent:") {
			t.Error("expected 'parent:' label in output")
		}
		if !strings.Contains(result, "issue-parent") {
			t.Error("expected parent ID in output")
		}
		if !strings.Contains(result, "blocking:") {
			t.Error("expected 'blocking:' label in output")
		}
		if !strings.Contains(result, "issue-child") {
			t.Error("expected blocking ID in output")
		}
		if !strings.Contains(result, "blocked by:") {
			t.Error("expected 'blocked by:' label in output")
		}
		if !strings.Contains(result, "issue-dep1") {
			t.Error("expected first blocker ID in output")
		}
		if !strings.Contains(result, "issue-dep2") {
			t.Error("expected second blocker ID in output")
		}

		// Verify ordering: parent before blocking before blocked by
		parentIdx := strings.Index(result, "parent:")
		blockingIdx := strings.Index(result, "blocking:")
		blockedByIdx := strings.Index(result, "blocked by:")
		if parentIdx >= blockingIdx {
			t.Error("expected 'parent:' to appear before 'blocking:'")
		}
		if blockingIdx >= blockedByIdx {
			t.Error("expected 'blocking:' to appear before 'blocked by:'")
		}
	})

	t.Run("multiple blocked by entries", func(t *testing.T) {
		b := &issue.Issue{
			BlockedBy: []string{"issue-a", "issue-b", "issue-c"},
		}
		result := formatRelationships(b)
		if strings.Count(result, "blocked by:") != 3 {
			t.Errorf("expected 3 'blocked by:' labels, got %d", strings.Count(result, "blocked by:"))
		}
	})

	t.Run("empty issue", func(t *testing.T) {
		b := &issue.Issue{}
		result := formatRelationships(b)
		if result != "" {
			t.Errorf("expected empty string for issue with no relationships, got %q", result)
		}
	})
}
