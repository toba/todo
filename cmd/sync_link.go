package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/integration"
)

var syncLinkJSON bool

var syncLinkCmd = &cobra.Command{
	Use:   "link <issue-id> <external-id>",
	Short: "Link an issue to an existing external task",
	Long: `Manually links an issue to an existing external task (e.g., ClickUp task or GitHub issue)
by storing the external ID in the issue's extension metadata.

This is useful when you have an existing task that you want to
associate with an issue, or when syncing fails and you need to fix the link.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		issueID := args[0]
		externalID := args[1]

		ctx := context.Background()

		// Detect integration
		integ, err := integration.Detect(cfg.Extensions, store)
		if err != nil {
			return fmt.Errorf("detecting integration: %w", err)
		}
		if integ == nil {
			return fmt.Errorf("no integration configured")
		}

		// Link it
		result, err := integ.Link(ctx, issueID, externalID)
		if err != nil {
			return err
		}

		// Get the issue for display
		b, getErr := store.Get(issueID)
		title := issueID
		if getErr == nil {
			title = b.Title
		}

		if syncLinkJSON {
			return outputLinkJSON(issueID, title, externalID, result.Action)
		}

		switch result.Action {
		case "already_linked":
			fmt.Printf("Skipped: %s already linked to %s\n", issueID, externalID)
		case "linked":
			fmt.Printf("Linked: %s → %s\n", issueID, externalID)
		}
		return nil
	},
}

func init() {
	syncLinkCmd.Flags().BoolVar(&syncLinkJSON, "json", false, "Output as JSON")
	syncCmd.AddCommand(syncLinkCmd)
}

func outputLinkJSON(issueID, issueTitle, externalID, action string) error {
	result := map[string]string{
		"issue_id":    issueID,
		"issue_title": issueTitle,
		"external_id": externalID,
		"action":      action,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
