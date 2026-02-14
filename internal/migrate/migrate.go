// Package migrate provides functionality to convert old-format beans projects
// (hmans/beans with .beans/ directory and beans: config key) to the new format
// (.issues/ directory with issues: config key).
package migrate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Result holds the outcome of a migration.
type Result struct {
	ActiveCount     int    `json:"active_count"`
	ArchivedCount   int    `json:"archived_count"`
	StatusConverted int    `json:"status_converted"`
	ConfigMigrated  bool   `json:"config_migrated"`
	ClickUpImported bool   `json:"clickup_imported"`
	NewDataDir      string `json:"new_data_dir"`
}

// Options controls migration behavior.
type Options struct {
	// SourceDir is the old data directory (e.g., ".beans").
	// If empty, auto-detected from old config.
	SourceDir string

	// ConfigPath is the path to the .todo.yml config file.
	ConfigPath string
}

// statusFrontmatterRe matches "status: todo" in YAML frontmatter.
// It handles optional quoting and varying whitespace.
var statusFrontmatterRe = regexp.MustCompile(`(?m)^(status:\s*)(?:"todo"|'todo'|todo)[ \t]*$`)

// Run performs the migration from old beans format to new issues format.
func Run(opts Options) (*Result, error) {
	result := &Result{}

	// Resolve config path
	configPath := opts.ConfigPath
	if configPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
		configPath = filepath.Join(cwd, ".todo.yml")
	}

	// Read and parse old config to find source directory
	sourceDir := opts.SourceDir
	if sourceDir == "" {
		detected, err := detectSourceDir(configPath)
		if err != nil {
			return nil, fmt.Errorf("detecting source directory: %w", err)
		}
		sourceDir = detected
	}

	// Make sourceDir absolute relative to config dir
	if !filepath.IsAbs(sourceDir) {
		sourceDir = filepath.Join(filepath.Dir(configPath), sourceDir)
	}

	// Validate source directory exists
	info, err := os.Stat(sourceDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("source directory does not exist: %s", sourceDir)
	}

	// Determine target directory
	targetDir := filepath.Join(filepath.Dir(configPath), ".issues")

	// Check if target already has bean files
	if hasExistingBeans(targetDir) {
		return nil, fmt.Errorf("target directory %s already contains bean files; aborting to avoid data loss", targetDir)
	}

	// Create target directories
	beansSubdir := filepath.Join(targetDir, "beans")
	archiveSubdir := filepath.Join(targetDir, "archive")
	if err := os.MkdirAll(beansSubdir, 0755); err != nil {
		return nil, fmt.Errorf("creating target directory: %w", err)
	}

	// Migrate bean files (skip archive/)
	activeConverted, err := migrateFiles(sourceDir, beansSubdir, "archive")
	if err != nil {
		return nil, fmt.Errorf("migrating active beans: %w", err)
	}
	result.ActiveCount = activeConverted.count
	result.StatusConverted = activeConverted.statusConverted

	// Migrate archived beans
	oldArchive := filepath.Join(sourceDir, "archive")
	if info, err := os.Stat(oldArchive); err == nil && info.IsDir() {
		if err := os.MkdirAll(archiveSubdir, 0755); err != nil {
			return nil, fmt.Errorf("creating archive directory: %w", err)
		}
		archiveConverted, err := migrateFiles(oldArchive, archiveSubdir, "")
		if err != nil {
			return nil, fmt.Errorf("migrating archived beans: %w", err)
		}
		result.ArchivedCount = archiveConverted.count
		result.StatusConverted += archiveConverted.statusConverted
	}

	result.NewDataDir = targetDir

	// Migrate config
	if err := MigrateConfig(configPath); err != nil {
		return nil, fmt.Errorf("migrating config: %w", err)
	}
	result.ConfigMigrated = true

	// Import ClickUp config from bean-me-up sources
	imported, err := ImportClickUpConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("importing ClickUp config: %w", err)
	}
	result.ClickUpImported = imported

	return result, nil
}

// detectSourceDir reads the config file and extracts the old beans.path.
func detectSourceDir(configPath string) (string, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ".beans", nil
		}
		return "", err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parsing config: %w", err)
	}

	// Look for beans.path
	if beansRaw, ok := raw["beans"]; ok {
		if beansMap, ok := beansRaw.(map[string]any); ok {
			if pathRaw, ok := beansMap["path"]; ok {
				if path, ok := pathRaw.(string); ok && path != "" {
					return path, nil
				}
			}
		}
	}

	return ".beans", nil
}

// migrateResult holds counts from migrating a set of files.
type migrateResult struct {
	count           int
	statusConverted int
}

// migrateFiles copies .md files from src to dst, rewriting status: todo → status: ready.
// skipDir is a directory name to skip (e.g., "archive"). Pass "" to skip nothing.
func migrateFiles(src, dst, skipDir string) (*migrateResult, error) {
	result := &migrateResult{}

	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// Skip directories (including archive)
		if entry.IsDir() {
			if skipDir != "" && entry.Name() == skipDir {
				continue
			}
			// Recurse into subdirectories (for hash-prefixed layouts)
			subResult, err := migrateFiles(filepath.Join(src, entry.Name()), dst, "")
			if err != nil {
				return nil, err
			}
			result.count += subResult.count
			result.statusConverted += subResult.statusConverted
			continue
		}

		// Only process .md files
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		// Rewrite status: todo → status: ready in frontmatter only
		newContent, converted := rewriteStatus(content)

		dstPath := filepath.Join(dst, entry.Name())
		if err := os.WriteFile(dstPath, newContent, 0644); err != nil {
			return nil, fmt.Errorf("writing %s: %w", entry.Name(), err)
		}

		result.count++
		if converted {
			result.statusConverted++
		}
	}

	return result, nil
}

// rewriteStatus replaces "status: todo" with "status: ready" in YAML frontmatter.
// Only rewrites within the frontmatter section (between --- delimiters).
// Returns the (possibly modified) content and whether a conversion was made.
func rewriteStatus(content []byte) ([]byte, bool) {
	s := string(content)

	// Find frontmatter boundaries
	if !strings.HasPrefix(s, "---\n") {
		return content, false
	}

	// Search for closing --- after the opening one
	endIdx := strings.Index(s[4:], "\n---\n")
	if endIdx < 0 {
		// Try end-of-file frontmatter (file ends with ---)
		if strings.HasSuffix(s, "\n---") {
			endIdx = len(s) - 4 - 4 // adjust for prefix and suffix
		} else {
			return content, false
		}
	}

	// Frontmatter is from position 4 to 4+endIdx (exclusive), the \n before --- is at 4+endIdx
	fmStart := 4
	fmEnd := fmStart + endIdx + 1 // include the trailing \n before ---
	frontmatter := s[fmStart:fmEnd]

	// Check if status: todo exists in frontmatter
	newFM := statusFrontmatterRe.ReplaceAllString(frontmatter, "${1}ready")
	if newFM == frontmatter {
		return content, false
	}

	return []byte(s[:fmStart] + newFM + s[fmEnd:]), true
}

// MigrateConfig rewrites a .todo.yml config file:
// - Renames beans: key → issues: key
// - Removes prefix and id_length from the issues map
// - Converts default_status: todo → default_status: ready
func MigrateConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config to migrate
		}
		return err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing config: %w", err)
	}

	modified := false

	// Move beans: → issues:
	if beansRaw, ok := raw["beans"]; ok {
		if _, hasIssues := raw["issues"]; !hasIssues {
			raw["issues"] = beansRaw
		}
		delete(raw, "beans")
		modified = true
	}

	// Process the issues section
	if issuesRaw, ok := raw["issues"]; ok {
		if issuesMap, ok := issuesRaw.(map[string]any); ok {
			// Remove prefix and id_length
			if _, ok := issuesMap["prefix"]; ok {
				delete(issuesMap, "prefix")
				modified = true
			}
			if _, ok := issuesMap["id_length"]; ok {
				delete(issuesMap, "id_length")
				modified = true
			}

			// Convert default_status: todo → ready
			if status, ok := issuesMap["default_status"]; ok {
				if statusStr, ok := status.(string); ok && statusStr == "todo" {
					issuesMap["default_status"] = "ready"
					modified = true
				}
			}

			// Update path from .beans to .issues if it was the default
			if path, ok := issuesMap["path"]; ok {
				if pathStr, ok := path.(string); ok && pathStr == ".beans" {
					issuesMap["path"] = ".issues"
					modified = true
				}
			}
		}
	}

	if !modified {
		return nil
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("serializing config: %w", err)
	}

	return os.WriteFile(configPath, out, 0644)
}

// ImportClickUpConfig detects and imports ClickUp configuration from
// bean-me-up sources into the main config file. It checks two locations:
//  1. .beans.clickup.yml (standalone file with beans.clickup.* structure)
//  2. .beans.yml (inline config with extensions.clickup section)
//
// The standalone file takes priority if both exist. If the main config already
// has extensions.clickup, this is a no-op. Returns true if config was imported.
func ImportClickUpConfig(configPath string) (bool, error) {
	// Read the main config to check if it already has extensions.clickup
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return false, fmt.Errorf("parsing config: %w", err)
	}

	// Check if extensions.clickup already exists
	if extRaw, ok := raw["extensions"]; ok {
		if extMap, ok := extRaw.(map[string]any); ok {
			if _, ok := extMap["clickup"]; ok {
				return false, nil // Already has ClickUp config
			}
		}
	}

	configDir := filepath.Dir(configPath)
	var clickupSection map[string]any

	// Priority 1: standalone .beans.clickup.yml
	standalonePath := filepath.Join(configDir, ".beans.clickup.yml")
	if standaloneData, err := os.ReadFile(standalonePath); err == nil {
		var standaloneRaw map[string]any
		if err := yaml.Unmarshal(standaloneData, &standaloneRaw); err == nil {
			clickupSection = extractClickUpFromStandalone(standaloneRaw)
		}
	}

	// Priority 2: .beans.yml extensions.clickup (only if standalone not found,
	// and only if .beans.yml is a different file than configPath)
	if clickupSection == nil {
		beansYmlPath := filepath.Join(configDir, ".beans.yml")
		absBeansYml, _ := filepath.Abs(beansYmlPath)
		absConfig, _ := filepath.Abs(configPath)
		if absBeansYml != absConfig {
			if beansData, err := os.ReadFile(beansYmlPath); err == nil {
				var beansRaw map[string]any
				if err := yaml.Unmarshal(beansData, &beansRaw); err == nil {
					clickupSection = extractClickUpFromExtensions(beansRaw)
				}
			}
		}
	}

	if clickupSection == nil {
		return false, nil
	}

	// Require list_id to be present
	if _, ok := clickupSection["list_id"]; !ok {
		return false, nil
	}

	// Convert status mapping keys (todo → ready)
	convertStatusMappingKeys(clickupSection)

	// Merge into main config under extensions.clickup
	extRaw, ok := raw["extensions"]
	if !ok {
		raw["extensions"] = map[string]any{"clickup": clickupSection}
	} else if extMap, ok := extRaw.(map[string]any); ok {
		extMap["clickup"] = clickupSection
	} else {
		raw["extensions"] = map[string]any{"clickup": clickupSection}
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return false, fmt.Errorf("serializing config: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0644); err != nil {
		return false, fmt.Errorf("writing config: %w", err)
	}

	return true, nil
}

// extractClickUpFromStandalone extracts ClickUp config from the standalone
// .beans.clickup.yml format (beans.clickup.* structure).
func extractClickUpFromStandalone(raw map[string]any) map[string]any {
	beansRaw, ok := raw["beans"]
	if !ok {
		return nil
	}
	beansMap, ok := beansRaw.(map[string]any)
	if !ok {
		return nil
	}
	clickupRaw, ok := beansMap["clickup"]
	if !ok {
		return nil
	}
	clickupMap, ok := clickupRaw.(map[string]any)
	if !ok {
		return nil
	}
	return clickupMap
}

// extractClickUpFromExtensions extracts ClickUp config from the extensions.clickup
// section of a .beans.yml file.
func extractClickUpFromExtensions(raw map[string]any) map[string]any {
	extRaw, ok := raw["extensions"]
	if !ok {
		return nil
	}
	extMap, ok := extRaw.(map[string]any)
	if !ok {
		return nil
	}
	clickupRaw, ok := extMap["clickup"]
	if !ok {
		return nil
	}
	clickupMap, ok := clickupRaw.(map[string]any)
	if !ok {
		return nil
	}
	return clickupMap
}

// convertStatusMappingKeys renames "todo" → "ready" in status_mapping keys.
// Only the keys (bean status names) are converted, not the values (ClickUp status names).
func convertStatusMappingKeys(section map[string]any) {
	smRaw, ok := section["status_mapping"]
	if !ok {
		return
	}
	sm, ok := smRaw.(map[string]any)
	if !ok {
		return
	}
	if val, ok := sm["todo"]; ok {
		if _, hasReady := sm["ready"]; !hasReady {
			sm["ready"] = val
		}
		delete(sm, "todo")
	}
}

// hasExistingBeans checks if the target directory has any .md files
// (indicating it already contains beans).
func hasExistingBeans(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Walk the directory looking for .md files
	found := false
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})

	return found
}
