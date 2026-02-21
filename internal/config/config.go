package config

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the config file at project root
	ConfigFileName = ".toba.yaml"
	// LegacyConfigFileName is the old config file name (pre-migration)
	LegacyConfigFileName = ".todo.yml"
	// DefaultDataPath is the default directory for storing issues
	DefaultDataPath = ".issues"
)

// TobaConfig is the top-level wrapper for the .toba.yaml file format.
// The todo configuration lives under the "todo" key to support
// a shared config format where multiple toba tools each have their own section.
type TobaConfig struct {
	Todo Config `yaml:"todo"`
}

// Status name constants.
const (
	StatusInProgress = "in-progress"
	StatusReady      = "ready"
	StatusDraft      = "draft"
	StatusCompleted  = "completed"
	StatusScrapped   = "scrapped"
)

// Type name constants.
const (
	TypeMilestone = "milestone"
	TypeEpic      = "epic"
	TypeBug       = "bug"
	TypeFeature   = "feature"
	TypeTask      = "task"
)

// Priority name constants.
const (
	PriorityCritical = "critical"
	PriorityHigh     = "high"
	PriorityNormal   = "normal"
	PriorityLow      = "low"
	PriorityDeferred = "deferred"
)

// Sort order constants.
const (
	SortDefault = "default"
)

// DefaultStatuses defines the hardcoded status configuration.
// Statuses are not configurable - they are hardcoded like types.
// Order determines sort priority: in-progress first (active work), then todo, draft, and done states last.
var DefaultStatuses = []StatusConfig{
	{Name: StatusInProgress, Color: "yellow", Description: "Currently being worked on"},
	{Name: StatusReady, Color: "green", Description: "Ready to be worked on"},
	{Name: StatusDraft, Color: "blue", Description: "Needs refinement before it can be worked on"},
	{Name: StatusCompleted, Color: "gray", Archive: true, Description: "Finished successfully"},
	{Name: StatusScrapped, Color: "gray", Archive: true, Description: "Will not be done"},
}

// DefaultTypes defines the default type configuration.
var DefaultTypes = []TypeConfig{
	{Name: TypeMilestone, Color: "cyan", Description: "A target release or checkpoint; group work that should ship together"},
	{Name: TypeEpic, Color: "purple", Description: "A thematic container for related work; should have child issues, not be worked on directly"},
	{Name: TypeBug, Color: "red", Description: "Something that is broken and needs fixing"},
	{Name: TypeFeature, Color: "green", Description: "A user-facing capability or enhancement"},
	{Name: TypeTask, Color: "blue", Description: "A concrete piece of work to complete (eg. a chore, or a sub-task for a feature)"},
}

// DefaultPriorities defines the hardcoded priority configuration.
// Priorities are ordered from highest to lowest urgency.
var DefaultPriorities = []PriorityConfig{
	{Name: PriorityCritical, Color: "red", Description: "Urgent, blocking work. When possible, address immediately"},
	{Name: PriorityHigh, Color: "yellow", Description: "Important, should be done before normal work"},
	{Name: PriorityNormal, Color: "white", Description: "Standard priority"},
	{Name: PriorityLow, Color: "gray", Description: "Less important, can be delayed"},
	{Name: PriorityDeferred, Color: "gray", Description: "Explicitly pushed back, avoid doing unless necessary"},
}

// StatusConfig defines a single status with its display color.
type StatusConfig struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Archive     bool   `yaml:"archive,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// TypeConfig defines a single issue type with its display color.
type TypeConfig struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description,omitempty"`
}

// PriorityConfig defines a single priority level with its display color.
type PriorityConfig struct {
	Name        string `yaml:"name"`
	Color       string `yaml:"color"`
	Description string `yaml:"description,omitempty"`
}

// Config holds the todo configuration.
// Note: Statuses are no longer stored in config - they are hardcoded like types.
type Config struct {
	// Path is the path to the issues directory (relative to config file location)
	Path           string `yaml:"path,omitempty"`
	DefaultStatus  string `yaml:"default_status,omitempty"`
	DefaultType    string `yaml:"default_type,omitempty"`
	DefaultSort    string `yaml:"default_sort,omitempty"`
	Editor         string `yaml:"editor,omitempty"`
	RequireIfMatch bool   `yaml:"require_if_match,omitempty"`
	Sync map[string]map[string]any `yaml:"sync,omitempty"`

	// configDir is the directory containing the config file (not serialized)
	// Used to resolve relative paths
	configDir string `yaml:"-"`
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Path:          DefaultDataPath,
		DefaultStatus: StatusReady,
		DefaultType:   TypeTask,
	}
}

// FindConfig searches upward from the given directory for a .toba.yaml config file,
// falling back to the legacy .todo.yml. If only a legacy file is found, it is
// automatically migrated to .toba.yaml (written in the new format, old file removed).
// Returns the absolute path to the config file, or empty string if not found.
func FindConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		newPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(newPath); err == nil {
			return newPath, nil
		}

		legacyPath := filepath.Join(dir, LegacyConfigFileName)
		if _, err := os.Stat(legacyPath); err == nil {
			// Auto-migrate legacy config to new format
			migrated, migrateErr := migrateLegacyConfig(legacyPath, newPath)
			if migrateErr != nil {
				return "", fmt.Errorf("migrating %s to %s: %w", LegacyConfigFileName, ConfigFileName, migrateErr)
			}
			if migrated {
				return newPath, nil
			}
			// If migration failed silently, fall back to legacy
			return legacyPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", nil
		}
		dir = parent
	}
}

// legacyConfig is used to parse the old .todo.yml format which had
// an "issues:" top-level key containing the issue settings.
type legacyConfig struct {
	Issues struct {
		Path           string `yaml:"path,omitempty"`
		DefaultStatus  string `yaml:"default_status,omitempty"`
		DefaultType    string `yaml:"default_type,omitempty"`
		DefaultSort    string `yaml:"default_sort,omitempty"`
		Editor         string `yaml:"editor,omitempty"`
		RequireIfMatch bool   `yaml:"require_if_match,omitempty"`
	} `yaml:"issues"`
	Sync map[string]map[string]any `yaml:"sync,omitempty"`
}

// migrateLegacyConfig reads a legacy .todo.yml, wraps it in the TobaConfig
// format, writes .toba.yaml, and removes the old file.
// Returns true if migration was performed successfully.
func migrateLegacyConfig(legacyPath, newPath string) (bool, error) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return false, err
	}

	// Parse the legacy flat format (has "issues:" top-level key)
	var legacy legacyConfig
	if err := yaml.Unmarshal(data, &legacy); err != nil {
		return false, err
	}

	// Flatten into the new Config format
	cfg := Config{
		Path:           legacy.Issues.Path,
		DefaultStatus:  legacy.Issues.DefaultStatus,
		DefaultType:    legacy.Issues.DefaultType,
		DefaultSort:    legacy.Issues.DefaultSort,
		Editor:         legacy.Issues.Editor,
		RequireIfMatch: legacy.Issues.RequireIfMatch,
		Sync:           legacy.Sync,
	}

	// Write in new TobaConfig wrapper format
	wrapper := TobaConfig{Todo: cfg}
	out, err := yaml.Marshal(&wrapper)
	if err != nil {
		return false, err
	}

	if err := os.WriteFile(newPath, out, 0644); err != nil {
		return false, err
	}

	// Remove legacy file
	if err := os.Remove(legacyPath); err != nil {
		// Non-fatal: new file is written, old file just lingers
		return true, nil
	}

	return true, nil
}

// Load reads configuration from the given config file path.
// Returns default config if the file doesn't exist.
// Handles both the new .toba.yaml format (with "todo:" wrapper) and
// the legacy .todo.yml flat format.
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	var cfg Config
	if isLegacyConfig(configPath) {
		// Legacy .todo.yml: has "issues:" top-level key
		var legacy legacyConfig
		if err := yaml.Unmarshal(data, &legacy); err != nil {
			return nil, err
		}
		cfg = Config{
			Path:           legacy.Issues.Path,
			DefaultStatus:  legacy.Issues.DefaultStatus,
			DefaultType:    legacy.Issues.DefaultType,
			DefaultSort:    legacy.Issues.DefaultSort,
			Editor:         legacy.Issues.Editor,
			RequireIfMatch: legacy.Issues.RequireIfMatch,
			Sync:           legacy.Sync,
		}
	} else {
		// New .toba.yaml: wrapped in "todo:" key
		var wrapper TobaConfig
		if err := yaml.Unmarshal(data, &wrapper); err != nil {
			return nil, err
		}
		cfg = wrapper.Todo
	}

	// Store the config directory for resolving relative paths
	cfg.configDir = filepath.Dir(configPath)

	// Apply defaults for missing values
	cfg.Path = cmp.Or(cfg.Path, DefaultDataPath)
	cfg.DefaultStatus = cmp.Or(cfg.DefaultStatus, StatusReady)
	cfg.DefaultType = cmp.Or(cfg.DefaultType, DefaultTypes[0].Name)

	return &cfg, nil
}

// isLegacyConfig returns true if the given path is a legacy .todo.yml file.
func isLegacyConfig(configPath string) bool {
	return filepath.Base(configPath) == LegacyConfigFileName
}

// LoadFromDirectory finds and loads the config file by searching upward from the given directory.
// If no config file is found, returns a default config anchored at the given directory.
func LoadFromDirectory(startDir string) (*Config, error) {
	configPath, err := FindConfig(startDir)
	if err != nil {
		return nil, err
	}

	if configPath == "" {
		// No config found, return default anchored at startDir
		cfg := Default()
		cfg.configDir = startDir
		return cfg, nil
	}

	return Load(configPath)
}

// ResolveDataPath returns the absolute path to the issues directory.
func (c *Config) ResolveDataPath() string {
	if filepath.IsAbs(c.Path) {
		return c.Path
	}
	if c.configDir == "" {
		// Fallback: use current directory
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, c.Path)
	}
	return filepath.Join(c.configDir, c.Path)
}

// ConfigDir returns the directory containing the config file.
func (c *Config) ConfigDir() string {
	return c.configDir
}

// SetConfigDir sets the config directory (for testing or when creating new configs).
func (c *Config) SetConfigDir(dir string) {
	c.configDir = dir
}

// Save writes the configuration to .toba.yaml using the TobaConfig wrapper format.
// If configDir is set, saves to that directory; otherwise saves to the given directory.
func (c *Config) Save(dir string) error {
	targetDir := c.configDir
	if targetDir == "" {
		targetDir = dir
	}
	path := filepath.Join(targetDir, ConfigFileName)

	wrapper := TobaConfig{Todo: *c}
	data, err := yaml.Marshal(&wrapper)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// named is a constraint for config types that have a Name field.
type named interface {
	StatusConfig | TypeConfig | PriorityConfig
}

// configNames extracts name strings from a slice of config items.
func configNames[T named](items []T, getName func(*T) string) []string {
	names := make([]string, len(items))
	for i := range items {
		names[i] = getName(&items[i])
	}
	return names
}

// configList returns a comma-separated list of names.
func configList[T named](items []T, getName func(*T) string) string {
	return strings.Join(configNames(items, getName), ", ")
}

// configFind returns a pointer to the item with the given name, or nil.
func configFind[T named](items []T, name string, getName func(*T) string) *T {
	for i := range items {
		if getName(&items[i]) == name {
			return &items[i]
		}
	}
	return nil
}

// configIsValid returns true if name matches any item.
func configIsValid[T named](items []T, name string, getName func(*T) string) bool {
	return slices.ContainsFunc(items, func(item T) bool {
		return getName(&item) == name
	})
}

func statusName(s *StatusConfig) string     { return s.Name }
func typeName(t *TypeConfig) string         { return t.Name }
func priorityName(p *PriorityConfig) string { return p.Name }

// IsValidStatus returns true if the status is a valid hardcoded status.
func (c *Config) IsValidStatus(status string) bool {
	return configIsValid(DefaultStatuses, status, statusName)
}

// StatusList returns a comma-separated list of valid statuses.
func (c *Config) StatusList() string {
	return configList(DefaultStatuses, statusName)
}

// StatusNames returns a slice of valid status names.
func (c *Config) StatusNames() []string {
	return configNames(DefaultStatuses, statusName)
}

// GetStatus returns the StatusConfig for a given status name, or nil if not found.
func (c *Config) GetStatus(name string) *StatusConfig {
	return configFind(DefaultStatuses, name, statusName)
}

// GetDefaultStatus returns the default status name for new issues.
func (c *Config) GetDefaultStatus() string {
	return cmp.Or(c.DefaultStatus, StatusReady)
}

// GetDefaultType returns the default type name for new issues.
func (c *Config) GetDefaultType() string {
	return c.DefaultType
}

// GetEditor returns the configured editor command, or empty string if unset.
func (c *Config) GetEditor() string {
	return c.Editor
}

// GetDefaultSort returns the default sort order for the TUI.
// Returns "default" if not set.
func (c *Config) GetDefaultSort() string {
	return cmp.Or(c.DefaultSort, SortDefault)
}

// IsArchiveStatus returns true if the given status is marked for archiving.
// Statuses are hardcoded and not configurable.
func (c *Config) IsArchiveStatus(name string) bool {
	if s := c.GetStatus(name); s != nil {
		return s.Archive
	}
	return false
}

// GetType returns the TypeConfig for a given type name, or nil if not found.
func (c *Config) GetType(name string) *TypeConfig {
	return configFind(DefaultTypes, name, typeName)
}

// TypeNames returns a slice of valid type names.
func (c *Config) TypeNames() []string {
	return configNames(DefaultTypes, typeName)
}

// IsValidType returns true if the type is a valid hardcoded type.
func (c *Config) IsValidType(name string) bool {
	return configIsValid(DefaultTypes, name, typeName)
}

// TypeList returns a comma-separated list of valid types.
func (c *Config) TypeList() string {
	return configList(DefaultTypes, typeName)
}

// IssueColors holds resolved color information for rendering an issue
type IssueColors struct {
	StatusColor   string
	TypeColor     string
	PriorityColor string
	IsArchive     bool
}

// GetIssueColors returns the resolved colors for an issue based on its status, type, and priority.
func (c *Config) GetIssueColors(status, typeName, priority string) IssueColors {
	colors := IssueColors{
		StatusColor:   "gray",
		TypeColor:     "",
		PriorityColor: "",
		IsArchive:     false,
	}

	if statusCfg := c.GetStatus(status); statusCfg != nil {
		colors.StatusColor = statusCfg.Color
	}
	colors.IsArchive = c.IsArchiveStatus(status)

	if typeCfg := c.GetType(typeName); typeCfg != nil {
		colors.TypeColor = typeCfg.Color
	}

	if priorityCfg := c.GetPriority(priority); priorityCfg != nil {
		colors.PriorityColor = priorityCfg.Color
	}

	return colors
}

// GetPriority returns the PriorityConfig for a given priority name, or nil if not found.
func (c *Config) GetPriority(name string) *PriorityConfig {
	return configFind(DefaultPriorities, name, priorityName)
}

// PriorityNames returns a slice of valid priority names in order from highest to lowest.
func (c *Config) PriorityNames() []string {
	return configNames(DefaultPriorities, priorityName)
}

// IsValidPriority returns true if the priority is a valid hardcoded priority.
// Empty string is valid (means no priority set).
func (c *Config) IsValidPriority(priority string) bool {
	if priority == "" {
		return true
	}
	return configIsValid(DefaultPriorities, priority, priorityName)
}

// SyncConfig returns the configuration data for a named sync integration,
// or nil if the integration has no configuration.
func (c *Config) SyncConfig(name string) map[string]any {
	if c.Sync == nil {
		return nil
	}
	return c.Sync[name]
}

// PriorityList returns a comma-separated list of valid priorities.
func (c *Config) PriorityList() string {
	return configList(DefaultPriorities, priorityName)
}

// DefaultStatusNames returns the names of all default statuses.
func DefaultStatusNames() []string {
	return configNames(DefaultStatuses, statusName)
}

// DefaultTypeNames returns the names of all default types.
func DefaultTypeNames() []string {
	return configNames(DefaultTypes, typeName)
}

// DefaultPriorityNames returns the names of all default priorities.
func DefaultPriorityNames() []string {
	return configNames(DefaultPriorities, priorityName)
}
