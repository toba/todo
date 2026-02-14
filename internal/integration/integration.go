// Package integration defines the interface for external integrations (e.g., ClickUp, GitHub).
package integration

import (
	"context"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/issue"
)

// SyncResult holds the result of syncing a single issue.
type SyncResult struct {
	IssueID     string // local issue ID
	IssueTitle  string // local issue title
	ExternalID  string // ClickUp task ID or GitHub issue number (as string)
	ExternalURL string // URL to the external resource
	Action      string // One of the Action* constants
	Error       error
}

// ProgressFunc is called when an issue sync completes.
type ProgressFunc func(result SyncResult, completed, total int)

// SyncOptions configures the sync operation.
type SyncOptions struct {
	DryRun          bool
	Force           bool
	NoRelationships bool
	OnProgress      ProgressFunc
}

// LinkResult holds the result of a link operation.
type LinkResult struct {
	Action     string // ActionLinked or ActionAlreadyLinked
	ExternalID string
}

// UnlinkResult holds the result of an unlink operation.
type UnlinkResult struct {
	Action     string // ActionUnlinked or ActionNotLinked
	ExternalID string // previous ID if unlinked
}

// Sync action constants identify the outcome of syncing a single issue.
const (
	ActionCreated     = "created"
	ActionUpdated     = "updated"
	ActionSkipped     = "skipped"
	ActionError       = "error"
	ActionUnchanged   = "unchanged"
	ActionWouldCreate = "would create"
	ActionWouldUpdate = "would update"
)

// Link/unlink action constants.
const (
	ActionLinked        = "linked"
	ActionAlreadyLinked = "already_linked"
	ActionUnlinked      = "unlinked"
	ActionNotLinked     = "not_linked"
)

// SyncKeySyncedAt is the common key used in sync metadata for the last sync timestamp.
const SyncKeySyncedAt = "synced_at"

// CheckStatus represents the result of a single check.
type CheckStatus string

const (
	CheckPass CheckStatus = "pass"
	CheckWarn CheckStatus = "warn"
	CheckFail CheckStatus = "fail"
)

// CheckResult holds the result of a single check.
type CheckResult struct {
	Name    string      `json:"name"`
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
}

// CheckSection groups related checks.
type CheckSection struct {
	Name   string        `json:"name"`
	Checks []CheckResult `json:"checks"`
}

// CheckReport is the full check output.
type CheckReport struct {
	Sections []CheckSection `json:"sections"`
	Summary  CheckSummary   `json:"summary"`
}

// CheckSummary summarizes the overall check results.
type CheckSummary struct {
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Failed   int `json:"failed"`
}

// CheckOptions configures the check operation.
type CheckOptions struct {
	SkipAPI bool
}

// Integration defines the interface for external issue tracker integrations.
type Integration interface {
	Name() string
	Sync(ctx context.Context, issues []*issue.Issue, opts SyncOptions) ([]SyncResult, error)
	Link(ctx context.Context, issueID, externalID string) (*LinkResult, error)
	Unlink(ctx context.Context, issueID string) (*UnlinkResult, error)
	Check(ctx context.Context, opts CheckOptions) (*CheckReport, error)
}

// Detect checks cfg.Sync for known integration keys and returns the appropriate integration.
// Returns nil, nil if no integration is configured.
func Detect(syncCfg map[string]map[string]any, c *core.Core) (Integration, error) {
	if syncCfg == nil {
		return nil, nil
	}

	// Check for ClickUp configuration
	if clickupCfg, ok := syncCfg["clickup"]; ok {
		integ, err := detectClickUp(clickupCfg, c)
		if err != nil {
			return nil, err
		}
		if integ != nil {
			return integ, nil
		}
	}

	// Check for GitHub configuration
	if githubCfg, ok := syncCfg["github"]; ok {
		integ, err := detectGitHub(githubCfg, c)
		if err != nil {
			return nil, err
		}
		if integ != nil {
			return integ, nil
		}
	}

	return nil, nil
}
