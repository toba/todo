package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/integration"
	"github.com/toba/todo/internal/ui"
)

var (
	syncCheckSkipAPI bool
	syncCheckJSON    bool
)

var syncCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify integration configuration and sync state health",
	Long: `Validates integration configuration, connectivity, and sync state.

Checks include:
  - Configuration (list ID, status/priority/type mappings, custom fields)
  - Mapping key validation (status/type/priority keys match project values)
  - Integration connectivity (API token, access)
  - Sync state (linked issues, stale syncs, missing tasks)

Use --skip-api to perform offline validation only.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Suppress usage on error since check errors are specific validation failures
		cmd.SilenceUsage = true

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Detect integration
		integ, err := integration.Detect(cfg.Sync, store)
		if err != nil {
			return fmt.Errorf("detecting integration: %w", err)
		}
		if integ == nil {
			if syncCheckJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{"error": "no integration configured"})
			}
			fmt.Println("No integration configured. Add a sync section (clickup or github) to .todo.yml.")
			return nil
		}

		opts := integration.CheckOptions{
			SkipAPI: syncCheckSkipAPI,
		}

		report, err := integ.Check(ctx, opts)
		if err != nil {
			return err
		}

		if syncCheckJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}

		// Text output
		printCheckReport(report)

		// Exit with error code if any checks failed
		if report.Summary.Failed > 0 {
			return fmt.Errorf("%d check(s) failed", report.Summary.Failed)
		}

		return nil
	},
}

func init() {
	syncCheckCmd.Flags().BoolVar(&syncCheckSkipAPI, "skip-api", false, "Skip API checks (offline validation only)")
	syncCheckCmd.Flags().BoolVar(&syncCheckJSON, "json", false, "Output as JSON")
	syncCmd.AddCommand(syncCheckCmd)
}

func printCheckReport(report *integration.CheckReport) {
	for _, section := range report.Sections {
		fmt.Println(ui.Bold.Render(section.Name))
		for _, check := range section.Checks {
			switch check.Status {
			case integration.CheckPass:
				fmt.Print(ui.Success.Render("  \u2713 "))
			case integration.CheckWarn:
				fmt.Print(ui.Warning.Render("  \u26a0 "))
			case integration.CheckFail:
				fmt.Print(ui.Danger.Render("  \u2717 "))
			}

			fmt.Print(check.Name)
			if check.Message != "" {
				fmt.Print(ui.Muted.Render(fmt.Sprintf(" (%s)", check.Message)))
			}
			fmt.Println()
		}
		fmt.Println()
	}

	// Print summary
	fmt.Print(ui.Bold.Render("Summary: "))
	fmt.Print(ui.Success.Render(fmt.Sprintf("%d passed", report.Summary.Passed)))
	if report.Summary.Warnings > 0 {
		fmt.Print(", ")
		fmt.Print(ui.Warning.Render(fmt.Sprintf("%d warnings", report.Summary.Warnings)))
	}
	if report.Summary.Failed > 0 {
		fmt.Print(", ")
		fmt.Print(ui.Danger.Render(fmt.Sprintf("%d failed", report.Summary.Failed)))
	}
	fmt.Println()
}
