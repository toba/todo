package cmd

import (
	_ "embed"
	"os"
	"text/template"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/config"
)

//go:embed prompt.tmpl
var agentPromptTemplate string

// promptData holds all data needed to render the prompt template.
type promptData struct {
	Types      []config.TypeConfig
	Statuses   []config.StatusConfig
	Priorities []config.PriorityConfig
	HasSync    bool
	SyncNames  []string
}

var primeCmd = &cobra.Command{
	Use:   "prime",
	Short: "Output instructions for AI coding agents",
	Long:  `Outputs a prompt that primes AI coding agents on how to use the issues CLI to manage project issues.`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no explicit path given, check if an issues project exists by searching
		// upward for a .todo.yml config file
		var cfg *config.Config
		if dataPath == "" && configPath == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return nil // Silently exit on error
			}
			configFile, err := config.FindConfig(cwd)
			if err != nil || configFile == "" {
				// No config file found - silently exit
				return nil
			}
			cfg, _ = config.Load(configFile)
		} else if configPath != "" {
			cfg, _ = config.Load(configPath)
		}

		tmpl, err := template.New("prompt").Parse(agentPromptTemplate)
		if err != nil {
			return err
		}

		data := promptData{
			Types:      config.DefaultTypes,
			Statuses:   config.DefaultStatuses,
			Priorities: config.DefaultPriorities,
		}

		if cfg != nil && cfg.Sync != nil {
			data.HasSync = true
			for name := range cfg.Sync {
				data.SyncNames = append(data.SyncNames, name)
			}
		}

		return tmpl.Execute(os.Stdout, data)
	},
}

func init() {
	rootCmd.AddCommand(primeCmd)
}
