package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/migrate"
	"github.com/toba/todo/internal/output"
)

var migrateSource string
var migrateJSON bool

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Migrate from old beans format to new issues format",
	Long: `Converts an old-format beans project (hmans/beans with .beans/ directory)
to the new issues format (.issues/ directory).

This command:
  - Rewrites .todo.yml: beans: key → issues: key, removes prefix/id_length
  - Copies bean files from old data dir into .issues/beans/ subfolder
  - Copies archived beans into .issues/archive/
  - Converts status: todo → status: ready in all frontmatter

Old IDs (beans-xxxx) are preserved. Cross-references remain valid.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := migrate.Options{
			SourceDir:  migrateSource,
			ConfigPath: configPath,
		}

		result, err := migrate.Run(opts)
		if err != nil {
			if migrateJSON {
				return output.Error(output.ErrFileError, err.Error())
			}
			return err
		}

		if migrateJSON {
			msg := fmt.Sprintf("Migrated %d active and %d archived beans (%d status conversions)",
				result.ActiveCount, result.ArchivedCount, result.StatusConverted)
			if result.ClickUpImported {
				msg += "; ClickUp config imported"
			}
			return output.JSON(output.Response{
				Success: true,
				Message: msg,
			})
		}

		fmt.Printf("Migration complete:\n")
		fmt.Printf("  Active beans migrated:   %d\n", result.ActiveCount)
		fmt.Printf("  Archived beans migrated: %d\n", result.ArchivedCount)
		fmt.Printf("  Status conversions:      %d (todo → ready)\n", result.StatusConverted)
		if result.ConfigMigrated {
			fmt.Printf("  Config rewritten:        yes\n")
		}
		if result.ClickUpImported {
			fmt.Printf("  ClickUp config imported: yes\n")
		}
		fmt.Printf("  Data directory:          %s\n", result.NewDataDir)

		return nil
	},
}

func init() {
	migrateCmd.Flags().StringVar(&migrateSource, "source", "", "Path to old data directory (default: auto-detect from config)")
	migrateCmd.Flags().BoolVar(&migrateJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(migrateCmd)
}
