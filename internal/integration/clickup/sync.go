package clickup

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/issue"
)

// SyncResult holds the result of syncing a single issue.
type SyncResult struct {
	IssueID    string
	IssueTitle string
	TaskID     string
	TaskURL    string
	Action     string // Matches integration.Action* constants
	Error      error
}

// ProgressFunc is called when an issue sync completes.
// It receives the result and the current progress (completed count, total count).
type ProgressFunc func(result SyncResult, completed, total int)

// SyncOptions configures the sync operation.
type SyncOptions struct {
	DryRun          bool
	Force           bool
	NoRelationships bool
	ListID          string
	OnProgress      ProgressFunc // Optional callback for progress updates
}

// Syncer handles syncing issues to ClickUp tasks.
type Syncer struct {
	client    *Client
	config    *Config
	opts      SyncOptions
	core      *core.Core
	syncStore SyncStateProvider

	// Tracking for relationship pass
	issueToTaskID map[string]string // issue ID -> ClickUp task ID

	// Space ID for space-level tag management
	spaceID string
}

// NewSyncer creates a new syncer with the given client and options.
func NewSyncer(client *Client, cfg *Config, opts SyncOptions, c *core.Core, syncStore SyncStateProvider) *Syncer {
	return &Syncer{
		client:        client,
		config:        cfg,
		opts:          opts,
		core:          c,
		syncStore:     syncStore,
		issueToTaskID: make(map[string]string),
	}
}

// SyncIssues syncs a list of issues to ClickUp tasks.
// Uses a multi-pass approach:
// 1. Create/update parent tasks (issues without parents, or parents not in this sync)
// 2. Create/update child tasks with parent references
// 3. Sync blocking relationships as dependencies
func (s *Syncer) SyncIssues(ctx context.Context, issues []*issue.Issue) ([]SyncResult, error) {
	// Pre-fetch authorized user to avoid per-task API calls
	if _, err := s.client.GetAuthorizedUser(ctx); err != nil {
		// Non-fatal - will just create unassigned tasks if this fails
		_ = err
	}

	// Pre-fetch list info for space ID, then populate space tag cache
	if list, err := s.client.GetList(ctx, s.opts.ListID); err == nil && list.SpaceID != "" {
		s.spaceID = list.SpaceID
		if err := s.client.PopulateSpaceTagCache(ctx, s.spaceID); err != nil {
			// Non-fatal - tags will still be added at task level
			_ = err
		}
	}

	// Pre-populate mapping with already-synced issues from sync store
	for _, b := range issues {
		taskID := s.syncStore.GetTaskID(b.ID)
		if taskID != nil && *taskID != "" {
			s.issueToTaskID[b.ID] = *taskID
		}
	}
	// Also pre-populate parents not in the batch so that parent
	// relationships are set even when the parent isn't being synced.
	for _, b := range issues {
		if b.Parent != "" {
			if _, exists := s.issueToTaskID[b.Parent]; !exists {
				// The sync store is only populated with batch issues, so look up
				// the parent directly from the issue store.
				if parent, err := s.core.Get(b.Parent); err == nil {
					if taskID := GetSyncString(parent, SyncKeyTaskID); taskID != "" {
						s.issueToTaskID[b.Parent] = taskID
					}
				}
			}
		}
	}

	// Build a set of issue IDs being synced
	syncingIDs := make(map[string]bool)
	for _, b := range issues {
		syncingIDs[b.ID] = true
	}

	// Separate issues into layers: parents first, then children
	// An issue is a "parent" if it has no parent, or its parent isn't being synced
	var parents, children []*issue.Issue
	for _, b := range issues {
		if b.Parent == "" || !syncingIDs[b.Parent] {
			parents = append(parents, b)
		} else {
			children = append(children, b)
		}
	}

	// Create index mapping for results
	issueIndex := make(map[string]int)
	for i, b := range issues {
		issueIndex[b.ID] = i
	}
	results := make([]SyncResult, len(issues))
	total := len(issues)

	var wg sync.WaitGroup
	var mu sync.Mutex // protects issueToTaskID and completed count
	var completed int

	// Helper to report progress
	reportProgress := func(result SyncResult) {
		if s.opts.OnProgress != nil {
			mu.Lock()
			completed++
			current := completed
			mu.Unlock()
			s.opts.OnProgress(result, current, total)
		}
	}

	// Pass 1: Create/update parent tasks in parallel
	for _, b := range parents {
		wg.Go(func() {
			result := s.syncIssue(ctx, b)
			idx := issueIndex[b.ID]
			results[idx] = result

			if result.Error == nil && result.Action != "skipped" && result.TaskID != "" {
				mu.Lock()
				s.issueToTaskID[b.ID] = result.TaskID
				mu.Unlock()
			}
			reportProgress(result)
		})
	}
	wg.Wait()

	// Pass 2: Create/update child tasks in parallel (parents now exist)
	for _, b := range children {
		wg.Go(func() {
			result := s.syncIssue(ctx, b)
			idx := issueIndex[b.ID]
			results[idx] = result

			if result.Error == nil && result.Action != "skipped" && result.TaskID != "" {
				mu.Lock()
				s.issueToTaskID[b.ID] = result.TaskID
				mu.Unlock()
			}
			reportProgress(result)
		})
	}
	wg.Wait()

	// Pass 3: Sync blocking relationships in parallel (if not disabled)
	if !s.opts.NoRelationships && !s.opts.DryRun {
		for _, b := range issues {
			wg.Go(func() {
				if err := s.syncRelationships(ctx, b); err != nil {
					// Log but don't fail - relationships are best-effort
					_ = err
				}
			})
		}
		wg.Wait()
	}

	return results, nil
}

// syncIssue syncs a single issue to a ClickUp task.
func (s *Syncer) syncIssue(ctx context.Context, b *issue.Issue) SyncResult {
	result := SyncResult{
		IssueID:    b.ID,
		IssueTitle: b.Title,
	}

	// Build the task description
	description := b.Body

	// Map issue status to ClickUp status
	clickUpStatus := s.getClickUpStatus(b.Status)

	// Map issue priority to ClickUp priority
	priority := s.getClickUpPriority(b.Priority)

	// Check if already linked (from sync store)
	taskID := s.syncStore.GetTaskID(b.ID)
	if taskID != nil && *taskID != "" {
		result.TaskID = *taskID

		// Check if issue has changed since last sync
		if !s.opts.Force && !s.needsSync(b) {
			result.Action = "skipped"
			return result
		}

		// Verify task still exists
		task, err := s.client.GetTask(ctx, *taskID)
		if err != nil {
			// Check if task was deleted - if so, unlink and create new
			if strings.Contains(err.Error(), "Task not found") || strings.Contains(err.Error(), "ITEM_013") {
				s.syncStore.Clear(b.ID)
				// Fall through to create new task below
			} else {
				result.Action = "error"
				result.Error = fmt.Errorf("fetching task %s: %w", *taskID, err)
				return result
			}
		} else {
			// Task exists - update it
			result.TaskURL = task.URL

			if s.opts.DryRun {
				result.Action = "would update"
				return result
			}

			// Build update request with only changed fields
			update := s.buildUpdateRequest(task, b, description, priority, clickUpStatus)

			// Check if any core fields changed
			if update.hasChanges() {
				updatedTask, err := s.client.UpdateTask(ctx, *taskID, update)
				if err != nil {
					result.Action = "error"
					result.Error = fmt.Errorf("updating task: %w", err)
					return result
				}
				result.TaskURL = updatedTask.URL
			}

			// Update custom fields only if changed (best-effort)
			customFieldsUpdated := s.updateChangedCustomFields(ctx, task, *taskID, b)

			// Sync tags (best-effort)
			tagsChanged := s.syncTags(ctx, *taskID, b, task.Tags)

			// Update synced_at timestamp in sync store
			s.syncStore.SetSyncedAt(b.ID, time.Now().UTC())

			if update.hasChanges() || customFieldsUpdated || tagsChanged {
				result.Action = "updated"
			} else {
				result.Action = "unchanged"
			}
			return result
		}
	}

	// Create new task
	if s.opts.DryRun {
		result.Action = "would create"
		return result
	}

	createReq := &CreateTaskRequest{
		Name:                b.Title,
		MarkdownDescription: description,
		Status:              clickUpStatus,
		Priority:            priority,
		Assignees:           s.getAssignees(ctx),
		CustomFields:        s.buildCustomFields(b),
		CustomItemID:        s.getClickUpCustomItemID(b.Type),
	}

	// Set due date if issue has one
	if b.Due != nil {
		millis := toLocalDateMillis(b.Due.Time)
		createReq.DueDate = &millis
		createReq.DueDatetime = new(false)
	}

	// Set parent task ID if issue has a parent that's already synced
	if b.Parent != "" {
		if parentTaskID, ok := s.issueToTaskID[b.Parent]; ok {
			createReq.Parent = &parentTaskID
		}
	}

	task, err := s.client.CreateTask(ctx, s.opts.ListID, createReq)
	if err != nil {
		result.Action = "error"
		result.Error = fmt.Errorf("creating task: %w", err)
		return result
	}

	result.TaskID = task.ID
	result.TaskURL = task.URL
	s.issueToTaskID[b.ID] = task.ID

	// Sync tags for new task (no existing tags to remove)
	s.syncTags(ctx, task.ID, b, nil)

	// Store task ID and sync timestamp in sync store
	s.syncStore.SetTaskID(b.ID, task.ID)
	s.syncStore.SetSyncedAt(b.ID, time.Now().UTC())

	result.Action = "created"
	return result
}

// needsSync checks if an issue needs to be synced based on timestamps.
func (s *Syncer) needsSync(b *issue.Issue) bool {
	syncedAt := s.syncStore.GetSyncedAt(b.ID)
	if syncedAt == nil {
		return true // Never synced
	}
	if b.UpdatedAt == nil {
		return false // No update time, assume in sync
	}
	return b.UpdatedAt.After(*syncedAt)
}

// getClickUpPriority maps an issue priority to a ClickUp priority value.
// Returns nil if no mapping exists (issue has no priority or unknown priority).
func (s *Syncer) getClickUpPriority(issuePriority string) *int {
	if issuePriority == "" {
		return nil
	}

	// Use custom mapping if configured
	if s.config != nil && s.config.PriorityMapping != nil {
		if priority, ok := s.config.PriorityMapping[issuePriority]; ok {
			return &priority
		}
	}

	// Fall back to default mapping
	if priority, ok := DefaultPriorityMapping[issuePriority]; ok {
		return &priority
	}

	return nil
}

// buildCustomFields builds the custom fields array for task creation.
func (s *Syncer) buildCustomFields(b *issue.Issue) []CustomField {
	if s.config == nil || s.config.CustomFields == nil {
		return nil
	}

	var fields []CustomField
	cf := s.config.CustomFields

	// Issue ID field (text)
	if cf.BeanID != "" {
		fields = append(fields, CustomField{
			ID:    cf.BeanID,
			Value: b.ID,
		})
	}

	// Created at field (date - Unix milliseconds)
	// Convert to local date at midnight to avoid timezone display issues in ClickUp
	if cf.CreatedAt != "" && b.CreatedAt != nil {
		fields = append(fields, CustomField{
			ID:    cf.CreatedAt,
			Value: toLocalDateMillis(*b.CreatedAt),
		})
	}

	// Updated at field (date - Unix milliseconds)
	// Convert to local date at midnight to avoid timezone display issues in ClickUp
	if cf.UpdatedAt != "" && b.UpdatedAt != nil {
		fields = append(fields, CustomField{
			ID:    cf.UpdatedAt,
			Value: toLocalDateMillis(*b.UpdatedAt),
		})
	}

	return fields
}

// toLocalDateMillis converts a timestamp to midnight of that date in local timezone.
// This ensures ClickUp displays the date the user expects (the local date when the issue
// was created) rather than potentially showing "Tomorrow" due to UTC offset.
func toLocalDateMillis(t time.Time) int64 {
	local := t.Local()
	midnight := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	return midnight.UnixMilli()
}

// getAssignees returns the assignee list for task creation.
// Returns token owner by default, configured assignee if set, or empty if assignee is 0.
func (s *Syncer) getAssignees(ctx context.Context) []int {
	// Check if explicitly configured
	if s.config != nil && s.config.Assignee != nil {
		if *s.config.Assignee == 0 {
			// Explicitly set to 0 means unassigned
			return nil
		}
		return []int{*s.config.Assignee}
	}

	// Default: assign to token owner
	user, err := s.client.GetAuthorizedUser(ctx)
	if err != nil {
		// Can't get user, leave unassigned
		return nil
	}
	return []int{user.ID}
}

// buildUpdateRequest builds an UpdateTaskRequest containing only fields that differ from current.
func (s *Syncer) buildUpdateRequest(current *TaskInfo, b *issue.Issue, description string, priority *int, clickUpStatus string) *UpdateTaskRequest {
	update := &UpdateTaskRequest{}

	// Only include name if changed
	if current.Name != b.Title {
		update.Name = &b.Title
	}

	// Only include description if changed
	if current.Description != description {
		update.MarkdownDescription = &description
	}

	// Only include priority if changed
	if !s.priorityEqual(current.Priority, priority) {
		update.Priority = priority
	}

	// Only include status if changed
	if clickUpStatus != "" && current.Status.Status != clickUpStatus {
		update.Status = &clickUpStatus
	}

	// Only include due date if changed
	newDueMillis := issueDueToMillis(b.Due)
	currentDueMillis := clickUpDueToMillis(current.DueDate)
	if !int64PtrEqual(currentDueMillis, newDueMillis) {
		if newDueMillis != nil {
			update.DueDate = newDueMillis
			update.DueDatetime = new(false)
		} else {
			// Clear due date: ClickUp accepts null to remove it
			zero := int64(0)
			update.DueDate = &zero
		}
	}

	// Only include custom item ID if changed
	newItemID := s.getClickUpCustomItemID(b.Type)
	if !intPtrEqual(current.CustomItemID, newItemID) {
		update.CustomItemID = newItemID
	}

	// Only include parent if changed
	var wantParent *string
	if b.Parent != "" {
		if parentTaskID, ok := s.issueToTaskID[b.Parent]; ok {
			wantParent = &parentTaskID
		}
	}
	if !stringPtrEqual(current.Parent, wantParent) {
		update.Parent = wantParent
	}

	return update
}

// priorityEqual compares a TaskPriority (from ClickUp response) with a target priority int pointer.
func (s *Syncer) priorityEqual(current *TaskPriority, target *int) bool {
	if current == nil && target == nil {
		return true
	}
	if current == nil || target == nil {
		return false
	}
	return current.ID == *target
}

// stringPtrEqual compares two string pointers for equality.
func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// intPtrEqual compares two int pointers for equality.
func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// updateChangedCustomFields updates only custom fields that have changed.
// Returns true if any field was updated.
func (s *Syncer) updateChangedCustomFields(ctx context.Context, current *TaskInfo, taskID string, b *issue.Issue) bool {
	if s.config == nil || s.config.CustomFields == nil {
		return false
	}

	cf := s.config.CustomFields
	updated := false

	// Build a map of current custom field values by ID for quick lookup
	currentFields := make(map[string]any)
	for _, f := range current.CustomFields {
		currentFields[f.ID] = f.Value
	}

	// Issue ID field (text)
	if cf.BeanID != "" {
		currentVal, _ := currentFields[cf.BeanID].(string)
		if currentVal != b.ID {
			if err := s.client.SetCustomFieldValue(ctx, taskID, cf.BeanID, b.ID); err == nil {
				updated = true
			}
		}
	}

	// Created at field (date - Unix milliseconds)
	if cf.CreatedAt != "" && b.CreatedAt != nil {
		newVal := toLocalDateMillis(*b.CreatedAt)
		if !customFieldDateEqual(currentFields[cf.CreatedAt], newVal) {
			if err := s.client.SetCustomFieldValue(ctx, taskID, cf.CreatedAt, newVal); err == nil {
				updated = true
			}
		}
	}

	// Updated at field (date - Unix milliseconds)
	if cf.UpdatedAt != "" && b.UpdatedAt != nil {
		newVal := toLocalDateMillis(*b.UpdatedAt)
		if !customFieldDateEqual(currentFields[cf.UpdatedAt], newVal) {
			if err := s.client.SetCustomFieldValue(ctx, taskID, cf.UpdatedAt, newVal); err == nil {
				updated = true
			}
		}
	}

	return updated
}

// customFieldDateEqual compares a custom field date value (from ClickUp, can be string or number)
// with a target milliseconds value.
func customFieldDateEqual(current any, target int64) bool {
	if current == nil {
		return false
	}

	// ClickUp returns date fields as string timestamps in milliseconds
	switch v := current.(type) {
	case string:
		// Parse string to int64
		var parsed int64
		if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil {
			return parsed == target
		}
	case float64:
		// JSON numbers are decoded as float64
		return int64(v) == target
	case int64:
		return v == target
	}
	return false
}

// getClickUpStatus maps an issue status to a ClickUp status name.
func (s *Syncer) getClickUpStatus(issueStatus string) string {
	// Use custom mapping if configured
	if s.config != nil && s.config.StatusMapping != nil {
		if status, ok := s.config.StatusMapping[issueStatus]; ok {
			return status
		}
	}

	// Fall back to default mapping
	if status, ok := DefaultStatusMapping[issueStatus]; ok {
		return status
	}

	return ""
}

// getClickUpCustomItemID maps an issue type to a ClickUp custom item ID.
// Returns nil if no mapping exists (task will use default type).
func (s *Syncer) getClickUpCustomItemID(issueType string) *int {
	if issueType == "" {
		return nil
	}

	// Use custom mapping if configured
	if s.config != nil && s.config.TypeMapping != nil {
		if customItemID, ok := s.config.TypeMapping[issueType]; ok {
			return &customItemID
		}
	}

	return nil
}

// syncTags syncs issue tags to ClickUp task tags.
// Returns true if any tags were added or removed.
func (s *Syncer) syncTags(ctx context.Context, taskID string, b *issue.Issue, currentTags []Tag) bool {
	// Build set of current ClickUp tag names
	current := make(map[string]bool)
	for _, t := range currentTags {
		current[t.Name] = true
	}

	// Build set of desired issue tag names
	desired := make(map[string]bool)
	for _, t := range b.Tags {
		desired[t] = true
	}

	changed := false

	// Add missing tags
	for _, t := range b.Tags {
		if !current[t] {
			// Ensure tag exists at space level so it's discoverable in the tag picker
			if s.spaceID != "" {
				if err := s.client.EnsureSpaceTag(ctx, s.spaceID, t); err != nil {
					_ = err // Best-effort
				}
			}
			if err := s.client.AddTagToTask(ctx, taskID, t); err != nil {
				_ = err // Best-effort
			} else {
				changed = true
			}
		}
	}

	// Remove extra tags
	for _, t := range currentTags {
		if !desired[t.Name] {
			if err := s.client.RemoveTagFromTask(ctx, taskID, t.Name); err != nil {
				_ = err // Best-effort
			} else {
				changed = true
			}
		}
	}

	return changed
}

// syncRelationships syncs blocking relationships for an issue.
func (s *Syncer) syncRelationships(ctx context.Context, b *issue.Issue) error {
	taskID, ok := s.issueToTaskID[b.ID]
	if !ok {
		return nil // Issue not synced
	}

	// Sync blocking relationships (dependencies)
	// In issues: issue A with blocking: [B, C] means A is blocking B and C
	// In ClickUp: we set B and C as "waiting on" A (depends_on = A)
	for _, blockedID := range b.Blocking {
		blockedTaskID, ok := s.issueToTaskID[blockedID]
		if !ok {
			continue // Blocked issue not synced
		}

		// Add dependency: blockedTaskID depends on taskID (taskID blocks blockedTaskID)
		if err := s.client.AddDependency(ctx, blockedTaskID, taskID); err != nil {
			// Dependencies might fail if already exists, continue
			_ = err
		}
	}

	return nil
}

// FilterIssuesNeedingSync returns only issues that need to be synced based on timestamps.
// An issue needs sync if: force is true, it has no sync record, or it was updated after last sync.
func FilterIssuesNeedingSync(issues []*issue.Issue, store SyncStateProvider, force bool) []*issue.Issue {
	var needSync []*issue.Issue
	for _, b := range issues {
		if force {
			needSync = append(needSync, b)
			continue
		}
		syncedAt := store.GetSyncedAt(b.ID)
		if syncedAt == nil {
			needSync = append(needSync, b) // Never synced
			continue
		}
		if b.UpdatedAt != nil && b.UpdatedAt.After(*syncedAt) {
			needSync = append(needSync, b) // Updated since last sync
		}
	}
	return needSync
}

// FilterIssuesForSync filters issues based on sync filter configuration.
func FilterIssuesForSync(issues []*issue.Issue, filter *SyncFilter) []*issue.Issue {
	if filter == nil {
		return issues
	}

	excludeStatus := make(map[string]bool)
	for _, s := range filter.ExcludeStatus {
		excludeStatus[s] = true
	}

	var filtered []*issue.Issue
	for _, b := range issues {
		if !excludeStatus[b.Status] {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// issueDueToMillis converts an issue due date to Unix milliseconds (local midnight).
// Returns nil if the issue has no due date.
func issueDueToMillis(due *issue.DueDate) *int64 {
	if due == nil {
		return nil
	}
	millis := toLocalDateMillis(due.Time)
	return &millis
}

// clickUpDueToMillis parses ClickUp's due_date string (Unix ms) into an *int64.
// Returns nil if the string is nil or empty.
func clickUpDueToMillis(s *string) *int64 {
	if s == nil || *s == "" {
		return nil
	}
	var millis int64
	if _, err := fmt.Sscanf(*s, "%d", &millis); err != nil {
		return nil
	}
	return &millis
}

// int64PtrEqual compares two int64 pointers for equality.
func int64PtrEqual(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

//go:fix inline
func ptr[T any](v T) *T { return new(v) }
