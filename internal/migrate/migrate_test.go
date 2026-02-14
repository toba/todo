package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to create a file with content in a temp dir
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRewriteStatus(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		converted bool
	}{
		{
			name: "basic todo to ready",
			input: `---
title: Test bean
status: todo
type: task
---

Some body content.
`,
			want: `---
title: Test bean
status: ready
type: task
---

Some body content.
`,
			converted: true,
		},
		{
			name: "quoted todo to ready",
			input: `---
title: Test bean
status: "todo"
type: task
---
`,
			want: `---
title: Test bean
status: ready
type: task
---
`,
			converted: true,
		},
		{
			name: "single-quoted todo to ready",
			input: `---
title: Test bean
status: 'todo'
type: task
---
`,
			want: `---
title: Test bean
status: ready
type: task
---
`,
			converted: true,
		},
		{
			name: "non-todo status unchanged",
			input: `---
title: Test bean
status: in-progress
type: task
---
`,
			want: `---
title: Test bean
status: in-progress
type: task
---
`,
			converted: false,
		},
		{
			name: "completed status unchanged",
			input: `---
title: Done bean
status: completed
type: bug
---
`,
			want: `---
title: Done bean
status: completed
type: bug
---
`,
			converted: false,
		},
		{
			name: "no frontmatter",
			input:     "Just some text",
			want:      "Just some text",
			converted: false,
		},
		{
			name: "preserves body with todo mention",
			input: `---
title: Test
status: todo
---

This body mentions todo items but should not be changed.
status: todo should stay here.
`,
			want: `---
title: Test
status: ready
---

This body mentions todo items but should not be changed.
status: todo should stay here.
`,
			converted: true,
		},
		{
			name: "preserves ID comment",
			input: `---
# beans-1234
title: Test bean
status: todo
type: task
---
`,
			want: `---
# beans-1234
title: Test bean
status: ready
type: task
---
`,
			converted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, converted := rewriteStatus([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
			if converted != tt.converted {
				t.Errorf("converted = %v, want %v", converted, tt.converted)
			}
		})
	}
}

func TestMigrateConfig(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantChecks []func(t *testing.T, content string)
	}{
		{
			name: "beans key renamed to issues",
			input: `beans:
  path: .beans
  default_status: todo
  prefix: beans
  id_length: 4
extensions:
  clickup:
    list_id: "123"
`,
			wantChecks: []func(t *testing.T, content string){
				func(t *testing.T, c string) {
					if strings.Contains(c, "beans:") {
						t.Error("config still contains 'beans:' key")
					}
				},
				func(t *testing.T, c string) {
					if !strings.Contains(c, "issues:") {
						t.Error("config missing 'issues:' key")
					}
				},
				func(t *testing.T, c string) {
					if strings.Contains(c, "prefix:") {
						t.Error("config still contains 'prefix'")
					}
				},
				func(t *testing.T, c string) {
					if strings.Contains(c, "id_length:") {
						t.Error("config still contains 'id_length'")
					}
				},
				func(t *testing.T, c string) {
					if strings.Contains(c, "default_status: todo") {
						t.Error("config still has default_status: todo")
					}
					if !strings.Contains(c, "default_status: ready") {
						t.Error("config missing default_status: ready")
					}
				},
				func(t *testing.T, c string) {
					if !strings.Contains(c, "clickup:") {
						t.Error("extensions section was lost")
					}
				},
				func(t *testing.T, c string) {
					if !strings.Contains(c, "path: .issues") {
						t.Error("config path not updated from .beans to .issues")
					}
				},
			},
		},
		{
			name: "already has issues key - no double migration",
			input: `issues:
  path: .issues
  default_status: ready
`,
			wantChecks: []func(t *testing.T, content string){
				func(t *testing.T, c string) {
					if !strings.Contains(c, "issues:") {
						t.Error("config missing 'issues:' key")
					}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, ".todo.yml")
			writeFile(t, configPath, tt.input)

			if err := MigrateConfig(configPath); err != nil {
				t.Fatal(err)
			}

			content := readFile(t, configPath)
			for _, check := range tt.wantChecks {
				check(t, content)
			}
		})
	}
}

func TestRun(t *testing.T) {
	t.Run("basic migration", func(t *testing.T) {
		dir := t.TempDir()

		// Create old config
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
  default_status: todo
  prefix: beans
  id_length: 4
`)

		// Create old data directory with beans
		beansDir := filepath.Join(dir, ".beans")
		writeFile(t, filepath.Join(beansDir, "beans-abcd--my-task.md"), `---
# beans-abcd
title: My task
status: todo
type: task
created_at: 2025-01-15T10:00:00Z
updated_at: 2025-01-15T10:00:00Z
---

Task description here.
`)
		writeFile(t, filepath.Join(beansDir, "beans-efgh--in-progress-task.md"), `---
# beans-efgh
title: In progress task
status: in-progress
type: bug
created_at: 2025-01-16T10:00:00Z
updated_at: 2025-01-16T10:00:00Z
parent: beans-abcd
---

This is in progress.
`)

		// Create archived bean
		writeFile(t, filepath.Join(beansDir, "archive", "beans-ijkl--done-task.md"), `---
# beans-ijkl
title: Done task
status: completed
type: task
created_at: 2025-01-14T10:00:00Z
updated_at: 2025-01-17T10:00:00Z
---

This was completed.
`)

		// Run migration
		result, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err != nil {
			t.Fatal(err)
		}

		// Check counts
		if result.ActiveCount != 2 {
			t.Errorf("ActiveCount = %d, want 2", result.ActiveCount)
		}
		if result.ArchivedCount != 1 {
			t.Errorf("ArchivedCount = %d, want 1", result.ArchivedCount)
		}
		if result.StatusConverted != 1 {
			t.Errorf("StatusConverted = %d, want 1", result.StatusConverted)
		}
		if !result.ConfigMigrated {
			t.Error("ConfigMigrated = false, want true")
		}

		// Verify files exist in new locations
		issuesDir := filepath.Join(dir, ".issues")
		if _, err := os.Stat(filepath.Join(issuesDir, "beans", "beans-abcd--my-task.md")); err != nil {
			t.Errorf("active bean not found in .issues/beans/: %v", err)
		}
		if _, err := os.Stat(filepath.Join(issuesDir, "beans", "beans-efgh--in-progress-task.md")); err != nil {
			t.Errorf("in-progress bean not found in .issues/beans/: %v", err)
		}
		if _, err := os.Stat(filepath.Join(issuesDir, "archive", "beans-ijkl--done-task.md")); err != nil {
			t.Errorf("archived bean not found in .issues/archive/: %v", err)
		}

		// Verify status conversion in migrated file
		content := readFile(t, filepath.Join(issuesDir, "beans", "beans-abcd--my-task.md"))
		if strings.Contains(content, "status: todo") {
			t.Error("bean still has status: todo after migration")
		}
		if !strings.Contains(content, "status: ready") {
			t.Error("bean missing status: ready after migration")
		}

		// Verify non-todo status is preserved
		content2 := readFile(t, filepath.Join(issuesDir, "beans", "beans-efgh--in-progress-task.md"))
		if !strings.Contains(content2, "status: in-progress") {
			t.Error("in-progress status was incorrectly modified")
		}

		// Verify cross-references are preserved
		if !strings.Contains(content2, "parent: beans-abcd") {
			t.Error("parent cross-reference was lost")
		}

		// Verify archived bean status is preserved (completed stays completed)
		content3 := readFile(t, filepath.Join(issuesDir, "archive", "beans-ijkl--done-task.md"))
		if !strings.Contains(content3, "status: completed") {
			t.Error("archived bean status was incorrectly modified")
		}

		// Verify body content is preserved
		if !strings.Contains(content, "Task description here.") {
			t.Error("body content was lost during migration")
		}

		// Verify config was rewritten
		configContent := readFile(t, filepath.Join(dir, ".todo.yml"))
		if strings.Contains(configContent, "beans:") {
			t.Error("config still contains 'beans:' key")
		}
		if !strings.Contains(configContent, "issues:") {
			t.Error("config missing 'issues:' key")
		}
	})

	t.Run("explicit source directory", func(t *testing.T) {
		dir := t.TempDir()

		// Config with non-standard path
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: custom-beans
  default_status: todo
`)

		// Create data in the explicit source
		srcDir := filepath.Join(dir, "my-old-beans")
		writeFile(t, filepath.Join(srcDir, "beans-test--hello.md"), `---
# beans-test
title: Hello
status: todo
type: task
---
`)

		result, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
			SourceDir:  srcDir,
		})
		if err != nil {
			t.Fatal(err)
		}

		if result.ActiveCount != 1 {
			t.Errorf("ActiveCount = %d, want 1", result.ActiveCount)
		}

		if _, err := os.Stat(filepath.Join(dir, ".issues", "beans", "beans-test--hello.md")); err != nil {
			t.Errorf("migrated file not found: %v", err)
		}
	})

	t.Run("source dir does not exist", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
`)

		_, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err == nil {
			t.Error("expected error for missing source directory")
		}
		if !strings.Contains(err.Error(), "source directory does not exist") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("target already has beans", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
`)

		// Create old source
		writeFile(t, filepath.Join(dir, ".beans", "beans-xxxx--test.md"), `---
title: Test
status: todo
---
`)

		// Pre-existing target with a bean file
		writeFile(t, filepath.Join(dir, ".issues", "existing.md"), `---
title: Existing
status: ready
---
`)

		_, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err == nil {
			t.Error("expected error when target already has beans")
		}
		if !strings.Contains(err.Error(), "already contains bean files") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("no archive directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
`)

		writeFile(t, filepath.Join(dir, ".beans", "beans-xxxx--test.md"), `---
title: Test
status: todo
type: task
---
`)

		result, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err != nil {
			t.Fatal(err)
		}

		if result.ActiveCount != 1 {
			t.Errorf("ActiveCount = %d, want 1", result.ActiveCount)
		}
		if result.ArchivedCount != 0 {
			t.Errorf("ArchivedCount = %d, want 0", result.ArchivedCount)
		}
	})

	t.Run("archived todo bean gets converted", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
`)

		writeFile(t, filepath.Join(dir, ".beans", "archive", "beans-arch--old.md"), `---
# beans-arch
title: Old archived todo
status: todo
type: task
---
`)

		result, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err != nil {
			t.Fatal(err)
		}

		if result.ArchivedCount != 1 {
			t.Errorf("ArchivedCount = %d, want 1", result.ArchivedCount)
		}
		if result.StatusConverted != 1 {
			t.Errorf("StatusConverted = %d, want 1", result.StatusConverted)
		}

		content := readFile(t, filepath.Join(dir, ".issues", "archive", "beans-arch--old.md"))
		if !strings.Contains(content, "status: ready") {
			t.Error("archived todo bean was not converted to ready")
		}
	})
}

func TestHasExistingBeans(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if hasExistingBeans(dir) {
			t.Error("empty dir should not have existing beans")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		if hasExistingBeans("/nonexistent/path") {
			t.Error("nonexistent dir should not have existing beans")
		}
	})

	t.Run("directory with md files", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "test.md"), "content")
		if !hasExistingBeans(dir) {
			t.Error("dir with .md files should report existing beans")
		}
	})

	t.Run("directory with nested md files", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "sub", "test.md"), "content")
		if !hasExistingBeans(dir) {
			t.Error("dir with nested .md files should report existing beans")
		}
	})

	t.Run("directory with non-md files only", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gitignore"), "*.tmp")
		if hasExistingBeans(dir) {
			t.Error("dir with only non-md files should not report existing beans")
		}
	})
}
