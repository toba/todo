package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
)

var store *core.Core
var cfg *config.Config
var dataPath string
var configPath string

var rootCmd = &cobra.Command{
	Use:   "todo",
	Short: "A file-based issue tracker for AI-first workflows",
	Long: `Todo is a lightweight issue tracker that stores issues as markdown files.
Track your work alongside your code and supercharge your coding agent with
a full view of your project.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip core initialization for init, prime, and version commands
		if cmd.Name() == "init" || cmd.Name() == "prime" || cmd.Name() == "version" {
			return nil
		}

		var err error

		// Load configuration
		if configPath != "" {
			// Use explicit config path
			cfg, err = config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config from %s: %w", configPath, err)
			}
		} else {
			// Search upward for .toba.yaml
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("getting current directory: %w", err)
			}
			cfg, err = config.LoadFromDirectory(cwd)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
		}

		// Determine data directory
		var root string
		if dataPath != "" {
			// Use explicit data path (overrides config)
			root = dataPath
			// Verify it exists
			if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
				return fmt.Errorf("data path does not exist or is not a directory: %s", root)
			}
		} else {
			// Use path from config
			root = cfg.ResolveDataPath()
			// Verify it exists
			if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
				return fmt.Errorf("no data directory found at %s (run 'todo init' to create one)", root)
			}
		}

		store = core.New(root, cfg)
		if err := store.Load(); err != nil {
			return fmt.Errorf("loading issues: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataPath, "data-path", "", "Path to data directory (overrides config)")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file (default: searches upward for .toba.yaml)")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
