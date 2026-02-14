package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const baseURL = "https://api.github.com"

// Default retry configuration for rate limit handling
const (
	defaultMaxRetries     = 5
	defaultBaseRetryDelay = 1 * time.Second
	defaultMaxRetryDelay  = 30 * time.Second
)

// RateLimitError represents a GitHub rate limit error.
type RateLimitError struct {
	Message    string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit: %s (retry after %v)", e.Message, e.RetryAfter)
}

// TransientError represents a transient error that can be retried.
type TransientError struct {
	Message string
}

func (e *TransientError) Error() string {
	return fmt.Sprintf("transient error: %s", e.Message)
}

// RetryConfig holds retry settings for rate limit handling.
type RetryConfig struct {
	MaxRetries     int
	BaseRetryDelay time.Duration
	MaxRetryDelay  time.Duration
}

// Client provides GitHub API access via REST.
type Client struct {
	token      string
	owner      string
	repo       string
	httpClient *http.Client

	// Retry configuration (uses defaults if nil)
	retryConfig *RetryConfig

	// Cached authenticated user
	authenticatedUser *User
	// Cached labels (label name -> true)
	labelCache map[string]bool
}

func (c *Client) getRetryConfig() RetryConfig {
	if c.retryConfig != nil {
		return *c.retryConfig
	}
	return RetryConfig{
		MaxRetries:     defaultMaxRetries,
		BaseRetryDelay: defaultBaseRetryDelay,
		MaxRetryDelay:  defaultMaxRetryDelay,
	}
}

// NewClient creates a new GitHub client.
func NewClient(token, owner, repo string) *Client {
	return &Client{
		token:      token,
		owner:      owner,
		repo:       repo,
		httpClient: &http.Client{},
	}
}

// GetRepo fetches repository metadata.
func (c *Client) GetRepo(ctx context.Context) (*Repo, error) {
	url := fmt.Sprintf("%s/repos/%s/%s", baseURL, c.owner, c.repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var resp Repo
	if err := c.doRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("getting repo: %w", err)
	}

	return &resp, nil
}

// GetIssue fetches an issue by number.
func (c *Client) GetIssue(ctx context.Context, number int) (*Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d", baseURL, c.owner, c.repo, number)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var resp Issue
	if err := c.doRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("getting issue: %w", err)
	}

	return &resp, nil
}

// CreateIssue creates a new issue.
func (c *Client) CreateIssue(ctx context.Context, issue *CreateIssueRequest) (*Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues", baseURL, c.owner, c.repo)

	body, err := json.Marshal(issue)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp Issue
	if err := c.doRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("creating issue: %w", err)
	}

	return &resp, nil
}

// UpdateIssue updates an existing issue.
func (c *Client) UpdateIssue(ctx context.Context, number int, update *UpdateIssueRequest) (*Issue, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d", baseURL, c.owner, c.repo, number)

	body, err := json.Marshal(update)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp Issue
	if err := c.doRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("updating issue: %w", err)
	}

	return &resp, nil
}

// GetAuthenticatedUser fetches the user associated with the API token.
// Results are cached for the lifetime of the client.
func (c *Client) GetAuthenticatedUser(ctx context.Context) (*User, error) {
	if c.authenticatedUser != nil {
		return c.authenticatedUser, nil
	}

	url := fmt.Sprintf("%s/user", baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	var resp User
	if err := c.doRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("getting authenticated user: %w", err)
	}

	c.authenticatedUser = &resp
	return c.authenticatedUser, nil
}

// ListLabels fetches all labels for the repository.
func (c *Client) ListLabels(ctx context.Context) ([]Label, error) {
	var allLabels []Label
	page := 1

	for {
		url := fmt.Sprintf("%s/repos/%s/%s/labels?per_page=100&page=%d", baseURL, c.owner, c.repo, page)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}

		var labels []Label
		if err := c.doRequest(req, &labels); err != nil {
			return nil, fmt.Errorf("listing labels: %w", err)
		}

		allLabels = append(allLabels, labels...)
		if len(labels) < 100 {
			break
		}
		page++
	}

	return allLabels, nil
}

// CreateLabel creates a new label in the repository.
func (c *Client) CreateLabel(ctx context.Context, name, color string) (*Label, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/labels", baseURL, c.owner, c.repo)

	body, err := json.Marshal(map[string]string{"name": name, "color": color})
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp Label
	if err := c.doRequest(req, &resp); err != nil {
		return nil, fmt.Errorf("creating label: %w", err)
	}

	if c.labelCache != nil {
		c.labelCache[name] = true
	}

	return &resp, nil
}

// PopulateLabelCache fetches all labels and caches them.
func (c *Client) PopulateLabelCache(ctx context.Context) error {
	labels, err := c.ListLabels(ctx)
	if err != nil {
		return err
	}

	c.labelCache = make(map[string]bool, len(labels))
	for _, l := range labels {
		c.labelCache[l.Name] = true
	}

	return nil
}

// EnsureLabel creates a label if it doesn't exist in the cache.
func (c *Client) EnsureLabel(ctx context.Context, name, color string) error {
	if c.labelCache != nil && c.labelCache[name] {
		return nil
	}

	_, err := c.CreateLabel(ctx, name, color)
	if err != nil {
		// 422 means label already exists (race or cache miss)
		if strings.Contains(err.Error(), "422") || strings.Contains(err.Error(), "already_exists") {
			if c.labelCache == nil {
				c.labelCache = make(map[string]bool)
			}
			c.labelCache[name] = true
			return nil
		}
		return err
	}

	return nil
}

// AddSubIssue adds a sub-issue to a parent issue using the GitHub sub-issues API.
func (c *Client) AddSubIssue(ctx context.Context, parentNumber, childIssueID int) error {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%d/sub_issues", baseURL, c.owner, c.repo, parentNumber)

	body, err := json.Marshal(&SubIssueRequest{SubIssueID: childIssueID})
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if err := c.doRequest(req, nil); err != nil {
		return fmt.Errorf("adding sub-issue: %w", err)
	}

	return nil
}

// doRequest executes an HTTP request and decodes the response.
// It automatically retries on rate limit errors and transient errors with exponential backoff.
func (c *Client) doRequest(req *http.Request, result any) error {
	cfg := c.getRetryConfig()

	// We need to be able to retry the request, so we need to save the body
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return fmt.Errorf("reading request body: %w", err)
		}
		_ = req.Body.Close()
	}

	// Reset the body after reading so the first attempt has a valid body
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	var lastErr error
	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff and jitter
			delay := min(cfg.BaseRetryDelay*time.Duration(1<<(attempt-1)), cfg.MaxRetryDelay)
			// Add jitter (0-25% of delay)
			jitter := time.Duration(rand.Int64N(int64(delay / 4)))
			delay += jitter

			select {
			case <-req.Context().Done():
				return req.Context().Err()
			case <-time.After(delay):
			}

			// Reset the body for retry
			if bodyBytes != nil {
				req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Check for transient network errors
			if isTransientNetworkError(err) {
				lastErr = &TransientError{Message: err.Error()}
				continue // Retry
			}
			return fmt.Errorf("executing request: %w", err)
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}

		if resp.StatusCode >= 400 {
			// Check for rate limit errors
			if resp.StatusCode == 429 || (resp.StatusCode == 403 && resp.Header.Get("X-RateLimit-Remaining") == "0") {
				retryAfter := 60 * time.Second // default
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if seconds, err := strconv.Atoi(ra); err == nil {
						retryAfter = time.Duration(seconds) * time.Second
					}
				}
				lastErr = &RateLimitError{
					Message:    string(body),
					RetryAfter: retryAfter,
				}
				continue // Retry
			}

			// Check for transient HTTP errors (5xx)
			if isTransientHTTPError(resp.StatusCode, body) {
				lastErr = &TransientError{Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
				continue // Retry
			}

			var errResp errorResponse
			if err := json.Unmarshal(body, &errResp); err == nil && errResp.Message != "" {
				return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, errResp.Message)
			}

			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		if result != nil && len(body) > 0 {
			if err := json.Unmarshal(body, result); err != nil {
				return fmt.Errorf("decoding response: %w", err)
			}
		}

		return nil
	}

	// All retries exhausted
	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isTransientNetworkError checks if an error is a transient network error that should be retried.
func isTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	if strings.Contains(errStr, "stream error") || strings.Contains(errStr, "INTERNAL_ERROR") {
		return true
	}
	if strings.Contains(errStr, "connection reset") || strings.Contains(errStr, "connection refused") {
		return true
	}
	if strings.Contains(errStr, "EOF") {
		return true
	}
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "Timeout") {
		return true
	}
	return false
}

// isTransientHTTPError checks if an HTTP error is transient and should be retried.
func isTransientHTTPError(statusCode int, body []byte) bool {
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	if statusCode == 502 || statusCode == 503 || statusCode == 504 {
		bodyStr := string(body)
		if strings.Contains(bodyStr, "try again") || strings.Contains(bodyStr, "Try again") {
			return true
		}
	}
	return false
}
