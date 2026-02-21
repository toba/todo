package refry

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/issue"
	"gopkg.in/yaml.v3"
)

const beansConfigFile = ".beans.yml"

// BeansConfig represents the .beans.yml configuration file format.
type BeansConfig struct {
	Beans struct {
		Path          string `yaml:"path"`
		Prefix        string `yaml:"prefix"`
		IDLength      int    `yaml:"id_length"`
		DefaultStatus string `yaml:"default_status"`
		DefaultType   string `yaml:"default_type"`
	} `yaml:"beans"`
}

// Result holds the outcome of a refry conversion.
type Result struct {
	ActiveCount     int
	ArchivedCount   int
	StatusConverted int
	ConfigMigrated  bool
	NewDataDir      string
	NewConfigPath   string
}

// Options configures the refry conversion.
type Options struct {
	SourceDir  string // overrides data dir from .beans.yml
	ProjectDir string // project root (default: cwd)
}

// statusRe matches status: todo (with optional quoting) within frontmatter.
var statusRe = regexp.MustCompile(`(?m)^(status:\s*)(?:"todo"|'todo'|todo)[ \t]*$`)

// Run performs the beans-to-todo conversion.
func Run(opts Options) (*Result, error) {
	if opts.ProjectDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
		opts.ProjectDir = cwd
	}

	// Parse .beans.yml
	beansCfg, err := parseBeansConfig(filepath.Join(opts.ProjectDir, beansConfigFile))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", beansConfigFile, err)
	}

	// Determine source directory
	sourceDir := opts.SourceDir
	if sourceDir == "" {
		sourceDir = beansCfg.Beans.Path
		if sourceDir == "" {
			sourceDir = ".beans"
		}
		if !filepath.IsAbs(sourceDir) {
			sourceDir = filepath.Join(opts.ProjectDir, sourceDir)
		}
	}

	targetDir := filepath.Join(opts.ProjectDir, config.DefaultDataPath)

	// Validate source exists
	if info, err := os.Stat(sourceDir); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("source directory does not exist: %s", sourceDir)
	}

	// If target already exists (different from source), check for existing .md files
	if sourceDir != targetDir {
		if _, err := os.Stat(targetDir); err == nil {
			if hasExistingIssues(targetDir) {
				return nil, fmt.Errorf("target directory %s already contains .md files", targetDir)
			}
		}
	}

	result := &Result{
		NewDataDir:    targetDir,
		NewConfigPath: filepath.Join(opts.ProjectDir, config.ConfigFileName),
	}

	// Step 1: Rename .beans/ -> .issues/
	if sourceDir != targetDir {
		if err := os.Rename(sourceDir, targetDir); err != nil {
			return nil, fmt.Errorf("renaming %s to %s: %w", sourceDir, targetDir, err)
		}
	}

	// Step 2: Bucket files in the root of .issues/ (skip subdirs like archive/)
	activeCount, activeStatusConverted, err := bucketFiles(targetDir)
	if err != nil {
		return nil, fmt.Errorf("bucketing files: %w", err)
	}
	result.ActiveCount = activeCount
	result.StatusConverted += activeStatusConverted

	// Step 3: Handle archive/ subdirectory — rewrite content in place
	archiveDir := filepath.Join(targetDir, "archive")
	if info, err := os.Stat(archiveDir); err == nil && info.IsDir() {
		archivedCount, archivedStatusConverted, err := rewriteFilesInPlace(archiveDir)
		if err != nil {
			return nil, fmt.Errorf("rewriting archived files: %w", err)
		}
		result.ArchivedCount = archivedCount
		result.StatusConverted += archivedStatusConverted
	}

	// Step 4: Convert config
	if err := convertConfig(opts.ProjectDir, beansCfg); err != nil {
		return nil, fmt.Errorf("converting config: %w", err)
	}
	result.ConfigMigrated = true

	// Step 5: Remove .beans.yml
	beansPath := filepath.Join(opts.ProjectDir, beansConfigFile)
	if err := os.Remove(beansPath); err != nil {
		return nil, fmt.Errorf("removing %s: %w", beansConfigFile, err)
	}

	return result, nil
}

// parseBeansConfig reads and parses a .beans.yml file.
func parseBeansConfig(path string) (*BeansConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg BeansConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// hasExistingIssues checks if a directory contains any .md files.
func hasExistingIssues(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			return true
		}
	}
	return false
}

// bucketFiles moves .md files from the root of dir into first-char bucket subdirs.
// Returns the count of files processed and the count of status rewrites.
func bucketFiles(dir string) (int, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}

	var count, statusCount int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		id, slug := issue.ParseFilename(e.Name())
		bucketedPath := issue.BuildPath(id, slug)
		oldPath := filepath.Join(dir, e.Name())
		newPath := filepath.Join(dir, bucketedPath)

		// Create bucket subdir
		bucketDir := filepath.Dir(newPath)
		if err := os.MkdirAll(bucketDir, 0755); err != nil {
			return 0, 0, fmt.Errorf("creating bucket dir %s: %w", bucketDir, err)
		}

		// Rename into bucket (preserves git history)
		if err := os.Rename(oldPath, newPath); err != nil {
			return 0, 0, fmt.Errorf("moving %s to %s: %w", e.Name(), bucketedPath, err)
		}

		// Rewrite content if needed
		converted, err := rewriteFileStatus(newPath)
		if err != nil {
			return 0, 0, fmt.Errorf("rewriting %s: %w", bucketedPath, err)
		}
		if converted {
			statusCount++
		}

		count++
	}

	return count, statusCount, nil
}

// rewriteFilesInPlace walks a directory and rewrites status in .md files.
// Returns the count of files processed and the count of status rewrites.
func rewriteFilesInPlace(dir string) (int, int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}

	var count, statusCount int
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, e.Name())
		converted, err := rewriteFileStatus(path)
		if err != nil {
			return 0, 0, fmt.Errorf("rewriting %s: %w", e.Name(), err)
		}
		if converted {
			statusCount++
		}
		count++
	}

	return count, statusCount, nil
}

// rewriteFileStatus reads a file and rewrites status: todo -> status: ready
// within frontmatter. Returns true if the file was modified.
func rewriteFileStatus(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	content := string(data)
	rewritten := rewriteStatus(content)
	if rewritten == content {
		return false, nil
	}

	if err := os.WriteFile(path, []byte(rewritten), 0644); err != nil {
		return false, err
	}
	return true, nil
}

// rewriteStatus replaces status: todo with status: ready within frontmatter boundaries.
func rewriteStatus(content string) string {
	// Find frontmatter boundaries (--- ... ---)
	if !strings.HasPrefix(content, "---") {
		return content
	}

	// Find closing ---
	endIdx := strings.Index(content[3:], "\n---")
	if endIdx < 0 {
		return content
	}
	endIdx += 3 // adjust for the offset

	frontmatter := content[:endIdx]
	rest := content[endIdx:]

	rewritten := statusRe.ReplaceAllString(frontmatter, "${1}ready")
	return rewritten + rest
}

// convertConfig creates a .toba.yaml config from beans config values.
func convertConfig(projectDir string, beansCfg *BeansConfig) error {
	cfg := config.Default()
	cfg.SetConfigDir(projectDir)

	// Convert default_status: todo -> ready
	if beansCfg.Beans.DefaultStatus == "todo" || beansCfg.Beans.DefaultStatus == "" {
		cfg.Issues.DefaultStatus = config.StatusReady
	} else {
		cfg.Issues.DefaultStatus = beansCfg.Beans.DefaultStatus
	}

	// Preserve default_type
	if beansCfg.Beans.DefaultType != "" {
		cfg.Issues.DefaultType = beansCfg.Beans.DefaultType
	}

	return cfg.Save(projectDir)
}
