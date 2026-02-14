package issue

import (
	"testing"
	"time"
)

func TestSortByStatusPriorityAndType(t *testing.T) {
	statusNames := []string{"draft", "todo", "in-progress", "completed"}
	priorityNames := []string{"critical", "high", "normal", "low", "deferred"}
	typeNames := []string{"bug", "feature", "task"}

	t.Run("sorts by status first", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "A", Status: "completed", Priority: "critical"},
			{ID: "2", Title: "B", Status: "todo", Priority: "low"},
			{ID: "3", Title: "C", Status: "draft", Priority: "high"},
		}

		SortByStatusPriorityAndType(beans, statusNames, priorityNames, typeNames)

		if beans[0].Status != "draft" {
			t.Errorf("First bean status = %q, want \"draft\"", beans[0].Status)
		}
		if beans[1].Status != "todo" {
			t.Errorf("Second bean status = %q, want \"todo\"", beans[1].Status)
		}
		if beans[2].Status != "completed" {
			t.Errorf("Third bean status = %q, want \"completed\"", beans[2].Status)
		}
	})

	t.Run("sorts by priority within same status", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "E Low", Status: "todo", Priority: "low"},
			{ID: "2", Title: "A Critical", Status: "todo", Priority: "critical"},
			{ID: "3", Title: "B High", Status: "todo", Priority: "high"},
			{ID: "4", Title: "C Normal", Status: "todo", Priority: "normal"},
			{ID: "5", Title: "D No Priority", Status: "todo", Priority: ""},
		}

		SortByStatusPriorityAndType(beans, statusNames, priorityNames, typeNames)

		// Order by priority: critical, high, normal (and empty), low, deferred
		// Within same priority, order by title alphabetically
		expectedOrder := []string{"A Critical", "B High", "C Normal", "D No Priority", "E Low"}
		for i, expected := range expectedOrder {
			if beans[i].Title != expected {
				t.Errorf("beans[%d].Title = %q, want %q", i, beans[i].Title, expected)
			}
		}
	})

	t.Run("empty priority treated as normal", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Low", Status: "todo", Priority: "low"},
			{ID: "2", Title: "Empty", Status: "todo", Priority: ""},
			{ID: "3", Title: "Normal", Status: "todo", Priority: "normal"},
			{ID: "4", Title: "High", Status: "todo", Priority: "high"},
		}

		SortByStatusPriorityAndType(beans, statusNames, priorityNames, typeNames)

		// High should come first, then Normal and Empty (same priority level), then Low
		if beans[0].Title != "High" {
			t.Errorf("First bean = %q, want \"High\"", beans[0].Title)
		}
		if beans[3].Title != "Low" {
			t.Errorf("Last bean = %q, want \"Low\"", beans[3].Title)
		}
		// Empty and Normal should be adjacent (both at normal priority level)
		normalIdx, emptyIdx := -1, -1
		for i, b := range beans {
			if b.Title == "Normal" {
				normalIdx = i
			}
			if b.Title == "Empty" {
				emptyIdx = i
			}
		}
		if normalIdx != 1 && normalIdx != 2 {
			t.Errorf("Normal should be at index 1 or 2, got %d", normalIdx)
		}
		if emptyIdx != 1 && emptyIdx != 2 {
			t.Errorf("Empty should be at index 1 or 2, got %d", emptyIdx)
		}
	})

	t.Run("sorts by type after priority", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Task", Status: "todo", Priority: "high", Type: "task"},
			{ID: "2", Title: "Bug", Status: "todo", Priority: "high", Type: "bug"},
			{ID: "3", Title: "Feature", Status: "todo", Priority: "high", Type: "feature"},
		}

		SortByStatusPriorityAndType(beans, statusNames, priorityNames, typeNames)

		if beans[0].Type != "bug" {
			t.Errorf("First bean type = %q, want \"bug\"", beans[0].Type)
		}
		if beans[1].Type != "feature" {
			t.Errorf("Second bean type = %q, want \"feature\"", beans[1].Type)
		}
		if beans[2].Type != "task" {
			t.Errorf("Third bean type = %q, want \"task\"", beans[2].Type)
		}
	})

	t.Run("sorts by title after type", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Zebra", Status: "todo", Priority: "high", Type: "bug"},
			{ID: "2", Title: "Apple", Status: "todo", Priority: "high", Type: "bug"},
			{ID: "3", Title: "Mango", Status: "todo", Priority: "high", Type: "bug"},
		}

		SortByStatusPriorityAndType(beans, statusNames, priorityNames, typeNames)

		if beans[0].Title != "Apple" {
			t.Errorf("First bean title = %q, want \"Apple\"", beans[0].Title)
		}
		if beans[1].Title != "Mango" {
			t.Errorf("Second bean title = %q, want \"Mango\"", beans[1].Title)
		}
		if beans[2].Title != "Zebra" {
			t.Errorf("Third bean title = %q, want \"Zebra\"", beans[2].Title)
		}
	})

	t.Run("handles nil priority names gracefully", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "A", Status: "todo", Priority: "high"},
			{ID: "2", Title: "B", Status: "todo", Priority: ""},
		}

		// Should not panic with nil priorityNames
		SortByStatusPriorityAndType(beans, statusNames, nil, typeNames)

		// Both should be sorted by status, type, then title
		if beans[0].Title != "A" {
			t.Errorf("First bean title = %q, want \"A\"", beans[0].Title)
		}
	})
}

//go:fix inline
func timePtr(t time.Time) *time.Time { return new(t) }

func TestComputeEffectiveDates(t *testing.T) {
	now := time.Now()

	t.Run("bean without children uses own date", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", CreatedAt: new(now)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		if !dates["1"].Equal(now) {
			t.Errorf("effective date = %v, want %v", dates["1"], now)
		}
	})

	t.Run("parent inherits newest child date", func(t *testing.T) {
		parentTime := now.Add(-2 * time.Hour)
		childTime := now.Add(-1 * time.Hour)
		newestChildTime := now

		beans := []*Issue{
			{ID: "parent", CreatedAt: new(parentTime)},
			{ID: "child1", Parent: "parent", CreatedAt: new(childTime)},
			{ID: "child2", Parent: "parent", CreatedAt: new(newestChildTime)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		if !dates["parent"].Equal(newestChildTime) {
			t.Errorf("parent effective date = %v, want %v", dates["parent"], newestChildTime)
		}
	})

	t.Run("parent keeps own date if newer than children", func(t *testing.T) {
		parentTime := now
		childTime := now.Add(-1 * time.Hour)

		beans := []*Issue{
			{ID: "parent", CreatedAt: new(parentTime)},
			{ID: "child", Parent: "parent", CreatedAt: new(childTime)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		if !dates["parent"].Equal(parentTime) {
			t.Errorf("parent effective date = %v, want %v", dates["parent"], parentTime)
		}
	})

	t.Run("propagates through grandchildren", func(t *testing.T) {
		grandchildTime := now

		beans := []*Issue{
			{ID: "root", CreatedAt: new(now.Add(-3 * time.Hour))},
			{ID: "child", Parent: "root", CreatedAt: new(now.Add(-2 * time.Hour))},
			{ID: "grandchild", Parent: "child", CreatedAt: new(grandchildTime)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		if !dates["root"].Equal(grandchildTime) {
			t.Errorf("root effective date = %v, want %v", dates["root"], grandchildTime)
		}
		if !dates["child"].Equal(grandchildTime) {
			t.Errorf("child effective date = %v, want %v", dates["child"], grandchildTime)
		}
	})

	t.Run("works with updated_at field", func(t *testing.T) {
		updatedTime := now

		beans := []*Issue{
			{ID: "parent", UpdatedAt: new(now.Add(-1 * time.Hour))},
			{ID: "child", Parent: "parent", UpdatedAt: new(updatedTime)},
		}
		dates := ComputeEffectiveDates(beans, "updated_at")
		if !dates["parent"].Equal(updatedTime) {
			t.Errorf("parent effective date = %v, want %v", dates["parent"], updatedTime)
		}
	})

	t.Run("handles beans with nil dates", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", CreatedAt: nil},
			{ID: "2", CreatedAt: new(now)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		if !dates["1"].IsZero() {
			t.Errorf("bean without date should have zero time, got %v", dates["1"])
		}
		if !dates["2"].Equal(now) {
			t.Errorf("bean with date = %v, want %v", dates["2"], now)
		}
	})
}

func TestSortByCreatedAt(t *testing.T) {
	now := time.Now()

	t.Run("sorts newest first", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Old", CreatedAt: new(now.Add(-2 * time.Hour))},
			{ID: "2", Title: "New", CreatedAt: new(now)},
			{ID: "3", Title: "Mid", CreatedAt: new(now.Add(-1 * time.Hour))},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		SortByCreatedAt(beans, dates)

		expected := []string{"New", "Mid", "Old"}
		for i, title := range expected {
			if beans[i].Title != title {
				t.Errorf("beans[%d].Title = %q, want %q", i, beans[i].Title, title)
			}
		}
	})

	t.Run("beans without dates sort last", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "No Date"},
			{ID: "2", Title: "Has Date", CreatedAt: new(now)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		SortByCreatedAt(beans, dates)

		if beans[0].Title != "Has Date" {
			t.Errorf("first = %q, want \"Has Date\"", beans[0].Title)
		}
		if beans[1].Title != "No Date" {
			t.Errorf("second = %q, want \"No Date\"", beans[1].Title)
		}
	})

	t.Run("ties broken by title", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Zebra", CreatedAt: new(now)},
			{ID: "2", Title: "Apple", CreatedAt: new(now)},
		}
		dates := ComputeEffectiveDates(beans, "created_at")
		SortByCreatedAt(beans, dates)

		if beans[0].Title != "Apple" {
			t.Errorf("first = %q, want \"Apple\"", beans[0].Title)
		}
	})
}

func TestSortByUpdatedAt(t *testing.T) {
	now := time.Now()

	t.Run("sorts newest first", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Old", UpdatedAt: new(now.Add(-2 * time.Hour))},
			{ID: "2", Title: "New", UpdatedAt: new(now)},
			{ID: "3", Title: "Mid", UpdatedAt: new(now.Add(-1 * time.Hour))},
		}
		dates := ComputeEffectiveDates(beans, "updated_at")
		SortByUpdatedAt(beans, dates)

		expected := []string{"New", "Mid", "Old"}
		for i, title := range expected {
			if beans[i].Title != title {
				t.Errorf("beans[%d].Title = %q, want %q", i, beans[i].Title, title)
			}
		}
	})

	t.Run("beans without dates sort last", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "No Date"},
			{ID: "2", Title: "Has Date", UpdatedAt: new(now)},
		}
		dates := ComputeEffectiveDates(beans, "updated_at")
		SortByUpdatedAt(beans, dates)

		if beans[0].Title != "Has Date" {
			t.Errorf("first = %q, want \"Has Date\"", beans[0].Title)
		}
	})
}

func TestSortByDueDate(t *testing.T) {
	t.Run("sorts soonest first", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Far", Due: NewDueDate(time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC))},
			{ID: "2", Title: "Soon", Due: NewDueDate(time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC))},
			{ID: "3", Title: "Mid", Due: NewDueDate(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))},
		}
		SortByDueDate(beans)

		expected := []string{"Soon", "Mid", "Far"}
		for i, title := range expected {
			if beans[i].Title != title {
				t.Errorf("beans[%d].Title = %q, want %q", i, beans[i].Title, title)
			}
		}
	})

	t.Run("nil due dates sort last", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "No Due"},
			{ID: "2", Title: "Has Due", Due: NewDueDate(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))},
			{ID: "3", Title: "Also No Due"},
		}
		SortByDueDate(beans)

		if beans[0].Title != "Has Due" {
			t.Errorf("first = %q, want \"Has Due\"", beans[0].Title)
		}
		// Nil dues sorted by title
		if beans[1].Title != "Also No Due" {
			t.Errorf("second = %q, want \"Also No Due\"", beans[1].Title)
		}
		if beans[2].Title != "No Due" {
			t.Errorf("third = %q, want \"No Due\"", beans[2].Title)
		}
	})

	t.Run("ties broken by title", func(t *testing.T) {
		same := NewDueDate(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
		beans := []*Issue{
			{ID: "1", Title: "Zebra", Due: same},
			{ID: "2", Title: "Apple", Due: same},
		}
		SortByDueDate(beans)

		if beans[0].Title != "Apple" {
			t.Errorf("first = %q, want \"Apple\"", beans[0].Title)
		}
	})

	t.Run("all nil due dates", func(t *testing.T) {
		beans := []*Issue{
			{ID: "1", Title: "Zebra"},
			{ID: "2", Title: "Apple"},
		}
		SortByDueDate(beans)

		if beans[0].Title != "Apple" {
			t.Errorf("first = %q, want \"Apple\"", beans[0].Title)
		}
	})
}
