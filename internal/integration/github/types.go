// Package github provides GitHub Issues API integration.
package github

// IssueType represents a GitHub issue type.
type IssueType struct {
	Name string `json:"name"`
}

// Issue represents a GitHub issue.
type Issue struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"` // "open" or "closed"
	HTMLURL   string     `json:"html_url"`
	Labels    []Label    `json:"labels"`
	Assignees []User     `json:"assignees"`
	Type      *IssueType `json:"type,omitempty"`
}

// Label represents a GitHub label.
type Label struct {
	Name  string `json:"name"`
	Color string `json:"color,omitempty"`
}

// User represents a GitHub user.
type User struct {
	Login string `json:"login"`
	ID    int    `json:"id"`
}

// Repo represents a GitHub repository.
type Repo struct {
	FullName string `json:"full_name"`
	Private  bool   `json:"private"`
	HTMLURL  string `json:"html_url"`
}

// CreateIssueRequest is the request body for creating an issue.
type CreateIssueRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	Type      string   `json:"type,omitempty"`
}

// UpdateIssueRequest is the request body for updating an issue.
type UpdateIssueRequest struct {
	Title     *string  `json:"title,omitempty"`
	Body      *string  `json:"body,omitempty"`
	State     *string  `json:"state,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	Assignees []string `json:"assignees,omitempty"`
	Type      *string  `json:"type,omitempty"`
}

// hasChanges returns true if any field in the update request is set.
func (u *UpdateIssueRequest) hasChanges() bool {
	return u.Title != nil ||
		u.Body != nil ||
		u.State != nil ||
		u.Labels != nil ||
		u.Assignees != nil ||
		u.Type != nil
}

// SubIssueRequest is the request body for adding a sub-issue.
type SubIssueRequest struct {
	SubIssueID int `json:"sub_issue_id"`
}

// SyncResult holds the result of syncing a single issue.
type SyncResult struct {
	IssueID     string
	IssueTitle  string
	ExternalID  string // GitHub issue number as string
	ExternalURL string // GitHub issue HTML URL
	Action      string // Matches integration.Action* constants
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

// errorResponse represents a GitHub API error.
type errorResponse struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url,omitempty"`
}
