package refry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRewriteStatus(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "basic todo status",
			in:   "---\ntitle: Test\nstatus: todo\ntype: task\n---\nBody text",
			want: "---\ntitle: Test\nstatus: ready\ntype: task\n---\nBody text",
		},
		{
			name: "double-quoted todo",
			in:   "---\nstatus: \"todo\"\n---\nBody",
			want: "---\nstatus: ready\n---\nBody",
		},
		{
			name: "single-quoted todo",
			in:   "---\nstatus: 'todo'\n---\nBody",
			want: "---\nstatus: ready\n---\nBody",
		},
		{
			name: "non-todo status unchanged",
			in:   "---\nstatus: in-progress\n---\nBody",
			want: "---\nstatus: in-progress\n---\nBody",
		},
		{
			name: "no frontmatter",
			in:   "Just a plain file\nstatus: todo\n",
			want: "Just a plain file\nstatus: todo\n",
		},
		{
			name: "todo in body not changed",
			in:   "---\nstatus: ready\n---\nstatus: todo in body",
			want: "---\nstatus: ready\n---\nstatus: todo in body",
		},
		{
			name: "status with extra spaces",
			in:   "---\nstatus:  todo\n---\nBody",
			want: "---\nstatus:  ready\n---\nBody",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteStatus(tt.in)
			if got != tt.want {
				t.Errorf("rewriteStatus() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	t.Run("basic migration", func(t *testing.T) {
		dir := t.TempDir()

		// Create .beans.yml
		beansYml := `beans:
  path: .beans
  prefix: beans
  id_length: 4
  default_status: todo
  default_type: task
`
		writeFile(t, filepath.Join(dir, ".beans.yml"), beansYml)

		// Create .beans/ directory with some issues
		beansDir := filepath.Join(dir, ".beans")
		must(t, os.MkdirAll(beansDir, 0755))

		issue1 := "---\ntitle: First issue\nstatus: todo\ntype: task\n---\nBody of first issue"
		writeFile(t, filepath.Join(beansDir, "beans-0ajg--first-issue.md"), issue1)

		issue2 := "---\ntitle: Second issue\nstatus: in-progress\ntype: bug\n---\nBody of second issue"
		writeFile(t, filepath.Join(beansDir, "beans-1xyz--second-issue.md"), issue2)

		result, err := Run(Options{ProjectDir: dir})
		must(t, err)

		// Check counts
		if result.ActiveCount != 2 {
			t.Errorf("ActiveCount = %d, want 2", result.ActiveCount)
		}
		if result.StatusConverted != 1 {
			t.Errorf("StatusConverted = %d, want 1", result.StatusConverted)
		}
		if !result.ConfigMigrated {
			t.Error("ConfigMigrated = false, want true")
		}

		// Check files are bucketed
		assertFileExists(t, filepath.Join(dir, ".issues", "b", "beans-0ajg--first-issue.md"))
		assertFileExists(t, filepath.Join(dir, ".issues", "b", "beans-1xyz--second-issue.md"))

		// Check status was rewritten in first issue
		content := readFile(t, filepath.Join(dir, ".issues", "b", "beans-0ajg--first-issue.md"))
		assertContains(t, content, "status: ready")
		assertNotContains(t, content, "status: todo")

		// Check second issue status unchanged
		content2 := readFile(t, filepath.Join(dir, ".issues", "b", "beans-1xyz--second-issue.md"))
		assertContains(t, content2, "status: in-progress")

		// Check .toba.yaml created
		assertFileExists(t, filepath.Join(dir, ".toba.yaml"))

		// Check .beans.yml removed
		assertFileNotExists(t, filepath.Join(dir, ".beans.yml"))

		// Check .beans/ removed
		assertFileNotExists(t, filepath.Join(dir, ".beans"))
	})

	t.Run("with archive", func(t *testing.T) {
		dir := t.TempDir()

		writeFile(t, filepath.Join(dir, ".beans.yml"), "beans:\n  path: .beans\n  default_status: todo\n")

		beansDir := filepath.Join(dir, ".beans")
		must(t, os.MkdirAll(filepath.Join(beansDir, "archive"), 0755))

		writeFile(t, filepath.Join(beansDir, "beans-aaaa--active.md"),
			"---\nstatus: todo\n---\nActive")
		writeFile(t, filepath.Join(beansDir, "archive", "beans-bbbb--done.md"),
			"---\nstatus: todo\n---\nArchived")

		result, err := Run(Options{ProjectDir: dir})
		must(t, err)

		if result.ActiveCount != 1 {
			t.Errorf("ActiveCount = %d, want 1", result.ActiveCount)
		}
		if result.ArchivedCount != 1 {
			t.Errorf("ArchivedCount = %d, want 1", result.ArchivedCount)
		}
		if result.StatusConverted != 2 {
			t.Errorf("StatusConverted = %d, want 2", result.StatusConverted)
		}

		// Active file bucketed
		assertFileExists(t, filepath.Join(dir, ".issues", "b", "beans-aaaa--active.md"))

		// Archived file stays in archive/
		assertFileExists(t, filepath.Join(dir, ".issues", "archive", "beans-bbbb--done.md"))

		// Archived file status rewritten
		content := readFile(t, filepath.Join(dir, ".issues", "archive", "beans-bbbb--done.md"))
		assertContains(t, content, "status: ready")
	})

	t.Run("target exists with md files", func(t *testing.T) {
		dir := t.TempDir()

		writeFile(t, filepath.Join(dir, ".beans.yml"), "beans:\n  path: .beans\n")

		beansDir := filepath.Join(dir, ".beans")
		must(t, os.MkdirAll(beansDir, 0755))
		writeFile(t, filepath.Join(beansDir, "beans-aaaa--test.md"), "---\nstatus: todo\n---\n")

		issuesDir := filepath.Join(dir, ".issues")
		must(t, os.MkdirAll(issuesDir, 0755))
		writeFile(t, filepath.Join(issuesDir, "existing.md"), "existing")

		_, err := Run(Options{ProjectDir: dir})
		if err == nil {
			t.Fatal("expected error when target has .md files")
		}
		assertContains(t, err.Error(), "already contains .md files")
	})

	t.Run("source missing", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".beans.yml"), "beans:\n  path: .beans\n")

		_, err := Run(Options{ProjectDir: dir})
		if err == nil {
			t.Fatal("expected error when source dir missing")
		}
		assertContains(t, err.Error(), "source directory does not exist")
	})

	t.Run("no config", func(t *testing.T) {
		dir := t.TempDir()

		_, err := Run(Options{ProjectDir: dir})
		if err == nil {
			t.Fatal("expected error when .beans.yml missing")
		}
		assertContains(t, err.Error(), beansConfigFile)
	})

	t.Run("explicit source", func(t *testing.T) {
		dir := t.TempDir()

		writeFile(t, filepath.Join(dir, ".beans.yml"), "beans:\n  path: .beans\n  default_status: todo\n")

		customDir := filepath.Join(dir, "custom-beans")
		must(t, os.MkdirAll(customDir, 0755))
		writeFile(t, filepath.Join(customDir, "beans-cccc--custom.md"),
			"---\nstatus: todo\n---\nCustom")

		result, err := Run(Options{ProjectDir: dir, SourceDir: customDir})
		must(t, err)

		if result.ActiveCount != 1 {
			t.Errorf("ActiveCount = %d, want 1", result.ActiveCount)
		}

		assertFileExists(t, filepath.Join(dir, ".issues", "b", "beans-cccc--custom.md"))
	})
}

// Test helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	must(t, os.WriteFile(path, []byte(content), 0644))
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return string(data)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

func assertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to not exist: %s", path)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !contains(s, substr) {
		t.Errorf("expected %q to contain %q", s, substr)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if contains(s, substr) {
		t.Errorf("expected %q to not contain %q", s, substr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
