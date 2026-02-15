package graph

import (
	"cmp"
	"slices"
	"time"

	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/graph/model"
	"github.com/toba/todo/internal/integration"
	"github.com/toba/todo/internal/issue"
)

// ApplyFilter applies IssueFilter to a slice of issues and returns filtered results.
// This is used by both the top-level issues query and relationship field resolvers.
func ApplyFilter(issues []*issue.Issue, filter *model.IssueFilter, core *core.Core) []*issue.Issue {
	if filter == nil {
		return issues
	}

	result := issues

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

	// Sync filters
	if filter.HasSync != nil && *filter.HasSync != "" {
		result = filterByHasSync(result, *filter.HasSync)
	}
	if filter.NoSync != nil && *filter.NoSync != "" {
		result = filterByNoSync(result, *filter.NoSync)
	}
	if filter.SyncStale != nil && *filter.SyncStale != "" {
		result = filterBySyncStale(result, *filter.SyncStale)
	}
	if filter.ChangedSince != nil {
		result = filterByChangedSince(result, *filter.ChangedSince)
	}

	return result
}

// filterByField filters issues to include only those where getter returns a value in values (OR logic).
func filterByField(issues []*issue.Issue, values []string, getter func(*issue.Issue) string) []*issue.Issue {
	valueSet := make(map[string]bool, len(values))
	for _, v := range values {
		valueSet[v] = true
	}

	var result []*issue.Issue
	for _, b := range issues {
		if valueSet[getter(b)] {
			result = append(result, b)
		}
	}
	return result
}

// excludeByField filters issues to exclude those where getter returns a value in values.
func excludeByField(issues []*issue.Issue, values []string, getter func(*issue.Issue) string) []*issue.Issue {
	valueSet := make(map[string]bool, len(values))
	for _, v := range values {
		valueSet[v] = true
	}

	var result []*issue.Issue
	for _, b := range issues {
		if !valueSet[getter(b)] {
			result = append(result, b)
		}
	}
	return result
}

// filterByPriority filters issues to include only those with matching priorities (OR logic).
// Empty priority in the issue is treated as "normal" for matching purposes.
func filterByPriority(issues []*issue.Issue, priorities []string) []*issue.Issue {
	prioritySet := make(map[string]bool, len(priorities))
	for _, p := range priorities {
		prioritySet[p] = true
	}

	var result []*issue.Issue
	for _, b := range issues {
		priority := cmp.Or(b.Priority, config.PriorityNormal)
		if prioritySet[priority] {
			result = append(result, b)
		}
	}
	return result
}

// excludeByPriority filters issues to exclude those with matching priorities.
// Empty priority in the issue is treated as "normal" for matching purposes.
func excludeByPriority(issues []*issue.Issue, priorities []string) []*issue.Issue {
	prioritySet := make(map[string]bool, len(priorities))
	for _, p := range priorities {
		prioritySet[p] = true
	}

	var result []*issue.Issue
	for _, b := range issues {
		priority := cmp.Or(b.Priority, config.PriorityNormal)
		if !prioritySet[priority] {
			result = append(result, b)
		}
	}
	return result
}

// filterByTags filters issues to include only those with any of the given tags (OR logic).
func filterByTags(issues []*issue.Issue, tags []string) []*issue.Issue {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []*issue.Issue
	for _, b := range issues {
		for _, t := range b.Tags {
			if tagSet[t] {
				result = append(result, b)
				break
			}
		}
	}
	return result
}

// excludeByTags filters issues to exclude those with any of the given tags.
func excludeByTags(issues []*issue.Issue, tags []string) []*issue.Issue {
	tagSet := make(map[string]bool, len(tags))
	for _, t := range tags {
		tagSet[t] = true
	}

	var result []*issue.Issue
outer:
	for _, b := range issues {
		for _, t := range b.Tags {
			if tagSet[t] {
				continue outer
			}
		}
		result = append(result, b)
	}
	return result
}

// filterByHasParent filters issues to include only those with a parent.
func filterByHasParent(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if b.Parent != "" {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoParent filters issues to include only those without a parent.
func filterByNoParent(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if b.Parent == "" {
			result = append(result, b)
		}
	}
	return result
}

// filterByParentID filters issues with specific parent ID.
func filterByParentID(issues []*issue.Issue, parentID string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if b.Parent == parentID {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasBlocking filters issues that are blocking other issues.
func filterByHasBlocking(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if len(b.Blocking) > 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByBlockingID filters issues that are blocking a specific issue ID.
func filterByBlockingID(issues []*issue.Issue, targetID string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if slices.Contains(b.Blocking, targetID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoBlocking filters issues that aren't blocking other issues.
func filterByNoBlocking(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if len(b.Blocking) == 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByIsBlocked filters issues that are blocked by others.
// an issue is considered blocked only if it has active (non-completed, non-scrapped) blockers.
func filterByIsBlocked(issues []*issue.Issue, core *core.Core) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if core.IsBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNotBlocked filters issues that are NOT blocked by others.
// an issue is considered not blocked if it has no active (non-completed, non-scrapped) blockers.
func filterByNotBlocked(issues []*issue.Issue, core *core.Core) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if !core.IsBlocked(b.ID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasBlockedBy filters issues that have explicit blocked_by entries.
func filterByHasBlockedBy(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if len(b.BlockedBy) > 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByBlockedByID filters issues that are blocked by a specific issue ID (via blocked_by field).
func filterByBlockedByID(issues []*issue.Issue, blockerID string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if slices.Contains(b.BlockedBy, blockerID) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoBlockedBy filters issues that have no explicit blocked_by entries.
func filterByNoBlockedBy(issues []*issue.Issue) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if len(b.BlockedBy) == 0 {
			result = append(result, b)
		}
	}
	return result
}

// filterByHasSync filters issues to include only those with sync data for the given name.
func filterByHasSync(issues []*issue.Issue, name string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if b.HasSync(name) {
			result = append(result, b)
		}
	}
	return result
}

// filterByNoSync filters issues to include only those without sync data for the given name.
func filterByNoSync(issues []*issue.Issue, name string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if !b.HasSync(name) {
			result = append(result, b)
		}
	}
	return result
}

// filterBySyncStale filters issues where updatedAt > sync[name]["synced_at"].
// If no synced_at or unparseable, the issue is treated as stale (conservative).
func filterBySyncStale(issues []*issue.Issue, name string) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if isSyncStale(b, name) {
			result = append(result, b)
		}
	}
	return result
}

// isSyncStale returns true if the issue's updatedAt is after the sync integration's synced_at.
func isSyncStale(b *issue.Issue, name string) bool {
	if b.UpdatedAt == nil {
		return false
	}

	if b.Sync == nil {
		return true
	}
	data, ok := b.Sync[name]
	if !ok {
		return true
	}
	syncedAtRaw, ok := data[integration.SyncKeySyncedAt]
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

// filterByChangedSince filters issues where updatedAt >= since.
func filterByChangedSince(issues []*issue.Issue, since time.Time) []*issue.Issue {
	var result []*issue.Issue
	for _, b := range issues {
		if b.UpdatedAt != nil && !b.UpdatedAt.Before(since) {
			result = append(result, b)
		}
	}
	return result
}
