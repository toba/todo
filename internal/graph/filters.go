package graph

import (
	"cmp"
	"slices"
	"time"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph/model"
)

// ApplyFilter applies BeanFilter to a slice of beans and returns filtered results.
// This is used by both the top-level beans query and relationship field resolvers.
func ApplyFilter(beans []*issue.Issue, filter *model.IssueFilter, core *core.Core) []*issue.Issue {
	if filter == nil {
		return beans
	}

	result := beans

	// Status filters
	if len(filter.Status) > 0 {
		result = filterByField(result, filter.Status, func(b *issue.Issue) string { return b.Status })
	}
	if len(filter.ExcludeStatus) > 0 {
		result = excludeByField(result, filter.ExcludeStatus, func(b *issue.Issue) string { return b.Status })
	}

	// Type filters
	if len(filter.Type) > 0 {
		result = filterByField(result, filter.Type, func(b *issue.Issue) string { return b.Type })
	}
	if len(filter.ExcludeType) > 0 {
		result = excludeByField(result, filter.ExcludeType, func(b *issue.Issue) string { return b.Type })
	}

	// Priority filters (empty priority treated as "normal")
	if len(filter.Priority) > 0 {
		result = filterByPriority(result, filter.Priority)
	}
	if len(filter.ExcludePriority) > 0 {
		result = excludeByPriority(result, filter.ExcludePriority)
	}

	// Tag filters
	if len(filter.Tags) > 0 {
		result = filterByTags(result, filter.Tags)
	}
	if len(filter.ExcludeTags) > 0 {
		result = excludeByTags(result, filter.ExcludeTags)
	}

	// Parent filters
	if filter.HasParent != nil && *filter.HasParent {
		result = filterByHasParent(result)
	}
	if filter.NoParent != nil && *filter.NoParent {
		result = filterByNoParent(result)
	}
	if filter.ParentID != nil && *filter.ParentID != "" {
		result = filterByParentID(result, *filter.ParentID)
	}

	// Blocking filters
	if filter.HasBlocking != nil && *filter.HasBlocking {
		result = filterByHasBlocking(result)
	}
	if filter.BlockingID != nil && *filter.BlockingID != "" {
		result = filterByBlockingID(result, *filter.BlockingID)
	}
	if filter.NoBlocking != nil && *filter.NoBlocking {
		result = filterByNoBlocking(result)
	}
	if filter.IsBlocked != nil {
		if *filter.IsBlocked {
			result = filterByIsBlocked(result, core)
		} else {
			result = filterByNotBlocked(result, core)
		}
	}

	// Blocked-by filters (for direct blocked_by field)
	if filter.HasBlockedBy != nil && *filter.HasBlockedBy {
		result = filterByHasBlockedBy(result)
	}
	if filter.BlockedByID != nil && *filter.BlockedByID != "" {
		result = filterByBlockedByID(result, *filter.BlockedByID)
	}
	if filter.NoBlockedBy != nil && *filter.NoBlockedBy {
		result = filterByNoBlockedBy(result)
	}

	// Extension filters
	if filter.HasExtension != nil && *filter.HasExtension != "" {
		result = filterByHasExtension(result, *filter.HasExtension)
	}
	if filter.NoExtension != nil && *filter.NoExtension != "" {
		result = filterByNoExtension(result, *filter.NoExtension)
	}
	if filter.ExtensionStale != nil && *filter.ExtensionStale != "" {
		result = filterByExtensionStale(result, *filter.ExtensionStale)
	}
	if filter.ChangedSince != nil {
		result = filterByChangedSince(result, *filter.ChangedSince)
	}

	return result
}

// filterByField filters beans to include only those where getter returns a value in values (OR logic).
func filterByField(beans []*issue.Issue, values []string, getter func(*issue.Issue) string) []*issue.Issue {
	valueSet := make(map[string]bool, len(values))
	for _, v := range values {
		valueSet[v] = true
	}

	var result []*issue.Issue
	for _, b := range beans {
		if valueSet[getter(b)] {
			result = append(result, b)
		}
	}
	return result
}

// excludeByField filters beans to exclude those where getter returns a value in values.
func excludeByField(beans []*issue.Issue, values []string, getter func(*issue.Issue) string) []*issue.Issue {
	valueSet := make(map[string]bool, len(values))
	for _, v := range values {
		valueSet[v] = true
	}

	var result []*issue.Issue
	for _, b := range beans {
		if !valueSet[getter(b)] {
			result = append(result, b)
		}
	}
	return result
}

// filterByPriority filters beans to include only those with matching priorities (OR logic).
// Empty priority in the issue is treated as "normal" for matching purposes.
func filterByPriority(beans []*issue.Issue, priorities []string) []*issue.Issue {
	prioritySet := make(map[string]bool, len(priorities))
	for _, p := range priorities {
		prioritySet[p] = true
	}

	var result []*issue.Issue
	for _, b := range beans {
		priority := cmp.Or(b.Priority, config.PriorityNormal)
		if prioritySet[priority] {
			result = append(result, b)
		}
	}
	return result
}

// excludeByPriority filters beans to exclude those with matching priorities.
// Empty priority in the issue is treated as "normal" for matching purposes.
func excludeByPriority(beans []*issue.Issue, priorities []string) []*issue.Issue {
	prioritySet := make(map[string]bool, len(priorities))
	for _, p := range priorities {
		prioritySet[p] = true
	}

	var result []*issue.Issue
	for _, b := range beans {
		priority := cmp.Or(b.Priority, config.PriorityNormal)
		if !prioritySet[priority] {
			result = append(result, b)
		}
	}
	return result
}

// filterByTags filters beans to include only those with any of the given tags (OR logic).
func filterByTags(beans []*issue.Issue, tags []string) []*issue.Issue {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []*issue.Issue
	for _, b := range beans {
		for _, t := range b.Tags {
			if tagSet[t] {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// excludeByTags filters beans to exclude those with any of the given tags.
func excludeByTags(beans []*issue.Issue, tags []string) []*issue.Issue {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []*issue.Issue
outer:
	for _, b := range beans {
		for _, t := range b.Tags {
			if tagSet[t] {
				continue outer
			}
		}
		result = append(result, b)
	}
	return result
}

// filterByHasParent filters beans to include only those with a parent.
func filterByHasParent(beans []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if b.Parent != "" {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoParent filters beans to include only those without a parent.
func filterByNoParent(beans []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if b.Parent == "" {
			result = append(result, b)
		}
	}
	return result
}

// filterByParentID filters beans with specific parent ID.
func filterByParentID(beans []*issue.Issue, parentID string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if b.Parent == parentID {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasBlocking filters beans that are blocking other issues.
func filterByHasBlocking(beans []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if len(b.Blocking) > 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByBlockingID filters beans that are blocking a specific issue ID.
func filterByBlockingID(beans []*issue.Issue, targetID string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if slices.Contains(b.Blocking, targetID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoBlocking filters beans that aren't blocking other issues.
func filterByNoBlocking(beans []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if len(b.Blocking) == 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByIsBlocked filters beans that are blocked by others.
// an issue is considered blocked only if it has active (non-completed, non-scrapped) blockers.
func filterByIsBlocked(beans []*issue.Issue, core *core.Core) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if core.IsBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNotBlocked filters beans that are NOT blocked by others.
// an issue is considered not blocked if it has no active (non-completed, non-scrapped) blockers.
func filterByNotBlocked(beans []*issue.Issue, core *core.Core) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if !core.IsBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasBlockedBy filters beans that have explicit blocked_by entries.
func filterByHasBlockedBy(beans []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if len(b.BlockedBy) > 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByBlockedByID filters beans that are blocked by a specific issue ID (via blocked_by field).
func filterByBlockedByID(beans []*issue.Issue, blockerID string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if slices.Contains(b.BlockedBy, blockerID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoBlockedBy filters beans that have no explicit blocked_by entries.
func filterByNoBlockedBy(beans []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if len(b.BlockedBy) == 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasExtension filters beans to include only those with extension data for the given name.
func filterByHasExtension(beans []*issue.Issue, name string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if b.HasExtension(name) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoExtension filters beans to include only those without extension data for the given name.
func filterByNoExtension(beans []*issue.Issue, name string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if !b.HasExtension(name) {
			result = append(result, b)
		}
	}
	return result
}

// filterByExtensionStale filters beans where updatedAt > extensions[name]["synced_at"].
// If no synced_at or unparseable, the issue is treated as stale (conservative).
func filterByExtensionStale(beans []*issue.Issue, name string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if isExtensionStale(b, name) {
			result = append(result, b)
		}
	}
	return result
}

// isExtensionStale returns true if the issue's updatedAt is after the extension's synced_at.
func isExtensionStale(b *issue.Issue, name string) bool {
	if b.UpdatedAt == nil {
		return false
	}

	if b.Extensions == nil {
		return true
	}
	data, ok := b.Extensions[name]
	if !ok {
		return true
	}
	syncedAtRaw, ok := data["synced_at"]
	if !ok {
		return true
	}
	syncedAtStr, ok := syncedAtRaw.(string)
	if !ok {
		return true
	}
	syncedAt, err := time.Parse(time.RFC3339, syncedAtStr)
	if err != nil {
		return true
	}
	return b.UpdatedAt.After(syncedAt)
}

// filterByChangedSince filters beans where updatedAt >= since.
func filterByChangedSince(beans []*issue.Issue, since time.Time) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range beans {
		if b.UpdatedAt != nil && !b.UpdatedAt.Before(since) {
			result = append(result, b)
		}
	}
	return result
}
