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

var syncLinkJSON bool

var syncLinkCmd = &cobra.Command{
	Use:   "link <issue-id> <task-id>",
	Short: "Link an issue to an existing external task",
	Long: `Manually links an issue to an existing external task (e.g., ClickUp task)
by storing the task ID in the issue's extension metadata.

This is useful when you have an existing task that you want to
associate with an issue, or when syncing fails and you need to fix the link.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]
		taskID := args[1]

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

		// Check if already linked to this task
		existingTaskID := clickup.GetExtensionString(b, clickup.ExtKeyTaskID)
		if existingTaskID == taskID {
			if syncLinkJSON {
				return outputLinkJSON(b.ID, b.Title, taskID, "already_linked")
			}
			fmt.Printf("Skipped: %s already linked to %s\n", b.ID, taskID)
			return nil
		}

		// Link it
		if err := integ.Link(ctx, issueID, taskID); err != nil {
			return err
		}

		if syncLinkJSON {
			return outputLinkJSON(b.ID, b.Title, taskID, "linked")
		}

		fmt.Printf("Linked: %s \u2192 %s\n", b.ID, taskID)
		return nil
	},
}

func init() {
	syncLinkCmd.Flags().BoolVar(&syncLinkJSON, "json", false, "Output as JSON")
	syncCmd.AddCommand(syncLinkCmd)
}

func outputLinkJSON(issueID, issueTitle, taskID, action string) error {
	result := map[string]string{
		"issue_id":    issueID,
		"issue_title": issueTitle,
		"task_id":     taskID,
		"action":      action,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
