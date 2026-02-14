package graph

import (
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
)

func TestFilterByHasExtension(t *testing.T) {
	beans := []*issue.Issue{
		{ID: "a", Extensions: map[string]map[string]any{"clickup": {"id": "1"}}},
		{ID: "b", Extensions: map[string]map[string]any{"jira": {"key": "X"}}},
		{ID: "c"},
	}

	got := filterByHasExtension(beans, "clickup")
	if len(got) != 1 {
		t.Fatalf("filterByHasExtension(clickup) count = %d, want 1", len(got))
	}
	if got[0].ID != "a" {
		t.Errorf("filterByHasExtension(clickup)[0].ID = %q, want 'a'", got[0].ID)
	}
}

func TestFilterByNoExtension(t *testing.T) {
	beans := []*issue.Issue{
		{ID: "a", Extensions: map[string]map[string]any{"clickup": {"id": "1"}}},
		{ID: "b", Extensions: map[string]map[string]any{"jira": {"key": "X"}}},
		{ID: "c"},
	}

	got := filterByNoExtension(beans, "clickup")
	if len(got) != 2 {
		t.Fatalf("filterByNoExtension(clickup) count = %d, want 2", len(got))
	}
	ids := map[string]bool{}
	for _, b := range got {
		ids[b.ID] = true
	}
	if !ids["b"] || !ids["c"] {
		t.Errorf("filterByNoExtension(clickup) = %v, want b and c", ids)
	}
}

func TestFilterByExtensionStale(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-1 * time.Hour)
	later := now.Add(1 * time.Hour)

	beans := []*issue.Issue{
		// Stale: updatedAt > synced_at
		{
			ID:        "stale",
			UpdatedAt: &now,
			Extensions: map[string]map[string]any{
				"clickup": {"synced_at": earlier.Format(time.RFC3339)},
			},
		},
		// Fresh: updatedAt < synced_at
		{
			ID:        "fresh",
			UpdatedAt: &now,
			Extensions: map[string]map[string]any{
				"clickup": {"synced_at": later.Format(time.RFC3339)},
			},
		},
		// No extension data (treated as stale)
		{
			ID:        "no-ext",
			UpdatedAt: &now,
		},
		// Has extension but no synced_at (treated as stale)
		{
			ID:        "no-synced",
			UpdatedAt: &now,
			Extensions: map[string]map[string]any{
				"clickup": {"task_id": "abc"},
			},
		},
		// No updatedAt (not stale - nothing to compare)
		{
			ID: "no-updated",
		},
		// Has different extension (stale for clickup since no clickup data)
		{
			ID:        "other-ext",
			UpdatedAt: &now,
			Extensions: map[string]map[string]any{
				"jira": {"synced_at": later.Format(time.RFC3339)},
			},
		},
	}

	got := filterByExtensionStale(beans, "clickup")
	ids := map[string]bool{}
	for _, b := range got {
		ids[b.ID] = true
	}

	if !ids["stale"] {
		t.Error("expected 'stale' in results")
	}
	if ids["fresh"] {
		t.Error("'fresh' should not be in results")
	}
	if !ids["no-ext"] {
		t.Error("expected 'no-ext' in results (treated as stale)")
	}
	if !ids["no-synced"] {
		t.Error("expected 'no-synced' in results (treated as stale)")
	}
	if ids["no-updated"] {
		t.Error("'no-updated' should not be in results (no updatedAt)")
	}
	if !ids["other-ext"] {
		t.Error("expected 'other-ext' in results (no clickup data = stale)")
	}
}

func TestFilterByChangedSince(t *testing.T) {
	now := time.Now().UTC()
	earlier := now.Add(-2 * time.Hour)
	since := now.Add(-1 * time.Hour)

	beans := []*issue.Issue{
		{ID: "recent", UpdatedAt: &now},
		{ID: "old", UpdatedAt: &earlier},
		{ID: "no-updated"},
		{ID: "exact", UpdatedAt: &since}, // exactly at threshold (should include)
	}

	got := filterByChangedSince(beans, since)
	ids := map[string]bool{}
	for _, b := range got {
		ids[b.ID] = true
	}

	if !ids["recent"] {
		t.Error("expected 'recent' in results")
	}
	if ids["old"] {
		t.Error("'old' should not be in results")
	}
	if ids["no-updated"] {
		t.Error("'no-updated' should not be in results")
	}
	if !ids["exact"] {
		t.Error("expected 'exact' in results (updatedAt == since)")
	}
}
