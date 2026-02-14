package github

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/issue"
)

// Syncer handles syncing issues to GitHub issues.
type Syncer struct {
	client    *Client
	config    *Config
	opts      SyncOptions
	core      *core.Core
	syncStore SyncStateProvider

	// Tracking for relationship pass
	issueToGHNumber map[string]int // local issue ID -> GitHub issue number
}

// NewSyncer creates a new syncer with the given client and options.
func NewSyncer(client *Client, cfg *Config, opts SyncOptions, c *core.Core, syncStore SyncStateProvider) *Syncer {
	return &Syncer{
		client:          client,
		config:          cfg,
		opts:            opts,
		core:            c,
		syncStore:       syncStore,
		issueToGHNumber: make(map[string]int),
	}
}

// SyncIssues syncs a list of issues to GitHub issues.
// Uses a multi-pass approach:
// 1. Create/update parent issues (issues without parents, or parents not in this sync)
// 2. Create/update child issues with sub-issue relationships
// 3. Update blocking references in issue bodies
func (s *Syncer) SyncIssues(ctx context.Context, issues []*issue.Issue) ([]SyncResult, error) {
	// Pre-fetch authenticated user to avoid per-issue API calls
	if _, err := s.client.GetAuthenticatedUser(ctx); err != nil {
		_ = err // Non-fatal
	}

	// Pre-populate label cache
	if err := s.client.PopulateLabelCache(ctx); err != nil {
		_ = err // Non-fatal
	}

	// Ensure all labels that will be needed exist
	s.ensureAllLabels(ctx, issues)

	// Pre-populate mapping with already-synced issues from sync store
	for _, b := range issues {
		issueNumber := s.syncStore.GetIssueNumber(b.ID)
		if issueNumber != nil && *issueNumber != 0 {
			s.issueToGHNumber[b.ID] = *issueNumber
		}
	}

	// Build a set of issue IDs being synced
	syncingIDs := make(map[string]bool)
	for _, b := range issues {
		syncingIDs[b.ID] = true
	}

	// Separate issues into layers: parents first, then children
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
	var mu sync.Mutex
	var completed int

	reportProgress := func(result SyncResult) {
		if s.opts.OnProgress != nil {
			mu.Lock()
			completed++
			current := completed
			mu.Unlock()
			s.opts.OnProgress(result, current, total)
		}
	}

	// Pass 1: Create/update parent issues in parallel
	for _, b := range parents {
		wg.Add(1)
		go func(iss *issue.Issue) {
			defer wg.Done()
			result := s.syncIssue(ctx, iss)
			idx := issueIndex[iss.ID]
			results[idx] = result

			if result.Error == nil && result.Action != "skipped" && result.ExternalID != "" {
				mu.Lock()
				var n int
				if _, err := fmt.Sscanf(result.ExternalID, "%d", &n); err == nil {
					s.issueToGHNumber[iss.ID] = n
				}
				mu.Unlock()
			}
			reportProgress(result)
		}(b)
	}
	wg.Wait()

	// Pass 2: Create/update child issues in parallel (parents now exist)
	for _, b := range children {
		wg.Add(1)
		go func(iss *issue.Issue) {
			defer wg.Done()
			result := s.syncIssue(ctx, iss)
			idx := issueIndex[iss.ID]
			results[idx] = result

			if result.Error == nil && result.Action != "skipped" && result.ExternalID != "" {
				mu.Lock()
				var n int
				if _, err := fmt.Sscanf(result.ExternalID, "%d", &n); err == nil {
					s.issueToGHNumber[iss.ID] = n
				}
				mu.Unlock()
			}
			reportProgress(result)
		}(b)
	}
	wg.Wait()

	// Pass 3: Update blocking references in issue bodies (if not disabled)
	if !s.opts.NoRelationships && !s.opts.DryRun {
		for _, b := range issues {
			wg.Add(1)
			go func(iss *issue.Issue) {
				defer wg.Done()
				if err := s.syncRelationships(ctx, iss); err != nil {
					_ = err // Best-effort
				}
			}(b)
		}
		wg.Wait()
	}

	return results, nil
}

// syncIssue syncs a single issue to a GitHub issue.
func (s *Syncer) syncIssue(ctx context.Context, b *issue.Issue) SyncResult {
	result := SyncResult{
		IssueID:    b.ID,
		IssueTitle: b.Title,
	}

	// Compute labels and state
	labels := s.computeLabels(b)
	state := s.getGitHubState(b.Status)
	body := s.buildIssueBody(b)

	// Check if already linked (from sync store)
	issueNumber := s.syncStore.GetIssueNumber(b.ID)
	if issueNumber != nil && *issueNumber != 0 {
		result.ExternalID = fmt.Sprintf("%d", *issueNumber)

		// Check if issue has changed since last sync
		if !s.opts.Force && !s.needsSync(b) {
			result.Action = "skipped"
			return result
		}

		// Verify issue still exists
		ghIssue, err := s.client.GetIssue(ctx, *issueNumber)
		if err != nil {
			if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
				s.syncStore.Clear(b.ID)
				// Fall through to create new issue
			} else {
				result.Action = "error"
				result.Error = fmt.Errorf("fetching issue #%d: %w", *issueNumber, err)
				return result
			}
		} else {
			// Issue exists - update it
			result.ExternalURL = ghIssue.HTMLURL

			if s.opts.DryRun {
				result.Action = "would update"
				return result
			}

			update := s.buildUpdateRequest(ghIssue, b, body, state, labels)

			if update.hasChanges() {
				updatedIssue, err := s.client.UpdateIssue(ctx, *issueNumber, update)
				if err != nil {
					result.Action = "error"
					result.Error = fmt.Errorf("updating issue: %w", err)
					return result
				}
				result.ExternalURL = updatedIssue.HTMLURL
				result.Action = "updated"
			} else {
				result.Action = "unchanged"
			}

			// Update synced_at timestamp
			s.syncStore.SetSyncedAt(b.ID, time.Now().UTC())
			return result
		}
	}

	// Create new issue
	if s.opts.DryRun {
		result.Action = "would create"
		return result
	}

	createReq := &CreateIssueRequest{
		Title:     b.Title,
		Body:      body,
		Labels:    labels,
		Assignees: s.getAssignees(ctx),
	}

	ghIssue, err := s.client.CreateIssue(ctx, createReq)
	if err != nil {
		result.Action = "error"
		result.Error = fmt.Errorf("creating issue: %w", err)
		return result
	}

	result.ExternalID = fmt.Sprintf("%d", ghIssue.Number)
	result.ExternalURL = ghIssue.HTMLURL
	s.issueToGHNumber[b.ID] = ghIssue.Number

	// Close issue if state should be closed (can't create closed issues directly)
	if state == "closed" {
		closedState := "closed"
		_, err := s.client.UpdateIssue(ctx, ghIssue.Number, &UpdateIssueRequest{State: &closedState})
		if err != nil {
			_ = err // Best-effort
		}
	}

	// Link as sub-issue if parent is synced
	if b.Parent != "" {
		if parentNumber, ok := s.issueToGHNumber[b.Parent]; ok {
			if err := s.client.AddSubIssue(ctx, parentNumber, ghIssue.Number); err != nil {
				_ = err // Best-effort
			}
		}
	}

	// Store issue number and sync timestamp
	s.syncStore.SetIssueNumber(b.ID, ghIssue.Number)
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

// buildIssueBody builds the GitHub issue body from a local issue.
// Includes the issue body and a hidden HTML comment with the issue ID.
func (s *Syncer) buildIssueBody(b *issue.Issue) string {
	var parts []string
	if b.Body != "" {
		parts = append(parts, b.Body)
	}
	parts = append(parts, fmt.Sprintf("<!-- bean:%s -->", b.ID))
	return strings.Join(parts, "\n\n")
}

// getGitHubState maps a bean status to a GitHub issue state.
func (s *Syncer) getGitHubState(beanStatus string) string {
	if target, ok := DefaultStatusMapping[beanStatus]; ok {
		return target.State
	}
	return "open"
}

// getStatusLabel returns the status label for a bean status, or empty string.
func (s *Syncer) getStatusLabel(beanStatus string) string {
	if target, ok := DefaultStatusMapping[beanStatus]; ok {
		return target.Label
	}
	return ""
}

// computeLabels computes all labels for an issue.
func (s *Syncer) computeLabels(b *issue.Issue) []string {
	var labels []string

	// Status label
	if label := s.getStatusLabel(b.Status); label != "" {
		labels = append(labels, label)
	}

	// Priority label
	if b.Priority != "" {
		if label, ok := DefaultPriorityMapping[b.Priority]; ok && label != "" {
			labels = append(labels, label)
		}
	}

	// Type label
	if b.Type != "" {
		if label, ok := DefaultTypeMapping[b.Type]; ok && label != "" {
			labels = append(labels, label)
		}
	}

	// Bean tags directly as labels
	labels = append(labels, b.Tags...)

	return labels
}

// ensureAllLabels pre-creates all labels that will be needed.
func (s *Syncer) ensureAllLabels(ctx context.Context, issues []*issue.Issue) {
	needed := make(map[string]bool)
	for _, b := range issues {
		for _, label := range s.computeLabels(b) {
			needed[label] = true
		}
	}

	for label := range needed {
		if err := s.client.EnsureLabel(ctx, label, "ededed"); err != nil {
			_ = err // Best-effort
		}
	}
}

// getAssignees returns the assignee list for issue creation.
func (s *Syncer) getAssignees(ctx context.Context) []string {
	// Assign to token owner
	user, err := s.client.GetAuthenticatedUser(ctx)
	if err != nil {
		return nil
	}
	return []string{user.Login}
}

// buildUpdateRequest builds an UpdateIssueRequest containing only fields that differ from current.
func (s *Syncer) buildUpdateRequest(current *Issue, b *issue.Issue, body, state string, labels []string) *UpdateIssueRequest {
	update := &UpdateIssueRequest{}

	// Only include title if changed
	if current.Title != b.Title {
		update.Title = &b.Title
	}

	// Only include body if changed
	if current.Body != body {
		update.Body = &body
	}

	// Only include state if changed
	if current.State != state {
		update.State = &state
	}

	// Only include labels if changed
	currentLabels := make([]string, len(current.Labels))
	for i, l := range current.Labels {
		currentLabels[i] = l.Name
	}
	sort.Strings(currentLabels)
	sortedNew := make([]string, len(labels))
	copy(sortedNew, labels)
	sort.Strings(sortedNew)
	if !slicesEqual(currentLabels, sortedNew) {
		update.Labels = labels
	}

	return update
}

// syncRelationships updates blocking references in issue bodies.
func (s *Syncer) syncRelationships(ctx context.Context, b *issue.Issue) error {
	if len(b.Blocking) == 0 {
		return nil
	}

	ghNumber, ok := s.issueToGHNumber[b.ID]
	if !ok {
		return nil
	}

	// Build blocking references
	var refs []string
	for _, blockedID := range b.Blocking {
		if blockedNumber, ok := s.issueToGHNumber[blockedID]; ok {
			refs = append(refs, fmt.Sprintf("#%d", blockedNumber))
		}
	}

	if len(refs) == 0 {
		return nil
	}

	// Get current issue to check body
	ghIssue, err := s.client.GetIssue(ctx, ghNumber)
	if err != nil {
		return err
	}

	// Build the blocking line
	blockingLine := fmt.Sprintf("**Blocks:** %s", strings.Join(refs, ", "))

	// Check if body already contains the blocking line
	newBody := ghIssue.Body

	// Remove any existing Blocks line
	lines := strings.Split(newBody, "\n")
	var filtered []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "**Blocks:**") {
			filtered = append(filtered, line)
		}
	}
	newBody = strings.Join(filtered, "\n")

	// Append before the bean ID comment
	if idx := strings.Index(newBody, "<!-- bean:"); idx >= 0 {
		newBody = newBody[:idx] + blockingLine + "\n\n" + newBody[idx:]
	} else {
		newBody = newBody + "\n\n" + blockingLine
	}

	if newBody != ghIssue.Body {
		_, err := s.client.UpdateIssue(ctx, ghNumber, &UpdateIssueRequest{Body: &newBody})
		return err
	}

	return nil
}

// FilterIssuesNeedingSync returns only issues that need to be synced based on timestamps.
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

func slicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
