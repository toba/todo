package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/integration"
)

var syncUnlinkJSON bool

var syncUnlinkCmd = &cobra.Command{
	Use:   "unlink <issue-id>",
	Short: "Remove the link between an issue and its external task",
	Long: `Removes the sync metadata from an issue's extension data,
unlinking it from its associated external task.

Note: This does not delete or modify the external task itself.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]
		ctx := context.Background()

		// Detect integration
		integ, err := integration.Detect(cfg.Extensions, store)
		if err != nil {
			return fmt.Errorf("detecting integration: %w", err)
		}
		if integ == nil {
			return fmt.Errorf("no integration configured")
		}

		// Unlink it
		result, err := integ.Unlink(ctx, issueID)
		if err != nil {
			return err
		}

		// Get the issue for display
		b, getErr := store.Get(issueID)
		title := issueID
		if getErr == nil {
			title = b.Title
		}

		if syncUnlinkJSON {
			return outputUnlinkJSON(issueID, title, result.ExternalID, result.Action)
		}

		switch result.Action {
		case "not_linked":
			fmt.Printf("Skipped: %s is not linked to an external task\n", issueID)
		case "unlinked":
			fmt.Printf("Unlinked: %s (was %s)\n", issueID, result.ExternalID)
		}
		return nil
	},
}

func init() {
	syncUnlinkCmd.Flags().BoolVar(&syncUnlinkJSON, "json", false, "Output as JSON")
	syncCmd.AddCommand(syncUnlinkCmd)
}

func outputUnlinkJSON(issueID, issueTitle, externalID, action string) error {
	result := map[string]string{
		"issue_id":    issueID,
		"issue_title": issueTitle,
		"action":      action,
	}
	if externalID != "" {
		result["external_id"] = externalID
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
