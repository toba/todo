package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/output"
	"github.com/spf13/cobra"
)

var (
	forceDelete bool
	deleteJSON  bool
)

// issueWithLinks holds an issue and its incoming links for batch processing
type issueWithLinks struct {
	issue  *issue.Issue
	links []core.IncomingLink
}

var deleteCmd = &cobra.Command{
	Use:     "delete <id> [id...]",
	Aliases: []string{"rm"},
	Short:   "Delete one or more issues",
	Long: `Deletes one or more beans after confirmation (use -f to skip confirmation).

If other issues reference the target issue(s) (as parent or via blocking), you will be
warned and those references will be removed after confirmation. Use -f to skip all warnings.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := &graph.Resolver{Core: store}

		// Collect all issues and their incoming links upfront (validate before deleting)
		var targets []issueWithLinks
		for _, id := range args {
			b, err := resolver.Query().Issue(ctx, id)
			if err != nil {
				return cmdError(deleteJSON, output.ErrNotFound, "failed to find issue: %v", err)
			}
			if b == nil {
				return cmdError(deleteJSON, output.ErrNotFound, "issue not found: %s", id)
			}
			targets = append(targets, issueWithLinks{
				issue:  b,
				links: store.FindIncomingLinks(b.ID),
			})
		}

		// Prompt for confirmation (JSON implies force)
		if !forceDelete && !deleteJSON {
			if !confirmDeleteMultiple(targets) {
				fmt.Println("Cancelled")
				return nil
			}
		}

		// Delete all issues via GraphQL mutation
		var deleted []*issue.Issue
		var totalLinksRemoved int
		for _, target := range targets {
			_, err := resolver.Mutation().DeleteIssue(ctx, target.issue.ID)
			if err != nil {
				return cmdError(deleteJSON, output.ErrFileError, "failed to delete issue %s: %v", target.issue.ID, err)
			}
			deleted = append(deleted, target.issue)
			totalLinksRemoved += len(target.links)
		}

		// Output results
		if deleteJSON {
			if len(deleted) == 1 {
				return output.Success(deleted[0], "Bean deleted")
			}
			return output.JSON(output.Response{
				Success: true,
				Issues:   deleted,
				Count:   len(deleted),
				Message: fmt.Sprintf("%d issues deleted", len(deleted)),
			})
		}

		if totalLinksRemoved > 0 {
			fmt.Printf("Removed %d reference(s)\n", totalLinksRemoved)
		}
		for _, b := range deleted {
			fmt.Printf("Deleted %s\n", b.Path)
		}
		return nil
	},
}

// confirmDeleteMultiple prompts the user to confirm deletion of one or more beans.
func confirmDeleteMultiple(targets []issueWithLinks) bool {
	beansWithLinks := 0
	totalLinks := 0
	for _, t := range targets {
		if len(t.links) > 0 {
			beansWithLinks++
			totalLinks += len(t.links)
		}
	}

	// Single issue: use simpler format
	if len(targets) == 1 {
		t := targets[0]
		if len(t.links) > 0 {
			fmt.Printf("Warning: %d issue(s) link to '%s':\n", len(t.links), t.issue.Title)
			for _, link := range t.links {
				fmt.Printf("  - %s (%s) via %s\n", link.FromBean.ID, link.FromBean.Title, link.LinkType)
			}
			fmt.Print("Delete anyway and remove references? [y/N] ")
		} else {
			fmt.Printf("Delete '%s' (%s)? [y/N] ", t.issue.Title, t.issue.Path)
		}
	} else {
		// Multiple beans: show batch summary
		fmt.Printf("About to delete %d issue(s):\n", len(targets))
		for _, t := range targets {
			if len(t.links) > 0 {
				fmt.Printf("  - %s (%s) ← %d incoming link(s)\n", t.issue.ID, t.issue.Title, len(t.links))
			} else {
				fmt.Printf("  - %s (%s)\n", t.issue.ID, t.issue.Title)
			}
		}
		if beansWithLinks > 0 {
			fmt.Printf("\nWarning: %d issue(s) have incoming references (%d total) that will be removed.\n", beansWithLinks, totalLinks)
		}
		fmt.Print("\nProceed with deletion? [y/N] ")
	}

	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes"
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Skip confirmation and warnings")
	deleteCmd.Flags().BoolVar(&deleteJSON, "json", false, "Output as JSON (implies --force)")
	rootCmd.AddCommand(deleteCmd)
}
