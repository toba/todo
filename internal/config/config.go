package config

import (
	"cmp"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// ConfigFileName is the name of the config file at project root
	ConfigFileName = ".todo.yml"
	// DefaultDataPath is the default directory for storing issues
	DefaultDataPath = ".issues"
)

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
	Issues     IssuesConfig              `yaml:"issues"`
	Extensions map[string]map[string]any `yaml:"extensions,omitempty"`

	// configDir is the directory containing the config file (not serialized)
	// Used to resolve relative paths
	configDir string `yaml:"-"`
}

// IssuesConfig defines settings for issue creation.
type IssuesConfig struct {
	// Path is the path to the issues directory (relative to config file location)
	Path           string `yaml:"path,omitempty"`
	DefaultStatus  string `yaml:"default_status,omitempty"`
	DefaultType    string `yaml:"default_type,omitempty"`
	DefaultSort    string `yaml:"default_sort,omitempty"`
	Editor         string `yaml:"editor,omitempty"`
	RequireIfMatch bool   `yaml:"require_if_match,omitempty"`
}

// Default returns a Config with default values.
func Default() *Config {
	return &Config{
		Issues: IssuesConfig{
			Path:          DefaultDataPath,
			DefaultStatus: StatusReady,
			DefaultType:   TypeTask,
		},
	}
}

// FindConfig searches upward from the given directory for a .todo.yml config file.
// Returns the absolute path to the config file, or empty string if not found.
func FindConfig(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", nil
		}
		dir = parent
	}
}

// Load reads configuration from the given config file path.
// Returns default config if the file doesn't exist.
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Store the config directory for resolving relative paths
	cfg.configDir = filepath.Dir(configPath)

	// Apply defaults for missing values
	cfg.Issues.Path = cmp.Or(cfg.Issues.Path, DefaultDataPath)
	cfg.Issues.DefaultStatus = cmp.Or(cfg.Issues.DefaultStatus, StatusReady)
	cfg.Issues.DefaultType = cmp.Or(cfg.Issues.DefaultType, DefaultTypes[0].Name)

	return &cfg, nil
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
	if filepath.IsAbs(c.Issues.Path) {
		return c.Issues.Path
	}
	if c.configDir == "" {
		// Fallback: use current directory
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, c.Issues.Path)
	}
	return filepath.Join(c.configDir, c.Issues.Path)
}

// ConfigDir returns the directory containing the config file.
func (c *Config) ConfigDir() string {
	return c.configDir
}

// SetConfigDir sets the config directory (for testing or when creating new configs).
func (c *Config) SetConfigDir(dir string) {
	c.configDir = dir
}

// Save writes the configuration to the config file.
// If configDir is set, saves to that directory; otherwise saves to the given directory.
func (c *Config) Save(dir string) error {
	targetDir := c.configDir
	if targetDir == "" {
		targetDir = dir
	}
	path := filepath.Join(targetDir, ConfigFileName)

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// IsValidStatus returns true if the status is a valid hardcoded status.
func (c *Config) IsValidStatus(status string) bool {
	for _, s := range DefaultStatuses {
		if s.Name == status {
			return true
		}
	}
	return false
}

// StatusList returns a comma-separated list of valid statuses.
// Statuses are hardcoded and not configurable.
func (c *Config) StatusList() string {
	names := make([]string, len(DefaultStatuses))
	for i, s := range DefaultStatuses {
		names[i] = s.Name
	}
	return strings.Join(names, ", ")
}

// StatusNames returns a slice of valid status names.
// Statuses are hardcoded and not configurable.
func (c *Config) StatusNames() []string {
	names := make([]string, len(DefaultStatuses))
	for i, s := range DefaultStatuses {
		names[i] = s.Name
	}
	return names
}

// GetStatus returns the StatusConfig for a given status name, or nil if not found.
// Statuses are hardcoded and not configurable.
func (c *Config) GetStatus(name string) *StatusConfig {
	for i := range DefaultStatuses {
		if DefaultStatuses[i].Name == name {
			return &DefaultStatuses[i]
		}
	}
	return nil
}

// GetDefaultStatus returns the default status name for new issues.
func (c *Config) GetDefaultStatus() string {
	return cmp.Or(c.Issues.DefaultStatus, StatusReady)
}

// GetDefaultType returns the default type name for new issues.
func (c *Config) GetDefaultType() string {
	return c.Issues.DefaultType
}

// GetEditor returns the configured editor command, or empty string if unset.
func (c *Config) GetEditor() string {
	return c.Issues.Editor
}

// GetDefaultSort returns the default sort order for the TUI.
// Returns "default" if not set.
func (c *Config) GetDefaultSort() string {
	return cmp.Or(c.Issues.DefaultSort, "default")
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
// Types are hardcoded and not configurable.
func (c *Config) GetType(name string) *TypeConfig {
	for i := range DefaultTypes {
		if DefaultTypes[i].Name == name {
			return &DefaultTypes[i]
		}
	}
	return nil
}

// TypeNames returns a slice of valid type names.
// Types are hardcoded and not configurable.
func (c *Config) TypeNames() []string {
	names := make([]string, len(DefaultTypes))
	for i, t := range DefaultTypes {
		names[i] = t.Name
	}
	return names
}

// IsValidType returns true if the type is a valid hardcoded type.
func (c *Config) IsValidType(typeName string) bool {
	for _, t := range DefaultTypes {
		if t.Name == typeName {
			return true
		}
	}
	return false
}

// TypeList returns a comma-separated list of valid types.
func (c *Config) TypeList() string {
	names := make([]string, len(DefaultTypes))
	for i, t := range DefaultTypes {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
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
	for i := range DefaultPriorities {
		if DefaultPriorities[i].Name == name {
			return &DefaultPriorities[i]
		}
	}
	return nil
}

// PriorityNames returns a slice of valid priority names in order from highest to lowest.
func (c *Config) PriorityNames() []string {
	names := make([]string, len(DefaultPriorities))
	for i, p := range DefaultPriorities {
		names[i] = p.Name
	}
	return names
}

// IsValidPriority returns true if the priority is a valid hardcoded priority.
// Empty string is valid (means no priority set).
func (c *Config) IsValidPriority(priority string) bool {
	if priority == "" {
		return true
	}
	for _, p := range DefaultPriorities {
		if p.Name == priority {
			return true
		}
	}
	return false
}

// ExtensionConfig returns the configuration data for a named extension,
// or nil if the extension has no configuration.
func (c *Config) ExtensionConfig(name string) map[string]any {
	if c.Extensions == nil {
		return nil
	}
	return c.Extensions[name]
}

// PriorityList returns a comma-separated list of valid priorities.
func (c *Config) PriorityList() string {
	names := make([]string, len(DefaultPriorities))
	for i, p := range DefaultPriorities {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}

// DefaultStatusNames returns the names of all default statuses.
func DefaultStatusNames() []string {
	names := make([]string, len(DefaultStatuses))
	for i, s := range DefaultStatuses {
		names[i] = s.Name
	}
	return names
}

// DefaultTypeNames returns the names of all default types.
func DefaultTypeNames() []string {
	names := make([]string, len(DefaultTypes))
	for i, t := range DefaultTypes {
		names[i] = t.Name
	}
	return names
}

// DefaultPriorityNames returns the names of all default priorities.
func DefaultPriorityNames() []string {
	names := make([]string, len(DefaultPriorities))
	for i, p := range DefaultPriorities {
		names[i] = p.Name
	}
	return names
}
