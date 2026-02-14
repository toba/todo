package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
)

// memorySyncProvider is a simple in-memory SyncStateProvider for tests.
type memorySyncProvider struct {
	mu           sync.RWMutex
	issueNumbers map[string]int
	syncedAt     map[string]*time.Time
}

func newMemorySyncProvider() *memorySyncProvider {
	return &memorySyncProvider{
		issueNumbers: make(map[string]int),
		syncedAt:     make(map[string]*time.Time),
	}
}

func (m *memorySyncProvider) GetIssueNumber(issueID string) *int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n, ok := m.issueNumbers[issueID]
	if !ok || n == 0 {
		return nil
	}
	return &n
}

func (m *memorySyncProvider) GetSyncedAt(issueID string) *time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncedAt[issueID]
}

func (m *memorySyncProvider) SetIssueNumber(issueID string, number int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.issueNumbers[issueID] = number
}

func (m *memorySyncProvider) SetSyncedAt(issueID string, t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	utc := t.UTC()
	m.syncedAt[issueID] = &utc
}

func (m *memorySyncProvider) Clear(issueID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.issueNumbers, issueID)
	delete(m.syncedAt, issueID)
}

func (m *memorySyncProvider) Flush() error { return nil }

func newTestSyncer(t *testing.T, client *Client) *Syncer {
	t.Helper()
	return &Syncer{
		client:          client,
		config:          &Config{Owner: "test-owner", Repo: "test-repo"},
		opts:            SyncOptions{},
		syncStore:       newMemorySyncProvider(),
		issueToGHNumber: make(map[string]int),
	}
}

func TestComputeLabels(t *testing.T) {
	syncer := &Syncer{
		config: &Config{},
	}

	tests := []struct {
		name       string
		issue      *issue.Issue
		wantLabels []string
	}{
		{
			name: "all labels",
			issue: &issue.Issue{
				Status:   "todo",
				Priority: "high",
				Type:     "bug",
				Tags:     []string{"urgent"},
			},
			wantLabels: []string{"status:todo", "priority:high", "type:bug", "urgent"},
		},
		{
			name: "no status label when mapping has empty label",
			issue: &issue.Issue{
				Status: "completed",
			},
			wantLabels: nil,
		},
		{
			name: "only tags",
			issue: &issue.Issue{
				Status: "unknown",
				Tags:   []string{"a", "b"},
			},
			wantLabels: []string{"a", "b"},
		},
		{
			name: "draft status",
			issue: &issue.Issue{
				Status: "draft",
			},
			wantLabels: []string{"status:draft"},
		},
		{
			name: "in-progress status",
			issue: &issue.Issue{
				Status: "in-progress",
			},
			wantLabels: []string{"status:in-progress"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := syncer.computeLabels(tt.issue)
			if !labelsMatch(got, tt.wantLabels) {
				t.Errorf("computeLabels() = %v, want %v", got, tt.wantLabels)
			}
		})
	}
}

func labelsMatch(a, b []string) bool {
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

func TestBuildIssueBody(t *testing.T) {
	syncer := &Syncer{config: &Config{}}

	tests := []struct {
		name     string
		issue    *issue.Issue
		wantBody string
	}{
		{
			name:     "with body",
			issue:    &issue.Issue{ID: "bean-1", Body: "Some description"},
			wantBody: "Some description\n\n<!-- bean:bean-1 -->",
		},
		{
			name:     "empty body",
			issue:    &issue.Issue{ID: "bean-2"},
			wantBody: "<!-- bean:bean-2 -->",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := syncer.buildIssueBody(tt.issue)
			if got != tt.wantBody {
				t.Errorf("buildIssueBody() = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestGetGitHubState(t *testing.T) {
	syncer := &Syncer{
		config: &Config{},
	}

	tests := []struct {
		status string
		want   string
	}{
		{"todo", "open"},
		{"draft", "open"},
		{"in-progress", "open"},
		{"completed", "closed"},
		{"scrapped", "closed"},
		{"unknown", "open"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := syncer.getGitHubState(tt.status)
			if got != tt.want {
				t.Errorf("getGitHubState(%q) = %q, want %q", tt.status, got, tt.want)
			}
		})
	}
}

func TestSyncIssue_Create(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/issues") {
			resp := Issue{
				Number:  42,
				Title:   "Test",
				State:   "open",
				HTMLURL: "https://github.com/test/repo/issues/42",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/user" {
			resp := User{Login: "testuser", ID: 1}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		owner: "test-owner",
		repo:  "test-repo",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	syncer := newTestSyncer(t, client)

	now := time.Now()
	b := &issue.Issue{
		ID:        "bean-1",
		Title:     "Test bean",
		Status:    "todo",
		Type:      "task",
		Tags:      []string{"frontend"},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "created" {
		t.Fatalf("expected action 'created', got %q", result.Action)
	}
	if result.ExternalID != "42" {
		t.Errorf("expected external ID '42', got %q", result.ExternalID)
	}
	if result.ExternalURL != "https://github.com/test/repo/issues/42" {
		t.Errorf("expected external URL, got %q", result.ExternalURL)
	}
}

func TestSyncIssue_Update(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/issues/") {
			resp := Issue{
				Number:  42,
				Title:   "Old title",
				Body:    "old body\n\n<!-- bean:bean-1 -->",
				State:   "open",
				HTMLURL: "https://github.com/test/repo/issues/42",
				Labels:  []Label{{Name: "status:todo"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/issues/") {
			resp := Issue{
				Number:  42,
				Title:   "Updated bean",
				State:   "open",
				HTMLURL: "https://github.com/test/repo/issues/42",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		owner: "test-owner",
		repo:  "test-repo",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	store := newMemorySyncProvider()
	store.SetIssueNumber("bean-1", 42)
	syncer := &Syncer{
		client:          client,
		config:          &Config{Owner: "test-owner", Repo: "test-repo"},
		opts:            SyncOptions{Force: true},
		syncStore:       store,
		issueToGHNumber: make(map[string]int),
	}

	now := time.Now()
	b := &issue.Issue{
		ID:        "bean-1",
		Title:     "Updated bean",
		Status:    "todo",
		Type:      "task",
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "updated" {
		t.Fatalf("expected action 'updated', got %q", result.Action)
	}
}

func TestSyncIssue_CreateWithLabels(t *testing.T) {
	var receivedLabels []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/issues") {
			var req CreateIssueRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			receivedLabels = req.Labels

			resp := Issue{
				Number:  1,
				Title:   req.Title,
				State:   "open",
				HTMLURL: "https://github.com/test/repo/issues/1",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/user" {
			resp := User{Login: "testuser", ID: 1}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		owner: "test-owner",
		repo:  "test-repo",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	syncer := &Syncer{
		client:          client,
		config:          &Config{Owner: "test-owner", Repo: "test-repo"},
		opts:            SyncOptions{},
		syncStore:       newMemorySyncProvider(),
		issueToGHNumber: make(map[string]int),
	}

	now := time.Now()
	b := &issue.Issue{
		ID:        "bean-1",
		Title:     "Test bug",
		Status:    "todo",
		Priority:  "high",
		Type:      "bug",
		Tags:      []string{"frontend"},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "created" {
		t.Fatalf("expected action 'created', got %q", result.Action)
	}

	sort.Strings(receivedLabels)
	expected := []string{"frontend", "priority:high", "status:todo", "type:bug"}
	sort.Strings(expected)

	if !labelsMatch(receivedLabels, expected) {
		t.Errorf("labels = %v, want %v", receivedLabels, expected)
	}
}

func TestFilterIssuesNeedingSync(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	store := newMemorySyncProvider()
	store.SetIssueNumber("bean-synced", 1)
	store.SetSyncedAt("bean-synced", now)
	store.SetIssueNumber("bean-stale", 2)
	store.SetSyncedAt("bean-stale", past)

	issues := []*issue.Issue{
		{ID: "bean-new", UpdatedAt: &now},
		{ID: "bean-synced", UpdatedAt: &past},
		{ID: "bean-stale", UpdatedAt: &future},
	}

	result := FilterIssuesNeedingSync(issues, store, false)

	var ids []string
	for _, b := range result {
		ids = append(ids, b.ID)
	}
	sort.Strings(ids)

	expected := []string{"bean-new", "bean-stale"}
	sort.Strings(expected)

	if !labelsMatch(ids, expected) {
		t.Errorf("FilterIssuesNeedingSync() = %v, want %v", ids, expected)
	}
}

func TestFilterIssuesNeedingSync_Force(t *testing.T) {
	now := time.Now()

	store := newMemorySyncProvider()
	store.SetIssueNumber("bean-1", 1)
	store.SetSyncedAt("bean-1", now)

	issues := []*issue.Issue{
		{ID: "bean-1", UpdatedAt: &now},
		{ID: "bean-2", UpdatedAt: &now},
	}

	result := FilterIssuesNeedingSync(issues, store, true)

	if len(result) != 2 {
		t.Errorf("expected 2 issues with force=true, got %d", len(result))
	}
}

func TestSyncIssue_DryRun_Create(t *testing.T) {
	syncer := &Syncer{
		config:          &Config{Owner: "test-owner", Repo: "test-repo"},
		opts:            SyncOptions{DryRun: true},
		syncStore:       newMemorySyncProvider(),
		issueToGHNumber: make(map[string]int),
	}

	b := &issue.Issue{
		ID:     "bean-1",
		Title:  "Test bean",
		Status: "todo",
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "would create" {
		t.Fatalf("expected action 'would create', got %q", result.Action)
	}
}

// redirectTransport redirects all requests to the test server.
type redirectTransport struct {
	target string
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return http.DefaultTransport.RoundTrip(req)
}
