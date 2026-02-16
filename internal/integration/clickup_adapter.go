package integration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/integration/clickup"
	"github.com/toba/todo/internal/issue"
	"golang.org/x/sync/errgroup"
)

// clickUpIntegration implements Integration for ClickUp.
type clickUpIntegration struct {
	cfg  *clickup.Config
	core *core.Core
}

func newClickUpIntegration(cfg *clickup.Config, c *core.Core) *clickUpIntegration {
	return &clickUpIntegration{cfg: cfg, core: c}
}

// detectClickUp checks if ClickUp config is valid and returns the integration.
func detectClickUp(cfgMap map[string]any, c *core.Core) (Integration, error) {
	cfg, err := clickup.ParseConfig(cfgMap)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return newClickUpIntegration(cfg, c), nil
	}
	return nil, nil
}

func (cu *clickUpIntegration) Name() string { return "clickup" }

func (cu *clickUpIntegration) getToken() (string, error) {
	token := os.Getenv("CLICKUP_TOKEN")
	if token == "" {
		return "", fmt.Errorf("CLICKUP_TOKEN environment variable not set")
	}
	return token, nil
}

func (cu *clickUpIntegration) Sync(ctx context.Context, issues []*issue.Issue, opts SyncOptions) ([]SyncResult, error) {
	token, err := cu.getToken()
	if err != nil {
		return nil, err
	}

	client := clickup.NewClient(token)

	// Create sync state provider from issue sync metadata
	syncProvider := clickup.NewSyncStateStore(cu.core, issues)

	// Filter issues based on sync filter config
	filtered := clickup.FilterIssuesForSync(issues, cu.cfg.SyncFilter)

	// Pre-filter to issues that actually need syncing
	toSync := clickup.FilterIssuesNeedingSync(filtered, syncProvider, opts.Force)
	if len(toSync) == 0 {
		return nil, nil
	}

	// Convert integration progress callback to clickup progress callback
	var clickupProgress clickup.ProgressFunc
	if opts.OnProgress != nil {
		clickupProgress = func(result clickup.SyncResult, completed, total int) {
			opts.OnProgress(convertClickUpResult(result), completed, total)
		}
	}

	// Create syncer
	syncOpts := clickup.SyncOptions{
		DryRun:          opts.DryRun,
		Force:           opts.Force,
		NoRelationships: opts.NoRelationships,
		ListID:          cu.cfg.ListID,
		OnProgress:      clickupProgress,
	}

	syncer := clickup.NewSyncer(client, cu.cfg, syncOpts, cu.core, syncProvider)

	// Run sync
	clickupResults, err := syncer.SyncIssues(ctx, toSync)
	if err != nil {
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	// Convert results
	results := make([]SyncResult, len(clickupResults))
	for i, r := range clickupResults {
		results[i] = convertClickUpResult(r)
	}

	// Flush sync state to issue sync metadata
	if !opts.DryRun {
		if flushErr := syncProvider.Flush(); flushErr != nil {
			return results, fmt.Errorf("saving sync state: %w", flushErr)
		}
	}

	return results, nil
}

// convertClickUpResult converts a clickup.SyncResult to an integration.SyncResult.
func convertClickUpResult(r clickup.SyncResult) SyncResult {
	return SyncResult{
		IssueID:     r.IssueID,
		IssueTitle:  r.IssueTitle,
		ExternalID:  r.TaskID,
		ExternalURL: r.TaskURL,
		Action:      r.Action,
		Error:       r.Error,
	}
}

func (cu *clickUpIntegration) Link(ctx context.Context, issueID, taskID string) (*LinkResult, error) {
	b, err := cu.core.Get(issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %s", issueID)
	}

	// Check if already linked to this task
	existingTaskID := clickup.GetSyncString(b, clickup.SyncKeyTaskID)
	if existingTaskID == taskID {
		return &LinkResult{Action: ActionAlreadyLinked, ExternalID: taskID}, nil
	}

	// Try to verify the task exists if we have a token
	token, tokenErr := cu.getToken()
	if tokenErr == nil {
		client := clickup.NewClient(token)
		if _, err := client.GetTask(ctx, taskID); err != nil {
			// Warn but don't fail
			fmt.Fprintf(os.Stderr, "Warning: Could not verify task %s: %v\n", taskID, err)
		}
	}

	// Set extension data on the issue
	data := map[string]any{
		clickup.SyncKeyTaskID:   taskID,
		clickup.SyncKeySyncedAt: time.Now().UTC().Format(time.RFC3339),
	}
	b.SetSync(clickup.SyncName, data)
	if err := cu.core.SaveSyncOnly(b, nil); err != nil {
		return nil, err
	}
	return &LinkResult{Action: ActionLinked, ExternalID: taskID}, nil
}

func (cu *clickUpIntegration) Unlink(ctx context.Context, issueID string) (*UnlinkResult, error) {
	b, err := cu.core.Get(issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %s", issueID)
	}

	// Check if linked
	taskID := clickup.GetSyncString(b, clickup.SyncKeyTaskID)
	if taskID == "" {
		return &UnlinkResult{Action: ActionNotLinked}, nil
	}

	b.RemoveSync(clickup.SyncName)
	if err := cu.core.SaveSyncOnly(b, nil); err != nil {
		return nil, err
	}
	return &UnlinkResult{Action: ActionUnlinked, ExternalID: taskID}, nil
}

func (cu *clickUpIntegration) Check(ctx context.Context, opts CheckOptions) (*CheckReport, error) {
	report := &CheckReport{
		Sections: make([]CheckSection, 0, 3),
	}

	// Configuration section
	configSection := cu.checkConfiguration(ctx, opts)
	report.Sections = append(report.Sections, configSection)

	// ClickUp Integration section
	integrationSection := cu.checkClickUpIntegration(ctx, opts)
	report.Sections = append(report.Sections, integrationSection)

	// Sync State section
	syncSection := cu.checkSyncState(ctx, opts)
	report.Sections = append(report.Sections, syncSection)

	// Calculate summary
	for _, section := range report.Sections {
		for _, check := range section.Checks {
			switch check.Status {
			case CheckPass:
				report.Summary.Passed++
			case CheckWarn:
				report.Summary.Warnings++
			case CheckFail:
				report.Summary.Failed++
			}
		}
	}

	return report, nil
}

func (cu *clickUpIntegration) checkConfiguration(ctx context.Context, opts CheckOptions) CheckSection {
	section := CheckSection{
		Name:   "Configuration",
		Checks: make([]CheckResult, 0),
	}

	// Check list_id
	if cu.cfg.ListID == "" {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "List ID configured",
			Status:  CheckFail,
			Message: "list_id is not set",
		})
	} else {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "List ID configured",
			Status:  CheckPass,
			Message: cu.cfg.ListID,
		})
	}

	// Check list accessibility (requires API)
	if !opts.SkipAPI && cu.cfg.ListID != "" {
		token, _ := cu.getToken()
		if token != "" {
			client := clickup.NewClient(token)
			list, err := client.GetList(ctx, cu.cfg.ListID)
			if err != nil {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "List accessible",
					Status:  CheckFail,
					Message: fmt.Sprintf("Cannot access list: %v", err),
				})
			} else {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "List accessible",
					Status:  CheckPass,
					Message: list.Name,
				})

				// Check status mapping against list statuses
				section.Checks = append(section.Checks, cu.checkStatusMapping(list)...)

				// Check custom fields if configured
				if cu.cfg.CustomFields != nil {
					section.Checks = append(section.Checks, cu.checkCustomFields(ctx, client)...)
				} else {
					section.Checks = append(section.Checks, CheckResult{
						Name:    "Custom fields configured",
						Status:  CheckWarn,
						Message: "Not configured",
					})
				}
			}
		}
	}

	// Check status mapping keys are valid project statuses
	section.Checks = append(section.Checks, cu.checkMappingKeys()...)

	// Check priority mapping
	priorityMapping := cu.cfg.GetPriorityMapping()
	var invalidPriorities []string
	for beanPriority, clickupPriority := range priorityMapping {
		if clickupPriority < 1 || clickupPriority > 4 {
			invalidPriorities = append(invalidPriorities, fmt.Sprintf("%s=%d", beanPriority, clickupPriority))
		}
	}
	if len(invalidPriorities) > 0 {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Priority mapping valid",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Invalid priorities (must be 1-4): %v", invalidPriorities),
		})
	} else {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Priority mapping valid",
			Status:  CheckPass,
			Message: fmt.Sprintf("%d mappings", len(priorityMapping)),
		})
	}

	// Check type mapping
	if len(cu.cfg.TypeMapping) > 0 {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Type mapping configured",
			Status:  CheckPass,
			Message: fmt.Sprintf("%d mappings", len(cu.cfg.TypeMapping)),
		})
	} else {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Type mapping configured",
			Status:  CheckWarn,
			Message: "Not configured (issue types won't map to ClickUp task types)",
		})
	}

	return section
}

func (cu *clickUpIntegration) checkMappingKeys() []CheckResult {
	var results []CheckResult
	projectCfg := cu.core.Config()

	// Check status mapping keys are valid project statuses
	statusMapping := cu.cfg.GetStatusMapping()
	var unknownStatuses []string
	for key := range statusMapping {
		if !projectCfg.IsValidStatus(key) {
			unknownStatuses = append(unknownStatuses, key)
		}
	}
	if len(unknownStatuses) > 0 {
		results = append(results, CheckResult{
			Name:    "Status mapping keys valid",
			Status:  CheckFail,
			Message: fmt.Sprintf("Unknown statuses in mapping: %v (valid: %s)", unknownStatuses, validStatusList(projectCfg)),
		})
	}

	// Check all project statuses have a mapping
	var unmappedStatuses []string
	for _, s := range config.DefaultStatuses {
		if _, ok := statusMapping[s.Name]; !ok {
			unmappedStatuses = append(unmappedStatuses, s.Name)
		}
	}
	if len(unmappedStatuses) > 0 {
		results = append(results, CheckResult{
			Name:    "Status mapping coverage",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Unmapped statuses (will use defaults or fail): %v", unmappedStatuses),
		})
	}

	if len(unknownStatuses) == 0 && len(unmappedStatuses) == 0 {
		results = append(results, CheckResult{
			Name:    "Status mapping keys valid",
			Status:  CheckPass,
			Message: fmt.Sprintf("All %d statuses mapped", len(statusMapping)),
		})
	}

	// Check type mapping keys are valid project types
	if len(cu.cfg.TypeMapping) > 0 {
		var unknownTypes []string
		for key := range cu.cfg.TypeMapping {
			if !projectCfg.IsValidType(key) {
				unknownTypes = append(unknownTypes, key)
			}
		}
		if len(unknownTypes) > 0 {
			results = append(results, CheckResult{
				Name:    "Type mapping keys valid",
				Status:  CheckWarn,
				Message: fmt.Sprintf("Unknown types in mapping: %v", unknownTypes),
			})
		}
	}

	// Check priority mapping keys are valid project priorities
	priorityMapping := cu.cfg.GetPriorityMapping()
	var unknownPriorities []string
	for key := range priorityMapping {
		if !projectCfg.IsValidPriority(key) {
			unknownPriorities = append(unknownPriorities, key)
		}
	}
	if len(unknownPriorities) > 0 {
		results = append(results, CheckResult{
			Name:    "Priority mapping keys valid",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Unknown priorities in mapping: %v", unknownPriorities),
		})
	}

	// Check sync filter exclude_status values are valid
	if cu.cfg.SyncFilter != nil {
		var unknownFilterStatuses []string
		for _, s := range cu.cfg.SyncFilter.ExcludeStatus {
			if !projectCfg.IsValidStatus(s) {
				unknownFilterStatuses = append(unknownFilterStatuses, s)
			}
		}
		if len(unknownFilterStatuses) > 0 {
			results = append(results, CheckResult{
				Name:    "Sync filter statuses valid",
				Status:  CheckWarn,
				Message: fmt.Sprintf("Unknown statuses in exclude filter: %v", unknownFilterStatuses),
			})
		}
	}

	return results
}

func validStatusList(cfg *config.Config) string {
	names := make([]string, len(config.DefaultStatuses))
	for i, s := range config.DefaultStatuses {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

func (cu *clickUpIntegration) checkStatusMapping(list *clickup.List) []CheckResult {
	var results []CheckResult

	statusMapping := cu.cfg.GetStatusMapping()
	if len(statusMapping) == 0 {
		results = append(results, CheckResult{
			Name:    "Status mapping valid",
			Status:  CheckWarn,
			Message: "No status mapping configured (using defaults)",
		})
		return results
	}

	// Build set of valid ClickUp statuses
	validStatuses := make(map[string]bool)
	for _, s := range list.Statuses {
		validStatuses[s.Status] = true
	}

	// Check each mapping
	var invalidMappings []string
	for beanStatus, clickupStatus := range statusMapping {
		if !validStatuses[clickupStatus] {
			invalidMappings = append(invalidMappings, fmt.Sprintf("%s\u2192%s", beanStatus, clickupStatus))
		}
	}

	if len(invalidMappings) > 0 {
		results = append(results, CheckResult{
			Name:    "Status mapping valid",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Unknown statuses: %v", invalidMappings),
		})
	} else {
		results = append(results, CheckResult{
			Name:    "Status mapping valid",
			Status:  CheckPass,
			Message: fmt.Sprintf("%d mappings", len(statusMapping)),
		})
	}

	return results
}

func (cu *clickUpIntegration) checkCustomFields(ctx context.Context, client *clickup.Client) []CheckResult {
	var results []CheckResult

	fields, err := client.GetAccessibleCustomFields(ctx, cu.cfg.ListID)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "Custom fields valid",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Cannot fetch fields: %v", err),
		})
		return results
	}

	// Build set of valid field IDs
	validFields := make(map[string]string) // ID -> name
	for _, f := range fields {
		validFields[f.ID] = f.Name
	}

	// Check configured fields
	cf := cu.cfg.CustomFields
	var invalidFields []string

	if cf.BeanID != "" {
		if _, ok := validFields[cf.BeanID]; !ok {
			invalidFields = append(invalidFields, "bean_id")
		}
	}
	if cf.CreatedAt != "" {
		if _, ok := validFields[cf.CreatedAt]; !ok {
			invalidFields = append(invalidFields, "created_at")
		}
	}
	if cf.UpdatedAt != "" {
		if _, ok := validFields[cf.UpdatedAt]; !ok {
			invalidFields = append(invalidFields, "updated_at")
		}
	}

	if len(invalidFields) > 0 {
		results = append(results, CheckResult{
			Name:    "Custom fields valid",
			Status:  CheckWarn,
			Message: fmt.Sprintf("Unknown field UUIDs: %v", invalidFields),
		})
	} else {
		configuredCount := 0
		if cf.BeanID != "" {
			configuredCount++
		}
		if cf.CreatedAt != "" {
			configuredCount++
		}
		if cf.UpdatedAt != "" {
			configuredCount++
		}
		results = append(results, CheckResult{
			Name:    "Custom fields valid",
			Status:  CheckPass,
			Message: fmt.Sprintf("%d fields configured", configuredCount),
		})
	}

	return results
}

func (cu *clickUpIntegration) checkClickUpIntegration(ctx context.Context, opts CheckOptions) CheckSection {
	section := CheckSection{
		Name:   "ClickUp Integration",
		Checks: make([]CheckResult, 0),
	}

	// Check CLICKUP_TOKEN
	token, err := cu.getToken()
	if err != nil {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "CLICKUP_TOKEN set",
			Status:  CheckFail,
			Message: "Environment variable not set",
		})
		return section
	}

	section.Checks = append(section.Checks, CheckResult{
		Name:    "CLICKUP_TOKEN set",
		Status:  CheckPass,
		Message: "Set",
	})

	if opts.SkipAPI {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "API token valid",
			Status:  CheckWarn,
			Message: "Skipped (--skip-api)",
		})
		return section
	}

	// Validate token by fetching authorized user
	client := clickup.NewClient(token)
	user, err := client.GetAuthorizedUser(ctx)
	if err != nil {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "API token valid",
			Status:  CheckFail,
			Message: fmt.Sprintf("Invalid token: %v", err),
		})
		return section
	}

	section.Checks = append(section.Checks, CheckResult{
		Name:    "API token valid",
		Status:  CheckPass,
		Message: user.Email,
	})

	return section
}

func (cu *clickUpIntegration) checkSyncState(ctx context.Context, opts CheckOptions) CheckSection {
	section := CheckSection{
		Name:   "Sync State",
		Checks: make([]CheckResult, 0),
	}

	// Load all issues and check sync metadata
	allIssues := cu.core.All()

	// Count linked issues
	linkedCount := 0
	var linkedIssues []*issue.Issue
	for _, b := range allIssues {
		if clickup.GetSyncString(b, clickup.SyncKeyTaskID) != "" {
			linkedCount++
			linkedIssues = append(linkedIssues, b)
		}
	}

	section.Checks = append(section.Checks, CheckResult{
		Name:    "Issues linked",
		Status:  CheckPass,
		Message: fmt.Sprintf("%d issues", linkedCount),
	})

	if linkedCount == 0 {
		return section
	}

	// Check for stale syncs (>7 days)
	staleThreshold := time.Now().AddDate(0, 0, -7)
	staleCount := 0
	for _, b := range linkedIssues {
		syncedAt := clickup.GetSyncTime(b, clickup.SyncKeySyncedAt)
		if syncedAt != nil && syncedAt.Before(staleThreshold) {
			staleCount++
		}
	}

	if staleCount > 0 {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Stale syncs",
			Status:  CheckWarn,
			Message: fmt.Sprintf("%d issues have stale sync (>7 days)", staleCount),
		})
	}

	// Verify linked tasks exist (if API is available)
	if !opts.SkipAPI {
		token, _ := cu.getToken()
		if token != "" {
			client := clickup.NewClient(token)
			var missingCount atomic.Int64
			var mu sync.Mutex

			g, gctx := errgroup.WithContext(ctx)
			g.SetLimit(5)

			for _, b := range linkedIssues {
				g.Go(func() error {
					taskID := clickup.GetSyncString(b, clickup.SyncKeyTaskID)
					_, err := client.GetTask(gctx, taskID)
					if err != nil {
						cur := missingCount.Add(1)
						// Only report first few missing for brevity
						if cur <= 3 {
							mu.Lock()
							section.Checks = append(section.Checks, CheckResult{
								Name:    "Task exists",
								Status:  CheckWarn,
								Message: fmt.Sprintf("%s \u2192 %s: not found", b.ID, taskID),
							})
							mu.Unlock()
						}
					}
					return nil
				})
			}
			_ = g.Wait()

			if missingCount.Load() == 0 {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "All linked tasks exist",
					Status:  CheckPass,
					Message: fmt.Sprintf("Verified %d tasks", linkedCount),
				})
			} else if missingCount.Load() > 3 {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "Missing tasks",
					Status:  CheckWarn,
					Message: fmt.Sprintf("...and %d more", missingCount.Load()-3),
				})
			}
		}
	}

	return section
}
