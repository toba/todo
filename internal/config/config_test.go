package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Issues.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", cfg.Issues.IDLength)
	}
	if cfg.Issues.Prefix != "" {
		t.Errorf("Prefix = %q, want empty", cfg.Issues.Prefix)
	}
	if cfg.Issues.DefaultStatus != "todo" {
		t.Errorf("DefaultStatus = %q, want \"todo\"", cfg.Issues.DefaultStatus)
	}
	if cfg.Issues.DefaultType != "task" {
		t.Errorf("DefaultType = %q, want \"task\"", cfg.Issues.DefaultType)
	}
	// Both types and statuses are hardcoded
	if len(DefaultTypes) != 5 {
		t.Errorf("len(DefaultTypes) = %d, want 5", len(DefaultTypes))
	}
	if len(DefaultStatuses) != 5 {
		t.Errorf("len(DefaultStatuses) = %d, want 5", len(DefaultStatuses))
	}
}

func TestDefaultWithPrefix(t *testing.T) {
	cfg := DefaultWithPrefix("myapp-")

	if cfg.Issues.Prefix != "myapp-" {
		t.Errorf("Prefix = %q, want \"myapp-\"", cfg.Issues.Prefix)
	}
	// Other defaults should still apply
	if cfg.Issues.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", cfg.Issues.IDLength)
	}
}

func TestIsValidStatus(t *testing.T) {
	cfg := Default()

	tests := []struct {
		status string
		want   bool
	}{
		{"draft", true},
		{"todo", true},
		{"in-progress", true},
		{"completed", true},
		{"scrapped", true},
		{"invalid", false},
		{"", false},
		{"TODO", false}, // case sensitive
		// Old status names should no longer be valid
		{"open", false},
		{"done", false},
		{"ready", false},
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
	want := "in-progress, todo, draft, completed, scrapped"

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
	expected := []string{"in-progress", "todo", "draft", "completed", "scrapped"}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("StatusNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestGetStatus(t *testing.T) {
	cfg := Default()

	t.Run("existing status", func(t *testing.T) {
		s := cfg.GetStatus("todo")
		if s == nil {
			t.Fatal("GetStatus(\"todo\") = nil, want non-nil")
		}
		if s.Name != "todo" {
			t.Errorf("Name = %q, want \"todo\"", s.Name)
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
		s = cfg.GetStatus("ready")
		if s != nil {
			t.Errorf("GetStatus(\"ready\") = %v, want nil (old status name)", s)
		}
	})
}

func TestGetDefaultStatus(t *testing.T) {
	cfg := Default()
	got := cfg.GetDefaultStatus()

	if got != "todo" {
		t.Errorf("GetDefaultStatus() = %q, want \"todo\"", got)
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
	if cfg.Issues.IDLength != 4 {
		t.Errorf("IDLength = %d, want 4", cfg.Issues.IDLength)
	}
}

func TestLoadAndSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create a config (statuses are no longer stored in config)
	cfg := &Config{
		Issues: IssuesConfig{
			Path:        ".todo",
			Prefix:      "test-",
			IDLength:    6,
			DefaultType: "bug",
		},
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
	if loaded.Issues.Prefix != "test-" {
		t.Errorf("Prefix = %q, want \"test-\"", loaded.Issues.Prefix)
	}
	if loaded.Issues.IDLength != 6 {
		t.Errorf("IDLength = %d, want 6", loaded.Issues.IDLength)
	}
	if loaded.Issues.DefaultType != "bug" {
		t.Errorf("DefaultType = %q, want \"bug\"", loaded.Issues.DefaultType)
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

	// Write minimal config (missing id_length and default_type)
	minimalConfig := `issues:
  prefix: "my-"
`
	if err := os.WriteFile(configPath, []byte(minimalConfig), 0644); err != nil {
		t.Fatalf("WriteFile error = %v", err)
	}

	// Load it
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify defaults were applied
	if cfg.Issues.IDLength != 4 {
		t.Errorf("IDLength default not applied: got %d, want 4", cfg.Issues.IDLength)
	}
	// Statuses are hardcoded, always 5
	if len(cfg.StatusNames()) != 5 {
		t.Errorf("Hardcoded statuses: got %d, want 5", len(cfg.StatusNames()))
	}
	// DefaultStatus is always "todo"
	if cfg.GetDefaultStatus() != "todo" {
		t.Errorf("DefaultStatus: got %q, want \"todo\"", cfg.GetDefaultStatus())
	}
	// DefaultType should be first type name when not specified
	if cfg.Issues.DefaultType != "milestone" {
		t.Errorf("DefaultType default not applied: got %q, want \"milestone\"", cfg.Issues.DefaultType)
	}
}

func TestStatusesAreHardcoded(t *testing.T) {
	// Statuses are hardcoded and not configurable (like types)
	// Verify that any config only uses hardcoded statuses
	cfg := Default()

	// All hardcoded statuses should be valid
	hardcodedStatuses := []string{"draft", "todo", "in-progress", "completed", "scrapped"}
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
	if cfg.IsArchiveStatus("todo") {
		t.Error("IsArchiveStatus(\"todo\") = true, want false")
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
		Issues: IssuesConfig{
			Path:        ".todo",
			Prefix:      "test-",
			IDLength:    4,
			DefaultType: "task",
		},
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
		configYAML := `issues:
  prefix: "test-"
  id_length: 4
  default_status: open
statuses:
  - name: open
    color: green
types:
  - name: custom-type
    color: pink
    description: "This should be ignored"
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
			"todo":        "Ready to be worked on",
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
		configYAML := `issues:
  prefix: "test-"
  id_length: 4
statuses:
  - name: custom-status
    color: pink
    description: "This should be ignored"
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
		if !loaded.IsValidStatus("todo") {
			t.Error("IsValidStatus(\"todo\") = false, want true")
		}
	})
}

func TestFindConfig(t *testing.T) {
	t.Run("finds config in current directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		if err := os.WriteFile(configPath, []byte("issues:\n  prefix: test-\n"), 0644); err != nil {
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
		if err := os.WriteFile(configPath, []byte("issues:\n  prefix: test-\n"), 0644); err != nil {
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
}

func TestLoadFromDirectory(t *testing.T) {
	t.Run("loads config from directory with .todo.yml", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `issues:
  path: custom-beans
  prefix: test-
  id_length: 6
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := LoadFromDirectory(tmpDir)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if cfg.Issues.Path != "custom-beans" {
			t.Errorf("Issues.Path = %q, want \"custom-beans\"", cfg.Issues.Path)
		}
		if cfg.Issues.Prefix != "test-" {
			t.Errorf("Prefix = %q, want \"test-\"", cfg.Issues.Prefix)
		}
		if cfg.Issues.IDLength != 6 {
			t.Errorf("IDLength = %d, want 6", cfg.Issues.IDLength)
		}
	})

	t.Run("returns default config when no config file exists", func(t *testing.T) {
		tmpDir := t.TempDir()

		cfg, err := LoadFromDirectory(tmpDir)
		if err != nil {
			t.Fatalf("LoadFromDirectory() error = %v", err)
		}
		if cfg.Issues.Path != DefaultDataPath {
			t.Errorf("Beans.Path = %q, want %q", cfg.Issues.Path, DefaultDataPath)
		}
		if cfg.ConfigDir() != tmpDir {
			t.Errorf("ConfigDir() = %q, want %q", cfg.ConfigDir(), tmpDir)
		}
	})
}

func TestResolveDataPath(t *testing.T) {
	t.Run("resolves relative path from config directory", func(t *testing.T) {
		cfg := &Config{
			Issues: IssuesConfig{Path: "custom-data"},
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
			Issues: IssuesConfig{Path: "/absolute/path/to/data"},
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

func TestDefaultHasBeansPath(t *testing.T) {
	cfg := Default()
	if cfg.Issues.Path != DefaultDataPath {
		t.Errorf("Default().Issues.Path = %q, want %q", cfg.Issues.Path, DefaultDataPath)
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
		cfg.Issues.Editor = "code --wait"
		if got := cfg.GetEditor(); got != "code --wait" {
			t.Errorf("GetEditor() = %q, want \"code --wait\"", got)
		}
	})

	t.Run("loads from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `issues:
  prefix: "test-"
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
		configYAML := `issues:
  prefix: "test-"
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
		cfg.Issues.DefaultSort = "updated"
		got := cfg.GetDefaultSort()
		if got != "updated" {
			t.Errorf("GetDefaultSort() = %q, want \"updated\"", got)
		}
	})

	t.Run("loads from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `issues:
  prefix: "test-"
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
		configYAML := `issues:
  prefix: "test-"
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

func TestExtensionConfig(t *testing.T) {
	t.Run("returns nil for default config", func(t *testing.T) {
		cfg := Default()
		if got := cfg.ExtensionConfig("clickup"); got != nil {
			t.Errorf("ExtensionConfig(\"clickup\") = %v, want nil", got)
		}
	})

	t.Run("returns nil for unknown extension", func(t *testing.T) {
		cfg := &Config{
			Extensions: map[string]map[string]any{
				"clickup": {"list_id": "123"},
			},
		}
		if got := cfg.ExtensionConfig("jira"); got != nil {
			t.Errorf("ExtensionConfig(\"jira\") = %v, want nil", got)
		}
	})

	t.Run("returns extension config data", func(t *testing.T) {
		cfg := &Config{
			Extensions: map[string]map[string]any{
				"clickup": {
					"list_id": "123",
					"status_mapping": map[string]any{
						"todo": "to do",
					},
				},
			},
		}
		got := cfg.ExtensionConfig("clickup")
		if got == nil {
			t.Fatal("ExtensionConfig(\"clickup\") = nil, want non-nil")
		}
		if got["list_id"] != "123" {
			t.Errorf("ExtensionConfig(\"clickup\")[\"list_id\"] = %v, want \"123\"", got["list_id"])
		}
	})

	t.Run("loads from YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `issues:
  prefix: "test-"
extensions:
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

		clickup := cfg.ExtensionConfig("clickup")
		if clickup == nil {
			t.Fatal("ExtensionConfig(\"clickup\") = nil, want non-nil")
		}
		if clickup["list_id"] != "456" {
			t.Errorf("clickup[\"list_id\"] = %v, want \"456\"", clickup["list_id"])
		}
		if clickup["workspace"] != "my-workspace" {
			t.Errorf("clickup[\"workspace\"] = %v, want \"my-workspace\"", clickup["workspace"])
		}

		jira := cfg.ExtensionConfig("jira")
		if jira == nil {
			t.Fatal("ExtensionConfig(\"jira\") = nil, want non-nil")
		}
		if jira["project_key"] != "PROJ" {
			t.Errorf("jira[\"project_key\"] = %v, want \"PROJ\"", jira["project_key"])
		}

		// Unknown extension returns nil
		if got := cfg.ExtensionConfig("unknown"); got != nil {
			t.Errorf("ExtensionConfig(\"unknown\") = %v, want nil", got)
		}
	})

	t.Run("no extensions section in YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, ConfigFileName)
		configYAML := `issues:
  prefix: "test-"
`
		if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
			t.Fatalf("WriteFile error = %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}

		if got := cfg.ExtensionConfig("clickup"); got != nil {
			t.Errorf("ExtensionConfig(\"clickup\") = %v, want nil", got)
		}
	})
}
