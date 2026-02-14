package cmd

import (
	"fmt"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/output"
	"github.com/spf13/cobra"
)

var archiveJSON bool

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Move completed/scrapped issues to the archive",
	Long: `Moves all issues with status "completed" or "scrapped" to the archive directory (.issues/archive/).
archived issues are preserved for project memory and remain visible in all queries.
The archive keeps the main data directory tidy while preserving project history.

Relationships (parent, blocking) are preserved in archived issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		allBeans := store.All()

		// Find beans with any archive status
		var archiveBeans []*issue.Issue
		archiveSet := make(map[string]bool)
		for _, b := range allBeans {
			if cfg.IsArchiveStatus(b.Status) {
				archiveBeans = append(archiveBeans, b)
				archiveSet[b.ID] = true
			}
		}

		if len(archiveBeans) == 0 {
			if archiveJSON {
				return output.SuccessMessage("No issues to archive")
			}
			fmt.Println("No issues with archive status to archive.")
			return nil
		}

		// Sort beans for consistent display
		issue.SortByStatusPriorityAndType(archiveBeans, cfg.StatusNames(), cfg.PriorityNames(), cfg.TypeNames())

		// Archive all issues with archive status
		var archived []string
		for _, b := range archiveBeans {
			if err := store.Archive(b.ID); err != nil {
				if archiveJSON {
					return output.Error(output.ErrFileError, fmt.Sprintf("failed to archive issue %s: %s", b.ID, err.Error()))
				}
				return fmt.Errorf("failed to archive issue %s: %w", b.ID, err)
			}
			archived = append(archived, b.ID)
		}

		if archiveJSON {
			return output.SuccessMessage(fmt.Sprintf("Archived %d issue(s) to .issues/archive/", len(archived)))
		}

		fmt.Printf("Archived %d issue(s) to .issues/archive/\n", len(archived))
		return nil
	},
}

func init() {
	archiveCmd.Flags().BoolVar(&archiveJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(archiveCmd)
}
