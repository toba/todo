package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/output"
)

var initJSON bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a todo project",
	Long:  `Creates a data directory and .toba.yaml config file in the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectDir string
		var dataDir string

		if dataPath != "" {
			// Use explicit path for data directory
			dataDir = dataPath
			projectDir = filepath.Dir(dataDir)
			// Create the directory using Core.Init to set up .gitignore
			c := core.New(dataDir, nil)
			if err := c.Init(); err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return fmt.Errorf("failed to create directory: %w", err)
			}
		} else {
			// Use current working directory
			dir, err := os.Getwd()
			if err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return err
			}

			if err := core.Init(dir); err != nil {
				if initJSON {
					return output.Error(output.ErrFileError, err.Error())
				}
				return fmt.Errorf("failed to initialize: %w", err)
			}

			projectDir = dir
			dataDir = filepath.Join(dir, config.DefaultDataPath)
		}

		// Create default config file
		// Config is saved at project root (not inside .todo/)
		defaultCfg := config.Default()
		defaultCfg.SetConfigDir(projectDir)
		if err := defaultCfg.Save(projectDir); err != nil {
			if initJSON {
				return output.Error(output.ErrFileError, err.Error())
			}
			return fmt.Errorf("failed to create config: %w", err)
		}

		if initJSON {
			return output.SuccessInit(dataDir)
		}

		fmt.Println("Initialized todo project")
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(initCmd)
}
