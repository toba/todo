package integration

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/integration/github"
	"github.com/toba/todo/internal/issue"
)

// gitHubIntegration implements Integration for GitHub Issues.
type gitHubIntegration struct {
	cfg  *github.Config
	core *core.Core
}

func newGitHubIntegration(cfg *github.Config, c *core.Core) *gitHubIntegration {
	return &gitHubIntegration{cfg: cfg, core: c}
}

// detectGitHub checks if GitHub config is valid and returns the integration.
func detectGitHub(cfgMap map[string]any, c *core.Core) (Integration, error) {
	cfg, err := github.ParseConfig(cfgMap)
	if err != nil {
		return nil, err
	}
	if cfg != nil {
		return newGitHubIntegration(cfg, c), nil
	}
	return nil, nil
}

func (gh *gitHubIntegration) Name() string { return "github" }

func (gh *gitHubIntegration) getToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}
	return token, nil
}

func (gh *gitHubIntegration) Sync(ctx context.Context, issues []*issue.Issue, opts SyncOptions) ([]SyncResult, error) {
	token, err := gh.getToken()
	if err != nil {
		return nil, err
	}

	client := github.NewClient(token, gh.cfg.Owner, gh.cfg.Repo)

	// Create sync state provider from issue extension metadata
	syncProvider := github.NewExtensionSyncProvider(gh.core, issues)

	// Pre-filter to issues that actually need syncing
	toSync := github.FilterIssuesNeedingSync(issues, syncProvider, opts.Force)
	if len(toSync) == 0 {
		return nil, nil
	}

	// Convert integration progress callback to github progress callback
	var ghProgress github.ProgressFunc
	if opts.OnProgress != nil {
		ghProgress = func(result github.SyncResult, completed, total int) {
			opts.OnProgress(convertGitHubResult(result), completed, total)
		}
	}

	// Create syncer
	syncOpts := github.SyncOptions{
		DryRun:          opts.DryRun,
		Force:           opts.Force,
		NoRelationships: opts.NoRelationships,
		OnProgress:      ghProgress,
	}

	syncer := github.NewSyncer(client, gh.cfg, syncOpts, gh.core, syncProvider)

	// Run sync
	ghResults, err := syncer.SyncIssues(ctx, toSync)
	if err != nil {
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	// Convert results
	results := make([]SyncResult, len(ghResults))
	for i, r := range ghResults {
		results[i] = convertGitHubResult(r)
	}

	// Flush sync state to issue extension metadata
	if !opts.DryRun {
		if flushErr := syncProvider.Flush(); flushErr != nil {
			return results, fmt.Errorf("saving sync state: %w", flushErr)
		}
	}

	return results, nil
}

// convertGitHubResult converts a github.SyncResult to an integration.SyncResult.
func convertGitHubResult(r github.SyncResult) SyncResult {
	return SyncResult{
		IssueID:     r.IssueID,
		IssueTitle:  r.IssueTitle,
		ExternalID:  r.ExternalID,
		ExternalURL: r.ExternalURL,
		Action:      r.Action,
		Error:       r.Error,
	}
}

func (gh *gitHubIntegration) Link(ctx context.Context, issueID, externalID string) (*LinkResult, error) {
	b, err := gh.core.Get(issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %s", issueID)
	}

	// Check if already linked to this issue number
	existingNumber := github.GetExtensionString(b, github.ExtKeyIssueNumber)
	if existingNumber == externalID {
		return &LinkResult{Action: "already_linked", ExternalID: externalID}, nil
	}

	// Try to verify the issue exists if we have a token
	token, tokenErr := gh.getToken()
	if tokenErr == nil {
		client := github.NewClient(token, gh.cfg.Owner, gh.cfg.Repo)
		var issueNumber int
		if _, err := fmt.Sscanf(externalID, "%d", &issueNumber); err == nil {
			if _, err := client.GetIssue(ctx, issueNumber); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Could not verify issue #%s: %v\n", externalID, err)
			}
		}
	}

	// Set extension data on the issue
	data := map[string]any{
		github.ExtKeyIssueNumber: externalID,
		github.ExtKeySyncedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	b.SetExtension(github.ExtensionName, data)
	if err := gh.core.SaveExtensionOnly(b, nil); err != nil {
		return nil, err
	}
	return &LinkResult{Action: "linked", ExternalID: externalID}, nil
}

func (gh *gitHubIntegration) Unlink(ctx context.Context, issueID string) (*UnlinkResult, error) {
	b, err := gh.core.Get(issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %s", issueID)
	}

	// Check if linked
	issueNumber := github.GetExtensionString(b, github.ExtKeyIssueNumber)
	if issueNumber == "" {
		return &UnlinkResult{Action: "not_linked"}, nil
	}

	b.RemoveExtension(github.ExtensionName)
	if err := gh.core.SaveExtensionOnly(b, nil); err != nil {
		return nil, err
	}
	return &UnlinkResult{Action: "unlinked", ExternalID: issueNumber}, nil
}

func (gh *gitHubIntegration) Check(ctx context.Context, opts CheckOptions) (*CheckReport, error) {
	report := &CheckReport{
		Sections: make([]CheckSection, 0, 3),
	}

	// Configuration section
	configSection := gh.checkConfiguration(ctx, opts)
	report.Sections = append(report.Sections, configSection)

	// GitHub Integration section
	integrationSection := gh.checkGitHubIntegration(ctx, opts)
	report.Sections = append(report.Sections, integrationSection)

	// Sync State section
	syncSection := gh.checkSyncState(ctx, opts)
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

func (gh *gitHubIntegration) checkConfiguration(ctx context.Context, opts CheckOptions) CheckSection {
	section := CheckSection{
		Name:   "Configuration",
		Checks: make([]CheckResult, 0),
	}

	// Check repo
	if gh.cfg.Owner == "" || gh.cfg.Repo == "" {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Repository configured",
			Status:  CheckFail,
			Message: "repo is not set or invalid (expected owner/repo)",
		})
	} else {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "Repository configured",
			Status:  CheckPass,
			Message: fmt.Sprintf("%s/%s", gh.cfg.Owner, gh.cfg.Repo),
		})
	}

	// Check repo accessibility (requires API)
	if !opts.SkipAPI && gh.cfg.Owner != "" && gh.cfg.Repo != "" {
		token, _ := gh.getToken()
		if token != "" {
			client := github.NewClient(token, gh.cfg.Owner, gh.cfg.Repo)
			repo, err := client.GetRepo(ctx)
			if err != nil {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "Repository accessible",
					Status:  CheckFail,
					Message: fmt.Sprintf("Cannot access repository: %v", err),
				})
			} else {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "Repository accessible",
					Status:  CheckPass,
					Message: repo.FullName,
				})
			}
		}
	}

	return section
}

func (gh *gitHubIntegration) checkGitHubIntegration(ctx context.Context, opts CheckOptions) CheckSection {
	section := CheckSection{
		Name:   "GitHub Integration",
		Checks: make([]CheckResult, 0),
	}

	// Check GITHUB_TOKEN
	token, err := gh.getToken()
	if err != nil {
		section.Checks = append(section.Checks, CheckResult{
			Name:    "GITHUB_TOKEN set",
			Status:  CheckFail,
			Message: "Environment variable not set",
		})
		return section
	}

	section.Checks = append(section.Checks, CheckResult{
		Name:    "GITHUB_TOKEN set",
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

	// Validate token by fetching authenticated user
	client := github.NewClient(token, gh.cfg.Owner, gh.cfg.Repo)
	user, err := client.GetAuthenticatedUser(ctx)
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
		Message: user.Login,
	})

	return section
}

func (gh *gitHubIntegration) checkSyncState(ctx context.Context, opts CheckOptions) CheckSection {
	section := CheckSection{
		Name:   "Sync State",
		Checks: make([]CheckResult, 0),
	}

	// Load all issues and check extension metadata
	allIssues := gh.core.All()

	// Count linked issues
	linkedCount := 0
	var linkedIssues []*issue.Issue
	for _, b := range allIssues {
		if github.GetExtensionString(b, github.ExtKeyIssueNumber) != "" {
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
		syncedAt := github.GetExtensionTime(b, github.ExtKeySyncedAt)
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

	// Verify linked issues exist (if API is available)
	if !opts.SkipAPI {
		token, _ := gh.getToken()
		if token != "" {
			client := github.NewClient(token, gh.cfg.Owner, gh.cfg.Repo)
			missingCount := 0

			for _, b := range linkedIssues {
				numberStr := github.GetExtensionString(b, github.ExtKeyIssueNumber)
				var number int
				if _, err := fmt.Sscanf(numberStr, "%d", &number); err != nil {
					continue
				}
				_, err := client.GetIssue(ctx, number)
				if err != nil {
					missingCount++
					if missingCount <= 3 {
						section.Checks = append(section.Checks, CheckResult{
							Name:    "Issue exists",
							Status:  CheckWarn,
							Message: fmt.Sprintf("%s → #%d: not found", b.ID, number),
						})
					}
				}
			}

			if missingCount == 0 {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "All linked issues exist",
					Status:  CheckPass,
					Message: fmt.Sprintf("Verified %d issues", linkedCount),
				})
			} else if missingCount > 3 {
				section.Checks = append(section.Checks, CheckResult{
					Name:    "Missing issues",
					Status:  CheckWarn,
					Message: fmt.Sprintf("...and %d more", missingCount-3),
				})
			}
		}
	}

	return section
}
