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

// DefaultStatusMapping maps issue statuses to GitHub issue states.
var DefaultStatusMapping = map[string]string{
	"draft":       "open",
	"ready":       "open",
	"in-progress": "open",
	"completed":   "closed",
	"scrapped":    "closed",
}

// DefaultTypeMapping maps issue types to GitHub native issue type names.
var DefaultTypeMapping = map[string]string{
	"bug":       "Bug",
	"feature":   "Feature",
	"task":      "Task",
	"milestone": "Task",
	"epic":      "Task",
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
