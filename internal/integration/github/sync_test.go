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

	"github.com/toba/todo/internal/integration/syncutil"
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
			name: "only tags become labels",
			issue: &issue.Issue{
				Status:   "ready",
				Priority: "high",
				Type:     "bug",
				Tags:     []string{"urgent"},
			},
			wantLabels: []string{"urgent"},
		},
		{
			name: "no tags means no labels",
			issue: &issue.Issue{
				Status: "completed",
			},
			wantLabels: nil,
		},
		{
			name: "multiple tags",
			issue: &issue.Issue{
				Status: "unknown",
				Tags:   []string{"a", "b"},
			},
			wantLabels: []string{"a", "b"},
		},
		{
			name: "status and priority do not produce labels",
			issue: &issue.Issue{
				Status:   "draft",
				Priority: "critical",
			},
			wantLabels: nil,
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
			issue:    &issue.Issue{ID: "test-1", Body: "Some description"},
			wantBody: "Some description\n\n" + syncutil.SyncFooter + "\n\n<!-- todo:test-1 -->",
		},
		{
			name:     "empty body",
			issue:    &issue.Issue{ID: "test-2"},
			wantBody: syncutil.SyncFooter + "\n\n<!-- todo:test-2 -->",
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
		{"ready", "open"},
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
		ID:        "test-1",
		Title:     "Test issue",
		Status:    "ready",
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
				Body:    "old body\n\n<!-- todo:test-1 -->",
				State:   "open",
				HTMLURL: "https://github.com/test/repo/issues/42",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PATCH" && strings.Contains(r.URL.Path, "/issues/") {
			resp := Issue{
				Number:  42,
				Title:   "Updated issue",
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
	store.SetIssueNumber("test-1", 42)
	syncer := &Syncer{
		client:          client,
		config:          &Config{Owner: "test-owner", Repo: "test-repo"},
		opts:            SyncOptions{Force: true},
		syncStore:       store,
		issueToGHNumber: make(map[string]int),
	}

	now := time.Now()
	b := &issue.Issue{
		ID:        "test-1",
		Title:     "Updated issue",
		Status:    "ready",
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
	var receivedType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/issues") {
			var req CreateIssueRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			receivedLabels = req.Labels
			receivedType = req.Type

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
		ID:        "test-1",
		Title:     "Test bug",
		Status:    "ready",
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

	// Only tags should appear as labels
	expected := []string{"frontend"}
	if !labelsMatch(receivedLabels, expected) {
		t.Errorf("labels = %v, want %v", receivedLabels, expected)
	}

	// Type should use GitHub's native type field
	if receivedType != "Bug" {
		t.Errorf("type = %q, want %q", receivedType, "Bug")
	}
}

func TestFilterIssuesNeedingSync(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	store := newMemorySyncProvider()
	store.SetIssueNumber("test-synced", 1)
	store.SetSyncedAt("test-synced", now)
	store.SetIssueNumber("test-stale", 2)
	store.SetSyncedAt("test-stale", past)

	issues := []*issue.Issue{
		{ID: "test-new", UpdatedAt: &now},
		{ID: "test-synced", UpdatedAt: &past},
		{ID: "test-stale", UpdatedAt: &future},
	}

	result := FilterIssuesNeedingSync(issues, store, false)

	var ids []string
	for _, b := range result {
		ids = append(ids, b.ID)
	}
	sort.Strings(ids)

	expected := []string{"test-new", "test-stale"}
	sort.Strings(expected)

	if !labelsMatch(ids, expected) {
		t.Errorf("FilterIssuesNeedingSync() = %v, want %v", ids, expected)
	}
}

func TestFilterIssuesNeedingSync_Force(t *testing.T) {
	now := time.Now()

	store := newMemorySyncProvider()
	store.SetIssueNumber("test-1", 1)
	store.SetSyncedAt("test-1", now)

	issues := []*issue.Issue{
		{ID: "test-1", UpdatedAt: &now},
		{ID: "test-2", UpdatedAt: &now},
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
		ID:     "test-1",
		Title:  "Test issue",
		Status: "ready",
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "would create" {
		t.Fatalf("expected action 'would create', got %q", result.Action)
	}
}

func TestGetGitHubType(t *testing.T) {
	syncer := &Syncer{config: &Config{}}

	tests := []struct {
		issueType string
		want      string
	}{
		{"bug", "Bug"},
		{"feature", "Feature"},
		{"task", "Task"},
		{"milestone", "Task"},
		{"epic", "Task"},
		{"unknown", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.issueType, func(t *testing.T) {
			got := syncer.getGitHubType(tt.issueType)
			if got != tt.want {
				t.Errorf("getGitHubType(%q) = %q, want %q", tt.issueType, got, tt.want)
			}
		})
	}
}

func TestBuildUpdateRequest_TypeChange(t *testing.T) {
	syncer := &Syncer{config: &Config{}}

	tests := []struct {
		name        string
		currentType *IssueType
		newType     string
		wantType    *string
	}{
		{
			name:        "type changed",
			currentType: &IssueType{Name: "Task"},
			newType:     "Bug",
			wantType:    new("Bug"),
		},
		{
			name:        "type unchanged",
			currentType: &IssueType{Name: "Bug"},
			newType:     "Bug",
			wantType:    nil,
		},
		{
			name:        "no current type, setting new",
			currentType: nil,
			newType:     "Feature",
			wantType:    new("Feature"),
		},
		{
			name:        "no current type, no new type",
			currentType: nil,
			newType:     "",
			wantType:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := &Issue{
				Title: "Test",
				Body:  "body\n\n<!-- todo:test-1 -->",
				State: "open",
				Type:  tt.currentType,
			}
			b := &issue.Issue{
				ID:    "test-1",
				Title: "Test",
			}
			update := syncer.buildUpdateRequest(current, b, "body\n\n<!-- todo:test-1 -->", "open", tt.newType, nil)
			if tt.wantType == nil && update.Type != nil {
				t.Errorf("expected nil Type, got %q", *update.Type)
			}
			if tt.wantType != nil {
				if update.Type == nil {
					t.Errorf("expected Type %q, got nil", *tt.wantType)
				} else if *update.Type != *tt.wantType {
					t.Errorf("Type = %q, want %q", *update.Type, *tt.wantType)
				}
			}
		})
	}
}

//go:fix inline
func ptr[T any](v T) *T { return new(v) }

// redirectTransport redirects all requests to the test server.
type redirectTransport struct {
	target string
}

func (rt *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(rt.target, "http://")
	return http.DefaultTransport.RoundTrip(req)
}
