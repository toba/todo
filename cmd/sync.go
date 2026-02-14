package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/toba/todo/internal/integration"
	"github.com/toba/todo/internal/issue"
)

var (
	syncDryRun          bool
	syncForce           bool
	syncNoRelationships bool
	syncJSON            bool
)

var syncCmd = &cobra.Command{
	Use:   "sync [issue-id...]",
	Short: "Sync issues to external integrations",
	Long: `Syncs issues to an external integration (e.g., ClickUp) configured in .todo.yml.

If issue IDs are provided, only those issues are synced. Otherwise, all issues
matching the sync filter are synced.

The sync operation:
1. Creates new tasks for issues without a linked task
2. Updates existing tasks if the issue has changed since last sync
3. Optionally syncs blocking relationships as task dependencies

Requires the appropriate API token environment variable to be set (e.g., CLICKUP_TOKEN).`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVar(&syncDryRun, "dry-run", false, "Show what would be done without making changes")
	syncCmd.Flags().BoolVar(&syncForce, "force", false, "Force update even if unchanged")
	syncCmd.Flags().BoolVar(&syncNoRelationships, "no-relationships", false, "Skip syncing blocking relationships as dependencies")
	syncCmd.Flags().BoolVar(&syncJSON, "json", false, "Output results as JSON")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Detect integration from config
	integ, err := integration.Detect(cfg.Sync, store)
	if err != nil {
		return fmt.Errorf("detecting integration: %w", err)
	}
	if integ == nil {
		if syncJSON {
			return outputSyncJSON(nil)
		}
		fmt.Println("No integration configured. Add a sync section (clickup or github) to .todo.yml.")
		return nil
	}

	// Get issues to sync
	var issueList []*issue.Issue
	if len(args) == 0 {
		issueList = store.All()
	}
	if len(args) > 0 {
		for _, id := range args {
			b, err := store.Get(id)
			if err != nil {
				return fmt.Errorf("issue not found: %s", id)
			}
			issueList = append(issueList, b)
		}
	}

	if len(issueList) == 0 {
		if syncJSON {
			return outputSyncJSON(nil)
		}
		fmt.Println("No issues to sync")
		return nil
	}

	// Build sync options
	opts := integration.SyncOptions{
		DryRun:          syncDryRun,
		Force:           syncForce,
		NoRelationships: syncNoRelationships,
	}

	// Show progress unless JSON output is requested
	if !syncJSON {
		fmt.Printf("Syncing %d issues to %s", len(issueList), integ.Name())
		if len(issueList) >= 5 {
			fmt.Print(" ")
			opts.OnProgress = func(result integration.SyncResult, completed, total int) {
				if result.Error != nil {
					fmt.Print("x")
				} else {
					fmt.Print(".")
				}
			}
		}
	}

	// Run sync
	results, err := integ.Sync(ctx, issueList, opts)

	// Print newline after progress dots
	if !syncJSON {
		fmt.Println()
	}

	if err != nil {
		return err
	}

	if results == nil {
		if syncJSON {
			return outputSyncJSON(nil)
		}
		fmt.Println("All issues up to date")
		return nil
	}

	// Output results
	if syncJSON {
		return outputSyncJSON(results)
	}
	return outputSyncText(results)
}

func outputSyncJSON(results []integration.SyncResult) error {
	type jsonResult struct {
		IssueID     string `json:"issue_id"`
		IssueTitle  string `json:"issue_title"`
		ExternalID  string `json:"external_id,omitempty"`
		ExternalURL string `json:"external_url,omitempty"`
		Action      string `json:"action"`
		Error       string `json:"error,omitempty"`
	}

	if results == nil {
		fmt.Println("[]")
		return nil
	}

	jsonResults := make([]jsonResult, len(results))
	for i, r := range results {
		jsonResults[i] = jsonResult{
			IssueID:     r.IssueID,
			IssueTitle:  r.IssueTitle,
			ExternalID:  r.ExternalID,
			ExternalURL: r.ExternalURL,
			Action:      r.Action,
		}
		if r.Error != nil {
			jsonResults[i].Error = r.Error.Error()
		}
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonResults)
}

func truncateTitle(title string, maxLen int) string {
	if len(title) <= maxLen {
		return title
	}
	return title[:maxLen] + "\u2026"
}

func outputSyncText(results []integration.SyncResult) error {
	var created, updated, unchanged, skipped, errors int

	for _, r := range results {
		switch r.Action {
		case integration.ActionCreated:
			created++
			fmt.Printf("  Created: %s \u2192 %s \"%s\"\n", r.IssueID, r.ExternalURL, truncateTitle(r.IssueTitle, 20))
		case integration.ActionUpdated:
			updated++
			fmt.Printf("  Updated: %s \u2192 %s \"%s\"\n", r.IssueID, r.ExternalURL, truncateTitle(r.IssueTitle, 20))
		case integration.ActionUnchanged:
			unchanged++
		case integration.ActionSkipped:
			skipped++
		case integration.ActionWouldCreate:
			fmt.Printf("  Would create: %s - %s\n", r.IssueID, r.IssueTitle)
		case integration.ActionWouldUpdate:
			fmt.Printf("  Would update: %s - %s\n", r.IssueID, r.IssueTitle)
		case integration.ActionError:
			errors++
			fmt.Printf("  Error: %s - %v\n", r.IssueID, r.Error)
		}
	}

	fmt.Printf("\nSummary: %d created, %d updated, %d unchanged, %d skipped, %d errors\n",
		created, updated, unchanged, skipped, errors)
	return nil
}
