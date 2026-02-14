package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
)

func setupTestCore(t *testing.T) (*Core, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	cfg := config.Default()
	core := New(dataDir, cfg)
	core.SetWarnWriter(nil) // suppress warnings in tests
	if err := core.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return core, dataDir
}

func setupTestCoreWithRequireIfMatch(t *testing.T) (*Core, string) {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	cfg := config.Default()
	cfg.Issues.RequireIfMatch = true
	core := New(dataDir, cfg)
	core.SetWarnWriter(nil) // suppress warnings in tests
	if err := core.Load(); err != nil {
		t.Fatalf("failed to load core: %v", err)
	}

	return core, dataDir
}

func createTestBean(t *testing.T, core *Core, id, title, status string) *issue.Issue {
	t.Helper()
	b := &issue.Issue{
		ID:     id,
		Slug:   issue.Slugify(title),
		Title:  title,
		Status: status,
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("failed to create test bean: %v", err)
	}
	return b
}

func TestNew(t *testing.T) {
	cfg := config.Default()
	core := New("/some/path", cfg)

	if core.Root() != "/some/path" {
		t.Errorf("Root() = %q, want %q", core.Root(), "/some/path")
	}
	if core.Config() != cfg {
		t.Error("Config() returned different config")
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, DataDir)

	core := New(dataDir, nil)
	err := core.Init()
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	info, err := os.Stat(dataDir)
	if err != nil {
		t.Fatalf(".beans directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Error(".beans is not a directory")
	}
}

func TestInitIdempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, DataDir)

	core := New(dataDir, nil)

	// Call Init twice - should not error
	if err := core.Init(); err != nil {
		t.Fatalf("first Init() error = %v", err)
	}
	if err := core.Init(); err != nil {
		t.Fatalf("second Init() error = %v", err)
	}
}

func TestCreate(t *testing.T) {
	core, dataDir := setupTestCore(t)

	b := &issue.Issue{
		ID:     "abc-def",
		Slug:   "test-bean",
		Title:  "Test Bean",
		Status: "todo",
		Body:   "Some content here.",
	}

	err := core.Create(b)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Check file exists in hash subfolder
	expectedPath := filepath.Join(dataDir, "a", "abc-def--test-bean.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("bean file not created at %s", expectedPath)
	}

	// Check timestamps were set
	if b.CreatedAt == nil {
		t.Error("CreatedAt not set")
	}
	if b.UpdatedAt == nil {
		t.Error("UpdatedAt not set")
	}

	// Check Path was set to hash subfolder path
	if b.Path != filepath.Join("a", "abc-def--test-bean.md") {
		t.Errorf("Path = %q, want %q", b.Path, filepath.Join("a", "abc-def--test-bean.md"))
	}

	// Check in-memory state
	all := core.All()
	if len(all) != 1 {
		t.Errorf("All() returned %d beans, want 1", len(all))
	}
}

func TestCreateGeneratesID(t *testing.T) {
	core, _ := setupTestCore(t)

	b := &issue.Issue{
		Title:  "Auto ID Bean",
		Status: "todo",
	}

	err := core.Create(b)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if b.ID == "" {
		t.Error("ID was not generated")
	}
	// xxx-xxx format: 7 chars total with hyphen at position 3
	if len(b.ID) != 7 {
		t.Errorf("ID length = %d, want 7 (xxx-xxx format)", len(b.ID))
	}
	if b.ID[3] != '-' {
		t.Errorf("ID = %q, want hyphen at position 3", b.ID)
	}
}

func TestAll(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "aaa1", "First Bean", "todo")
	createTestBean(t, core, "bbb2", "Second Bean", "in-progress")
	createTestBean(t, core, "ccc3", "Third Bean", "completed")

	beans := core.All()
	if len(beans) != 3 {
		t.Errorf("All() returned %d beans, want 3", len(beans))
	}
}

func TestAllEmpty(t *testing.T) {
	core, _ := setupTestCore(t)

	beans := core.All()
	if len(beans) != 0 {
		t.Errorf("All() returned %d beans, want 0", len(beans))
	}
}

func TestGet(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "abc1", "First", "todo")
	createTestBean(t, core, "def2", "Second", "todo")

	t.Run("exact match", func(t *testing.T) {
		b, err := core.Get("abc1")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if b.ID != "abc1" {
			t.Errorf("ID = %q, want %q", b.ID, "abc1")
		}
	})

	t.Run("partial ID not found", func(t *testing.T) {
		_, err := core.Get("abc")
		if err != ErrNotFound {
			t.Errorf("Get() error = %v, want ErrNotFound", err)
		}
	})
}

func TestGetNotFound(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "abc1", "Test", "todo")

	_, err := core.Get("xyz")
	if err != ErrNotFound {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}


func TestUpdate(t *testing.T) {
	core, _ := setupTestCore(t)

	b := createTestBean(t, core, "upd1", "Original Title", "todo")
	originalCreatedAt := *b.CreatedAt

	// Update the issue
	b.Title = "Updated Title"
	b.Status = "in-progress"

	err := core.Update(b, nil)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// CreatedAt should be preserved
	if !b.CreatedAt.Equal(originalCreatedAt) {
		t.Errorf("CreatedAt changed: got %v, want %v", b.CreatedAt, originalCreatedAt)
	}

	// UpdatedAt should be refreshed (might be same second, so just check it's set)
	if b.UpdatedAt == nil {
		t.Error("UpdatedAt not set")
	}

	// Verify in-memory state
	loaded, err := core.Get("upd1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Title != "Updated Title" {
		t.Errorf("Title = %q, want %q", loaded.Title, "Updated Title")
	}
	if loaded.Status != "in-progress" {
		t.Errorf("Status = %q, want %q", loaded.Status, "in-progress")
	}
}

func TestUpdateNotFound(t *testing.T) {
	core, _ := setupTestCore(t)

	b := &issue.Issue{
		ID:     "nonexistent",
		Title:  "Ghost Bean",
		Status: "todo",
	}

	err := core.Update(b, nil)
	if err != ErrNotFound {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
}

func TestDelete(t *testing.T) {
	core, dataDir := setupTestCore(t)

	b := createTestBean(t, core, "del1", "To Delete", "todo")
	filePath := filepath.Join(dataDir, b.Path)

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("bean file should exist before delete")
	}

	// Delete
	err := core.Delete("del1")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("bean file should not exist after delete")
	}

	// Verify in-memory state
	_, err = core.Get("del1")
	if err != ErrNotFound {
		t.Error("bean should not be in memory after delete")
	}
}

func TestDeleteNotFound(t *testing.T) {
	core, _ := setupTestCore(t)

	err := core.Delete("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}


func TestDeletePartialIDNotFound(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "unique123", "Test", "todo")

	// Partial ID should not match
	err := core.Delete("unique")
	if err != ErrNotFound {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}

	// Verify bean still exists
	_, err = core.Get("unique123")
	if err != nil {
		t.Errorf("bean should still exist, got error: %v", err)
	}
}

func TestFullPath(t *testing.T) {
	core := New("/path/to/.beans", nil)

	b := &issue.Issue{
		ID:   "abc1",
		Path: "abc1--test.md",
	}

	got := core.FullPath(b)
	want := "/path/to/.beans/abc1--test.md"

	if got != want {
		t.Errorf("FullPath() = %q, want %q", got, want)
	}
}

func TestLoad(t *testing.T) {
	core, dataDir := setupTestCore(t)

	// Create an issue file manually
	content := `---
title: Manual Bean
status: open
---

Manual content.
`
	if err := os.WriteFile(filepath.Join(dataDir, "man1--manual.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Reload
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	b, err := core.Get("man1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if b.Title != "Manual Bean" {
		t.Errorf("Title = %q, want %q", b.Title, "Manual Bean")
	}
}

func TestLoadIgnoresNonMdFiles(t *testing.T) {
	core, dataDir := setupTestCore(t)

	createTestBean(t, core, "abc1", "Real Bean", "todo")

	// Create non-.md files that should be ignored
	os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("config"), 0644)
	os.WriteFile(filepath.Join(dataDir, "README.txt"), []byte("readme"), 0644)
	os.Mkdir(filepath.Join(dataDir, "subdir"), 0755)

	// Reload
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	beans := core.All()
	if len(beans) != 1 {
		t.Errorf("All() returned %d beans, want 1 (should ignore non-.md files)", len(beans))
	}
}

func TestBlocksPreserved(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create bean A that blocks bean B
	beanA := &issue.Issue{
		ID:       "aaa1",
		Slug:     "blocker",
		Title:    "Blocker Bean",
		Status:   "todo",
		Blocking: []string{"bbb2"},
	}
	if err := core.Create(beanA); err != nil {
		t.Fatalf("Create beanA error = %v", err)
	}

	// Create bean B
	beanB := &issue.Issue{
		ID:     "bbb2",
		Slug:   "blocked",
		Title:  "Blocked Bean",
		Status: "todo",
	}
	if err := core.Create(beanB); err != nil {
		t.Fatalf("Create beanB error = %v", err)
	}

	// Reload from disk
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Find the issues
	loadedA, err := core.Get("aaa1")
	if err != nil {
		t.Fatalf("Get aaa1 error = %v", err)
	}
	loadedB, err := core.Get("bbb2")
	if err != nil {
		t.Fatalf("Get bbb2 error = %v", err)
	}

	// Bean A should have direct blocks link
	if !loadedA.IsBlocking("bbb2") {
		t.Errorf("Bean A Blocks = %v, want [bbb2]", loadedA.Blocking)
	}

	// Bean B should have no blocks
	if len(loadedB.Blocking) != 0 {
		t.Errorf("Bean B Blocks = %v, want empty", loadedB.Blocking)
	}
}

func TestConcurrentAccess(t *testing.T) {
	core, _ := setupTestCore(t)

	// Create some initial beans
	for range 10 {
		createTestBean(t, core, issue.NewID(), "Initial Bean", "todo")
	}

	// Run concurrent operations
	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Readers
	for range 10 {
		wg.Go(func() {
			for range 100 {
				_ = core.All()
			}
		})
	}

	// Writers (create)
	for range 5 {
		wg.Go(func() {
			for range 10 {
				b := &issue.Issue{
					Title:  "Concurrent Bean",
					Status: "todo",
				}
				if err := core.Create(b); err != nil {
					errors <- err
				}
			}
		})
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent operation error: %v", err)
	}
}

func TestWatch(t *testing.T) {
	core, dataDir := setupTestCore(t)

	createTestBean(t, core, "wat1", "Initial Bean", "todo")

	// Start watching
	changeCount := 0
	var mu sync.Mutex

	err := core.Watch(func() {
		mu.Lock()
		changeCount++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a new bean file manually (simulating external change)
	content := `---
title: External Bean
status: open
---
`
	if err := os.WriteFile(filepath.Join(dataDir, "ext1--external.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for debounce + processing
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := changeCount
	mu.Unlock()

	if count == 0 {
		t.Error("onChange callback was not invoked")
	}

	// Verify the new bean is in memory
	_, err = core.Get("ext1")
	if err != nil {
		t.Errorf("external bean not loaded: %v", err)
	}

	// Stop watching
	if err := core.Unwatch(); err != nil {
		t.Fatalf("Unwatch() error = %v", err)
	}
}

func TestWatchDeletedBean(t *testing.T) {
	core, dataDir := setupTestCore(t)

	b := createTestBean(t, core, "del1", "To Delete", "todo")

	// Start watching
	changed := make(chan struct{}, 1)
	err := core.Watch(func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	})
	if err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Delete the file manually
	if err := os.Remove(filepath.Join(dataDir, b.Path)); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Wait for change notification
	select {
	case <-changed:
		// OK
	case <-time.After(500 * time.Millisecond):
		t.Error("onChange callback was not invoked for delete")
	}

	// Verify the issue is gone from memory
	_, err = core.Get("del1")
	if err != ErrNotFound {
		t.Errorf("deleted bean still in memory: %v", err)
	}

	if err := core.Unwatch(); err != nil {
		t.Fatalf("Unwatch() error = %v", err)
	}
}

func TestUnwatchIdempotent(t *testing.T) {
	core, _ := setupTestCore(t)

	// Unwatch without watching should not error
	if err := core.Unwatch(); err != nil {
		t.Errorf("Unwatch() without Watch() error = %v", err)
	}

	// Start watching
	if err := core.Watch(func() {}); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Unwatch twice should not error
	if err := core.Unwatch(); err != nil {
		t.Errorf("first Unwatch() error = %v", err)
	}
	if err := core.Unwatch(); err != nil {
		t.Errorf("second Unwatch() error = %v", err)
	}
}

func TestClose(t *testing.T) {
	core, _ := setupTestCore(t)

	// Start watching
	if err := core.Watch(func() {}); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	// Close should stop the watcher
	if err := core.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestSubscribe(t *testing.T) {
	core, dataDir := setupTestCore(t)

	// Start watching
	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	// Subscribe to events
	ch, unsub := core.Subscribe()
	defer unsub()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create an issue file (should trigger EventCreated)
	content := `---
title: New Bean
status: todo
---
`
	if err := os.WriteFile(filepath.Join(dataDir, "new1--new.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Wait for events
	select {
	case events := <-ch:
		if len(events) == 0 {
			t.Error("expected at least one event")
		}
		found := false
		for _, e := range events {
			if e.Type == EventCreated && e.IssueID == "new1" {
				found = true
				if e.Issue == nil {
					t.Error("EventCreated should include Bean")
				}
			}
		}
		if !found {
			t.Errorf("expected EventCreated for new1, got: %+v", events)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for events")
	}
}

func TestSubscribeMultiple(t *testing.T) {
	core, dataDir := setupTestCore(t)

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	// Create two subscribers
	ch1, unsub1 := core.Subscribe()
	defer unsub1()
	ch2, unsub2 := core.Subscribe()
	defer unsub2()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create an issue file
	content := `---
title: Multi Test
status: todo
---
`
	if err := os.WriteFile(filepath.Join(dataDir, "mult--multi.md"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Both subscribers should receive events
	received1, received2 := false, false
	timeout := time.After(500 * time.Millisecond)

	for !received1 || !received2 {
		select {
		case <-ch1:
			received1 = true
		case <-ch2:
			received2 = true
		case <-timeout:
			t.Fatalf("timeout: received1=%v, received2=%v", received1, received2)
		}
	}
}

func TestUnsubscribe(t *testing.T) {
	core, _ := setupTestCore(t)

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	ch, unsub := core.Subscribe()
	unsub()

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestEventTypes(t *testing.T) {
	core, dataDir := setupTestCore(t)

	// Create an initial bean
	createTestBean(t, core, "evt1", "Event Test", "todo")

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	ch, unsub := core.Subscribe()
	defer unsub()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	t.Run("update event", func(t *testing.T) {
		// Modify the existing issue file
		content := `---
title: Updated Title
status: in-progress
---
`
		if err := os.WriteFile(filepath.Join(dataDir, "evt1--event-test.md"), []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		select {
		case events := <-ch:
			found := false
			for _, e := range events {
				if e.Type == EventUpdated && e.IssueID == "evt1" {
					found = true
					if e.Issue == nil {
						t.Error("EventUpdated should include Bean")
					}
					if e.Issue.Title != "Updated Title" {
						t.Errorf("expected updated title, got %q", e.Issue.Title)
					}
				}
			}
			if !found {
				t.Errorf("expected EventUpdated for evt1, got: %+v", events)
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout waiting for update event")
		}
	})

	t.Run("delete event", func(t *testing.T) {
		// Delete the issue file
		if err := os.Remove(filepath.Join(dataDir, "evt1--event-test.md")); err != nil {
			t.Fatalf("failed to delete file: %v", err)
		}

		select {
		case events := <-ch:
			found := false
			for _, e := range events {
				if e.Type == EventDeleted && e.IssueID == "evt1" {
					found = true
					if e.Issue != nil {
						t.Error("EventDeleted should have nil Bean")
					}
				}
			}
			if !found {
				t.Errorf("expected EventDeleted for evt1, got: %+v", events)
			}
		case <-time.After(500 * time.Millisecond):
			t.Error("timeout waiting for delete event")
		}
	})
}

func TestSubscribersClosedOnUnwatch(t *testing.T) {
	core, _ := setupTestCore(t)

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}

	ch, _ := core.Subscribe() // Note: not calling unsub

	// Unwatch should close subscriber channels
	if err := core.Unwatch(); err != nil {
		t.Fatalf("Unwatch() error = %v", err)
	}

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after Unwatch")
	}
}

func TestMultipleChangesInDebounceWindow(t *testing.T) {
	core, dataDir := setupTestCore(t)

	// Create an initial bean to update
	createTestBean(t, core, "upd1", "To Update", "todo")

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	ch, unsub := core.Subscribe()
	defer unsub()

	time.Sleep(50 * time.Millisecond)

	// Make multiple changes rapidly (within debounce window)
	// 1. Create a new bean
	content1 := `---
title: New Bean
status: todo
---
`
	os.WriteFile(filepath.Join(dataDir, "new1--new.md"), []byte(content1), 0644)

	// 2. Update existing issue
	content2 := `---
title: Updated Bean
status: in-progress
---
`
	os.WriteFile(filepath.Join(dataDir, "upd1--to-update.md"), []byte(content2), 0644)

	// 3. Create another issue then delete it (net effect: nothing)
	os.WriteFile(filepath.Join(dataDir, "tmp1--temp.md"), []byte(content1), 0644)
	os.Remove(filepath.Join(dataDir, "tmp1--temp.md"))

	// Wait for debounced events
	select {
	case events := <-ch:
		// Should have events for new1 (created) and upd1 (updated)
		// tmp1 might or might not appear depending on timing
		foundNew := false
		foundUpd := false
		for _, e := range events {
			if e.IssueID == "new1" && e.Type == EventCreated {
				foundNew = true
			}
			if e.IssueID == "upd1" && e.Type == EventUpdated {
				foundUpd = true
			}
		}
		if !foundNew {
			t.Error("expected EventCreated for new1")
		}
		if !foundUpd {
			t.Error("expected EventUpdated for upd1")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for events")
	}

	// Verify state is correct
	_, err := core.Get("new1")
	if err != nil {
		t.Errorf("new1 should exist: %v", err)
	}

	upd, err := core.Get("upd1")
	if err != nil {
		t.Fatalf("upd1 should exist: %v", err)
	}
	if upd.Title != "Updated Bean" {
		t.Errorf("upd1 title = %q, want %q", upd.Title, "Updated Bean")
	}

	// tmp1 should not exist
	_, err = core.Get("tmp1")
	if err != ErrNotFound {
		t.Error("tmp1 should not exist (was created then deleted)")
	}
}

func TestInvalidFileIgnored(t *testing.T) {
	core, dataDir := setupTestCore(t)

	// Create a valid bean first
	createTestBean(t, core, "val1", "Valid Bean", "todo")

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	ch, unsub := core.Subscribe()
	defer unsub()

	time.Sleep(50 * time.Millisecond)

	// Create an invalid bean file (malformed YAML frontmatter)
	invalidContent := `---
title: [unclosed bracket
status: {broken yaml
---
`
	os.WriteFile(filepath.Join(dataDir, "bad1--invalid.md"), []byte(invalidContent), 0644)

	// Also create a valid bean to verify processing continues
	validContent := `---
title: Another Valid
status: todo
---
`
	os.WriteFile(filepath.Join(dataDir, "val2--another.md"), []byte(validContent), 0644)

	// Wait for events
	select {
	case events := <-ch:
		// Should have event for val2 (created), bad1 should be skipped
		foundVal2 := false
		for _, e := range events {
			if e.IssueID == "val2" && e.Type == EventCreated {
				foundVal2 = true
			}
			if e.IssueID == "bad1" {
				t.Error("bad1 should not produce an event (invalid file)")
			}
		}
		if !foundVal2 {
			t.Error("expected EventCreated for val2")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for events")
	}

	// Valid beans should still be accessible
	if _, err := core.Get("val1"); err != nil {
		t.Errorf("val1 should still exist: %v", err)
	}
	if _, err := core.Get("val2"); err != nil {
		t.Errorf("val2 should exist: %v", err)
	}
}

func TestRapidUpdatesToSameFile(t *testing.T) {
	core, dataDir := setupTestCore(t)

	createTestBean(t, core, "rap1", "Rapid Updates", "todo")

	if err := core.StartWatching(); err != nil {
		t.Fatalf("StartWatching() error = %v", err)
	}
	defer core.Unwatch()

	ch, unsub := core.Subscribe()
	defer unsub()

	time.Sleep(50 * time.Millisecond)

	// Write to the same file multiple times rapidly
	for i := 1; i <= 5; i++ {
		content := fmt.Sprintf(`---
title: Update %d
status: todo
---
`, i)
		os.WriteFile(filepath.Join(dataDir, "rap1--rapid-updates.md"), []byte(content), 0644)
		time.Sleep(10 * time.Millisecond) // Small delay but within debounce
	}

	// Should get a single batch of events (debounced)
	select {
	case events := <-ch:
		// Count events for rap1 - should be exactly one
		rap1Count := 0
		var lastEvent IssueEvent
		for _, e := range events {
			if e.IssueID == "rap1" {
				rap1Count++
				lastEvent = e
			}
		}
		if rap1Count != 1 {
			t.Errorf("expected 1 event for rap1, got %d", rap1Count)
		}
		if lastEvent.Type != EventUpdated {
			t.Errorf("expected EventUpdated, got %v", lastEvent.Type)
		}
		// Should have the final value
		if lastEvent.Issue != nil && lastEvent.Issue.Title != "Update 5" {
			t.Errorf("expected title 'Update 5', got %q", lastEvent.Issue.Title)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for events")
	}
}

// Archive functionality tests

func TestArchive(t *testing.T) {
	core, dataDir := setupTestCore(t)

	b := createTestBean(t, core, "arc-001", "To Archive", "completed")
	originalFilename := filepath.Base(b.Path)

	// Archive the issue
	err := core.Archive("arc-001")
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Verify file moved to archive directory
	archivePath := filepath.Join(dataDir, ArchiveDir, originalFilename)
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("bean file should exist in archive directory")
	}

	// Verify file no longer in hash subfolder
	subfolderPath := filepath.Join(dataDir, "a", "arc-001--to-archive.md")
	if _, err := os.Stat(subfolderPath); !os.IsNotExist(err) {
		t.Error("bean file should not exist in hash subfolder")
	}

	// Verify bean is still accessible in memory
	archived, err := core.Get("arc-001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// Verify path is updated
	if archived.Path != filepath.Join(ArchiveDir, "arc-001--to-archive.md") {
		t.Errorf("Path = %q, want %q", archived.Path, filepath.Join(ArchiveDir, "arc-001--to-archive.md"))
	}
}

func TestArchiveIdempotent(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "arc-001", "To Archive", "completed")

	// Archive twice should not error
	if err := core.Archive("arc-001"); err != nil {
		t.Fatalf("first Archive() error = %v", err)
	}
	if err := core.Archive("arc-001"); err != nil {
		t.Fatalf("second Archive() error = %v", err)
	}
}

func TestArchiveNotFound(t *testing.T) {
	core, _ := setupTestCore(t)

	err := core.Archive("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Archive() error = %v, want ErrNotFound", err)
	}
}

func TestUnarchive(t *testing.T) {
	core, dataDir := setupTestCore(t)

	b := createTestBean(t, core, "una-001", "To Unarchive", "completed")
	originalFilename := filepath.Base(b.Path)

	// Archive first
	if err := core.Archive("una-001"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Unarchive
	err := core.Unarchive("una-001")
	if err != nil {
		t.Fatalf("Unarchive() error = %v", err)
	}

	// Verify file moved to hash subfolder
	subfolderPath := filepath.Join(dataDir, "u", originalFilename)
	if _, err := os.Stat(subfolderPath); os.IsNotExist(err) {
		t.Error("bean file should exist in hash subfolder")
	}

	// Verify file no longer in archive
	archivePath := filepath.Join(dataDir, ArchiveDir, originalFilename)
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("bean file should not exist in archive directory")
	}

	// Verify path is updated to hash subfolder path
	unarchived, err := core.Get("una-001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	expectedPath := filepath.Join("u", "una-001--to-unarchive.md")
	if unarchived.Path != expectedPath {
		t.Errorf("Path = %q, want %q", unarchived.Path, expectedPath)
	}
}

func TestUnarchiveIdempotent(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "una-001", "To Unarchive", "completed")

	// Unarchive non-archived bean should not error
	if err := core.Unarchive("una-001"); err != nil {
		t.Fatalf("Unarchive() on non-archived bean error = %v", err)
	}
}

func TestIsArchived(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "isa-001", "Test Archived", "completed")

	t.Run("not archived", func(t *testing.T) {
		if core.IsArchived("isa-001") {
			t.Error("IsArchived() should return false for non-archived bean")
		}
	})

	// Archive the issue
	if err := core.Archive("isa-001"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	t.Run("archived", func(t *testing.T) {
		if !core.IsArchived("isa-001") {
			t.Error("IsArchived() should return true for archived bean")
		}
	})

	t.Run("nonexistent", func(t *testing.T) {
		if core.IsArchived("nonexistent") {
			t.Error("IsArchived() should return false for nonexistent bean")
		}
	})
}

func TestArchivedBeansAlwaysLoaded(t *testing.T) {
	core, dataDir := setupTestCore(t)

	// Create beans and archive one
	createTestBean(t, core, "act-001", "Active Bean", "todo")
	createTestBean(t, core, "arc-001", "Archived Bean", "completed")
	if err := core.Archive("arc-001"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Create a new core and load - archived issues should always be included
	core2 := New(dataDir, config.Default())
	core2.SetWarnWriter(nil)
	if err := core2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	t.Run("all issues loaded including archived", func(t *testing.T) {
		beans := core2.All()
		if len(beans) != 2 {
			t.Errorf("All() returned %d beans, want 2 (active + archived)", len(beans))
		}
	})

	t.Run("active bean accessible", func(t *testing.T) {
		if _, err := core2.Get("act-001"); err != nil {
			t.Errorf("active bean should be found: %v", err)
		}
	})

	t.Run("archived bean accessible", func(t *testing.T) {
		if _, err := core2.Get("arc-001"); err != nil {
			t.Errorf("archived bean should be found: %v", err)
		}
	})

	t.Run("archived bean has correct path", func(t *testing.T) {
		b, _ := core2.Get("arc-001")
		if !core2.IsArchived("arc-001") {
			t.Error("archived bean should be identified as archived")
		}
		if b.Path != "archive/arc-001--archived-bean.md" {
			t.Errorf("archived bean path = %q, want %q", b.Path, "archive/arc-001--archived-bean.md")
		}
	})
}

func TestLoadFromSubdirectories(t *testing.T) {
	// Create a core with beans in various subdirectories
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, DataDir)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create test .beans dir: %v", err)
	}

	// Create subdirectories
	milestone1Dir := filepath.Join(dataDir, "milestone-1")
	milestone2Dir := filepath.Join(dataDir, "milestone-2")
	nestedDir := filepath.Join(dataDir, "epics", "auth")
	if err := os.MkdirAll(milestone1Dir, 0755); err != nil {
		t.Fatalf("failed to create milestone-1 dir: %v", err)
	}
	if err := os.MkdirAll(milestone2Dir, 0755); err != nil {
		t.Fatalf("failed to create milestone-2 dir: %v", err)
	}
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	// Create beans in different locations
	writeTestBeanFile(t, filepath.Join(dataDir, "root1--root-issue.md"), "root1", "Root Bean", "todo")
	writeTestBeanFile(t, filepath.Join(milestone1Dir, "m1b1--milestone-one-issue.md"), "m1b1", "Milestone One Bean", "todo")
	writeTestBeanFile(t, filepath.Join(milestone2Dir, "m2b1--milestone-two-issue.md"), "m2b1", "Milestone Two Bean", "in-progress")
	writeTestBeanFile(t, filepath.Join(nestedDir, "auth1--auth-issue.md"), "auth1", "Auth Bean", "todo")

	// Load and verify all issues are found
	core := New(dataDir, config.Default())
	core.SetWarnWriter(nil)
	if err := core.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	beans := core.All()
	if len(beans) != 4 {
		t.Errorf("All() returned %d beans, want 4", len(beans))
	}

	// Verify each issue is accessible and has correct path
	testCases := []struct {
		id           string
		expectedPath string
	}{
		{"root1", "root1--root-issue.md"},
		{"m1b1", "milestone-1/m1b1--milestone-one-issue.md"},
		{"m2b1", "milestone-2/m2b1--milestone-two-issue.md"},
		{"auth1", "epics/auth/auth1--auth-issue.md"},
	}

	for _, tc := range testCases {
		t.Run(tc.id, func(t *testing.T) {
			b, err := core.Get(tc.id)
			if err != nil {
				t.Fatalf("Get(%q) error = %v", tc.id, err)
			}
			if b.Path != tc.expectedPath {
				t.Errorf("Path = %q, want %q", b.Path, tc.expectedPath)
			}
		})
	}
}

// writeTestBeanFile creates an issue file directly on disk (for testing load scenarios)
func writeTestBeanFile(t *testing.T, path, id, title, status string) {
	t.Helper()
	content := fmt.Sprintf(`---
title: %s
status: %s
type: task
---

Test bean content.
`, title, status)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test bean file: %v", err)
	}
}

func TestGetFromArchive(t *testing.T) {
	core, dataDir := setupTestCore(t)

	createTestBean(t, core, "gfa-001", "Get From Archive", "completed")
	if err := core.Archive("gfa-001"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Create a new core - archived issues are loaded but GetFromArchive reads directly from disk
	core2 := New(dataDir, config.Default())
	core2.SetWarnWriter(nil)
	if err := core2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	t.Run("bean in archive", func(t *testing.T) {
		b, err := core2.GetFromArchive("gfa-001")
		if err != nil {
			t.Fatalf("GetFromArchive() error = %v", err)
		}
		if b == nil {
			t.Fatal("GetFromArchive() returned nil")
		}
		if b.ID != "gfa-001" {
			t.Errorf("ID = %q, want %q", b.ID, "gfa-001")
		}
	})

	t.Run("bean not in archive", func(t *testing.T) {
		b, err := core2.GetFromArchive("nonexistent")
		if err != nil {
			t.Fatalf("GetFromArchive() error = %v", err)
		}
		if b != nil {
			t.Error("GetFromArchive() should return nil for nonexistent bean")
		}
	})

	t.Run("no archive directory", func(t *testing.T) {
		// Create a fresh core with no archive
		tmpDir := t.TempDir()
		freshBeansDir := filepath.Join(tmpDir, DataDir)
		if err := os.MkdirAll(freshBeansDir, 0755); err != nil {
			t.Fatalf("failed to create .beans dir: %v", err)
		}
		core3 := New(freshBeansDir, config.Default())
		core3.SetWarnWriter(nil)
		if err := core3.Load(); err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		b, err := core3.GetFromArchive("anything")
		if err != nil {
			t.Fatalf("GetFromArchive() error = %v", err)
		}
		if b != nil {
			t.Error("GetFromArchive() should return nil when archive doesn't exist")
		}
	})
}

func TestLoadAndUnarchive(t *testing.T) {
	core, dataDir := setupTestCore(t)

	createTestBean(t, core, "lau-001", "Load And Unarchive", "completed")
	if err := core.Archive("lau-001"); err != nil {
		t.Fatalf("Archive() error = %v", err)
	}

	// Create a new core - archived issues are now always loaded
	core2 := New(dataDir, config.Default())
	core2.SetWarnWriter(nil)
	if err := core2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Bean should be accessible (archived issues are always loaded)
	b, err := core2.Get("lau-001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !core2.IsArchived("lau-001") {
		t.Error("bean should be identified as archived before LoadAndUnarchive")
	}

	// Load and unarchive should move the file
	unarchived, err := core2.LoadAndUnarchive("lau-001")
	if err != nil {
		t.Fatalf("LoadAndUnarchive() error = %v", err)
	}
	if unarchived == nil {
		t.Fatal("LoadAndUnarchive() returned nil")
	}
	if unarchived.ID != b.ID {
		t.Errorf("LoadAndUnarchive returned different bean: got %q, want %q", unarchived.ID, b.ID)
	}

	// Bean should no longer be archived
	if core2.IsArchived("lau-001") {
		t.Error("bean should not be archived after LoadAndUnarchive")
	}

	// File should be in hash subfolder, not archive
	subfolderPath := filepath.Join(dataDir, "l", "lau-001--load-and-unarchive.md")
	if _, err := os.Stat(subfolderPath); os.IsNotExist(err) {
		t.Error("bean file should exist in hash subfolder after LoadAndUnarchive")
	}

	// File should NOT be in archive directory
	archivePath := filepath.Join(dataDir, "archive", "lau-001--load-and-unarchive.md")
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Error("bean file should not exist in archive directory after LoadAndUnarchive")
	}
}

func TestLoadAndUnarchiveNotFound(t *testing.T) {
	core, _ := setupTestCore(t)

	_, err := core.LoadAndUnarchive("nonexistent")
	if err != ErrNotFound {
		t.Errorf("LoadAndUnarchive() error = %v, want ErrNotFound", err)
	}
}


func TestNormalizeID(t *testing.T) {
	core, _ := setupTestCore(t)

	createTestBean(t, core, "abc-def", "Test Bean", "todo")

	t.Run("exact match returns same ID", func(t *testing.T) {
		normalized, found := core.NormalizeID("abc-def")
		if !found {
			t.Error("NormalizeID() should find exact match")
		}
		if normalized != "abc-def" {
			t.Errorf("NormalizeID() = %q, want %q", normalized, "abc-def")
		}
	})

	t.Run("nonexistent ID returns original", func(t *testing.T) {
		normalized, found := core.NormalizeID("nonexistent")
		if found {
			t.Error("NormalizeID() should not find nonexistent ID")
		}
		if normalized != "nonexistent" {
			t.Errorf("NormalizeID() = %q, want %q", normalized, "nonexistent")
		}
	})
}

func TestUpdateWithETag(t *testing.T) {
	core, _ := setupTestCore(t)

	t.Run("update with correct etag succeeds", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-test-1",
			Title:  "ETag Test",
			Status: "todo",
			Body:   "Original",
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		currentETag := b.ETag()
		b.Title = "Updated"
		err := core.Update(b, &currentETag)
		if err != nil {
			t.Errorf("Update() with correct etag failed: %v", err)
		}
	})

	t.Run("update with wrong etag fails", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-test-2",
			Title:  "ETag Test",
			Status: "todo",
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		wrongETag := "wrongetag123"
		b.Title = "Should Fail"
		err := core.Update(b, &wrongETag)

		if _, ok := errors.AsType[*ETagMismatchError](err); !ok {
			t.Errorf("Update() with wrong etag should return ETagMismatchError, got %T: %v", err, err)
		}
	})

	t.Run("update without etag succeeds when not required", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-test-3",
			Title:  "ETag Test",
			Status: "todo",
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		b.Title = "No ETag"
		err := core.Update(b, nil)
		if err != nil {
			t.Errorf("Update() without etag failed: %v", err)
		}
	})
}

func TestUpdateWithETagRequired(t *testing.T) {
	core, _ := setupTestCoreWithRequireIfMatch(t)

	t.Run("update without etag fails when required", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-req-test-1",
			Title:  "ETag Required Test",
			Status: "todo",
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		b.Title = "Should Fail"
		err := core.Update(b, nil)

		if _, ok := errors.AsType[*ETagRequiredError](err); !ok {
			t.Errorf("Update() without etag should return ETagRequiredError when required, got %T: %v", err, err)
		}
	})

	t.Run("update with empty etag fails when required", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-req-test-2",
			Title:  "ETag Required Test",
			Status: "todo",
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		emptyETag := ""
		b.Title = "Should Fail"
		err := core.Update(b, &emptyETag)

		if _, ok := errors.AsType[*ETagRequiredError](err); !ok {
			t.Errorf("Update() with empty etag should return ETagRequiredError when required, got %T: %v", err, err)
		}
	})

	t.Run("update with correct etag succeeds even when required", func(t *testing.T) {
		b := &issue.Issue{
			ID:     "etag-req-test-3",
			Title:  "ETag Required Test",
			Status: "todo",
		}
		if err := core.Create(b); err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		currentETag := b.ETag()
		b.Title = "Success"
		err := core.Update(b, &currentETag)
		if err != nil {
			t.Errorf("Update() with correct etag failed: %v", err)
		}
	})
}
func TestUpdateWithETagDebug(t *testing.T) {
	core, _ := setupTestCore(t)

	b := &issue.Issue{
		ID:     "etag-debug",
		Title:  "ETag Test",
		Status: "todo",
		Body:   "Original",
	}
	if err := core.Create(b); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	etagAfterCreate := b.ETag()
	t.Logf("ETag after create: %s", etagAfterCreate)

	// Get from core to see what's stored
	stored, _ := core.Get("etag-debug")
	storedEtag := stored.ETag()
	t.Logf("ETag of stored bean: %s", storedEtag)

	// Modify our local copy
	b.Title = "Updated"
	modifiedEtag := b.ETag()
	t.Logf("ETag of modified local bean: %s", modifiedEtag)

	// What will Update see?
	err := core.Update(b, &etagAfterCreate)
	if err != nil {
		t.Logf("Update failed: %v", err)
	}
}
