package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/output"
	"github.com/toba/todo/internal/refry"
)

var refrySource string
var refryJSON bool

var refryCmd = &cobra.Command{
	Use:   "refry",
	Short: "Convert an hmans/beans project to toba/todo format",
	Long: `Converts a project using the original hmans/beans issue tracker
(.beans.yml config, .beans/ data directory) to toba/todo format
(.toba.yaml config, .issues/ with bucketed subdirectories).

File moves use os.Rename to preserve git history.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := refry.Options{
			SourceDir: refrySource,
		}

		result, err := refry.Run(opts)
		if err != nil {
			if refryJSON {
				return output.Error(output.ErrFileError, err.Error())
			}
			return err
		}

		if refryJSON {
			return output.JSON(output.Response{
				Success: true,
				Message: fmt.Sprintf("Converted %d active and %d archived issues (%d status rewrites)",
					result.ActiveCount, result.ArchivedCount, result.StatusConverted),
				Path: result.NewDataDir,
			})
		}

		fmt.Printf("Converted %d active issue(s)\n", result.ActiveCount)
		if result.ArchivedCount > 0 {
			fmt.Printf("Converted %d archived issue(s)\n", result.ArchivedCount)
		}
		if result.StatusConverted > 0 {
			fmt.Printf("Rewrote %d status(es) from todo → ready\n", result.StatusConverted)
		}
		fmt.Printf("Config: %s\n", result.NewConfigPath)
		fmt.Printf("Data:   %s\n", result.NewDataDir)

		return nil
	},
}

func init() {
	refryCmd.Flags().StringVar(&refrySource, "source", "", "Source directory (overrides .beans.yml path)")
	refryCmd.Flags().BoolVar(&refryJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(refryCmd)
}
