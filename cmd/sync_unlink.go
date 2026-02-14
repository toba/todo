package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/integration"
	"github.com/toba/todo/internal/integration/clickup"
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

		// Get the issue to check current state
		b, err := store.Get(issueID)
		if err != nil {
			return fmt.Errorf("issue not found: %s", issueID)
		}

		// Check if linked
		taskID := clickup.GetExtensionString(b, clickup.ExtKeyTaskID)
		if taskID == "" {
			if syncUnlinkJSON {
				return outputUnlinkJSON(b.ID, b.Title, "", "not_linked")
			}
			fmt.Printf("Skipped: %s is not linked to an external task\n", b.ID)
			return nil
		}

		// Unlink it
		if err := integ.Unlink(ctx, issueID); err != nil {
			return err
		}

		if syncUnlinkJSON {
			return outputUnlinkJSON(b.ID, b.Title, taskID, "unlinked")
		}

		fmt.Printf("Unlinked: %s (was %s)\n", b.ID, taskID)
		return nil
	},
}

func init() {
	syncUnlinkCmd.Flags().BoolVar(&syncUnlinkJSON, "json", false, "Output as JSON")
	syncCmd.AddCommand(syncUnlinkCmd)
}

func outputUnlinkJSON(issueID, issueTitle, taskID, action string) error {
	result := map[string]string{
		"issue_id":    issueID,
		"issue_title": issueTitle,
		"action":      action,
	}
	if taskID != "" {
		result["task_id"] = taskID
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
