package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestPollForChanges(t *testing.T) {
	// Create a temp directory structure mimicking .issues/
	root := t.TempDir()
	subdir := filepath.Join(root, "ab")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create an initial .md file
	file1 := filepath.Join(subdir, "ab-cd01--test-issue.md")
	if err := os.WriteFile(file1, []byte("# initial"), 0o644); err != nil {
		t.Fatal(err)
	}

	c := &Core{root: root}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	// Seed the mtime map
	mtimes := c.snapshotMtimes()

	if len(mtimes) != 1 {
		t.Fatalf("expected 1 entry in mtimes, got %d", len(mtimes))
	}
	if _, ok := mtimes[file1]; !ok {
		t.Fatalf("expected %s in mtimes", file1)
	}

	// Poll with no changes — should return empty
	changes := c.pollForChanges(mtimes, watcher)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes, got %d", len(changes))
	}

	// Modify the file (ensure mtime changes by waiting briefly)
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(file1, []byte("# modified"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes = c.pollForChanges(mtimes, watcher)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change after modify, got %d", len(changes))
	}
	if op, ok := changes[file1]; !ok || op != fsnotify.Write {
		t.Fatalf("expected Write op for %s, got %v", file1, changes)
	}

	// Poll again with no further changes
	changes = c.pollForChanges(mtimes, watcher)
	if len(changes) != 0 {
		t.Fatalf("expected 0 changes after re-poll, got %d", len(changes))
	}

	// Create a new file in a new subdirectory
	newSubdir := filepath.Join(root, "ef")
	if err := os.MkdirAll(newSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	file2 := filepath.Join(newSubdir, "ef-gh23--new-issue.md")
	if err := os.WriteFile(file2, []byte("# new"), 0o644); err != nil {
		t.Fatal(err)
	}

	changes = c.pollForChanges(mtimes, watcher)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change after create, got %d", len(changes))
	}
	if op, ok := changes[file2]; !ok || op != fsnotify.Create {
		t.Fatalf("expected Create op for %s, got %v", file2, changes)
	}

	// Verify mtimes map now has both files
	if len(mtimes) != 2 {
		t.Fatalf("expected 2 entries in mtimes, got %d", len(mtimes))
	}

	// Delete a file
	if err := os.Remove(file1); err != nil {
		t.Fatal(err)
	}

	changes = c.pollForChanges(mtimes, watcher)
	if len(changes) != 1 {
		t.Fatalf("expected 1 change after delete, got %d", len(changes))
	}
	if op, ok := changes[file1]; !ok || op != fsnotify.Remove {
		t.Fatalf("expected Remove op for %s, got %v", file1, changes)
	}

	// Verify mtimes map now has only file2
	if len(mtimes) != 1 {
		t.Fatalf("expected 1 entry in mtimes after delete, got %d", len(mtimes))
	}
	if _, ok := mtimes[file2]; !ok {
		t.Fatalf("expected %s to remain in mtimes", file2)
	}
}

func TestSnapshotMtimes(t *testing.T) {
	root := t.TempDir()

	// Create nested structure
	sub1 := filepath.Join(root, "aa")
	sub2 := filepath.Join(root, "bb")
	if err := os.MkdirAll(sub1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub2, 0o755); err != nil {
		t.Fatal(err)
	}

	f1 := filepath.Join(sub1, "aa-0001--one.md")
	f2 := filepath.Join(sub2, "bb-0002--two.md")
	txt := filepath.Join(sub1, "notes.txt") // non-.md file, should be ignored

	for _, f := range []string{f1, f2, txt} {
		if err := os.WriteFile(f, []byte("content"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	c := &Core{root: root}
	mtimes := c.snapshotMtimes()

	if len(mtimes) != 2 {
		t.Fatalf("expected 2 .md files in snapshot, got %d", len(mtimes))
	}
	if _, ok := mtimes[f1]; !ok {
		t.Fatalf("expected %s in snapshot", f1)
	}
	if _, ok := mtimes[f2]; !ok {
		t.Fatalf("expected %s in snapshot", f2)
	}
	if _, ok := mtimes[txt]; ok {
		t.Fatalf("did not expect %s (non-.md) in snapshot", txt)
	}
}
