package github

import (
	"fmt"
	"strings"
)

// Sync metadata constants for GitHub.
const (
	SyncName          = "github"
	SyncKeyIssueNumber = "issue_number"
	SyncKeySyncedAt    = "synced_at"
)

// Config holds GitHub integration configuration.
type Config struct {
	Owner string // Repository owner
	Repo  string // Repository name
}

// StatusTarget maps an issue status to a GitHub issue state and optional label.
type StatusTarget struct {
	State string // "open" or "closed"
	Label string // e.g. "status:draft", or empty
}

// DefaultStatusMapping provides standard issue-to-GitHub status mapping.
var DefaultStatusMapping = map[string]StatusTarget{
	"draft":       {State: "open", Label: "status:draft"},
	"todo":        {State: "open", Label: "status:todo"},
	"in-progress": {State: "open", Label: "status:in-progress"},
	"completed":   {State: "closed"},
	"scrapped":    {State: "closed", Label: "status:scrapped"},
}

// DefaultPriorityMapping provides standard issue priority-to-GitHub label mapping.
var DefaultPriorityMapping = map[string]string{
	"critical": "priority:critical",
	"high":     "priority:high",
	"normal":   "priority:normal",
	"low":      "priority:low",
	"deferred": "priority:low",
}

// DefaultTypeMapping provides standard issue type-to-GitHub label mapping.
var DefaultTypeMapping = map[string]string{
	"bug":       "type:bug",
	"feature":   "type:feature",
	"milestone": "type:milestone",
	"epic":      "type:epic",
	"task":      "type:task",
}

// ParseConfig parses GitHub config from a sync config map.
// Returns nil, nil if the config has no repo set.
func ParseConfig(cfgMap map[string]any) (*Config, error) {
	if cfgMap == nil {
		return nil, nil
	}

	repoVal, ok := cfgMap["repo"]
	if !ok {
		return nil, nil
	}

	repoStr, ok := repoVal.(string)
	if !ok || repoStr == "" {
		return nil, nil
	}

	owner, repo, err := ParseRepo(repoStr)
	if err != nil {
		return nil, err
	}

	return &Config{
		Owner: owner,
		Repo:  repo,
	}, nil
}

// ParseRepo splits a "owner/repo" string into owner and repo.
func ParseRepo(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo format %q: expected owner/repo", repo)
	}
	return parts[0], parts[1], nil
}
