package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.DefaultStatus != "ready" {
		t.Errorf("DefaultStatus = %q, want \"ready\"", cfg.DefaultStatus)
	}
	if cfg.DefaultType != "task" {
		t.Errorf("DefaultType = %q, want \"task\"", cfg.DefaultType)
	}
	// Both types and statuses are hardcoded
	if len(DefaultTypes) != 5 {
		t.Errorf("len(DefaultTypes) = %d, want 5", len(DefaultTypes))
	}
	if len(DefaultStatuses) != 5 {
		t.Errorf("len(DefaultStatuses) = %d, want 5", len(DefaultStatuses))
	}
}

func TestIsValidStatus(t *testing.T) {
	cfg := Default()

	tests := []struct {
		status string
		want   bool
	}{
		{"draft", true},
		{"ready", true},
		{"in-progress", true},
		{"completed", true},
		{"scrapped", true},
		{"invalid", false},
		{"", false},
		{"READY", false}, // case sensitive
		// Old status names should no longer be valid
		{"open", false},
		{"done", false},
		{"todo", false},
		{"not-ready", false},
		{"backlog", false}, // renamed to draft
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := cfg.IsValidStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestStatusList(t *testing.T) {
	cfg := Default()
	got := cfg.StatusList()
	want := "in-progress, ready, draft, completed, scrapped"

	if got != want {
		t.Errorf("StatusList() = %q, want %q", got, want)
	}
}

func TestStatusNames(t *testing.T) {
	cfg := Default()
	got := cfg.StatusNames()

	if len(got) != 5 {
		t.Fatalf("len(StatusNames()) = %d, want 5", len(got))
	}
	expected := []string{"in-progress", "ready", "draft", "completed", "scrapped"}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("StatusNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestGetStatus(t *testing.T) {
	cfg := Default()

	t.Run("existing status", func(t *testing.T) {
		s := cfg.GetStatus("ready")
		if s == nil {
			t.Fatal("GetStatus(\"ready\") = nil, want non-nil")
		}
		if s.Name != "ready" {
			t.Errorf("Name = %q, want \"ready\"", s.Name)
		}
		if s.Color != "green" {
			t.Errorf("Color = %q, want \"green\"", s.Color)
		}
	})

	t.Run("non-existing status", func(t *testing.T) {
		s := cfg.GetStatus("invalid")
		if s != nil {
			t.Errorf("GetStatus(\"invalid\") = %v, want nil", s)
		}
	})

	t.Run("old status names not valid", func(t *testing.T) {
		s := cfg.GetStatus("open")
		if s != nil {
			t.Errorf("GetStatus(\"open\") = %v, want nil (old status name)", s)
		}
		s = cfg.GetStatus("done")
		if s != nil {
			t.Errorf("GetStatus(\"done\") = %v, want nil (old status name)", s)
		}
		s = cfg.GetStatus("todo")
		if s != nil {
			t.Errorf("GetStatus(\"todo\") = %v, want nil (old status name)", s)
		}
	})
}

func TestGetDefaultStatus(t *testing.T) {
	cfg := Default()
	got := cfg.GetDefaultStatus()

	if got != "ready" {
		t.Errorf("GetDefaultStatus() = %q, want \"ready\"", got)
	}
}

func TestGetDefaultType(t *testing.T) {
	cfg := Default()
	got := cfg.GetDefaultType()

	if got != "task" {
		t.Errorf("GetDefaultType() = %q, want \"task\"", got)
	}
}

func TestIsArchiveStatus(t *testing.T) {
	cfg := Default()

	tests := []struct {
		status string
		want   bool
	}{
		{"completed", true},
		{"scrapped", true},
		{"draft", false},
		{"todo", false},
		{"in-progress", false},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			got := cfg.IsArchiveStatus(tt.status)
			if got != tt.want {
				t.Errorf("IsArchiveStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestLoadNonExistent(t *testing.T) {
	// Load from non-existent directory should return defaults
	cfg, err := Load("/nonexistent/path/that/does/not/exist")
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}

	// Should have default values
	if cfg.Path != DefaultDataPath {
		t.Errorf("Path = %q, want %q", cfg.Path, DefaultDataPath)
	}
}

func TestLoadAndSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a config (statuses are no longer stored in config)
	cfg := &Config{
		Path:        ".todo",
		DefaultType: "bug",
	}
	cfg.SetConfigDir(tmpDir)

	// Save it
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load it back
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify values
	if loaded.DefaultType != "bug" {
		t.Errorf("DefaultType = %q, want \"bug\"", loaded.DefaultType)
	}
	// Statuses are hardcoded, not stored in config
	if len(loaded.StatusNames()) != 5 {
		t.Errorf("len(StatusNames()) = %d, want 5", len(loaded.StatusNames()))
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	// Create temp directory with minimal config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ConfigFileName)

	// Write minimal config (missing default_type) in new .toba.yaml format
	minimalConfig := `todo:
    path: ".issues"
`
	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Load it
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Statuses are hardcoded, always 5
	if len(cfg.StatusNames()) != 5 {
		t.Errorf("Hardcoded statuses: got %d, want 5", len(cfg.StatusNames()))
	}
	// DefaultStatus is always "ready"
	if cfg.GetDefaultStatus() != "ready" {
		t.Errorf("DefaultStatus: got %q, want \"ready\"", cfg.GetDefaultStatus())
	}
	// DefaultType should be first type name when not specified
	if cfg.DefaultType != "milestone" {
		t.Errorf("DefaultType default not applied: got %q, want \"milestone\"", cfg.DefaultType)
	}
}

func TestStatusesAreHardcoded(t *testing.T) {
	// Statuses are hardcoded and not configurable (like types)
	// Verify that any config only uses hardcoded statuses
	cfg := Default()

	// All hardcoded statuses should be valid
	hardcodedStatuses := []string{"draft", "ready", "in-progress", "completed", "scrapped"}
	for _, status := range hardcodedStatuses {
		if !cfg.IsValidStatus(status) {
			t.Errorf("IsValidStatus(%q) = false, want true", status)
		}
	}

	// Archive statuses should be completed and scrapped
	if !cfg.IsArchiveStatus("completed") {
		t.Error("IsArchiveStatus(\"completed\") = false, want true")
	}
	if !cfg.IsArchiveStatus("scrapped") {
		t.Error("IsArchiveStatus(\"scrapped\") = false, want true")
	}
	if cfg.IsArchiveStatus("ready") {
		t.Error("IsArchiveStatus(\"ready\") = true, want false")
	}
}

func TestIsValidType(t *testing.T) {
	cfg := Default()

	tests := []struct {
		typeName string
		want     bool
	}{
		{"epic", true},
		{"milestone", true},
		{"feature", true},
		{"bug", true},
		{"task", true},
		{"invalid", false},
		{"", false},
		{"TASK", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			got := cfg.IsValidType(tt.typeName)
			if got != tt.want {
				t.Errorf("IsValidType(%q) = %v, want %v", tt.typeName, got, tt.want)
			}
		})
	}
}

func TestTypeList(t *testing.T) {
	cfg := Default()
	got := cfg.TypeList()
	want := "milestone, epic, bug, feature, task"

	if got != want {
		t.Errorf("TypeList() = %q, want %q", got, want)
	}
}

func TestGetType(t *testing.T) {
	cfg := Default()

	t.Run("existing type", func(t *testing.T) {
		typ := cfg.GetType("bug")
		if typ == nil {
			t.Fatal("GetType(\"bug\") = nil, want non-nil")
		}
		if typ.Name != "bug" {
			t.Errorf("Name = %q, want \"bug\"", typ.Name)
		}
		if typ.Color != "red" {
			t.Errorf("Color = %q, want \"red\"", typ.Color)
		}
	})

	t.Run("non-existing type", func(t *testing.T) {
		// GetType returns nil for unknown types
		typ := cfg.GetType("invalid-type")
		if typ != nil {
			t.Errorf("GetType(\"invalid-type\") = %v, want nil", typ)
		}
	})

	t.Run("all hardcoded types exist", func(t *testing.T) {
		expectedTypes := []string{"milestone", "epic", "bug", "feature", "task"}
		for _, typeName := range expectedTypes {
			typ := cfg.GetType(typeName)
			if typ == nil {
				t.Errorf("GetType(%q) = nil, want non-nil", typeName)
			}
		}
	})
}

func TestTypesAreHardcoded(t *testing.T) {
	// Types are hardcoded and not stored in config
	// Verify that saving and loading a config doesn't affect types

	tmpDir := t.TempDir()

	cfg := &Config{
		Path:        ".todo",
		DefaultType: "task",
	}
	cfg.SetConfigDir(tmpDir)

	// Save it
	if err := cfg.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Load it back
	configPath := filepath.Join(tmpDir, ConfigFileName)
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Types should always come from DefaultTypes, not config
	if len(loaded.TypeNames()) != 5 {
		t.Errorf("len(TypeNames()) = %d, want 5", len(loaded.TypeNames()))
	}

	// All default types should be accessible
	for _, typeName := range []string{"milestone", "epic", "bug", "feature", "task"} {
		if !loaded.IsValidType(typeName) {
			t.Errorf("IsValidType(%q) = false, want true", typeName)
		}
	}

	// Statuses should also be hardcoded
	if len(loaded.StatusNames()) != 5 {
		t.Errorf("len(StatusNames()) = %d, want 5", len(loaded.StatusNames()))
	}
}

func TestTypeDescriptions(t *testing.T) {
	t.Run("hardcoded types have descriptions", func(t *testing.T) {
		cfg := Default()

		expectedDescriptions := map[string]string{
			"epic":      "A thematic container for related work; should have child issues, not be worked on directly",
			"milestone": "A target release or checkpoint; group work that should ship together",
			"feature":   "A user-facing capability or enhancement",
			"bug":       "Something that is broken and needs fixing",
			"task":      "A concrete piece of work to complete (eg. a chore, or a sub-task for a feature)",
		}

		for typeName, expectedDesc := range expectedDescriptions {
			typ := cfg.GetType(typeName)
			if typ == nil {
				t.Errorf("GetType(%q) = nil, want non-nil", typeName)
				continue
			}
			if typ.Description != expectedDesc {
				t.Errorf("Type %q description = %q, want %q", typeName, typ.Description, expectedDesc)
			}
		}
	})

	t.Run("types in config file are ignored", func(t *testing.T) {
		// Even if a config file has custom types, they should be ignored
		// and hardcoded types should be used instead
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		// Config with custom types (should be ignored)
		configYAML := `todo:
    default_type: task
    default_status: open
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Custom type should not be valid
		if loaded.IsValidType("custom-type") {
			t.Error("IsValidType(\"custom-type\") = true, want false (custom types should be ignored)")
		}

		// Hardcoded types should still work
		if !loaded.IsValidType("bug") {
			t.Error("IsValidType(\"bug\") = false, want true")
		}
	})
}

func TestStatusDescriptions(t *testing.T) {
	t.Run("hardcoded statuses have descriptions", func(t *testing.T) {
		cfg := Default()

		expectedDescriptions := map[string]string{
			"draft":       "Needs refinement before it can be worked on",
			"ready":       "Ready to be worked on",
			"in-progress": "Currently being worked on",
			"completed":   "Finished successfully",
			"scrapped":    "Will not be done",
		}

		for statusName, expectedDesc := range expectedDescriptions {
			status := cfg.GetStatus(statusName)
			if status == nil {
				t.Errorf("GetStatus(%q) = nil, want non-nil", statusName)
				continue
			}
			if status.Description != expectedDesc {
				t.Errorf("Status %q description = %q, want %q", statusName, status.Description, expectedDesc)
			}
		}
	})

	t.Run("statuses in config file are ignored", func(t *testing.T) {
		// Even if a config file has custom statuses, they should be ignored
		// and hardcoded statuses should be used instead
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)

		// Config with custom statuses (should be ignored)
		configYAML := `todo:
    default_type: task
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		loaded, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		// Custom status should not be valid
		if loaded.IsValidStatus("custom-status") {
			t.Error("IsValidStatus(\"custom-status\") = true, want false (custom statuses should be ignored)")
		}

		// Hardcoded statuses should still work
		if !loaded.IsValidStatus("ready") {
			t.Error("IsValidStatus(\"ready\") = false, want true")
		}
	})
}

func TestFindConfig(t *testing.T) {
	t.Run("finds config in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte("todo:\n    path: .issues\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		found, err := FindConfig(tmpDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}
		if found != configPath {
			t.Errorf("FindConfig() = %q, want %q", found, configPath)
		}
	})

	t.Run("finds config in parent directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "sub", "dir")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatalf("MkdirAll error = %v", err)
		}

		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte("todo:\n    path: .issues\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		found, err := FindConfig(subDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}
		if found != configPath {
			t.Errorf("FindConfig() = %q, want %q", found, configPath)
		}
	})

	t.Run("returns empty string when no config found", func(t *testing.T) {
		tmpDir := t.TempDir()

		found, err := FindConfig(tmpDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}
		if found != "" {
			t.Errorf("FindConfig() = %q, want empty string", found)
		}
	})

	t.Run("finds and migrates legacy .todo.yml", func(t *testing.T) {
		tmpDir := t.TempDir()
		legacyPath := filepath.Join(tmpDir, LegacyConfigFileName)
		if err := os.WriteFile(legacyPath, []byte("issues:\n  path: .issues\n  default_type: bug\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		found, err := FindConfig(tmpDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}

		// Should return the new .toba.yaml path
		expectedPath := filepath.Join(tmpDir, ConfigFileName)
		if found != expectedPath {
			t.Errorf("FindConfig() = %q, want %q", found, expectedPath)
		}

		// New file should exist
		if _, err := os.Stat(expectedPath); err != nil {
			t.Errorf(".toba.yaml was not created: %v", err)
		}

		// Legacy file should be removed
		if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
			t.Error("legacy .todo.yml was not removed after migration")
		}

		// Should be loadable in new format
		cfg, err := Load(found)
		if err != nil {
			t.Fatalf("Load() after migration error = %v", err)
		}
		if cfg.DefaultType != "bug" {
			t.Errorf("DefaultType = %q, want \"bug\"", cfg.DefaultType)
		}
	})

	t.Run("prefers .toba.yaml over .todo.yml", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Write both files
		newPath := filepath.Join(tmpDir, ConfigFileName)
		legacyPath := filepath.Join(tmpDir, LegacyConfigFileName)
		if err := os.WriteFile(newPath, []byte("todo:\n    default_type: feature\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}
		if err := os.WriteFile(legacyPath, []byte("issues:\n  default_type: bug\n"), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		found, err := FindConfig(tmpDir)
		if err != nil {
			t.Fatalf("FindConfig() error = %v", err)
		}
		if found != newPath {
			t.Errorf("FindConfig() = %q, want %q (should prefer .toba.yaml)", found, newPath)
		}
	})
}

func TestLoadFromDirectory(t *testing.T) {
	t.Run("loads config from directory with .toba.yaml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    path: custom-issues
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := LoadFromDirectory(tmpDir)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if cfg.Path != "custom-issues" {
			t.Errorf("Path = %q, want \"custom-issues\"", cfg.Path)
		}
	})

	t.Run("returns default config when no config file exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg, err := LoadFromDirectory(tmpDir)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if cfg.Path != DefaultDataPath {
			t.Errorf("Path = %q, want %q", cfg.Path, DefaultDataPath)
		}
		if cfg.ConfigDir() != tmpDir {
			t.Errorf("ConfigDir() = %q, want %q", cfg.ConfigDir(), tmpDir)
		}
	})
}

func TestResolveDataPath(t *testing.T) {
	t.Run("resolves relative path from config directory", func(t *testing.T) {
		cfg := &Config{
			Path: "custom-data",
		}
		cfg.SetConfigDir("/project/root")

		got := cfg.ResolveDataPath()
		want := "/project/root/custom-data"
		if got != want {
			t.Errorf("ResolveDataPath() = %q, want %q", got, want)
		}
	})

	t.Run("returns absolute path unchanged", func(t *testing.T) {
		cfg := &Config{
			Path: "/absolute/path/to/data",
		}
		cfg.SetConfigDir("/project/root")

		got := cfg.ResolveDataPath()
		want := "/absolute/path/to/data"
		if got != want {
			t.Errorf("ResolveDataPath() = %q, want %q", got, want)
		}
	})

	t.Run("uses default .issues path", func(t *testing.T) {
		cfg := Default()
		cfg.SetConfigDir("/project/root")

		got := cfg.ResolveDataPath()
		want := "/project/root/.issues"
		if got != want {
			t.Errorf("ResolveDataPath() = %q, want %q", got, want)
		}
	})
}

func TestDefaultHasIssuesPath(t *testing.T) {
	cfg := Default()
	if cfg.Path != DefaultDataPath {
		t.Errorf("Default().Path = %q, want %q", cfg.Path, DefaultDataPath)
	}
}

func TestIsValidPriority(t *testing.T) {
	cfg := Default()

	tests := []struct {
		priority string
		want     bool
	}{
		{"critical", true},
		{"high", true},
		{"normal", true},
		{"low", true},
		{"deferred", true},
		{"", true}, // empty is valid (means no priority)
		{"invalid", false},
		{"CRITICAL", false}, // case sensitive
		{"medium", false},   // not a valid priority
	}

	for _, tt := range tests {
		t.Run(tt.priority, func(t *testing.T) {
			got := cfg.IsValidPriority(tt.priority)
			if got != tt.want {
				t.Errorf("IsValidPriority(%q) = %v, want %v", tt.priority, got, tt.want)
			}
		})
	}
}

func TestPriorityList(t *testing.T) {
	cfg := Default()
	got := cfg.PriorityList()
	want := "critical, high, normal, low, deferred"

	if got != want {
		t.Errorf("PriorityList() = %q, want %q", got, want)
	}
}

func TestPriorityNames(t *testing.T) {
	cfg := Default()
	got := cfg.PriorityNames()

	if len(got) != 5 {
		t.Fatalf("len(PriorityNames()) = %d, want 5", len(got))
	}
	expected := []string{"critical", "high", "normal", "low", "deferred"}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("PriorityNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestGetPriority(t *testing.T) {
	cfg := Default()

	t.Run("existing priority", func(t *testing.T) {
		p := cfg.GetPriority("high")
		if p == nil {
			t.Fatal("GetPriority(\"high\") = nil, want non-nil")
		}
		if p.Name != "high" {
			t.Errorf("Name = %q, want \"high\"", p.Name)
		}
		if p.Color != "yellow" {
			t.Errorf("Color = %q, want \"yellow\"", p.Color)
		}
	})

	t.Run("non-existing priority", func(t *testing.T) {
		p := cfg.GetPriority("invalid")
		if p != nil {
			t.Errorf("GetPriority(\"invalid\") = %v, want nil", p)
		}
	})

	t.Run("empty priority returns nil", func(t *testing.T) {
		p := cfg.GetPriority("")
		if p != nil {
			t.Errorf("GetPriority(\"\") = %v, want nil", p)
		}
	})
}

func TestPriorityDescriptions(t *testing.T) {
	cfg := Default()

	expectedDescriptions := map[string]string{
		"critical": "Urgent, blocking work. When possible, address immediately",
		"high":     "Important, should be done before normal work",
		"normal":   "Standard priority",
		"low":      "Less important, can be delayed",
		"deferred": "Explicitly pushed back, avoid doing unless necessary",
	}

	for priorityName, expectedDesc := range expectedDescriptions {
		p := cfg.GetPriority(priorityName)
		if p == nil {
			t.Errorf("GetPriority(%q) = nil, want non-nil", priorityName)
			continue
		}
		if p.Description != expectedDesc {
			t.Errorf("Priority %q description = %q, want %q", priorityName, p.Description, expectedDesc)
		}
	}
}

func TestDefaultPrioritiesCount(t *testing.T) {
	if len(DefaultPriorities) != 5 {
		t.Errorf("len(DefaultPriorities) = %d, want 5", len(DefaultPriorities))
	}
}

func TestGetEditor(t *testing.T) {
	t.Run("returns empty when not set", func(t *testing.T) {
		cfg := Default()
		if got := cfg.GetEditor(); got != "" {
			t.Errorf("GetEditor() = %q, want empty", got)
		}
	})

	t.Run("returns configured value", func(t *testing.T) {
		cfg := Default()
		cfg.Editor = "code --wait"
		if got := cfg.GetEditor(); got != "code --wait" {
			t.Errorf("GetEditor() = %q, want \"code --wait\"", got)
		}
	})

	t.Run("loads from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    default_type: task
    editor: "vim"
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetEditor() != "vim" {
			t.Errorf("GetEditor() = %q, want \"vim\"", cfg.GetEditor())
		}
	})

	t.Run("empty in YAML returns empty", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    default_type: task
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetEditor() != "" {
			t.Errorf("GetEditor() = %q, want empty", cfg.GetEditor())
		}
	})
}

func TestGetDefaultSort(t *testing.T) {
	t.Run("returns default when not set", func(t *testing.T) {
		cfg := Default()
		got := cfg.GetDefaultSort()
		if got != "default" {
			t.Errorf("GetDefaultSort() = %q, want \"default\"", got)
		}
	})

	t.Run("returns configured value", func(t *testing.T) {
		cfg := Default()
		cfg.DefaultSort = "updated"
		got := cfg.GetDefaultSort()
		if got != "updated" {
			t.Errorf("GetDefaultSort() = %q, want \"updated\"", got)
		}
	})

	t.Run("loads from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    default_type: task
    default_sort: created
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetDefaultSort() != "created" {
			t.Errorf("GetDefaultSort() = %q, want \"created\"", cfg.GetDefaultSort())
		}
	})

	t.Run("empty in YAML returns default", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    default_type: task
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if cfg.GetDefaultSort() != "default" {
			t.Errorf("GetDefaultSort() = %q, want \"default\"", cfg.GetDefaultSort())
		}
	})
}

func TestSyncConfig(t *testing.T) {
	t.Run("returns nil for default config", func(t *testing.T) {
		cfg := Default()
		if got := cfg.SyncConfig("clickup"); got != nil {
			t.Errorf("SyncConfig(\"clickup\") = %v, want nil", got)
		}
	})

	t.Run("returns nil for unknown extension", func(t *testing.T) {
		cfg := &Config{
			Sync: map[string]map[string]any{
				"clickup": {"list_id": "123"},
			},
		}
		if got := cfg.SyncConfig("jira"); got != nil {
			t.Errorf("SyncConfig(\"jira\") = %v, want nil", got)
		}
	})

	t.Run("returns extension config data", func(t *testing.T) {
		cfg := &Config{
			Sync: map[string]map[string]any{
				"clickup": {
					"list_id": "123",
					"status_mapping": map[string]any{
						"todo": "to do",
					},
				},
			},
		}
		got := cfg.SyncConfig("clickup")
		if got == nil {
			t.Fatal("SyncConfig(\"clickup\") = nil, want non-nil")
		}
		if got["list_id"] != "123" {
			t.Errorf("SyncConfig(\"clickup\")[\"list_id\"] = %v, want \"123\"", got["list_id"])
		}
	})

	t.Run("loads from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    default_type: task
    sync:
        clickup:
            list_id: "456"
            workspace: "my-workspace"
        jira:
            project_key: "PROJ"
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		clickup := cfg.SyncConfig("clickup")
		if clickup == nil {
			t.Fatal("SyncConfig(\"clickup\") = nil, want non-nil")
		}
		if clickup["list_id"] != "456" {
			t.Errorf("clickup[\"list_id\"] = %v, want \"456\"", clickup["list_id"])
		}
		if clickup["workspace"] != "my-workspace" {
			t.Errorf("clickup[\"workspace\"] = %v, want \"my-workspace\"", clickup["workspace"])
		}

		jira := cfg.SyncConfig("jira")
		if jira == nil {
			t.Fatal("SyncConfig(\"jira\") = nil, want non-nil")
		}
		if jira["project_key"] != "PROJ" {
			t.Errorf("jira[\"project_key\"] = %v, want \"PROJ\"", jira["project_key"])
		}

		// Unknown extension returns nil
		if got := cfg.SyncConfig("unknown"); got != nil {
			t.Errorf("SyncConfig(\"unknown\") = %v, want nil", got)
		}
	})

	t.Run("no sync section in YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `todo:
    default_type: task
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got := cfg.SyncConfig("clickup"); got != nil {
			t.Errorf("SyncConfig(\"clickup\") = %v, want nil", got)
		}
	})
}
