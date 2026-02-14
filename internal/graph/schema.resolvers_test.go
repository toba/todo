package graph

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph/model"
)

func setupTestResolver(t *testing.T) (*Resolver, *core.Core) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".issues")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create test .issues dir: %v", err)
	}

	cfg := config.Default()
	c := core.New(dataDir, cfg)
	if err := c.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return &Resolver{Core: c}, c
}

func createTestBean(t *testing.T, c *core.Core, id, title, status string) *issue.Issue {
	t.Helper()
	b := &issue.Issue{
		ID:     id,
		Slug:   issue.Slugify(title),
		Title:  title,
		Status: status,
	}
	if err := c.Create(b); err != nil {
		t.Fatalf("failed to create test bean: %v", err)
	}
	return b
}

func TestQueryBean(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test bean
	createTestBean(t, c, "test-1", "Test Bean", "todo")

	// Test exact match
	t.Run("exact match", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Issue(ctx, "test-1")
		if err != nil {
			t.Fatalf("Bean() error = %v", err)
		}
		if got == nil {
			t.Fatal("Bean() returned nil")
		}
		if got.ID != "test-1" {
			t.Errorf("Bean().ID = %q, want %q", got.ID, "test-1")
		}
	})

	// Test partial ID not found (no prefix matching)
	t.Run("partial ID not found", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Issue(ctx, "test")
		if err != nil {
			t.Fatalf("Bean() error = %v", err)
		}
		if got != nil {
			t.Errorf("Bean() = %v, want nil (partial IDs should not match)", got)
		}
	})

	// Test not found
	t.Run("not found", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Issue(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Bean() error = %v", err)
		}
		if got != nil {
			t.Errorf("Bean() = %v, want nil", got)
		}
	})
}

func TestQueryBeans(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test beans
	createTestBean(t, c, "bean-1", "First Bean", "todo")
	createTestBean(t, c, "bean-2", "Second Bean", "in-progress")
	createTestBean(t, c, "bean-3", "Third Bean", "completed")

	t.Run("no filter", func(t *testing.T) {
		qr := resolver.Query()
		got, err := qr.Issues(ctx, nil)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("Beans() count = %d, want 3", len(got))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Status: []string{"todo"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
		if got[0].ID != "bean-1" {
			t.Errorf("Beans()[0].ID = %q, want %q", got[0].ID, "bean-1")
		}
	})

	t.Run("filter by multiple statuses", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Status: []string{"todo", "in-progress"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
	})

	t.Run("exclude status", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			ExcludeStatus: []string{"completed"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
	})
}

func TestQueryBeansWithTags(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test beans with tags
	b1 := &issue.Issue{ID: "tag-1", Title: "Tagged 1", Status: "todo", Tags: []string{"frontend", "urgent"}}
	b2 := &issue.Issue{ID: "tag-2", Title: "Tagged 2", Status: "todo", Tags: []string{"backend"}}
	b3 := &issue.Issue{ID: "tag-3", Title: "No Tags", Status: "todo"}
	c.Create(b1)
	c.Create(b2)
	c.Create(b3)

	t.Run("filter by tag", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Tags: []string{"frontend"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
	})

	t.Run("filter by multiple tags (OR)", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Tags: []string{"frontend", "backend"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
	})

	t.Run("exclude by tag", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			ExcludeTags: []string{"urgent"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
	})
}

func TestQueryBeansWithPriority(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test beans with various priorities
	// Empty priority should be treated as "normal"
	b1 := &issue.Issue{ID: "pri-1", Title: "Critical", Status: "todo", Priority: "critical"}
	b2 := &issue.Issue{ID: "pri-2", Title: "High", Status: "todo", Priority: "high"}
	b3 := &issue.Issue{ID: "pri-3", Title: "Normal Explicit", Status: "todo", Priority: "normal"}
	b4 := &issue.Issue{ID: "pri-4", Title: "Normal Implicit", Status: "todo", Priority: ""} // empty = normal
	b5 := &issue.Issue{ID: "pri-5", Title: "Low", Status: "todo", Priority: "low"}
	c.Create(b1)
	c.Create(b2)
	c.Create(b3)
	c.Create(b4)
	c.Create(b5)

	t.Run("filter by normal includes empty priority", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Priority: []string{"normal"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		// Should include both explicit "normal" and implicit (empty) priority
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}
		if !ids["pri-3"] || !ids["pri-4"] {
			t.Errorf("Beans() should include pri-3 and pri-4, got %v", ids)
		}
	})

	t.Run("filter by critical", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Priority: []string{"critical"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
		if got[0].ID != "pri-1" {
			t.Errorf("Beans()[0].ID = %q, want %q", got[0].ID, "pri-1")
		}
	})

	t.Run("filter by multiple priorities", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			Priority: []string{"critical", "high"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
	})

	t.Run("exclude normal excludes empty priority", func(t *testing.T) {
		qr := resolver.Query()
		filter := &model.IssueFilter{
			ExcludePriority: []string{"normal"},
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		// Should exclude both explicit "normal" and implicit (empty) priority
		if len(got) != 3 {
			t.Errorf("Beans() count = %d, want 3", len(got))
		}
		for _, b := range got {
			if b.ID == "pri-3" || b.ID == "pri-4" {
				t.Errorf("Beans() should not include %s", b.ID)
			}
		}
	})
}

func TestBeanRelationships(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create beans with relationships
	parent := &issue.Issue{ID: "parent-1", Title: "Parent", Status: "todo"}
	child1 := &issue.Issue{
		ID:     "child-1",
		Title:  "Child 1",
		Status: "todo",
		Parent: "parent-1",
	}
	child2 := &issue.Issue{
		ID:     "child-2",
		Title:  "Child 2",
		Status: "todo",
		Parent: "parent-1",
	}
	blocker := &issue.Issue{
		ID:       "blocker-1",
		Title:    "Blocker",
		Status:   "todo",
		Blocking: []string{"child-1"},
	}

	c.Create(parent)
	c.Create(child1)
	c.Create(child2)
	c.Create(blocker)

	t.Run("parent resolver", func(t *testing.T) {
		br := resolver.Issue()
		got, err := br.Parent(ctx, child1)
		if err != nil {
			t.Fatalf("Parent() error = %v", err)
		}
		if got == nil {
			t.Fatal("Parent() returned nil")
		}
		if got.ID != "parent-1" {
			t.Errorf("Parent().ID = %q, want %q", got.ID, "parent-1")
		}
	})

	t.Run("children resolver", func(t *testing.T) {
		br := resolver.Issue()
		got, err := br.Children(ctx, parent, nil)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Children() count = %d, want 2", len(got))
		}
	})

	t.Run("blockedBy resolver", func(t *testing.T) {
		br := resolver.Issue()
		got, err := br.BlockedBy(ctx, child1, nil)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("BlockedBy() count = %d, want 1", len(got))
		}
		if got[0].ID != "blocker-1" {
			t.Errorf("BlockedBy()[0].ID = %q, want %q", got[0].ID, "blocker-1")
		}
	})

	t.Run("blocks resolver", func(t *testing.T) {
		br := resolver.Issue()
		got, err := br.Blocking(ctx, blocker, nil)
		if err != nil {
			t.Fatalf("Blocks() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Blocks() count = %d, want 1", len(got))
		}
		if got[0].ID != "child-1" {
			t.Errorf("Blocks()[0].ID = %q, want %q", got[0].ID, "child-1")
		}
	})
}

func TestBrokenLinksFiltered(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create bean with broken link
	b := &issue.Issue{
		ID:     "orphan-1",
		Title:  "Orphan",
		Status: "todo",
		Parent: "nonexistent",
	}
	c.Create(b)

	t.Run("broken parent link returns nil", func(t *testing.T) {
		br := resolver.Issue()
		got, err := br.Parent(ctx, b)
		if err != nil {
			t.Fatalf("Parent() error = %v", err)
		}
		if got != nil {
			t.Errorf("Parent() = %v, want nil for broken link", got)
		}
	})
}

func TestQueryBeansWithParentAndBlocks(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create beans with various relationship configurations
	noRels := &issue.Issue{ID: "no-rels", Title: "No Relationships", Status: "todo"}
	hasParent := &issue.Issue{
		ID:     "has-parent",
		Title:  "Has Parent",
		Status: "todo",
		Parent: "no-rels",
	}
	hasBlocks := &issue.Issue{
		ID:       "has-blocks",
		Title:    "Has Blocks",
		Status:   "todo",
		Blocking: []string{"has-parent"},
	}

	c.Create(noRels)
	c.Create(hasParent)
	c.Create(hasBlocks)

	t.Run("filter hasParent", func(t *testing.T) {
		qr := resolver.Query()
		hasParentBool := true
		filter := &model.IssueFilter{
			HasParent: &hasParentBool,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-parent" {
			t.Errorf("Beans()[0].ID = %q, want %q", got[0].ID, "has-parent")
		}
	})

	t.Run("filter noParent", func(t *testing.T) {
		qr := resolver.Query()
		noParentBool := true
		filter := &model.IssueFilter{
			NoParent: &noParentBool,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
	})

	t.Run("filter hasBlocks", func(t *testing.T) {
		qr := resolver.Query()
		hasBlocksBool := true
		filter := &model.IssueFilter{
			HasBlocking: &hasBlocksBool,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-blocks" {
			t.Errorf("Beans()[0].ID = %q, want %q", got[0].ID, "has-blocks")
		}
	})

	t.Run("filter isBlocked true", func(t *testing.T) {
		qr := resolver.Query()
		isBlockedBool := true
		filter := &model.IssueFilter{
			IsBlocked: &isBlockedBool,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-parent" {
			t.Errorf("Beans()[0].ID = %q, want %q", got[0].ID, "has-parent")
		}
	})

	t.Run("filter isBlocked false", func(t *testing.T) {
		qr := resolver.Query()
		isBlockedBool := false
		filter := &model.IssueFilter{
			IsBlocked: &isBlockedBool,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		// Should return all issues except "has-parent" (which is blocked by "has-blocks")
		if len(got) != 2 {
			t.Errorf("Beans() count = %d, want 2", len(got))
		}
		// Verify "has-parent" is not in results
		for _, b := range got {
			if b.ID == "has-parent" {
				t.Errorf("Beans() should not contain blocked bean 'has-parent'")
			}
		}
	})

	t.Run("filter by parentId", func(t *testing.T) {
		qr := resolver.Query()
		parentID := "no-rels"
		filter := &model.IssueFilter{
			ParentID: &parentID,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Beans() count = %d, want 1", len(got))
		}
		if got[0].ID != "has-parent" {
			t.Errorf("Beans()[0].ID = %q, want %q", got[0].ID, "has-parent")
		}
	})
}

func TestIsBlockedFilterWithResolvedBlockers(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create beans to test blocking with various blocker statuses
	activeBlocker := &issue.Issue{
		ID:       "active-blocker",
		Title:    "Active Blocker",
		Status:   "todo",
		Blocking: []string{"blocked-by-active"},
	}
	completedBlocker := &issue.Issue{
		ID:       "completed-blocker",
		Title:    "Completed Blocker",
		Status:   "completed",
		Blocking: []string{"blocked-by-completed"},
	}
	scrappedBlocker := &issue.Issue{
		ID:       "scrapped-blocker",
		Title:    "Scrapped Blocker",
		Status:   "scrapped",
		Blocking: []string{"blocked-by-scrapped"},
	}
	blockedByActive := &issue.Issue{
		ID:     "blocked-by-active",
		Title:  "Blocked by Active",
		Status: "todo",
	}
	blockedByCompleted := &issue.Issue{
		ID:     "blocked-by-completed",
		Title:  "Blocked by Completed",
		Status: "todo",
	}
	blockedByScrapped := &issue.Issue{
		ID:     "blocked-by-scrapped",
		Title:  "Blocked by Scrapped",
		Status: "todo",
	}
	notBlocked := &issue.Issue{
		ID:     "not-blocked",
		Title:  "Not Blocked",
		Status: "todo",
	}
	// Bean with mixed blockers (one active, one completed)
	mixedBlocker := &issue.Issue{
		ID:       "mixed-blocker",
		Title:    "Mixed Blocker (active)",
		Status:   "in-progress",
		Blocking: []string{"mixed-blocked"},
	}
	mixedBlockerCompleted := &issue.Issue{
		ID:       "mixed-blocker-completed",
		Title:    "Mixed Blocker (completed)",
		Status:   "completed",
		Blocking: []string{"mixed-blocked"},
	}
	mixedBlocked := &issue.Issue{
		ID:     "mixed-blocked",
		Title:  "Mixed Blocked",
		Status: "todo",
	}

	beans := []*issue.Issue{
		activeBlocker, completedBlocker, scrappedBlocker,
		blockedByActive, blockedByCompleted, blockedByScrapped,
		notBlocked, mixedBlocker, mixedBlockerCompleted, mixedBlocked,
	}
	for _, b := range beans {
		if err := c.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	t.Run("isBlocked true returns only beans with active blockers", func(t *testing.T) {
		qr := resolver.Query()
		isBlocked := true
		filter := &model.IssueFilter{
			IsBlocked: &isBlocked,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}

		// Should only return beans blocked by active blockers
		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}

		if !ids["blocked-by-active"] {
			t.Error("expected blocked-by-active in results (has active blocker)")
		}
		if !ids["mixed-blocked"] {
			t.Error("expected mixed-blocked in results (has one active blocker)")
		}
		if ids["blocked-by-completed"] {
			t.Error("blocked-by-completed should NOT be in results (blocker is completed)")
		}
		if ids["blocked-by-scrapped"] {
			t.Error("blocked-by-scrapped should NOT be in results (blocker is scrapped)")
		}
		if ids["not-blocked"] {
			t.Error("not-blocked should NOT be in results (no blockers)")
		}
	})

	t.Run("isBlocked false excludes beans with active blockers", func(t *testing.T) {
		qr := resolver.Query()
		isBlocked := false
		filter := &model.IssueFilter{
			IsBlocked: &isBlocked,
		}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}

		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}

		// Should include beans with no active blockers
		if !ids["blocked-by-completed"] {
			t.Error("expected blocked-by-completed in results (blocker is completed)")
		}
		if !ids["blocked-by-scrapped"] {
			t.Error("expected blocked-by-scrapped in results (blocker is scrapped)")
		}
		if !ids["not-blocked"] {
			t.Error("expected not-blocked in results (no blockers)")
		}
		// Should exclude beans with active blockers
		if ids["blocked-by-active"] {
			t.Error("blocked-by-active should NOT be in results (has active blocker)")
		}
		if ids["mixed-blocked"] {
			t.Error("mixed-blocked should NOT be in results (has active blocker)")
		}
	})
}

func TestMutationCreateIssue(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("create with required fields only", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateIssueInput{
			Title: "New Bean",
		}
		got, err := mr.CreateIssue(ctx, input)
		if err != nil {
			t.Fatalf("CreateIssue() error = %v", err)
		}
		if got == nil {
			t.Fatal("CreateIssue() returned nil")
		}
		if got.Title != "New Bean" {
			t.Errorf("CreateIssue().Title = %q, want %q", got.Title, "New Bean")
		}
		// Type defaults to "task"
		if got.Type != "task" {
			t.Errorf("CreateIssue().Type = %q, want %q (default)", got.Type, "task")
		}
		if got.ID == "" {
			t.Error("CreateIssue().ID is empty")
		}
	})

	t.Run("create with all fields", func(t *testing.T) {
		// Create parent and target issues first
		parentBean := &issue.Issue{
			ID:     "some-parent",
			Title:  "Parent Bean",
			Status: "todo",
			Type:   "epic",
		}
		targetBean := &issue.Issue{
			ID:     "some-target",
			Title:  "Target Bean",
			Status: "todo",
			Type:   "task",
		}
		c.Create(parentBean)
		c.Create(targetBean)

		mr := resolver.Mutation()
		beanType := "feature"
		status := "in-progress"
		priority := "high"
		body := "Test body content"
		parent := "some-parent"
		input := model.CreateIssueInput{
			Title:    "Full Bean",
			Type:     &beanType,
			Status:   &status,
			Priority: &priority,
			Body:     &body,
			Tags:     []string{"tag1", "tag2"},
			Parent:   &parent,
			Blocking: []string{"some-target"},
		}
		got, err := mr.CreateIssue(ctx, input)
		if err != nil {
			t.Fatalf("CreateIssue() error = %v", err)
		}
		if got.Type != "feature" {
			t.Errorf("CreateIssue().Type = %q, want %q", got.Type, "feature")
		}
		if got.Status != "in-progress" {
			t.Errorf("CreateIssue().Status = %q, want %q", got.Status, "in-progress")
		}
		if got.Priority != "high" {
			t.Errorf("CreateIssue().Priority = %q, want %q", got.Priority, "high")
		}
		if got.Body != "Test body content" {
			t.Errorf("CreateIssue().Body = %q, want %q", got.Body, "Test body content")
		}
		if len(got.Tags) != 2 {
			t.Errorf("CreateIssue().Tags count = %d, want 2", len(got.Tags))
		}
		if got.Parent != "some-parent" {
			t.Errorf("CreateIssue().Parent = %q, want %q", got.Parent, "some-parent")
		}
		if len(got.Blocking) != 1 {
			t.Errorf("CreateIssue().Blocking count = %d, want 1", len(got.Blocking))
		}
	})
}

func TestMutationCreateIssueGeneratesID(t *testing.T) {
	resolver, _ := setupTestResolver(t)
	ctx := context.Background()

	mr := resolver.Mutation()
	input := model.CreateIssueInput{
		Title: "Auto ID Bean",
	}
	got, err := mr.CreateIssue(ctx, input)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	// ID should be xxx-xxx format (7 chars, hyphen at pos 3)
	if len(got.ID) != 7 {
		t.Errorf("CreateIssue().ID length = %d, want 7", len(got.ID))
	}
	if got.ID[3] != '-' {
		t.Errorf("CreateIssue().ID = %q, want hyphen at position 3", got.ID)
	}
}

func TestMutationUpdateIssue(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create a test bean
	b := &issue.Issue{
		ID:       "update-test",
		Title:    "Original Title",
		Status:   "todo",
		Type:     "task",
		Priority: "normal",
		Body:     "Original body",
		Tags:     []string{"original"},
	}
	c.Create(b)

	t.Run("update single field", func(t *testing.T) {
		mr := resolver.Mutation()
		newStatus := "in-progress"
		input := model.UpdateIssueInput{
			Status: &newStatus,
		}
		got, err := mr.UpdateIssue(ctx, "update-test", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if got.Status != "in-progress" {
			t.Errorf("UpdateIssue().Status = %q, want %q", got.Status, "in-progress")
		}
		// Other fields unchanged
		if got.Title != "Original Title" {
			t.Errorf("UpdateIssue().Title = %q, want %q", got.Title, "Original Title")
		}
	})

	t.Run("update multiple fields", func(t *testing.T) {
		mr := resolver.Mutation()
		newTitle := "Updated Title"
		newPriority := "high"
		newBody := "Updated body"
		input := model.UpdateIssueInput{
			Title:    &newTitle,
			Priority: &newPriority,
			Body:     &newBody,
		}
		got, err := mr.UpdateIssue(ctx, "update-test", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if got.Title != "Updated Title" {
			t.Errorf("UpdateIssue().Title = %q, want %q", got.Title, "Updated Title")
		}
		if got.Priority != "high" {
			t.Errorf("UpdateIssue().Priority = %q, want %q", got.Priority, "high")
		}
		if got.Body != "Updated body" {
			t.Errorf("UpdateIssue().Body = %q, want %q", got.Body, "Updated body")
		}
	})

	t.Run("replace tags", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.UpdateIssueInput{
			Tags: []string{"new-tag-1", "new-tag-2"},
		}
		got, err := mr.UpdateIssue(ctx, "update-test", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if len(got.Tags) != 2 {
			t.Errorf("UpdateIssue().Tags count = %d, want 2", len(got.Tags))
		}
	})

	t.Run("update nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		newTitle := "Whatever"
		input := model.UpdateIssueInput{
			Title: &newTitle,
		}
		_, err := mr.UpdateIssue(ctx, "nonexistent", input)
		if err == nil {
			t.Error("UpdateIssue() expected error for nonexistent bean")
		}
	})
}

func TestMutationSetParent(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test beans
	parent := &issue.Issue{ID: "parent-1", Title: "Parent", Status: "todo", Type: "epic"}
	child := &issue.Issue{ID: "child-1", Title: "Child", Status: "todo", Type: "task"}
	c.Create(parent)
	c.Create(child)

	t.Run("set parent", func(t *testing.T) {
		mr := resolver.Mutation()
		parentID := "parent-1"
		got, err := mr.SetParent(ctx, "child-1", &parentID, nil)
		if err != nil {
			t.Fatalf("SetParent() error = %v", err)
		}
		if got.Parent != "parent-1" {
			t.Errorf("SetParent().Parent = %q, want %q", got.Parent, "parent-1")
		}
	})

	t.Run("clear parent", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.SetParent(ctx, "child-1", nil, nil)
		if err != nil {
			t.Fatalf("SetParent() error = %v", err)
		}
		if got.Parent != "" {
			t.Errorf("SetParent().Parent = %q, want empty", got.Parent)
		}
	})

	t.Run("set parent on nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		parentID := "parent-1"
		_, err := mr.SetParent(ctx, "nonexistent", &parentID, nil)
		if err == nil {
			t.Error("SetParent() expected error for nonexistent bean")
		}
	})
}

func TestMutationAddRemoveBlocking(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test beans
	blocker := &issue.Issue{ID: "blocker-1", Title: "Blocker", Status: "todo", Type: "task"}
	target := &issue.Issue{ID: "target-1", Title: "Target", Status: "todo", Type: "task"}
	c.Create(blocker)
	c.Create(target)

	t.Run("add block", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.AddBlocking(ctx, "blocker-1", "target-1", nil)
		if err != nil {
			t.Fatalf("AddBlocking() error = %v", err)
		}
		if len(got.Blocking) != 1 {
			t.Errorf("AddBlocking().Blocking count = %d, want 1", len(got.Blocking))
		}
		if got.Blocking[0] != "target-1" {
			t.Errorf("AddBlocking().Blocking[0] = %q, want %q", got.Blocking[0], "target-1")
		}
	})

	t.Run("remove block", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.RemoveBlocking(ctx, "blocker-1", "target-1", nil)
		if err != nil {
			t.Fatalf("RemoveBlocking() error = %v", err)
		}
		if len(got.Blocking) != 0 {
			t.Errorf("RemoveBlocking().Blocking count = %d, want 0", len(got.Blocking))
		}
	})

	t.Run("add block to nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.AddBlocking(ctx, "nonexistent", "target-1", nil)
		if err == nil {
			t.Error("AddBlocking() expected error for nonexistent bean")
		}
	})
}

func TestMutationDeleteIssue(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("delete existing issue", func(t *testing.T) {
		// Create an issue to delete
		b := &issue.Issue{ID: "delete-me", Title: "Delete Me", Status: "todo", Type: "task"}
		c.Create(b)

		mr := resolver.Mutation()
		got, err := mr.DeleteIssue(ctx, "delete-me")
		if err != nil {
			t.Fatalf("DeleteIssue() error = %v", err)
		}
		if !got {
			t.Error("DeleteIssue() = false, want true")
		}

		// Verify it's gone
		qr := resolver.Query()
		bean, _ := qr.Issue(ctx, "delete-me")
		if bean != nil {
			t.Error("Issue still exists after delete")
		}
	})

	t.Run("delete removes incoming links", func(t *testing.T) {
		// Create target issue
		target := &issue.Issue{ID: "target-bean", Title: "Target", Status: "todo", Type: "task"}
		c.Create(target)

		// Create bean that links to target
		linker := &issue.Issue{
			ID:       "linker-bean",
			Title:    "Linker",
			Status:   "todo",
			Type:     "task",
			Blocking: []string{"target-bean"},
		}
		c.Create(linker)

		// Delete target - should remove the link from linker
		mr := resolver.Mutation()
		_, err := mr.DeleteIssue(ctx, "target-bean")
		if err != nil {
			t.Fatalf("DeleteIssue() error = %v", err)
		}

		// Verify linker no longer has the link
		qr := resolver.Query()
		updated, _ := qr.Issue(ctx, "linker-bean")
		if updated == nil {
			t.Fatal("Linker bean was deleted unexpectedly")
		}
		if len(updated.Blocking) != 0 {
			t.Errorf("Linker still has %d blocks, want 0", len(updated.Blocking))
		}
	})

	t.Run("delete nonexistent bean", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.DeleteIssue(ctx, "nonexistent")
		if err == nil {
			t.Error("DeleteIssue() expected error for nonexistent bean")
		}
	})
}

func TestRelationshipFieldsWithFilter(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create a parent (milestone) with multiple children (tasks) of different statuses
	parent := &issue.Issue{
		ID:     "parent-filter-test",
		Title:  "Parent Milestone",
		Type:   "milestone",
		Status: "in-progress",
	}
	child1 := &issue.Issue{
		ID:     "child-todo",
		Title:  "Todo Task",
		Type:   "task",
		Status: "todo",
		Parent: "parent-filter-test",
	}
	child2 := &issue.Issue{
		ID:     "child-completed",
		Title:  "Completed Task",
		Type:   "task",
		Status: "completed",
		Parent: "parent-filter-test",
	}
	child3 := &issue.Issue{
		ID:       "child-inprogress",
		Title:    "In Progress Task",
		Type:     "task",
		Status:   "in-progress",
		Parent:   "parent-filter-test",
		Priority: "high",
	}

	// Create blocking relationships with different types
	blocker1 := &issue.Issue{
		ID:       "blocker-bug",
		Title:    "Blocking Bug",
		Type:     "bug",
		Status:   "todo",
		Blocking: []string{"child-todo"},
	}
	blocker2 := &issue.Issue{
		ID:       "blocker-task",
		Title:    "Blocking Task",
		Type:     "task",
		Status:   "completed",
		Blocking: []string{"child-todo"},
	}

	for _, b := range []*issue.Issue{parent, child1, child2, child3, blocker1, blocker2} {
		if err := c.Create(b); err != nil {
			t.Fatalf("Failed to create bean %s: %v", b.ID, err)
		}
	}

	br := resolver.Issue()

	t.Run("children with status filter", func(t *testing.T) {
		filter := &model.IssueFilter{
			Status: []string{"todo"},
		}
		got, err := br.Children(ctx, parent, filter)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Children(filter status=todo) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "child-todo" {
			t.Errorf("Children(filter status=todo)[0].ID = %q, want %q", got[0].ID, "child-todo")
		}
	})

	t.Run("children with excludeStatus filter", func(t *testing.T) {
		filter := &model.IssueFilter{
			ExcludeStatus: []string{"completed"},
		}
		got, err := br.Children(ctx, parent, filter)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Children(filter excludeStatus=completed) count = %d, want 2", len(got))
		}
	})

	t.Run("children with priority filter", func(t *testing.T) {
		filter := &model.IssueFilter{
			Priority: []string{"high"},
		}
		got, err := br.Children(ctx, parent, filter)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Children(filter priority=high) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "child-inprogress" {
			t.Errorf("Children(filter priority=high)[0].ID = %q, want %q", got[0].ID, "child-inprogress")
		}
	})

	t.Run("children with nil filter returns all", func(t *testing.T) {
		got, err := br.Children(ctx, parent, nil)
		if err != nil {
			t.Fatalf("Children() error = %v", err)
		}
		if len(got) != 3 {
			t.Errorf("Children(nil filter) count = %d, want 3", len(got))
		}
	})

	t.Run("blockedBy with type filter", func(t *testing.T) {
		filter := &model.IssueFilter{
			Type: []string{"bug"},
		}
		got, err := br.BlockedBy(ctx, child1, filter)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("BlockedBy(filter type=bug) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "blocker-bug" {
			t.Errorf("BlockedBy(filter type=bug)[0].ID = %q, want %q", got[0].ID, "blocker-bug")
		}
	})

	t.Run("blockedBy with excludeStatus filter", func(t *testing.T) {
		filter := &model.IssueFilter{
			ExcludeStatus: []string{"completed"},
		}
		got, err := br.BlockedBy(ctx, child1, filter)
		if err != nil {
			t.Fatalf("BlockedBy() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("BlockedBy(filter excludeStatus=completed) count = %d, want 1", len(got))
		}
		if len(got) > 0 && got[0].ID != "blocker-bug" {
			t.Errorf("BlockedBy(filter excludeStatus=completed)[0].ID = %q, want %q", got[0].ID, "blocker-bug")
		}
	})

	t.Run("blocking with status filter", func(t *testing.T) {
		filter := &model.IssueFilter{
			Status: []string{"todo"},
		}
		got, err := br.Blocking(ctx, blocker1, filter)
		if err != nil {
			t.Fatalf("Blocking() error = %v", err)
		}
		if len(got) != 1 {
			t.Errorf("Blocking(filter status=todo) count = %d, want 1", len(got))
		}
	})

	t.Run("blocking filter excludes all", func(t *testing.T) {
		filter := &model.IssueFilter{
			Status: []string{"completed"},
		}
		got, err := br.Blocking(ctx, blocker1, filter)
		if err != nil {
			t.Fatalf("Blocking() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("Blocking(filter status=completed) count = %d, want 0", len(got))
		}
	})
}


// setupTestResolverWithRequireIfMatch creates a test resolver with require_if_match enabled.
func setupTestResolverWithRequireIfMatch(t *testing.T) (*Resolver, *core.Core) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".issues")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create test .issues dir: %v", err)
	}

	cfg := config.Default()
	cfg.Issues.RequireIfMatch = true
	c := core.New(dataDir, cfg)
	if err := c.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return &Resolver{Core: c}, c
}

func TestETagValidation(t *testing.T) {
	t.Run("update with correct etag succeeds", func(t *testing.T) {
		resolver, c := setupTestResolver(t)
		ctx := context.Background()

		b := &issue.Issue{ID: "etag-test-1", Title: "Test", Status: "todo"}
		c.Create(b)

		// Get current etag
		currentETag := b.ETag()

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateIssueInput{
			Title:   &newTitle,
			IfMatch: &currentETag,
		}
		got, err := mr.UpdateIssue(ctx, "etag-test-1", input)
		if err != nil {
			t.Fatalf("UpdateIssue() with correct etag error = %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateIssue().Title = %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("update with incorrect etag fails", func(t *testing.T) {
		resolver, c := setupTestResolver(t)
		ctx := context.Background()

		b := &issue.Issue{ID: "etag-test-2", Title: "Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		newTitle := "Updated"
		wrongETag := "wrongetagvalue1"
		input := model.UpdateIssueInput{
			Title:   &newTitle,
			IfMatch: &wrongETag,
		}
		_, err := mr.UpdateIssue(ctx, "etag-test-2", input)
		if err == nil {
			t.Error("UpdateIssue() with wrong etag should fail")
		}
		if !strings.Contains(err.Error(), "etag mismatch") {
			t.Errorf("Error should mention etag mismatch, got: %v", err)
		}
	})

	t.Run("update without etag succeeds when not required", func(t *testing.T) {
		resolver, c := setupTestResolver(t)
		ctx := context.Background()

		b := &issue.Issue{ID: "etag-test-3", Title: "Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateIssueInput{
			Title: &newTitle,
		}
		got, err := mr.UpdateIssue(ctx, "etag-test-3", input)
		if err != nil {
			t.Fatalf("UpdateIssue() without etag error = %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateIssue().Title = %q, want %q", got.Title, "Updated")
		}
	})
}

func TestRequireIfMatchConfig(t *testing.T) {
	t.Run("update without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, c := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b := &issue.Issue{ID: "require-etag-1", Title: "Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateIssueInput{
			Title: &newTitle,
		}
		_, err := mr.UpdateIssue(ctx, "require-etag-1", input)
		if err == nil {
			t.Error("UpdateIssue() without etag should fail when require_if_match is true")
		}
		if !strings.Contains(err.Error(), "if-match etag is required") {
			t.Errorf("Error should mention etag is required, got: %v", err)
		}
	})

	t.Run("update with correct etag succeeds when require_if_match is true", func(t *testing.T) {
		resolver, c := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b := &issue.Issue{ID: "require-etag-2", Title: "Test", Status: "todo"}
		c.Create(b)

		currentETag := b.ETag()

		mr := resolver.Mutation()
		newTitle := "Updated"
		input := model.UpdateIssueInput{
			Title:   &newTitle,
			IfMatch: &currentETag,
		}
		got, err := mr.UpdateIssue(ctx, "require-etag-2", input)
		if err != nil {
			t.Fatalf("UpdateIssue() with correct etag error = %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateIssue().Title = %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("setParent without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, c := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		parent := &issue.Issue{ID: "req-parent", Title: "Parent", Status: "todo", Type: "epic"}
		child := &issue.Issue{ID: "req-child", Title: "Child", Status: "todo", Type: "task"}
		c.Create(parent)
		c.Create(child)

		mr := resolver.Mutation()
		parentID := "req-parent"
		_, err := mr.SetParent(ctx, "req-child", &parentID, nil)
		if err == nil {
			t.Error("SetParent() without etag should fail when require_if_match is true")
		}
	})

	t.Run("addBlocking without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, c := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b1 := &issue.Issue{ID: "req-blocker", Title: "Blocker", Status: "todo"}
		b2 := &issue.Issue{ID: "req-target", Title: "Target", Status: "todo"}
		c.Create(b1)
		c.Create(b2)

		mr := resolver.Mutation()
		_, err := mr.AddBlocking(ctx, "req-blocker", "req-target", nil)
		if err == nil {
			t.Error("AddBlocking() without etag should fail when require_if_match is true")
		}
	})

	t.Run("removeBlocking without etag fails when require_if_match is true", func(t *testing.T) {
		resolver, c := setupTestResolverWithRequireIfMatch(t)
		ctx := context.Background()

		b1 := &issue.Issue{ID: "req-blocker2", Title: "Blocker", Status: "todo", Blocking: []string{"req-target2"}}
		b2 := &issue.Issue{ID: "req-target2", Title: "Target", Status: "todo"}
		c.Create(b1)
		c.Create(b2)

		mr := resolver.Mutation()
		_, err := mr.RemoveBlocking(ctx, "req-blocker2", "req-target2", nil)
		if err == nil {
			t.Error("RemoveBlocking() without etag should fail when require_if_match is true")
		}
	})
}

func TestDueDateResolver(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("create with due date", func(t *testing.T) {
		mr := resolver.Mutation()
		due := "2025-06-15"
		input := model.CreateIssueInput{
			Title: "Due Date Bean",
			Due:   &due,
		}
		got, err := mr.CreateIssue(ctx, input)
		if err != nil {
			t.Fatalf("CreateIssue() error = %v", err)
		}
		if got.Due == nil {
			t.Fatal("Due is nil, want non-nil")
		}
		if got.Due.String() != "2025-06-15" {
			t.Errorf("Due = %q, want %q", got.Due.String(), "2025-06-15")
		}
	})

	t.Run("create without due date", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateIssueInput{
			Title: "No Due Bean",
		}
		got, err := mr.CreateIssue(ctx, input)
		if err != nil {
			t.Fatalf("CreateIssue() error = %v", err)
		}
		if got.Due != nil {
			t.Errorf("Due = %v, want nil", got.Due)
		}
	})

	t.Run("create with invalid due date", func(t *testing.T) {
		mr := resolver.Mutation()
		due := "not-a-date"
		input := model.CreateIssueInput{
			Title: "Bad Due",
			Due:   &due,
		}
		_, err := mr.CreateIssue(ctx, input)
		if err == nil {
			t.Error("expected error for invalid due date")
		}
	})

	t.Run("update to set due date", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "due-update-test",
			Title:  "Update Due",
			Status: "todo",
			Type:   "task",
		}
		c.Create(b)

		mr := resolver.Mutation()
		due := "2025-09-01"
		input := model.UpdateIssueInput{
			Due: &due,
		}
		got, err := mr.UpdateIssue(ctx, "due-update-test", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if got.Due == nil {
			t.Fatal("Due is nil after update")
		}
		if got.Due.String() != "2025-09-01" {
			t.Errorf("Due = %q, want %q", got.Due.String(), "2025-09-01")
		}
	})

	t.Run("update to clear due date", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "due-clear-test",
			Title:  "Clear Due",
			Status: "todo",
			Type:   "task",
			Due:    issue.NewDueDate(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
		}
		c.Create(b)

		mr := resolver.Mutation()
		emptyDue := ""
		input := model.UpdateIssueInput{
			Due: &emptyDue,
		}
		got, err := mr.UpdateIssue(ctx, "due-clear-test", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if got.Due != nil {
			t.Errorf("Due = %v, want nil after clearing", got.Due)
		}
	})

	t.Run("due resolver returns formatted string", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "due-resolver-test",
			Title:  "Resolver Due",
			Status: "todo",
			Type:   "task",
			Due:    issue.NewDueDate(time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)),
		}
		c.Create(b)

		br := resolver.Issue()
		dueStr, err := br.Due(ctx, b)
		if err != nil {
			t.Fatalf("Due() error = %v", err)
		}
		if dueStr == nil {
			t.Fatal("Due() returned nil")
		}
		if *dueStr != "2025-03-15" {
			t.Errorf("Due() = %q, want %q", *dueStr, "2025-03-15")
		}
	})

	t.Run("due resolver returns nil for no due date", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "due-nil-resolver-test",
			Title:  "No Due Resolver",
			Status: "todo",
			Type:   "task",
		}
		c.Create(b)

		br := resolver.Issue()
		dueStr, err := br.Due(ctx, b)
		if err != nil {
			t.Fatalf("Due() error = %v", err)
		}
		if dueStr != nil {
			t.Errorf("Due() = %q, want nil", *dueStr)
		}
	})
}

func TestUpdateIssueWithBodyMod(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("bodyMod with single replacement only", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-1",
			Title:  "Test",
			Status: "todo",
			Body:   "## Tasks\n- [ ] Task 1\n- [ ] Task 2",
		}
		c.Create(b)

		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Task 1", New: "- [x] Task 1"},
				},
			},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-1", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		want := "## Tasks\n- [x] Task 1\n- [ ] Task 2"
		if got.Body != want {
			t.Errorf("UpdateIssue().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with append only", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-2",
			Title:  "Test",
			Status: "todo",
			Body:   "Existing content",
		}
		c.Create(b)

		appendText := "## Notes\n\nNew section"
		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Append: &appendText,
			},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-2", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		want := "Existing content\n\n## Notes\n\nNew section"
		if got.Body != want {
			t.Errorf("UpdateIssue().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with replacement and append combined", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-3",
			Title:  "Test",
			Status: "todo",
			Body:   "## Tasks\n- [ ] Deploy",
		}
		c.Create(b)

		appendText := "## Summary\n\nCompleted"
		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Deploy", New: "- [x] Deploy"},
				},
				Append: &appendText,
			},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-3", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		want := "## Tasks\n- [x] Deploy\n\n## Summary\n\nCompleted"
		if got.Body != want {
			t.Errorf("UpdateIssue().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with multiple replacements sequential", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-4",
			Title:  "Test",
			Status: "todo",
			Body:   "- [ ] Task 1\n- [ ] Task 2\n- [ ] Task 3",
		}
		c.Create(b)

		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Task 1", New: "- [x] Task 1"},
					{Old: "- [ ] Task 2", New: "- [x] Task 2"},
					{Old: "- [ ] Task 3", New: "- [x] Task 3"},
				},
			},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-4", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		want := "- [x] Task 1\n- [x] Task 2\n- [x] Task 3"
		if got.Body != want {
			t.Errorf("UpdateIssue().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("bodyMod with metadata update", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-5",
			Title:  "Test",
			Status: "todo",
			Body:   "- [ ] Task",
		}
		c.Create(b)

		status := "completed"
		appendText := "## Done"
		input := model.UpdateIssueInput{
			Status: &status,
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Task", New: "- [x] Task"},
				},
				Append: &appendText,
			},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-5", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if got.Status != "completed" {
			t.Errorf("UpdateIssue().Status = %q, want %q", got.Status, "completed")
		}
		want := "- [x] Task\n\n## Done"
		if got.Body != want {
			t.Errorf("UpdateIssue().Body = %q, want %q", got.Body, want)
		}
	})

	t.Run("error when both body and bodyMod provided", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-6",
			Title:  "Test",
			Status: "todo",
			Body:   "Original",
		}
		c.Create(b)

		bodyText := "New body"
		appendText := "Append"
		input := model.UpdateIssueInput{
			Body: &bodyText,
			BodyMod: &model.BodyModification{
				Append: &appendText,
			},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-6", input)
		if err == nil {
			t.Error("UpdateIssue() expected error when both body and bodyMod provided")
		}
		if !strings.Contains(err.Error(), "cannot specify both body and bodyMod") {
			t.Errorf("Error should mention mutual exclusivity, got: %v", err)
		}
	})

	t.Run("error when replacement text not found", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-7",
			Title:  "Test",
			Status: "todo",
			Body:   "Hello world",
		}
		c.Create(b)

		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "nonexistent", New: "fail"},
				},
			},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-7", input)
		if err == nil {
			t.Error("UpdateIssue() expected error when replacement text not found")
		}
		if !strings.Contains(err.Error(), "text not found") {
			t.Errorf("Error should mention text not found, got: %v", err)
		}
	})

	t.Run("error when replacement text found multiple times", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-8",
			Title:  "Test",
			Status: "todo",
			Body:   "foo foo foo",
		}
		c.Create(b)

		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "foo", New: "bar"},
				},
			},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-8", input)
		if err == nil {
			t.Error("UpdateIssue() expected error when replacement text found multiple times")
		}
		if !strings.Contains(err.Error(), "found 3 times") {
			t.Errorf("Error should mention multiple matches, got: %v", err)
		}
	})

	t.Run("transactional: later replacement fails, nothing saved", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-9",
			Title:  "Test",
			Status: "todo",
			Body:   "Task 1\nTask 2",
		}
		c.Create(b)
		originalBody := b.Body

		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "Task 1", New: "Done 1"},    // This should succeed
					{Old: "nonexistent", New: "fail"}, // This should fail
				},
			},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-9", input)
		if err == nil {
			t.Error("UpdateIssue() expected error")
		}

		// Verify bean wasn't modified
		updated, _ := c.Get("bodymod-test-9")
		if updated.Body != originalBody {
			t.Errorf("Issue body was modified despite error. Got %q, want %q", updated.Body, originalBody)
		}
	})

	t.Run("empty append is no-op", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-10",
			Title:  "Test",
			Status: "todo",
			Body:   "Original content",
		}
		c.Create(b)

		emptyAppend := ""
		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Append: &emptyAppend,
			},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-10", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}
		if got.Body != "Original content" {
			t.Errorf("UpdateIssue().Body = %q, want %q (no-op for empty append)", got.Body, "Original content")
		}
	})

	t.Run("transactional: later replacement fails, nothing saved", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "bodymod-test-9",
			Title:  "Test",
			Status: "todo",
			Body:   "Task 1\nTask 2",
		}
		c.Create(b)
		originalBody := b.Body

		input := model.UpdateIssueInput{
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "Task 1", New: "Done 1"},    // This should succeed
					{Old: "nonexistent", New: "fail"}, // This should fail
				},
			},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "bodymod-test-9", input)
		if err == nil {
			t.Error("UpdateIssue() expected error")
		}

		// Verify bean wasn't modified
		updated, _ := c.Get("bodymod-test-9")
		if updated.Body != originalBody {
			t.Errorf("Issue body was modified despite error. Got %q, want %q", updated.Body, originalBody)
		}
	})
}

func TestExtensionsResolver(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("empty extensions returns empty list", func(t *testing.T) {
		b := &issue.Issue{ID: "ext-empty", Title: "No Extensions", Status: "todo"}
		c.Create(b)

		br := resolver.Issue()
		got, err := br.Extensions(ctx, b)
		if err != nil {
			t.Fatalf("Extensions() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("Extensions() count = %d, want 0", len(got))
		}
	})

	t.Run("returns sorted entries", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "ext-sorted",
			Title:  "With Extensions",
			Status: "todo",
			Extensions: map[string]map[string]any{
				"jira":    {"issue_key": "PROJ-123"},
				"clickup": {"task_id": "abc"},
			},
		}
		c.Create(b)

		br := resolver.Issue()
		got, err := br.Extensions(ctx, b)
		if err != nil {
			t.Fatalf("Extensions() error = %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("Extensions() count = %d, want 2", len(got))
		}
		// Should be sorted alphabetically
		if got[0].Name != "clickup" {
			t.Errorf("Extensions()[0].Name = %q, want 'clickup'", got[0].Name)
		}
		if got[1].Name != "jira" {
			t.Errorf("Extensions()[1].Name = %q, want 'jira'", got[1].Name)
		}
		if got[0].Data["task_id"] != "abc" {
			t.Errorf("Extensions()[0].Data = %v, want task_id=abc", got[0].Data)
		}
	})
}

func TestMutationSetExtensionData(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("set extension data", func(t *testing.T) {
		b := &issue.Issue{ID: "set-ext-1", Title: "Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		data := map[string]any{"task_id": "abc123", "synced_at": "2026-01-01T00:00:00Z"}
		got, err := mr.SetExtensionData(ctx, "set-ext-1", "clickup", data, nil)
		if err != nil {
			t.Fatalf("SetExtensionData() error = %v", err)
		}
		if !got.HasExtension("clickup") {
			t.Error("SetExtensionData() didn't set extension data")
		}
		if got.Extensions["clickup"]["task_id"] != "abc123" {
			t.Errorf("Extensions[clickup][task_id] = %v, want abc123", got.Extensions["clickup"]["task_id"])
		}
	})

	t.Run("replace existing extension data", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "set-ext-2",
			Title:  "Test",
			Status: "todo",
			Extensions: map[string]map[string]any{
				"clickup": {"task_id": "old"},
			},
		}
		c.Create(b)

		mr := resolver.Mutation()
		data := map[string]any{"task_id": "new", "extra": "field"}
		got, err := mr.SetExtensionData(ctx, "set-ext-2", "clickup", data, nil)
		if err != nil {
			t.Fatalf("SetExtensionData() error = %v", err)
		}
		if got.Extensions["clickup"]["task_id"] != "new" {
			t.Errorf("Extensions[clickup][task_id] = %v, want new", got.Extensions["clickup"]["task_id"])
		}
		if got.Extensions["clickup"]["extra"] != "field" {
			t.Errorf("Extensions[clickup][extra] = %v, want field", got.Extensions["clickup"]["extra"])
		}
	})

	t.Run("empty name fails", func(t *testing.T) {
		b := &issue.Issue{ID: "set-ext-3", Title: "Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		_, err := mr.SetExtensionData(ctx, "set-ext-3", "", map[string]any{"key": "val"}, nil)
		if err == nil {
			t.Error("SetExtensionData() should fail with empty name")
		}
	})

	t.Run("nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.SetExtensionData(ctx, "nonexistent", "clickup", map[string]any{"key": "val"}, nil)
		if err == nil {
			t.Error("SetExtensionData() should fail for nonexistent bean")
		}
	})

	t.Run("persists to disk", func(t *testing.T) {
		b := &issue.Issue{ID: "set-ext-disk", Title: "Disk Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		data := map[string]any{"task_id": "persist-test"}
		_, err := mr.SetExtensionData(ctx, "set-ext-disk", "clickup", data, nil)
		if err != nil {
			t.Fatalf("SetExtensionData() error = %v", err)
		}

		// Re-read from disk
		reloaded, err := c.Get("set-ext-disk")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !reloaded.HasExtension("clickup") {
			t.Error("Extension data not persisted to disk")
		}
	})
}

func TestMutationRemoveExtensionData(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("remove existing extension data", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "rm-ext-1",
			Title:  "Test",
			Status: "todo",
			Extensions: map[string]map[string]any{
				"clickup": {"task_id": "abc"},
				"jira":    {"issue_key": "PROJ-123"},
			},
		}
		c.Create(b)

		mr := resolver.Mutation()
		got, err := mr.RemoveExtensionData(ctx, "rm-ext-1", "clickup", nil)
		if err != nil {
			t.Fatalf("RemoveExtensionData() error = %v", err)
		}
		if got.HasExtension("clickup") {
			t.Error("RemoveExtensionData() didn't remove clickup data")
		}
		if !got.HasExtension("jira") {
			t.Error("RemoveExtensionData() removed wrong extension")
		}
	})

	t.Run("remove nonexistent extension is no-op", func(t *testing.T) {
		b := &issue.Issue{ID: "rm-ext-2", Title: "Test", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		got, err := mr.RemoveExtensionData(ctx, "rm-ext-2", "clickup", nil)
		if err != nil {
			t.Fatalf("RemoveExtensionData() error = %v", err)
		}
		if got.Extensions != nil {
			t.Errorf("Extensions should be nil, got %v", got.Extensions)
		}
	})

	t.Run("nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.RemoveExtensionData(ctx, "nonexistent", "clickup", nil)
		if err == nil {
			t.Error("RemoveExtensionData() should fail for nonexistent bean")
		}
	})
}

func TestUpdateIssueWithRelationships(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("atomic update with parent and blocking", func(t *testing.T) {
		epic := &issue.Issue{ID: "epic-1", Title: "Epic", Type: "epic", Status: "todo"}
		task := &issue.Issue{ID: "task-1", Title: "Task", Type: "task", Status: "todo"}
		blocker := &issue.Issue{ID: "blocker-1", Title: "Blocker", Type: "task", Status: "todo"}
		c.Create(epic)
		c.Create(task)
		c.Create(blocker)

		input := model.UpdateIssueInput{
			Status:      new("in-progress"),
			Parent:      new("epic-1"),
			AddBlocking: []string{"blocker-1"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-1", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if got.Status != "in-progress" {
			t.Errorf("UpdateIssue().Status = %q, want %q", got.Status, "in-progress")
		}
		if got.Parent != "epic-1" {
			t.Errorf("UpdateIssue().Parent = %q, want %q", got.Parent, "epic-1")
		}
		if len(got.Blocking) != 1 || got.Blocking[0] != "blocker-1" {
			t.Errorf("UpdateIssue().Blocking = %v, want [blocker-1]", got.Blocking)
		}
	})

	t.Run("atomic update with bodyMod and relationships", func(t *testing.T) {
		epic := &issue.Issue{ID: "epic-2", Title: "Epic", Type: "epic", Status: "todo"}
		task := &issue.Issue{ID: "task-2", Title: "Task", Type: "task", Status: "todo", Body: "- [ ] Step 1"}
		blocker := &issue.Issue{ID: "blocker-2", Title: "Blocker", Type: "task", Status: "todo"}
		c.Create(epic)
		c.Create(task)
		c.Create(blocker)

		input := model.UpdateIssueInput{
			Status: new("completed"),
			Parent: new("epic-2"),
			BodyMod: &model.BodyModification{
				Replace: []*model.ReplaceOperation{
					{Old: "- [ ] Step 1", New: "- [x] Step 1"},
				},
				Append: new("## Done"),
			},
			AddBlocking: []string{"blocker-2"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-2", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if got.Status != "completed" {
			t.Errorf("Status = %q, want completed", got.Status)
		}
		if got.Parent != "epic-2" {
			t.Errorf("Parent = %q, want epic-2", got.Parent)
		}
		if !strings.Contains(got.Body, "- [x] Step 1") {
			t.Errorf("Body missing completed task")
		}
		if !strings.Contains(got.Body, "## Done") {
			t.Errorf("Body missing appended content")
		}
		if len(got.Blocking) != 1 {
			t.Errorf("Blocking count = %d, want 1", len(got.Blocking))
		}
	})

	t.Run("parent validation fails for invalid type hierarchy", func(t *testing.T) {
		task1 := &issue.Issue{ID: "task-invalid-1", Title: "Task 1", Type: "task", Status: "todo"}
		task2 := &issue.Issue{ID: "task-invalid-2", Title: "Task 2", Type: "task", Status: "todo"}
		c.Create(task1)
		c.Create(task2)

		input := model.UpdateIssueInput{
			Parent: new("task-invalid-2"),
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-invalid-1", input)
		if err == nil {
			t.Error("UpdateIssue() should fail for invalid parent type")
		}
	})

	t.Run("blocking self-reference validation", func(t *testing.T) {
		task := &issue.Issue{ID: "task-self", Title: "Task", Type: "task", Status: "todo"}
		c.Create(task)

		input := model.UpdateIssueInput{
			AddBlocking: []string{"task-self"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-self", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when bean blocks itself")
		}
		if !strings.Contains(err.Error(), "block itself") {
			t.Errorf("Error should mention self-blocking, got: %v", err)
		}
	})

	t.Run("blocking cycle detection", func(t *testing.T) {
		task1 := &issue.Issue{ID: "task-block-1", Title: "Task 1", Type: "task", Status: "todo"}
		task2 := &issue.Issue{ID: "task-block-2", Title: "Task 2", Type: "task", Status: "todo", Blocking: []string{"task-block-1"}}
		c.Create(task1)
		c.Create(task2)

		// Try to make task-1 block task-2 (would create cycle)
		input := model.UpdateIssueInput{
			AddBlocking: []string{"task-block-2"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-block-1", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when creating blocking cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocking target not found", func(t *testing.T) {
		task := &issue.Issue{ID: "task-notfound", Title: "Task", Type: "task", Status: "todo"}
		c.Create(task)

		input := model.UpdateIssueInput{
			AddBlocking: []string{"nonexistent"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-notfound", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when blocking target doesn't exist")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("remove blocking relationships", func(t *testing.T) {
		task := &issue.Issue{ID: "task-remove-1", Title: "Task", Type: "task", Status: "todo", Blocking: []string{"other-1", "other-2"}}
		other1 := &issue.Issue{ID: "other-1", Title: "Other 1", Type: "task", Status: "todo"}
		other2 := &issue.Issue{ID: "other-2", Title: "Other 2", Type: "task", Status: "todo"}
		c.Create(task)
		c.Create(other1)
		c.Create(other2)

		input := model.UpdateIssueInput{
			RemoveBlocking: []string{"other-1"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-remove-1", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.Blocking) != 1 || got.Blocking[0] != "other-2" {
			t.Errorf("Blocking = %v, want [other-2]", got.Blocking)
		}
	})

	t.Run("blockedBy self-reference validation", func(t *testing.T) {
		task := &issue.Issue{ID: "task-blockedby-self", Title: "Task", Type: "task", Status: "todo"}
		c.Create(task)

		input := model.UpdateIssueInput{
			AddBlockedBy: []string{"task-blockedby-self"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-blockedby-self", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when bean is blocked by itself")
		}
		if !strings.Contains(err.Error(), "blocked by itself") {
			t.Errorf("Error should mention self-blocking, got: %v", err)
		}
	})

	t.Run("blockedBy target not found", func(t *testing.T) {
		task := &issue.Issue{ID: "task-blockedby-notfound", Title: "Task", Type: "task", Status: "todo"}
		c.Create(task)

		input := model.UpdateIssueInput{
			AddBlockedBy: []string{"nonexistent"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-blockedby-notfound", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when blocker doesn't exist")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("combined add and remove operations", func(t *testing.T) {
		task := &issue.Issue{ID: "task-combined", Title: "Task", Type: "task", Status: "todo", Blocking: []string{"old-1"}}
		old1 := &issue.Issue{ID: "old-1", Title: "Old", Type: "task", Status: "todo"}
		new1 := &issue.Issue{ID: "new-1", Title: "New", Type: "task", Status: "todo"}
		c.Create(task)
		c.Create(old1)
		c.Create(new1)

		input := model.UpdateIssueInput{
			RemoveBlocking: []string{"old-1"},
			AddBlocking:    []string{"new-1"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-combined", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.Blocking) != 1 || got.Blocking[0] != "new-1" {
			t.Errorf("Blocking = %v, want [new-1]", got.Blocking)
		}
	})

	t.Run("blockedBy cycle detection", func(t *testing.T) {
		task1 := &issue.Issue{ID: "task-blockedby-cycle-1", Title: "Task 1", Type: "task", Status: "todo"}
		task2 := &issue.Issue{ID: "task-blockedby-cycle-2", Title: "Task 2", Type: "task", Status: "todo", BlockedBy: []string{"task-blockedby-cycle-1"}}
		c.Create(task1)
		c.Create(task2)

		// Try to make task-1 blocked by task-2 (would create cycle)
		input := model.UpdateIssueInput{
			AddBlockedBy: []string{"task-blockedby-cycle-2"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-blockedby-cycle-1", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when creating blockedBy cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("remove parent", func(t *testing.T) {
		epic := &issue.Issue{ID: "epic-parent-remove", Title: "Epic", Type: "epic", Status: "todo"}
		task := &issue.Issue{ID: "task-parent-remove", Title: "Task", Type: "task", Status: "todo", Parent: "epic-parent-remove"}
		c.Create(epic)
		c.Create(task)

		// Remove parent by setting to empty string
		emptyParent := ""
		input := model.UpdateIssueInput{
			Parent: &emptyParent,
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-parent-remove", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if got.Parent != "" {
			t.Errorf("Parent = %q, want empty string", got.Parent)
		}
	})

	t.Run("remove blockedBy relationships", func(t *testing.T) {
		task := &issue.Issue{ID: "task-remove-blockedby", Title: "Task", Type: "task", Status: "todo", BlockedBy: []string{"blocker-1", "blocker-2"}}
		blocker1 := &issue.Issue{ID: "blocker-1", Title: "Blocker 1", Type: "task", Status: "todo"}
		blocker2 := &issue.Issue{ID: "blocker-2", Title: "Blocker 2", Type: "task", Status: "todo"}
		c.Create(task)
		c.Create(blocker1)
		c.Create(blocker2)

		input := model.UpdateIssueInput{
			RemoveBlockedBy: []string{"blocker-1"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-remove-blockedby", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.BlockedBy) != 1 || got.BlockedBy[0] != "blocker-2" {
			t.Errorf("BlockedBy = %v, want [blocker-2]", got.BlockedBy)
		}
	})

	t.Run("multiple blocking additions", func(t *testing.T) {
		task := &issue.Issue{ID: "task-multi-blocking", Title: "Task", Type: "task", Status: "todo"}
		target1 := &issue.Issue{ID: "target-1", Title: "Target 1", Type: "task", Status: "todo"}
		target2 := &issue.Issue{ID: "target-2", Title: "Target 2", Type: "task", Status: "todo"}
		c.Create(task)
		c.Create(target1)
		c.Create(target2)

		input := model.UpdateIssueInput{
			AddBlocking: []string{"target-1", "target-2"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-multi-blocking", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.Blocking) != 2 {
			t.Errorf("Blocking count = %d, want 2", len(got.Blocking))
		}
	})

	t.Run("all relationship types combined", func(t *testing.T) {
		epic := &issue.Issue{ID: "epic-all", Title: "Epic", Type: "epic", Status: "todo"}
		task := &issue.Issue{ID: "task-all", Title: "Task", Type: "task", Status: "todo", Blocking: []string{"old-blocking"}}
		blocker := &issue.Issue{ID: "new-blocker", Title: "Blocker", Type: "task", Status: "todo"}
		blocked := &issue.Issue{ID: "new-blocked", Title: "Blocked", Type: "task", Status: "todo"}
		oldBlocking := &issue.Issue{ID: "old-blocking", Title: "Old Blocking", Type: "task", Status: "todo"}
		c.Create(epic)
		c.Create(task)
		c.Create(blocker)
		c.Create(blocked)
		c.Create(oldBlocking)

		input := model.UpdateIssueInput{
			Status:         new("in-progress"),
			Parent:         new("epic-all"),
			AddBlocking:    []string{"new-blocked"},
			RemoveBlocking: []string{"old-blocking"},
			AddBlockedBy:   []string{"new-blocker"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-all", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if got.Status != "in-progress" {
			t.Errorf("Status = %q, want in-progress", got.Status)
		}
		if got.Parent != "epic-all" {
			t.Errorf("Parent = %q, want epic-all", got.Parent)
		}
		if len(got.Blocking) != 1 || got.Blocking[0] != "new-blocked" {
			t.Errorf("Blocking = %v, want [new-blocked]", got.Blocking)
		}
		if len(got.BlockedBy) != 1 || got.BlockedBy[0] != "new-blocker" {
			t.Errorf("BlockedBy = %v, want [new-blocker]", got.BlockedBy)
		}
	})

	t.Run("add tags", func(t *testing.T) {
		task := &issue.Issue{ID: "task-tags-1", Title: "Task", Type: "task", Status: "todo", Tags: []string{"existing"}}
		c.Create(task)

		input := model.UpdateIssueInput{
			AddTags: []string{"new1", "new2"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-tags-1", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.Tags) != 3 {
			t.Errorf("Tags count = %d, want 3", len(got.Tags))
		}
		tagSet := make(map[string]bool)
		for _, tag := range got.Tags {
			tagSet[tag] = true
		}
		if !tagSet["existing"] || !tagSet["new1"] || !tagSet["new2"] {
			t.Errorf("Tags = %v, want [existing new1 new2]", got.Tags)
		}
	})

	t.Run("remove tags", func(t *testing.T) {
		task := &issue.Issue{ID: "task-tags-2", Title: "Task", Type: "task", Status: "todo", Tags: []string{"tag1", "tag2", "tag3"}}
		c.Create(task)

		input := model.UpdateIssueInput{
			RemoveTags: []string{"tag2"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-tags-2", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.Tags) != 2 {
			t.Errorf("Tags count = %d, want 2", len(got.Tags))
		}
		for _, tag := range got.Tags {
			if tag == "tag2" {
				t.Error("Tag 'tag2' should have been removed")
			}
		}
	})

	t.Run("add and remove tags in one operation", func(t *testing.T) {
		task := &issue.Issue{ID: "task-tags-3", Title: "Task", Type: "task", Status: "todo", Tags: []string{"old1", "old2", "keep"}}
		c.Create(task)

		input := model.UpdateIssueInput{
			AddTags:    []string{"new1", "new2"},
			RemoveTags: []string{"old1", "old2"},
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "task-tags-3", input)
		if err != nil {
			t.Fatalf("UpdateIssue() error = %v", err)
		}

		if len(got.Tags) != 3 {
			t.Errorf("Tags count = %d, want 3", len(got.Tags))
		}
		tagSet := make(map[string]bool)
		for _, tag := range got.Tags {
			tagSet[tag] = true
		}
		if !tagSet["keep"] || !tagSet["new1"] || !tagSet["new2"] {
			t.Errorf("Tags = %v, want [keep new1 new2]", got.Tags)
		}
		if tagSet["old1"] || tagSet["old2"] {
			t.Errorf("Tags = %v, should not contain old1 or old2", got.Tags)
		}
	})

	t.Run("tags and addTags are mutually exclusive", func(t *testing.T) {
		task := &issue.Issue{ID: "task-tags-4", Title: "Task", Type: "task", Status: "todo"}
		c.Create(task)

		input := model.UpdateIssueInput{
			Tags:    []string{"tag1"},
			AddTags: []string{"tag2"},
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "task-tags-4", input)
		if err == nil {
			t.Error("UpdateIssue() should fail when both tags and addTags are specified")
		}
		if !strings.Contains(err.Error(), "cannot specify both") {
			t.Errorf("Error should mention conflict, got: %v", err)
		}
	})
}

func TestQueryBeansWithExtensionFilters(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)
	later := now.Add(1 * time.Hour)

	// Bean with clickup extension data, synced before updatedAt (stale)
	b1 := &issue.Issue{
		ID:     "ext-filter-1",
		Title:  "With ClickUp (stale)",
		Status: "todo",
		Extensions: map[string]map[string]any{
			"clickup": {"task_id": "abc", "synced_at": earlier.Format(time.RFC3339)},
		},
	}
	// Bean with clickup extension data, synced after updatedAt (fresh)
	b2 := &issue.Issue{
		ID:     "ext-filter-2",
		Title:  "With ClickUp (fresh)",
		Status: "todo",
		Extensions: map[string]map[string]any{
			"clickup": {"task_id": "def", "synced_at": later.Format(time.RFC3339)},
		},
	}
	// Bean with jira extension data
	b3 := &issue.Issue{
		ID:     "ext-filter-3",
		Title:  "With Jira",
		Status: "todo",
		Extensions: map[string]map[string]any{
			"jira": {"issue_key": "PROJ-123"},
		},
	}
	// Bean with no extension data
	b4 := &issue.Issue{
		ID:     "ext-filter-4",
		Title:  "No Extensions",
		Status: "todo",
	}

	for _, b := range []*issue.Issue{b1, b2, b3, b4} {
		if err := c.Create(b); err != nil {
			t.Fatalf("Create error: %v", err)
		}
	}

	qr := resolver.Query()

	t.Run("hasExtension filter", func(t *testing.T) {
		name := "clickup"
		filter := &model.IssueFilter{HasExtension: &name}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans(hasExtension=clickup) count = %d, want 2", len(got))
		}
	})

	t.Run("noExtension filter", func(t *testing.T) {
		name := "clickup"
		filter := &model.IssueFilter{NoExtension: &name}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 2 {
			t.Errorf("Beans(noExtension=clickup) count = %d, want 2", len(got))
		}
		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}
		if !ids["ext-filter-3"] || !ids["ext-filter-4"] {
			t.Errorf("Expected ext-filter-3 and ext-filter-4, got %v", ids)
		}
	})

	t.Run("extensionStale filter", func(t *testing.T) {
		name := "clickup"
		filter := &model.IssueFilter{ExtensionStale: &name}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		// b1 is stale (updatedAt > synced_at), b3 is stale (no clickup data), b4 is stale (no extension data)
		// b2 is fresh (synced_at > updatedAt)
		ids := make(map[string]bool)
		for _, b := range got {
			ids[b.ID] = true
		}
		if !ids["ext-filter-1"] {
			t.Error("Expected ext-filter-1 (stale sync) in results")
		}
		if ids["ext-filter-2"] {
			t.Error("Did not expect ext-filter-2 (fresh sync) in results")
		}
		if !ids["ext-filter-3"] {
			t.Error("Expected ext-filter-3 (no clickup data, treated as stale) in results")
		}
	})

	t.Run("changedSince filter", func(t *testing.T) {
		// all issues were created at ~now, so use a future threshold to exclude all
		future := now.Add(1 * time.Hour)
		filter := &model.IssueFilter{ChangedSince: &future}
		got, err := qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("Beans(changedSince=future) count = %d, want 0", len(got))
		}

		// Use a past threshold to include all
		past := now.Add(-1 * time.Hour)
		filter = &model.IssueFilter{ChangedSince: &past}
		got, err = qr.Issues(ctx, filter)
		if err != nil {
			t.Fatalf("Beans() error = %v", err)
		}
		if len(got) != 4 {
			t.Errorf("Beans(changedSince=past) count = %d, want 4", len(got))
		}
	})
}

// Helper function for tests
//
//go:fix inline
func stringPtr(s string) *string {
	return new(s)
}

func TestBlockedByCycleDetection(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("blocked_by self-reference fails", func(t *testing.T) {
		b := &issue.Issue{ID: "self-ref", Title: "Self Reference", Status: "todo"}
		c.Create(b)

		mr := resolver.Mutation()
		_, err := mr.AddBlockedBy(ctx, "self-ref", "self-ref", nil)
		if err == nil {
			t.Error("AddBlockedBy() should fail for self-reference")
		}
		if !strings.Contains(err.Error(), "blocked by itself") {
			t.Errorf("Error should mention self-reference, got: %v", err)
		}
	})

	t.Run("blocked_by cycle via blocked_by only is detected", func(t *testing.T) {
		// This tests the scenario where cycles are created using only blocked_by
		a := &issue.Issue{ID: "cycle-a", Title: "Issue A", Status: "todo"}
		b := &issue.Issue{ID: "cycle-b", Title: "Issue B", Status: "todo"}
		c.Create(a)
		c.Create(b)

		mr := resolver.Mutation()

		// A is blocked by B (B → A)
		_, err := mr.AddBlockedBy(ctx, "cycle-a", "cycle-b", nil)
		if err != nil {
			t.Fatalf("AddBlockedBy(A, B) error = %v", err)
		}

		// B is blocked by A (A → B) - should create cycle A → B → A
		_, err = mr.AddBlockedBy(ctx, "cycle-b", "cycle-a", nil)
		if err == nil {
			t.Error("AddBlockedBy(B, A) should fail - would create cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocked_by cycle via blocking is detected", func(t *testing.T) {
		// A blocks B, then B is blocked_by A creates a conflict
		a := &issue.Issue{ID: "cross-a", Title: "Issue A", Status: "todo"}
		b := &issue.Issue{ID: "cross-b", Title: "Issue B", Status: "todo"}
		c.Create(a)
		c.Create(b)

		mr := resolver.Mutation()

		// A blocks B (A → B)
		_, err := mr.AddBlocking(ctx, "cross-a", "cross-b", nil)
		if err != nil {
			t.Fatalf("AddBlocking(A, B) error = %v", err)
		}

		// A is blocked by B (B → A) - should create cycle
		_, err = mr.AddBlockedBy(ctx, "cross-a", "cross-b", nil)
		if err == nil {
			t.Error("AddBlockedBy(A, B) should fail - would create cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocking cycle via blocked_by is detected", func(t *testing.T) {
		// A is blocked_by B, then A blocking B creates a conflict
		a := &issue.Issue{ID: "cross2-a", Title: "Issue A", Status: "todo"}
		b := &issue.Issue{ID: "cross2-b", Title: "Issue B", Status: "todo"}
		c.Create(a)
		c.Create(b)

		mr := resolver.Mutation()

		// A is blocked by B (B → A)
		_, err := mr.AddBlockedBy(ctx, "cross2-a", "cross2-b", nil)
		if err != nil {
			t.Fatalf("AddBlockedBy(A, B) error = %v", err)
		}

		// A blocks B (A → B) - should create cycle
		_, err = mr.AddBlocking(ctx, "cross2-a", "cross2-b", nil)
		if err == nil {
			t.Error("AddBlocking(A, B) should fail - would create cycle")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("blocker issue not found fails", func(t *testing.T) {
		a := &issue.Issue{ID: "exists-a", Title: "Issue A", Status: "todo"}
		c.Create(a)

		mr := resolver.Mutation()
		_, err := mr.AddBlockedBy(ctx, "exists-a", "nonexistent", nil)
		if err == nil {
			t.Error("AddBlockedBy() should fail when blocker doesn't exist")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})
}

func TestCreateIssueBlockedByValidation(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("create with blocked_by referencing nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateIssueInput{
			Title:     "New Bean",
			BlockedBy: []string{"nonexistent"},
		}
		_, err := mr.CreateIssue(ctx, input)
		if err == nil {
			t.Error("CreateIssue() should fail when blocked_by references nonexistent bean")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("create with blocking referencing nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		input := model.CreateIssueInput{
			Title:    "New Bean",
			Blocking: []string{"nonexistent"},
		}
		_, err := mr.CreateIssue(ctx, input)
		if err == nil {
			t.Error("CreateIssue() should fail when blocking references nonexistent bean")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("Error should mention not found, got: %v", err)
		}
	})

	t.Run("create with same bean in both blocking and blocked_by fails", func(t *testing.T) {
		target := &issue.Issue{ID: "target-bean", Title: "Target", Status: "todo"}
		c.Create(target)

		mr := resolver.Mutation()
		input := model.CreateIssueInput{
			Title:     "Cyclic Bean",
			Blocking:  []string{"target-bean"},
			BlockedBy: []string{"target-bean"},
		}
		_, err := mr.CreateIssue(ctx, input)
		if err == nil {
			t.Error("CreateIssue() should fail when same bean is in both blocking and blocked_by")
		}
		if !strings.Contains(err.Error(), "cycle") {
			t.Errorf("Error should mention cycle, got: %v", err)
		}
	})

	t.Run("create with valid blocked_by succeeds", func(t *testing.T) {
		blocker := &issue.Issue{ID: "valid-blocker", Title: "Blocker", Status: "todo"}
		c.Create(blocker)

		mr := resolver.Mutation()
		input := model.CreateIssueInput{
			Title:     "Blocked Bean",
			BlockedBy: []string{"valid-blocker"},
		}
		got, err := mr.CreateIssue(ctx, input)
		if err != nil {
			t.Fatalf("CreateIssue() error = %v", err)
		}
		if len(got.BlockedBy) != 1 {
			t.Errorf("CreateIssue().BlockedBy count = %d, want 1", len(got.BlockedBy))
		}
		if got.BlockedBy[0] != "valid-blocker" {
			t.Errorf("CreateIssue().BlockedBy[0] = %q, want %q", got.BlockedBy[0], "valid-blocker")
		}
	})
}

func TestMutationAddRemoveBlockedBy(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create test beans
	blocked := &issue.Issue{ID: "blocked-1", Title: "Blocked", Status: "todo"}
	blocker := &issue.Issue{ID: "blocker-1", Title: "Blocker", Status: "todo"}
	c.Create(blocked)
	c.Create(blocker)

	t.Run("add blocked_by", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.AddBlockedBy(ctx, "blocked-1", "blocker-1", nil)
		if err != nil {
			t.Fatalf("AddBlockedBy() error = %v", err)
		}
		if len(got.BlockedBy) != 1 {
			t.Errorf("AddBlockedBy().BlockedBy count = %d, want 1", len(got.BlockedBy))
		}
		if got.BlockedBy[0] != "blocker-1" {
			t.Errorf("AddBlockedBy().BlockedBy[0] = %q, want %q", got.BlockedBy[0], "blocker-1")
		}
	})

	t.Run("remove blocked_by", func(t *testing.T) {
		mr := resolver.Mutation()
		got, err := mr.RemoveBlockedBy(ctx, "blocked-1", "blocker-1", nil)
		if err != nil {
			t.Fatalf("RemoveBlockedBy() error = %v", err)
		}
		if len(got.BlockedBy) != 0 {
			t.Errorf("RemoveBlockedBy().BlockedBy count = %d, want 0", len(got.BlockedBy))
		}
	})

	t.Run("add blocked_by to nonexistent bean fails", func(t *testing.T) {
		mr := resolver.Mutation()
		_, err := mr.AddBlockedBy(ctx, "nonexistent", "blocker-1", nil)
		if err == nil {
			t.Error("AddBlockedBy() expected error for nonexistent bean")
		}
	})
}

func TestUpdateIssueWithETag(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	t.Run("update with correct etag succeeds", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-update-1",
			Title:  "Test",
			Status: "todo",
		}
		c.Create(b)

		currentETag := b.ETag()
		newTitle := "Updated"
		input := model.UpdateIssueInput{
			Title:   &newTitle,
			IfMatch: &currentETag,
		}

		got, err := resolver.Mutation().UpdateIssue(ctx, "etag-update-1", input)
		if err != nil {
			t.Fatalf("UpdateIssue() with correct etag failed: %v", err)
		}
		if got.Title != "Updated" {
			t.Errorf("UpdateIssue().Title = %q, want %q", got.Title, "Updated")
		}
	})

	t.Run("update with wrong etag fails", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-update-2",
			Title:  "Test",
			Status: "todo",
		}
		c.Create(b)

		wrongETag := "wrongetag123"
		newTitle := "Should Fail"
		input := model.UpdateIssueInput{
			Title:   &newTitle,
			IfMatch: &wrongETag,
		}

		_, err := resolver.Mutation().UpdateIssue(ctx, "etag-update-2", input)
		if err == nil {
			t.Error("UpdateIssue() with wrong etag should fail")
		}

		if _, ok := errors.AsType[*core.ETagMismatchError](err); !ok {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestSetParentWithETag(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create parent
	parent := &issue.Issue{
		ID:     "parent-etag",
		Title:  "Parent",
		Status: "todo",
		Type:   "epic",
	}
	c.Create(parent)

	t.Run("setParent with correct etag succeeds", func(t *testing.T) {
		child := &issue.Issue{
			ID:     "child-etag-1",
			Title:  "Child",
			Status: "todo",
			Type:   "task",
		}
		c.Create(child)

		currentETag := child.ETag()
		parentID := "parent-etag"

		got, err := resolver.Mutation().SetParent(ctx, "child-etag-1", &parentID, &currentETag)
		if err != nil {
			t.Fatalf("SetParent() with correct etag failed: %v", err)
		}
		if got.Parent != "parent-etag" {
			t.Errorf("SetParent().Parent = %q, want %q", got.Parent, "parent-etag")
		}
	})

	t.Run("setParent with wrong etag fails", func(t *testing.T) {
		child := &issue.Issue{
			ID:     "child-etag-2",
			Title:  "Child",
			Status: "todo",
			Type:   "task",
		}
		c.Create(child)

		wrongETag := "wrongetag123"
		parentID := "parent-etag"

		_, err := resolver.Mutation().SetParent(ctx, "child-etag-2", &parentID, &wrongETag)
		if err == nil {
			t.Error("SetParent() with wrong etag should fail")
		}

		if _, ok := errors.AsType[*core.ETagMismatchError](err); !ok {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestAddBlockingWithETag(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create target issue
	target := &issue.Issue{
		ID:     "target-etag",
		Title:  "Target",
		Status: "todo",
		Type:   "task",
	}
	c.Create(target)

	t.Run("addBlocking with correct etag succeeds", func(t *testing.T) {
		blocker := &issue.Issue{
			ID:     "blocker-etag-1",
			Title:  "Blocker",
			Status: "todo",
			Type:   "task",
		}
		c.Create(blocker)

		currentETag := blocker.ETag()

		got, err := resolver.Mutation().AddBlocking(ctx, "blocker-etag-1", "target-etag", &currentETag)
		if err != nil {
			t.Fatalf("AddBlocking() with correct etag failed: %v", err)
		}
		if len(got.Blocking) != 1 || got.Blocking[0] != "target-etag" {
			t.Errorf("AddBlocking().Blocking = %v, want [target-etag]", got.Blocking)
		}
	})

	t.Run("addBlocking with wrong etag fails", func(t *testing.T) {
		blocker := &issue.Issue{
			ID:     "blocker-etag-2",
			Title:  "Blocker",
			Status: "todo",
			Type:   "task",
		}
		c.Create(blocker)

		wrongETag := "wrongetag123"

		_, err := resolver.Mutation().AddBlocking(ctx, "blocker-etag-2", "target-etag", &wrongETag)
		if err == nil {
			t.Error("AddBlocking() with wrong etag should fail")
		}

		if _, ok := errors.AsType[*core.ETagMismatchError](err); !ok {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}

func TestRemoveBlockingWithETag(t *testing.T) {
	resolver, c := setupTestResolver(t)
	ctx := context.Background()

	// Create target issue
	target := &issue.Issue{
		ID:     "target-rm-etag",
		Title:  "Target",
		Status: "todo",
		Type:   "task",
	}
	c.Create(target)

	t.Run("removeBlocking with correct etag succeeds", func(t *testing.T) {
		blocker := &issue.Issue{
			ID:       "blocker-rm-etag-1",
			Title:    "Blocker",
			Status:   "todo",
			Type:     "task",
			Blocking: []string{"target-rm-etag"},
		}
		c.Create(blocker)

		currentETag := blocker.ETag()

		got, err := resolver.Mutation().RemoveBlocking(ctx, "blocker-rm-etag-1", "target-rm-etag", &currentETag)
		if err != nil {
			t.Fatalf("RemoveBlocking() with correct etag failed: %v", err)
		}
		if len(got.Blocking) != 0 {
			t.Errorf("RemoveBlocking().Blocking = %v, want []", got.Blocking)
		}
	})

	t.Run("removeBlocking with wrong etag fails", func(t *testing.T) {
		blocker := &issue.Issue{
			ID:       "blocker-rm-etag-2",
			Title:    "Blocker",
			Status:   "todo",
			Type:     "task",
			Blocking: []string{"target-rm-etag"},
		}
		c.Create(blocker)

		wrongETag := "wrongetag123"

		_, err := resolver.Mutation().RemoveBlocking(ctx, "blocker-rm-etag-2", "target-rm-etag", &wrongETag)
		if err == nil {
			t.Error("RemoveBlocking() with wrong etag should fail")
		}

		if _, ok := errors.AsType[*core.ETagMismatchError](err); !ok {
			t.Errorf("Expected ETagMismatchError, got %T: %v", err, err)
		}
	})
}
