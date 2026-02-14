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
						t.Error("sync section was lost")
					}
				},
				func(t *testing.T, c string) {
					if strings.Contains(c, "\nextensions:") || strings.HasPrefix(c, "extensions:") {
						t.Error("config still contains 'extensions:' key")
					}
				},
				func(t *testing.T, c string) {
					if !strings.Contains(c, "sync:") {
						t.Error("config missing 'sync:' key")
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

			if _, err := MigrateConfig(configPath); err != nil {
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

	t.Run("renames extensions to sync in frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
`)

		writeFile(t, filepath.Join(dir, ".beans", "beans-xxxx--linked.md"), `---
# beans-xxxx
title: Linked task
status: todo
type: task
extensions:
  clickup:
    task_id: "abc123"
    synced_at: "2025-01-15T10:00:00Z"
---

Linked body.
`)

		result, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err != nil {
			t.Fatal(err)
		}

		if result.SyncKeyRenamed != 1 {
			t.Errorf("SyncKeyRenamed = %d, want 1", result.SyncKeyRenamed)
		}

		content := readFile(t, filepath.Join(dir, ".issues", "beans", "beans-xxxx--linked.md"))
		if strings.Contains(content, "extensions:") {
			t.Error("migrated bean still has extensions: key")
		}
		if !strings.Contains(content, "sync:") {
			t.Error("migrated bean missing sync: key")
		}
		// Verify the nested data is preserved
		if !strings.Contains(content, "clickup:") {
			t.Error("clickup section lost during migration")
		}
		if !strings.Contains(content, "task_id:") {
			t.Error("task_id lost during migration")
		}
	})

	t.Run("imports clickup config from bean-me-up", func(t *testing.T) {
		dir := t.TempDir()

		// Create old config (no extensions)
		writeFile(t, filepath.Join(dir, ".todo.yml"), `beans:
  path: .beans
  default_status: todo
  prefix: beans
  id_length: 4
`)

		// Create standalone .beans.clickup.yml
		writeFile(t, filepath.Join(dir, ".beans.clickup.yml"), `beans:
  clickup:
    list_id: "901234567890"
    status_mapping:
      todo: "to do"
      in-progress: "in progress"
      completed: "complete"
`)

		// Create old data directory with a bean
		writeFile(t, filepath.Join(dir, ".beans", "beans-abcd--test.md"), `---
# beans-abcd
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

		if !result.ClickUpImported {
			t.Error("ClickUpImported = false, want true")
		}

		// Verify ClickUp config was merged into main config
		content := readFile(t, filepath.Join(dir, ".todo.yml"))
		if !strings.Contains(content, "clickup:") {
			t.Error("config missing clickup section after migration")
		}
		if !strings.Contains(content, `"901234567890"`) {
			t.Error("config missing list_id after migration")
		}
		// Status mapping key should be converted
		if strings.Contains(content, "todo:") {
			t.Error("status mapping still has 'todo' key")
		}
		if !strings.Contains(content, "ready:") {
			t.Error("status mapping missing 'ready' key")
		}
	})

	t.Run("renames .beans.yml to .todo.yml", func(t *testing.T) {
		dir := t.TempDir()

		// Create .beans.yml (no .todo.yml)
		writeFile(t, filepath.Join(dir, ".beans.yml"), `beans:
  path: .beans
  default_status: todo
  prefix: bup-
  id_length: 4
extensions:
  clickup:
    list_id: "123"
`)

		// Create old data directory
		writeFile(t, filepath.Join(dir, ".beans", "bup-xxxx--test.md"), `---
# bup-xxxx
title: Test
status: todo
type: task
extensions:
  clickup:
    task_id: "abc"
---
`)

		result, err := Run(Options{
			ConfigPath: filepath.Join(dir, ".todo.yml"),
		})
		if err != nil {
			t.Fatal(err)
		}

		// .beans.yml should no longer exist
		if _, err := os.Stat(filepath.Join(dir, ".beans.yml")); err == nil {
			t.Error(".beans.yml still exists after migration")
		}

		// .todo.yml should exist with migrated content
		configContent := readFile(t, filepath.Join(dir, ".todo.yml"))
		if strings.Contains(configContent, "beans:") {
			t.Error("config still contains 'beans:' key")
		}
		if !strings.Contains(configContent, "issues:") {
			t.Error("config missing 'issues:' key")
		}
		if strings.Contains(configContent, "prefix:") {
			t.Error("config still contains 'prefix'")
		}
		if strings.Contains(configContent, "id_length:") {
			t.Error("config still contains 'id_length'")
		}
		if !strings.Contains(configContent, "default_status: ready") {
			t.Error("default_status not converted to ready")
		}
		if !strings.Contains(configContent, "sync:") {
			t.Error("config missing 'sync:' key")
		}
		if strings.Contains(configContent, "\nextensions:") || strings.HasPrefix(configContent, "extensions:") {
			t.Error("config still contains 'extensions:' key")
		}

		if result.ActiveCount != 1 {
			t.Errorf("ActiveCount = %d, want 1", result.ActiveCount)
		}
		if !result.ConfigMigrated {
			t.Error("ConfigMigrated = false, want true")
		}

		// Verify bean was migrated with conversions
		content := readFile(t, filepath.Join(dir, ".issues", "beans", "bup-xxxx--test.md"))
		if !strings.Contains(content, "status: ready") {
			t.Error("bean status not converted")
		}
		if !strings.Contains(content, "sync:") {
			t.Error("bean extensions: not renamed to sync:")
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

func TestImportClickUpConfig(t *testing.T) {
	t.Run("imports from standalone .beans.clickup.yml", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)
		writeFile(t, filepath.Join(dir, ".beans.clickup.yml"), `beans:
  clickup:
    list_id: "901234567890"
    status_mapping:
      todo: "to do"
      in-progress: "in progress"
      completed: "complete"
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !imported {
			t.Fatal("expected ClickUp config to be imported")
		}

		content := readFile(t, configPath)
		if !strings.Contains(content, "clickup:") {
			t.Error("config missing clickup section")
		}
		if !strings.Contains(content, "list_id:") {
			t.Error("config missing list_id")
		}
		if !strings.Contains(content, `"901234567890"`) {
			t.Error("config missing list_id value")
		}
	})

	t.Run("imports from .beans.yml extensions.clickup", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)
		writeFile(t, filepath.Join(dir, ".beans.yml"), `beans:
  path: .beans
extensions:
  clickup:
    list_id: "555666777"
    status_mapping:
      todo: "open"
      completed: "done"
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !imported {
			t.Fatal("expected ClickUp config to be imported")
		}

		content := readFile(t, configPath)
		if !strings.Contains(content, "clickup:") {
			t.Error("config missing clickup section")
		}
		if !strings.Contains(content, `"555666777"`) {
			t.Error("config missing list_id value")
		}
	})

	t.Run("skips when sync.clickup already in main config", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
sync:
  clickup:
    list_id: "existing"
`)
		writeFile(t, filepath.Join(dir, ".beans.clickup.yml"), `beans:
  clickup:
    list_id: "should-not-overwrite"
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if imported {
			t.Error("should not import when ClickUp config already exists")
		}

		content := readFile(t, configPath)
		if strings.Contains(content, "should-not-overwrite") {
			t.Error("existing ClickUp config was overwritten")
		}
	})

	t.Run("skips when neither file exists", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if imported {
			t.Error("should not import when no source files exist")
		}
	})

	t.Run("skips when no list_id", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)
		writeFile(t, filepath.Join(dir, ".beans.clickup.yml"), `beans:
  clickup:
    status_mapping:
      todo: "to do"
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if imported {
			t.Error("should not import when list_id is missing")
		}
	})

	t.Run("prefers standalone over inline", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)
		writeFile(t, filepath.Join(dir, ".beans.clickup.yml"), `beans:
  clickup:
    list_id: "from-standalone"
`)
		writeFile(t, filepath.Join(dir, ".beans.yml"), `beans:
  path: .beans
extensions:
  clickup:
    list_id: "from-inline"
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !imported {
			t.Fatal("expected ClickUp config to be imported")
		}

		content := readFile(t, configPath)
		if !strings.Contains(content, "from-standalone") {
			t.Error("should prefer standalone .beans.clickup.yml")
		}
		if strings.Contains(content, "from-inline") {
			t.Error("should not use inline config when standalone exists")
		}
	})

	t.Run("converts todo to ready in status mapping keys", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".todo.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)
		writeFile(t, filepath.Join(dir, ".beans.clickup.yml"), `beans:
  clickup:
    list_id: "123"
    status_mapping:
      todo: "to do"
      in-progress: "in progress"
      completed: "complete"
`)

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !imported {
			t.Fatal("expected ClickUp config to be imported")
		}

		content := readFile(t, configPath)
		// The key "todo" should be renamed to "ready"
		if strings.Contains(content, "todo:") {
			t.Error("status mapping still has 'todo' key")
		}
		if !strings.Contains(content, "ready:") {
			t.Error("status mapping missing 'ready' key")
		}
		// The value "to do" should be preserved
		if !strings.Contains(content, "to do") {
			t.Error("status mapping value 'to do' was lost")
		}
	})

	t.Run("skips .beans.yml when it is the same as configPath", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, ".beans.yml")
		writeFile(t, configPath, `issues:
  path: .issues
`)
		// No standalone file, and .beans.yml IS the config file
		// so it should not try to read extensions from itself

		imported, err := ImportClickUpConfig(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if imported {
			t.Error("should not import from self")
		}
	})
}

func TestRewriteExtensionsKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		renamed bool
	}{
		{
			name: "renames extensions to sync",
			input: `---
title: Test bean
status: in-progress
extensions:
  clickup:
    task_id: "abc123"
---

Body content.
`,
			want: `---
title: Test bean
status: in-progress
sync:
  clickup:
    task_id: "abc123"
---

Body content.
`,
			renamed: true,
		},
		{
			name: "no extensions key",
			input: `---
title: Test bean
status: in-progress
---

Body content.
`,
			want: `---
title: Test bean
status: in-progress
---

Body content.
`,
			renamed: false,
		},
		{
			name: "extensions in body not affected",
			input: `---
title: Test bean
status: ready
---

The extensions: key should not change here.
`,
			want: `---
title: Test bean
status: ready
---

The extensions: key should not change here.
`,
			renamed: false,
		},
		{
			name: "no frontmatter",
			input:   "Just some text with extensions: mentioned",
			want:    "Just some text with extensions: mentioned",
			renamed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, renamed := rewriteExtensionsKey([]byte(tt.input))
			if string(got) != tt.want {
				t.Errorf("content mismatch:\ngot:  %q\nwant: %q", string(got), tt.want)
			}
			if renamed != tt.renamed {
				t.Errorf("renamed = %v, want %v", renamed, tt.renamed)
			}
		})
	}
}

func TestConvertStatusMappingKeys(t *testing.T) {
	t.Run("renames todo to ready", func(t *testing.T) {
		section := map[string]any{
			"list_id": "123",
			"status_mapping": map[string]any{
				"todo":        "to do",
				"in-progress": "in progress",
				"completed":   "complete",
			},
		}
		convertStatusMappingKeys(section)
		sm := section["status_mapping"].(map[string]any)
		if _, ok := sm["todo"]; ok {
			t.Error("todo key should be removed")
		}
		if val, ok := sm["ready"]; !ok || val != "to do" {
			t.Errorf("ready key should have value 'to do', got %v", val)
		}
		if val := sm["in-progress"]; val != "in progress" {
			t.Error("other keys should be preserved")
		}
	})

	t.Run("preserves existing ready key", func(t *testing.T) {
		section := map[string]any{
			"status_mapping": map[string]any{
				"todo":  "backlog",
				"ready": "open",
			},
		}
		convertStatusMappingKeys(section)
		sm := section["status_mapping"].(map[string]any)
		if _, ok := sm["todo"]; ok {
			t.Error("todo key should be removed")
		}
		if val := sm["ready"]; val != "open" {
			t.Errorf("existing ready value should be preserved, got %v", val)
		}
	})

	t.Run("handles no status_mapping", func(t *testing.T) {
		section := map[string]any{
			"list_id": "123",
		}
		// Should not panic
		convertStatusMappingKeys(section)
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
