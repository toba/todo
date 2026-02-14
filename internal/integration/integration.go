// Package integration defines the interface for external integrations (e.g., ClickUp, GitHub).
package integration

import (
	"context"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/integration/clickup"
	"github.com/toba/todo/internal/issue"
)

// SyncResult holds the result of syncing a single issue.
type SyncResult = clickup.SyncResult

// SyncOptions configures the sync operation.
type SyncOptions struct {
	DryRun          bool
	Force           bool
	NoRelationships bool
	OnProgress      clickup.ProgressFunc
}

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
	Link(ctx context.Context, issueID, externalID string) error
	Unlink(ctx context.Context, issueID string) error
	Check(ctx context.Context, opts CheckOptions) (*CheckReport, error)
}

// Detect checks cfg.Extensions for known integration keys and returns the appropriate integration.
// Returns nil, nil if no integration is configured.
func Detect(extensions map[string]map[string]any, c *core.Core) (Integration, error) {
	if extensions == nil {
		return nil, nil
	}

	// Check for ClickUp configuration
	if clickupCfg, ok := extensions["clickup"]; ok {
		cfg, err := clickup.ParseConfig(clickupCfg)
		if err != nil {
			return nil, err
		}
		if cfg != nil {
			return newClickUpIntegration(cfg, c), nil
		}
	}

	return nil, nil
}
