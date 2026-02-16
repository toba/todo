package clickup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/issue"
)

// memorySyncProvider is a simple in-memory SyncStateProvider for tests.
type memorySyncProvider struct {
	mu       sync.RWMutex
	taskIDs  map[string]string
	syncedAt map[string]*time.Time
}

func newMemorySyncProvider() *memorySyncProvider {
	return &memorySyncProvider{
		taskIDs:  make(map[string]string),
		syncedAt: make(map[string]*time.Time),
	}
}

func (m *memorySyncProvider) GetTaskID(issueID string) *string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	id, ok := m.taskIDs[issueID]
	if !ok || id == "" {
		return nil
	}
	return &id
}

func (m *memorySyncProvider) GetSyncedAt(issueID string) *time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.syncedAt[issueID]
}

func (m *memorySyncProvider) SetTaskID(issueID, taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskIDs[issueID] = taskID
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
	delete(m.taskIDs, issueID)
	delete(m.syncedAt, issueID)
}

func (m *memorySyncProvider) Flush() error { return nil }

func newTestSyncer(t *testing.T, client *Client) *Syncer {
	t.Helper()
	return &Syncer{
		client:        client,
		config:        &Config{},
		opts:          SyncOptions{ListID: "test-list"},
		syncStore:     newMemorySyncProvider(),
		issueToTaskID: make(map[string]string),
	}
}

func TestSyncTags_SetDiff(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.URL.Path, "/tag/")
		if len(parts) == 2 {
			calls = append(calls, r.Method+" "+parts[1])
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	syncer := newTestSyncer(t, client)

	tests := []struct {
		name        string
		issueTags   []string
		currentTags []Tag
		wantAdds    []string
		wantRemoves []string
		wantChanged bool
	}{
		{
			name:        "add all tags to new task",
			issueTags:   []string{"urgent", "backend"},
			currentTags: nil,
			wantAdds:    []string{"backend", "urgent"},
			wantChanged: true,
		},
		{
			name:        "remove extra tags",
			issueTags:   nil,
			currentTags: []Tag{{Name: "old-tag"}},
			wantRemoves: []string{"old-tag"},
			wantChanged: true,
		},
		{
			name:        "add and remove tags",
			issueTags:   []string{"keep", "new-tag"},
			currentTags: []Tag{{Name: "keep"}, {Name: "old-tag"}},
			wantAdds:    []string{"new-tag"},
			wantRemoves: []string{"old-tag"},
			wantChanged: true,
		},
		{
			name:        "no changes when tags match",
			issueTags:   []string{"a", "b"},
			currentTags: []Tag{{Name: "a"}, {Name: "b"}},
			wantChanged: false,
		},
		{
			name:        "no changes when both empty",
			issueTags:   nil,
			currentTags: nil,
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls = nil

			b := &issue.Issue{
				ID:   "issue-1",
				Tags: tt.issueTags,
			}

			changed := syncer.syncTags(context.Background(), "task-1", b, tt.currentTags)

			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}

			var gotAdds, gotRemoves []string
			for _, call := range calls {
				parts := strings.SplitN(call, " ", 2)
				switch parts[0] {
				case "POST":
					gotAdds = append(gotAdds, parts[1])
				case "DELETE":
					gotRemoves = append(gotRemoves, parts[1])
				}
			}

			sort.Strings(gotAdds)
			sort.Strings(gotRemoves)

			if !slicesEqual(gotAdds, tt.wantAdds) {
				t.Errorf("added tags = %v, want %v", gotAdds, tt.wantAdds)
			}
			if !slicesEqual(gotRemoves, tt.wantRemoves) {
				t.Errorf("removed tags = %v, want %v", gotRemoves, tt.wantRemoves)
			}
		})
	}
}

func TestSyncIssue_CreateWithTags(t *testing.T) {
	var tagCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/list/") {
			resp := taskResponse{
				ID:     "task-123",
				Name:   "Test",
				Status: Status{Status: "to do"},
				URL:    "https://app.clickup.com/t/task-123",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.Contains(r.URL.Path, "/tag/") {
			parts := strings.Split(r.URL.Path, "/tag/")
			tagCalls = append(tagCalls, r.Method+" "+parts[len(parts)-1])
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{}"))
			return
		}
		if r.URL.Path == "/api/v2/user" {
			resp := userResponse{User: AuthorizedUser{ID: 1, Username: "test"}}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	syncer := newTestSyncer(t, client)

	now := time.Now()
	b := &issue.Issue{
		ID:        "issue-1",
		Title:     "Test issue",
		Status:    "ready",
		Type:      "task",
		Tags:      []string{"frontend", "urgent"},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "created" {
		t.Fatalf("expected action 'created', got %q", result.Action)
	}

	sort.Strings(tagCalls)
	expectedCalls := []string{"POST frontend", "POST urgent"}
	sort.Strings(expectedCalls)

	if !slicesEqual(tagCalls, expectedCalls) {
		t.Errorf("tag calls = %v, want %v", tagCalls, expectedCalls)
	}
}

func TestSyncIssue_UpdateWithTagChanges(t *testing.T) {
	var tagCalls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/task/") {
			resp := taskResponse{
				ID:     "task-123",
				Name:   "Test issue",
				Status: Status{Status: "to do"},
				URL:    "https://app.clickup.com/t/task-123",
				Tags:   []Tag{{Name: "old-tag"}, {Name: "keep"}},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if strings.Contains(r.URL.Path, "/tag/") {
			parts := strings.Split(r.URL.Path, "/tag/")
			tagCalls = append(tagCalls, r.Method+" "+parts[len(parts)-1])
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	store := newMemorySyncProvider()
	store.SetTaskID("issue-1", "task-123")
	syncer := &Syncer{
		client:        client,
		config:        &Config{},
		opts:          SyncOptions{ListID: "test-list", Force: true},
		syncStore:     store,
		issueToTaskID: make(map[string]string),
	}

	now := time.Now()
	b := &issue.Issue{
		ID:        "issue-1",
		Title:     "Test issue",
		Status:    "ready",
		Type:      "task",
		Tags:      []string{"keep", "new-tag"},
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	result := syncer.syncIssue(context.Background(), b)

	if result.Action != "updated" {
		t.Fatalf("expected action 'updated', got %q", result.Action)
	}

	sort.Strings(tagCalls)
	expectedCalls := []string{"DELETE old-tag", "POST new-tag"}
	sort.Strings(expectedCalls)

	if !slicesEqual(tagCalls, expectedCalls) {
		t.Errorf("tag calls = %v, want %v", tagCalls, expectedCalls)
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

func TestSyncTags_EnsureSpaceTagBeforeAdd(t *testing.T) {
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// Space tag creation: POST /api/v2/space/{id}/tag (path ends with /tag, not /tag/{name})
		if r.Method == "POST" && strings.Contains(path, "/space/") && strings.Contains(path, "/tag") && !strings.Contains(path, "/task/") {
			calls = append(calls, "space-create")
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{}"))
			return
		}
		// Task tag addition: POST /api/v2/task/{id}/tag/{tagName}
		if r.Method == "POST" && strings.Contains(path, "/task/") && strings.Contains(path, "/tag/") {
			parts := strings.Split(path, "/tag/")
			calls = append(calls, "task-add:"+parts[len(parts)-1])
			w.WriteHeader(200)
			_, _ = w.Write([]byte("{}"))
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
		spaceTags: make(map[string]bool), // empty cache
	}

	syncer := newTestSyncer(t, client)
	syncer.spaceID = "space-1"

	b := &issue.Issue{
		ID:   "issue-1",
		Tags: []string{"new-tag"},
	}

	syncer.syncTags(context.Background(), "task-1", b, nil)

	// Verify space tag creation happens before task tag addition
	expected := []string{"space-create", "task-add:new-tag"}
	if !slicesEqual(calls, expected) {
		t.Errorf("calls = %v, want %v", calls, expected)
	}
}

func TestSyncTags_CachePreventsRedundantSpaceTagCreation(t *testing.T) {
	var spaceCreateCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/space/") && strings.HasSuffix(r.URL.Path, "/tag") {
			spaceCreateCalls++
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
		spaceTags: map[string]bool{"cached-tag": true}, // pre-cached tag
	}

	syncer := newTestSyncer(t, client)
	syncer.spaceID = "space-1"

	// Sync an issue with an already-cached tag
	b := &issue.Issue{
		ID:   "issue-1",
		Tags: []string{"cached-tag"},
	}
	syncer.syncTags(context.Background(), "task-1", b, nil)

	if spaceCreateCalls != 0 {
		t.Errorf("expected 0 space tag creations for cached tag, got %d", spaceCreateCalls)
	}

	// Now sync an issue with a new tag - should create once
	b2 := &issue.Issue{
		ID:   "issue-2",
		Tags: []string{"new-tag"},
	}
	syncer.syncTags(context.Background(), "task-2", b2, nil)

	if spaceCreateCalls != 1 {
		t.Errorf("expected 1 space tag creation for new tag, got %d", spaceCreateCalls)
	}

	// Sync same new tag again - should not create again (now cached)
	b3 := &issue.Issue{
		ID:   "issue-3",
		Tags: []string{"new-tag"},
	}
	syncer.syncTags(context.Background(), "task-3", b3, nil)

	if spaceCreateCalls != 1 {
		t.Errorf("expected still 1 space tag creation (cached), got %d", spaceCreateCalls)
	}
}

func TestSyncIssue_CreateWithDueDate(t *testing.T) {
	var capturedReq CreateTaskRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/list/") {
			_ = json.NewDecoder(r.Body).Decode(&capturedReq)
			resp := taskResponse{
				ID:     "task-456",
				Name:   "Test",
				Status: Status{Status: "to do"},
				URL:    "https://app.clickup.com/t/task-456",
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.URL.Path == "/api/v2/user" {
			resp := userResponse{User: AuthorizedUser{ID: 1, Username: "test"}}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	syncer := newTestSyncer(t, client)

	t.Run("with due date", func(t *testing.T) {
		due, _ := issue.ParseDueDate("2025-06-15")
		now := time.Now()
		b := &issue.Issue{
			ID:        "issue-due",
			Title:     "Issue with due date",
			Status:    "ready",
			Type:      "task",
			Due:       due,
			CreatedAt: &now,
			UpdatedAt: &now,
		}

		result := syncer.syncIssue(context.Background(), b)
		if result.Action != "created" {
			t.Fatalf("expected action 'created', got %q", result.Action)
		}
		if capturedReq.DueDate == nil {
			t.Fatal("expected DueDate to be set in create request")
		}
		if capturedReq.DueDatetime == nil || *capturedReq.DueDatetime != false {
			t.Error("expected DueDatetime to be false")
		}
	})

	t.Run("without due date", func(t *testing.T) {
		capturedReq = CreateTaskRequest{} // reset
		now := time.Now()
		b := &issue.Issue{
			ID:        "issue-nodue",
			Title:     "Issue without due date",
			Status:    "ready",
			Type:      "task",
			CreatedAt: &now,
			UpdatedAt: &now,
		}

		result := syncer.syncIssue(context.Background(), b)
		if result.Action != "created" {
			t.Fatalf("expected action 'created', got %q", result.Action)
		}
		if capturedReq.DueDate != nil {
			t.Errorf("expected DueDate to be nil, got %v", *capturedReq.DueDate)
		}
	})
}

func TestSyncIssue_UpdateDueDate(t *testing.T) {
	existingDue := "1750000000000" // existing due date in ClickUp (Unix ms)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/task/") {
			resp := taskResponse{
				ID:      "task-789",
				Name:    "Test issue",
				Status:  Status{Status: "to do"},
				URL:     "https://app.clickup.com/t/task-789",
				DueDate: &existingDue,
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/task/") {
			resp := taskResponse{
				ID:     "task-789",
				Name:   "Test issue",
				Status: Status{Status: "to do"},
				URL:    "https://app.clickup.com/t/task-789",
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
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	store := newMemorySyncProvider()
	store.SetTaskID("issue-1", "task-789")
	syncer := &Syncer{
		client:        client,
		config:        &Config{},
		opts:          SyncOptions{ListID: "test-list", Force: true},
		syncStore:     store,
		issueToTaskID: make(map[string]string),
	}

	due, _ := issue.ParseDueDate("2025-12-25")
	now := time.Now()
	b := &issue.Issue{
		ID:        "issue-1",
		Title:     "Test issue",
		Status:    "ready",
		Type:      "task",
		Due:       due,
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	result := syncer.syncIssue(context.Background(), b)
	if result.Action != "updated" {
		t.Fatalf("expected action 'updated', got %q", result.Action)
	}
}

func TestBuildUpdateRequest_Parent(t *testing.T) {
	syncer := &Syncer{
		config:        &Config{},
		issueToTaskID: map[string]string{
			"parent-issue": "clickup-parent-123",
		},
	}

	tests := []struct {
		name       string
		current    *TaskInfo
		issue      *issue.Issue
		wantParent *string
	}{
		{
			name:    "set parent on previously unparented task",
			current: &TaskInfo{Name: "Child", Status: Status{Status: "to do"}},
			issue: &issue.Issue{
				Title:  "Child",
				Status: "ready",
				Parent: "parent-issue",
			},
			wantParent: strPtr("clickup-parent-123"),
		},
		{
			name:    "no change when parent matches",
			current: &TaskInfo{Name: "Child", Status: Status{Status: "to do"}, Parent: strPtr("clickup-parent-123")},
			issue: &issue.Issue{
				Title:  "Child",
				Status: "ready",
				Parent: "parent-issue",
			},
			wantParent: nil, // nil means no change in update request
		},
		{
			name:    "no change when both have no parent",
			current: &TaskInfo{Name: "Task", Status: Status{Status: "to do"}},
			issue: &issue.Issue{
				Title:  "Task",
				Status: "ready",
			},
			wantParent: nil,
		},
		{
			name:    "skip when parent issue not in issueToTaskID map",
			current: &TaskInfo{Name: "Child", Status: Status{Status: "to do"}},
			issue: &issue.Issue{
				Title:  "Child",
				Status: "ready",
				Parent: "unknown-parent",
			},
			wantParent: nil, // can't resolve, so don't update
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			update := syncer.buildUpdateRequest(tt.current, tt.issue, "", nil, "")
			if tt.wantParent == nil && update.Parent != nil {
				t.Errorf("expected no parent update, got %q", *update.Parent)
			}
			if tt.wantParent != nil && update.Parent == nil {
				t.Error("expected parent update, got nil")
			}
			if tt.wantParent != nil && update.Parent != nil && *tt.wantParent != *update.Parent {
				t.Errorf("parent = %q, want %q", *update.Parent, *tt.wantParent)
			}
		})
	}
}

func TestSyncIssues_ParentNotInBatch(t *testing.T) {
	// When syncing a child issue whose parent is NOT in the batch but HAS been
	// previously synced, SyncIssues should resolve the parent task ID from
	// the parent issue's sync metadata and set the parent on the ClickUp task.
	var capturedParent *string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// GetAuthorizedUser
		if strings.Contains(r.URL.Path, "/user") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"user": map[string]any{"id": 1},
			})
			return
		}
		// GetList (for space ID)
		if r.Method == "GET" && strings.Contains(r.URL.Path, "/list/") && !strings.Contains(r.URL.Path, "/task") {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "test-list", "space": map[string]any{"id": "space-1"}})
			return
		}
		// GetSpaceTags
		if strings.Contains(r.URL.Path, "/space/") && strings.Contains(r.URL.Path, "/tag") {
			_ = json.NewEncoder(w).Encode(map[string]any{"tags": []any{}})
			return
		}
		// CreateTask - capture the parent field
		if r.Method == "POST" && strings.Contains(r.URL.Path, "/list/") && strings.Contains(r.URL.Path, "/task") {
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			if p, ok := req["parent"]; ok && p != nil {
				s := p.(string)
				capturedParent = &s
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":   "new-task-456",
				"name": req["name"],
				"url":  "https://app.clickup.com/t/new-task-456",
				"status": map[string]any{
					"status": "to do",
				},
			})
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := &Client{
		token: "test",
		httpClient: &http.Client{
			Transport: &redirectTransport{target: server.URL},
		},
	}

	// Set up a core.Core with the parent issue that has sync metadata
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, ".issues")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	cfg := config.Default()
	c := core.New(dataDir, cfg)
	c.SetWarnWriter(nil)
	if err := c.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	// Create parent issue with clickup sync metadata (already synced)
	parentIssue := &issue.Issue{
		ID:     "parent-issue",
		Slug:   "parent-issue",
		Title:  "Parent epic",
		Status: "in-progress",
		Type:   "epic",
	}
	parentIssue.SetSync(SyncName, map[string]any{
		SyncKeyTaskID: "clickup-parent-789",
	})
	if err := c.Create(parentIssue); err != nil {
		t.Fatalf("failed to create parent issue: %v", err)
	}

	store := newMemorySyncProvider()

	syncer := &Syncer{
		client:        client,
		config:        &Config{},
		opts:          SyncOptions{ListID: "test-list"},
		core:          c,
		syncStore:     store,
		issueToTaskID: make(map[string]string),
	}

	now := time.Now()
	childIssue := &issue.Issue{
		ID:        "child-issue",
		Title:     "Child task",
		Status:    "ready",
		Type:      "task",
		Parent:    "parent-issue", // parent NOT in the batch
		CreatedAt: &now,
		UpdatedAt: &now,
	}

	// Sync only the child - parent is not in the batch
	results, err := syncer.SyncIssues(context.Background(), []*issue.Issue{childIssue})
	if err != nil {
		t.Fatalf("SyncIssues failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != "created" {
		t.Fatalf("expected action 'created', got %q", results[0].Action)
	}
	if capturedParent == nil {
		t.Fatal("expected parent to be set in create request, but it was nil")
	}
	if *capturedParent != "clickup-parent-789" {
		t.Errorf("parent = %q, want %q", *capturedParent, "clickup-parent-789")
	}
}

func strPtr(s string) *string { return &s }

func slicesEqual(a, b []string) bool {
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
