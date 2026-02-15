package tui

import (
	"testing"

	"github.com/toba/todo/internal/issue"
)

func TestSortIssues(t *testing.T) {
	// Define the expected order from DefaultStatuses, DefaultPriorities, and DefaultTypes
	statusNames := []string{"draft", "todo", "in-progress", "completed", "scrapped"}
	priorityNames := []string{"critical", "high", "normal", "low", "deferred"}
	typeNames := []string{"milestone", "epic", "bug", "feature", "task"}

	t.Run("sorts by status order first", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "completed", Type: "task", Title: "A"},
			{ID: "2", Status: "draft", Type: "task", Title: "B"},
			{ID: "3", Status: "in-progress", Type: "task", Title: "C"},
			{ID: "4", Status: "todo", Type: "task", Title: "D"},
			{ID: "5", Status: "scrapped", Type: "task", Title: "E"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		expected := []string{"draft", "todo", "in-progress", "completed", "scrapped"}
		for i, want := range expected {
			if issues[i].Status != want {
				t.Errorf("index %d: got status %q, want %q", i, issues[i].Status, want)
			}
		}
	})

	t.Run("sorts by priority within same status", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "todo", Type: "task", Priority: "low", Title: "A"},
			{ID: "2", Status: "todo", Type: "task", Priority: "critical", Title: "B"},
			{ID: "3", Status: "todo", Type: "task", Priority: "high", Title: "C"},
			{ID: "4", Status: "todo", Type: "task", Priority: "", Title: "D"},       // empty = normal
			{ID: "5", Status: "todo", Type: "task", Priority: "deferred", Title: "E"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		// Order: critical, high, normal (empty), low, deferred
		expectedPriorities := []string{"critical", "high", "", "low", "deferred"}
		for i, want := range expectedPriorities {
			if issues[i].Priority != want {
				t.Errorf("index %d: got priority %q, want %q", i, issues[i].Priority, want)
			}
		}
	})

	t.Run("sorts by type order within same status and priority", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "todo", Type: "task", Title: "A"},
			{ID: "2", Status: "todo", Type: "milestone", Title: "B"},
			{ID: "3", Status: "todo", Type: "bug", Title: "C"},
			{ID: "4", Status: "todo", Type: "epic", Title: "D"},
			{ID: "5", Status: "todo", Type: "feature", Title: "E"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		expected := []string{"milestone", "epic", "bug", "feature", "task"}
		for i, want := range expected {
			if issues[i].Type != want {
				t.Errorf("index %d: got type %q, want %q", i, issues[i].Type, want)
			}
		}
	})

	t.Run("sorts by title within same status, priority, and type", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "todo", Type: "task", Title: "Zebra"},
			{ID: "2", Status: "todo", Type: "task", Title: "Apple"},
			{ID: "3", Status: "todo", Type: "task", Title: "Mango"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		expected := []string{"Apple", "Mango", "Zebra"}
		for i, want := range expected {
			if issues[i].Title != want {
				t.Errorf("index %d: got title %q, want %q", i, issues[i].Title, want)
			}
		}
	})

	t.Run("title sort is case-insensitive", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "todo", Type: "task", Title: "zebra"},
			{ID: "2", Status: "todo", Type: "task", Title: "Apple"},
			{ID: "3", Status: "todo", Type: "task", Title: "MANGO"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		expected := []string{"Apple", "MANGO", "zebra"}
		for i, want := range expected {
			if issues[i].Title != want {
				t.Errorf("index %d: got title %q, want %q", i, issues[i].Title, want)
			}
		}
	})

	t.Run("combined sort order: status > priority > type > title", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "completed", Type: "bug", Title: "Z"},
			{ID: "2", Status: "todo", Type: "task", Priority: "low", Title: "A"},
			{ID: "3", Status: "todo", Type: "bug", Priority: "high", Title: "B"},
			{ID: "4", Status: "todo", Type: "bug", Priority: "high", Title: "A"},
			{ID: "5", Status: "draft", Type: "epic", Title: "X"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		// Expected order:
		// 1. draft/epic/X (ID: 5)
		// 2. todo/high/bug/A (ID: 4)
		// 3. todo/high/bug/B (ID: 3)
		// 4. todo/low/task/A (ID: 2)
		// 5. completed/bug/Z (ID: 1)
		expectedIDs := []string{"5", "4", "3", "2", "1"}
		for i, want := range expectedIDs {
			if issues[i].ID != want {
				t.Errorf("index %d: got ID %q, want %q (status=%s, priority=%s, type=%s, title=%s)",
					i, issues[i].ID, want, issues[i].Status, issues[i].Priority, issues[i].Type, issues[i].Title)
			}
		}
	})

	t.Run("unrecognized status sorts last", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "unknown", Type: "task", Title: "A"},
			{ID: "2", Status: "todo", Type: "task", Title: "B"},
			{ID: "3", Status: "draft", Type: "task", Title: "C"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		// unknown status should be last
		if issues[2].Status != "unknown" {
			t.Errorf("unrecognized status should be last, got %q at position 2", issues[2].Status)
		}
	})

	t.Run("unrecognized type sorts last within status", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "todo", Type: "unknown", Title: "A"},
			{ID: "2", Status: "todo", Type: "task", Title: "B"},
			{ID: "3", Status: "todo", Type: "bug", Title: "C"},
		}

		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)

		// unknown type should be last within todo status
		if issues[2].Type != "unknown" {
			t.Errorf("unrecognized type should be last, got %q at position 2", issues[2].Type)
		}
	})

	t.Run("empty slice does not panic", func(t *testing.T) {
		issues := []*issue.Issue{}
		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)
		// No assertion needed, just checking it doesn't panic
	})

	t.Run("single issue does not panic", func(t *testing.T) {
		issues := []*issue.Issue{
			{ID: "1", Status: "todo", Type: "task", Title: "A"},
		}
		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)
		if issues[0].ID != "1" {
			t.Error("single issue should remain unchanged")
		}
	})
}

func TestSubstringFilter(t *testing.T) {
	targets := []string{
		"Update the map",
		"My name is Peter",
		"Fix mapping logic",
		"Deploy to prod",
	}

	t.Run("matches contiguous substring", func(t *testing.T) {
		ranks := substringFilter("map", targets)
		// Should match "Update the map" and "Fix mapping logic", but NOT "My name is Peter"
		if len(ranks) != 2 {
			t.Fatalf("expected 2 matches, got %d", len(ranks))
		}
		if ranks[0].Index != 0 {
			t.Errorf("expected first match at index 0, got %d", ranks[0].Index)
		}
		if ranks[1].Index != 2 {
			t.Errorf("expected second match at index 2, got %d", ranks[1].Index)
		}
	})

	t.Run("does not match scattered characters", func(t *testing.T) {
		ranks := substringFilter("map", []string{"My name is Peter"})
		if len(ranks) != 0 {
			t.Errorf("expected 0 matches for scattered characters, got %d", len(ranks))
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		ranks := substringFilter("MAP", targets)
		if len(ranks) != 2 {
			t.Fatalf("expected 2 matches with uppercase term, got %d", len(ranks))
		}

		ranks = substringFilter("Map", []string{"update the map"})
		if len(ranks) != 1 {
			t.Fatalf("expected 1 match with mixed case, got %d", len(ranks))
		}
	})

	t.Run("empty term returns all items", func(t *testing.T) {
		ranks := substringFilter("", targets)
		if len(ranks) != len(targets) {
			t.Errorf("expected %d matches for empty term, got %d", len(targets), len(ranks))
		}
	})

	t.Run("matched indexes are correct", func(t *testing.T) {
		ranks := substringFilter("map", []string{"Update the map"})
		if len(ranks) != 1 {
			t.Fatalf("expected 1 match, got %d", len(ranks))
		}
		// "map" starts at index 11 in "Update the map"
		expected := []int{11, 12, 13}
		if len(ranks[0].MatchedIndexes) != len(expected) {
			t.Fatalf("expected %d matched indexes, got %d", len(expected), len(ranks[0].MatchedIndexes))
		}
		for i, idx := range ranks[0].MatchedIndexes {
			if idx != expected[i] {
				t.Errorf("matched index %d: got %d, want %d", i, idx, expected[i])
			}
		}
	})

	t.Run("no matches returns empty slice", func(t *testing.T) {
		ranks := substringFilter("xyz", targets)
		if len(ranks) != 0 {
			t.Errorf("expected 0 matches, got %d", len(ranks))
		}
	})
}

func TestCompareIssuesByStatusPriorityAndType(t *testing.T) {
	statusNames := []string{"draft", "todo", "in-progress", "completed", "scrapped"}
	priorityNames := []string{"critical", "high", "normal", "low", "deferred"}
	typeNames := []string{"milestone", "epic", "bug", "feature", "task"}

	t.Run("compares by status first", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "task", Title: "A"}
		b := &issue.Issue{ID: "2", Status: "draft", Type: "task", Title: "B"}

		// draft < todo, so b should come before a
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("draft issue should come before todo issue")
		}
		if !compareIssuesByStatusPriorityAndType(b, a, statusNames, priorityNames, typeNames) {
			t.Error("draft issue should come before todo issue")
		}
	})

	t.Run("compares by priority within same status", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "task", Priority: "low", Title: "A"}
		b := &issue.Issue{ID: "2", Status: "todo", Type: "task", Priority: "high", Title: "B"}

		// high < low, so b should come before a
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("high priority issue should come before low priority issue")
		}
		if !compareIssuesByStatusPriorityAndType(b, a, statusNames, priorityNames, typeNames) {
			t.Error("high priority issue should come before low priority issue")
		}
	})

	t.Run("compares by type within same status and priority", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "task", Title: "A"}
		b := &issue.Issue{ID: "2", Status: "todo", Type: "bug", Title: "B"}

		// bug < task, so b should come before a
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("bug issue should come before task issue")
		}
		if !compareIssuesByStatusPriorityAndType(b, a, statusNames, priorityNames, typeNames) {
			t.Error("bug issue should come before task issue")
		}
	})

	t.Run("compares by title within same status, priority, and type", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "task", Title: "Zebra"}
		b := &issue.Issue{ID: "2", Status: "todo", Type: "task", Title: "Apple"}

		// Apple < Zebra, so b should come before a
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("Apple issue should come before Zebran issue")
		}
		if !compareIssuesByStatusPriorityAndType(b, a, statusNames, priorityNames, typeNames) {
			t.Error("Apple issue should come before Zebran issue")
		}
	})

	t.Run("title comparison is case-insensitive", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "task", Title: "zebra"}
		b := &issue.Issue{ID: "2", Status: "todo", Type: "task", Title: "APPLE"}

		// apple < zebra (case-insensitive), so b should come before a
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("APPLE issue should come before zebran issue (case-insensitive)")
		}
	})

	t.Run("empty priority treated as normal", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "task", Priority: "", Title: "A"}
		b := &issue.Issue{ID: "2", Status: "todo", Type: "task", Priority: "normal", Title: "B"}

		// Both should be equivalent in priority ordering
		// Since titles differ, A < B, so a should come before b
		if !compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("empty priority should be treated as normal")
		}
	})

	t.Run("unrecognized status sorts last", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "unknown", Type: "task", Title: "A"}
		b := &issue.Issue{ID: "2", Status: "scrapped", Type: "task", Title: "B"}

		// scrapped is last known status, unknown should be after it
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("unknown status should sort after scrapped")
		}
		if !compareIssuesByStatusPriorityAndType(b, a, statusNames, priorityNames, typeNames) {
			t.Error("scrapped should sort before unknown")
		}
	})

	t.Run("unrecognized type sorts last within status", func(t *testing.T) {
		a := &issue.Issue{ID: "1", Status: "todo", Type: "unknown", Title: "A"}
		b := &issue.Issue{ID: "2", Status: "todo", Type: "task", Title: "B"}

		// task is last known type, unknown should be after it
		if compareIssuesByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames) {
			t.Error("unknown type should sort after task")
		}
		if !compareIssuesByStatusPriorityAndType(b, a, statusNames, priorityNames, typeNames) {
			t.Error("task should sort before unknown")
		}
	})
}

func TestIssueItemFilterValue(t *testing.T) {
	deepSearch := false
	item := issueItem{
		issue:       &issue.Issue{ID: "abc", Title: "Fix bug", Body: "Login page crashes"},
		deepSearch: &deepSearch,
	}

	t.Run("normal mode", func(t *testing.T) {
		deepSearch = false
		if got := item.FilterValue(); got != "Fix bug abc" {
			t.Errorf("got %q, want %q", got, "Fix bug abc")
		}
	})

	t.Run("deep search mode", func(t *testing.T) {
		deepSearch = true
		want := "Fix bug abc Login page crashes"
		if got := item.FilterValue(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("nil pointer defaults to normal", func(t *testing.T) {
		item2 := issueItem{issue: &issue.Issue{ID: "x", Title: "T"}}
		if got := item2.FilterValue(); got != "T x" {
			t.Errorf("got %q, want %q", got, "T x")
		}
	})

	t.Run("deep search pointer invalidated by value copy (reproduces bug)", func(t *testing.T) {
		// This test reproduces the actual bug: listModel has `deepSearch bool`
		// as a plain field, and items store `&m.deepSearch`. When the model is
		// value-copied (as Bubble Tea does on every Update), the copy gets its
		// own `deepSearch` field at a different address. Toggling deepSearch on
		// the copy does NOT affect items that point to the original's field.
		type brokenModel struct {
			deepSearch bool // plain bool — address changes on copy
		}

		original := brokenModel{deepSearch: false}

		item := issueItem{
			issue:       &issue.Issue{ID: "abc", Title: "Fix bug", Body: "Homepage crashes on load"},
			deepSearch: &original.deepSearch,
		}

		// Simulate Bubble Tea value copy
		copied := original
		// Toggle on the copy
		copied.deepSearch = true

		// The item still points to original.deepSearch which is still false!
		got := item.FilterValue()
		// This SHOULD include the body, but the bug means it won't
		wantBroken := "Fix bug abc" // bug: body is missing
		if got != wantBroken {
			t.Errorf("expected broken behavior: got %q, want %q", got, wantBroken)
		}
	})

	t.Run("deep search survives value copy with heap-allocated pointer (fix)", func(t *testing.T) {
		// The fix: use *bool (heap-allocated) so all copies share the same
		// underlying bool value.
		type fixedModel struct {
			deepSearch *bool // pointer to heap — survives value copy
		}

		ds := false
		original := fixedModel{deepSearch: &ds}

		item := issueItem{
			issue:       &issue.Issue{ID: "abc", Title: "Fix bug", Body: "Homepage crashes on load"},
			deepSearch: original.deepSearch,
		}

		// Simulate Bubble Tea value copy
		copied := original
		// Toggle on the copy — both copies share the same *bool
		*copied.deepSearch = true

		// The item sees the updated value
		want := "Fix bug abc Homepage crashes on load"
		if got := item.FilterValue(); got != want {
			t.Errorf("after value copy toggle: got %q, want %q", got, want)
		}
	})
}
