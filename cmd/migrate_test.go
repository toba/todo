package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateCommand(t *testing.T) {
	t.Run("end-to-end migration", func(t *testing.T) {
		dir := t.TempDir()

		// Create old config
		configPath := filepath.Join(dir, ".todo.yml")
		os.WriteFile(configPath, []byte(`beans:
  path: .beans
  default_status: todo
  prefix: beans
  id_length: 4
`), 0644)

		// Create old data directory
		beansDir := filepath.Join(dir, ".beans")
		os.MkdirAll(beansDir, 0755)
		os.WriteFile(filepath.Join(beansDir, "beans-aaaa--task-one.md"), []byte(`---
# beans-aaaa
title: Task One
status: todo
type: task
created_at: 2025-06-01T10:00:00Z
updated_at: 2025-06-01T10:00:00Z
---

First task body.
`), 0644)
		os.WriteFile(filepath.Join(beansDir, "beans-bbbb--task-two.md"), []byte(`---
# beans-bbbb
title: Task Two
status: in-progress
type: feature
created_at: 2025-06-02T10:00:00Z
updated_at: 2025-06-02T10:00:00Z
parent: beans-aaaa
blocking:
    - beans-cccc
---

Second task body.
`), 0644)

		// Create archived bean
		archiveDir := filepath.Join(beansDir, "archive")
		os.MkdirAll(archiveDir, 0755)
		os.WriteFile(filepath.Join(archiveDir, "beans-cccc--done-task.md"), []byte(`---
# beans-cccc
title: Done Task
status: completed
type: task
---
`), 0644)

		// Execute migrate command
		rootCmd.SetArgs([]string{"migrate", "--config", configPath, "--json"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("migrate command failed: %v", err)
		}

		// Verify bean files exist in new locations
		issuesDir := filepath.Join(dir, ".issues")

		activeBean1 := filepath.Join(issuesDir, "beans", "beans-aaaa--task-one.md")
		if _, err := os.Stat(activeBean1); err != nil {
			t.Errorf("active bean 1 not migrated: %v", err)
		}

		activeBean2 := filepath.Join(issuesDir, "beans", "beans-bbbb--task-two.md")
		if _, err := os.Stat(activeBean2); err != nil {
			t.Errorf("active bean 2 not migrated: %v", err)
		}

		archivedBean := filepath.Join(issuesDir, "archive", "beans-cccc--done-task.md")
		if _, err := os.Stat(archivedBean); err != nil {
			t.Errorf("archived bean not migrated: %v", err)
		}

		// Verify status conversion
		data, _ := os.ReadFile(activeBean1)
		content := string(data)
		if strings.Contains(content, "status: todo") {
			t.Error("bean 1 still has status: todo")
		}
		if !strings.Contains(content, "status: ready") {
			t.Error("bean 1 missing status: ready")
		}

		// Verify non-todo status preserved
		data2, _ := os.ReadFile(activeBean2)
		content2 := string(data2)
		if !strings.Contains(content2, "status: in-progress") {
			t.Error("bean 2 status was incorrectly changed")
		}

		// Verify relationships preserved
		if !strings.Contains(content2, "parent: beans-aaaa") {
			t.Error("parent relationship lost")
		}
		if !strings.Contains(content2, "beans-cccc") {
			t.Error("blocking relationship lost")
		}

		// Verify config migration
		cfgData, _ := os.ReadFile(configPath)
		cfgContent := string(cfgData)
		if strings.Contains(cfgContent, "beans:") {
			t.Error("config still has 'beans:' key")
		}
		if !strings.Contains(cfgContent, "issues:") {
			t.Error("config missing 'issues:' key")
		}
		if strings.Contains(cfgContent, "prefix:") {
			t.Error("config still has prefix")
		}
	})

	t.Run("migration with clickup config", func(t *testing.T) {
		dir := t.TempDir()

		// Create old config
		configPath := filepath.Join(dir, ".todo.yml")
		os.WriteFile(configPath, []byte(`beans:
  path: .beans
  default_status: todo
  prefix: beans
  id_length: 4
`), 0644)

		// Create standalone ClickUp config
		os.WriteFile(filepath.Join(dir, ".beans.clickup.yml"), []byte(`beans:
  clickup:
    list_id: "901234567890"
    status_mapping:
      todo: "to do"
      in-progress: "in progress"
      completed: "complete"
`), 0644)

		// Create old data directory
		beansDir := filepath.Join(dir, ".beans")
		os.MkdirAll(beansDir, 0755)
		os.WriteFile(filepath.Join(beansDir, "beans-aaaa--task-one.md"), []byte(`---
# beans-aaaa
title: Task One
status: todo
type: task
---

Task body.
`), 0644)

		// Execute migrate command
		rootCmd.SetArgs([]string{"migrate", "--config", configPath, "--json"})
		err := rootCmd.Execute()
		if err != nil {
			t.Fatalf("migrate command failed: %v", err)
		}

		// Verify ClickUp config was imported
		cfgData, _ := os.ReadFile(configPath)
		cfgContent := string(cfgData)
		if !strings.Contains(cfgContent, "clickup:") {
			t.Error("config missing clickup section")
		}
		if !strings.Contains(cfgContent, `"901234567890"`) {
			t.Error("config missing list_id")
		}
		// Status mapping key should be converted
		if strings.Contains(cfgContent, "todo:") {
			t.Error("status mapping still has 'todo' key after migration")
		}
		if !strings.Contains(cfgContent, "ready:") {
			t.Error("status mapping missing 'ready' key after migration")
		}
	})
}
