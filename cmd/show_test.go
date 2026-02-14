package cmd

import (
	"strings"
	"testing"

	"github.com/toba/todo/internal/issue"
)

func TestFormatRelationships(t *testing.T) {
	t.Run("blocked by only", func(t *testing.T) {
		b := &issue.Issue{
			BlockedBy: []string{"bean-abc"},
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "blocked by:") {
			t.Error("expected 'blocked by:' label in output")
		}
		if !strings.Contains(result, "bean-abc") {
			t.Error("expected blocker ID 'bean-abc' in output")
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
			Blocking:  []string{"bean-def"},
			BlockedBy: []string{"bean-ghi"},
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "blocking:") {
			t.Error("expected 'blocking:' label in output")
		}
		if !strings.Contains(result, "bean-def") {
			t.Error("expected blocking ID 'bean-def' in output")
		}
		if !strings.Contains(result, "blocked by:") {
			t.Error("expected 'blocked by:' label in output")
		}
		if !strings.Contains(result, "bean-ghi") {
			t.Error("expected blocker ID 'bean-ghi' in output")
		}
	})

	t.Run("parent only", func(t *testing.T) {
		b := &issue.Issue{
			Parent: "bean-parent",
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "parent:") {
			t.Error("expected 'parent:' label in output")
		}
		if !strings.Contains(result, "bean-parent") {
			t.Error("expected parent ID 'bean-parent' in output")
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
			Parent:    "bean-parent",
			Blocking:  []string{"bean-child"},
			BlockedBy: []string{"bean-dep1", "bean-dep2"},
		}
		result := formatRelationships(b)
		if !strings.Contains(result, "parent:") {
			t.Error("expected 'parent:' label in output")
		}
		if !strings.Contains(result, "bean-parent") {
			t.Error("expected parent ID in output")
		}
		if !strings.Contains(result, "blocking:") {
			t.Error("expected 'blocking:' label in output")
		}
		if !strings.Contains(result, "bean-child") {
			t.Error("expected blocking ID in output")
		}
		if !strings.Contains(result, "blocked by:") {
			t.Error("expected 'blocked by:' label in output")
		}
		if !strings.Contains(result, "bean-dep1") {
			t.Error("expected first blocker ID in output")
		}
		if !strings.Contains(result, "bean-dep2") {
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
			BlockedBy: []string{"bean-a", "bean-b", "bean-c"},
		}
		result := formatRelationships(b)
		if strings.Count(result, "blocked by:") != 3 {
			t.Errorf("expected 3 'blocked by:' labels, got %d", strings.Count(result, "blocked by:"))
		}
	})

	t.Run("empty bean", func(t *testing.T) {
		b := &issue.Issue{}
		result := formatRelationships(b)
		if result != "" {
			t.Errorf("expected empty string for bean with no relationships, got %q", result)
		}
	})
}
