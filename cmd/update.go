package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/graph/model"
	"github.com/toba/todo/internal/output"
	"github.com/toba/todo/internal/ui"
	"github.com/spf13/cobra"
)

var (
	updateStatus          string
	updateType            string
	updatePriority        string
	updateTitle           string
	updateBody            string
	updateBodyFile        string
	updateBodyReplaceOld  string
	updateBodyReplaceNew  string
	updateBodyAppend      string
	updateDue             string
	updateParent          string
	updateRemoveParent    bool
	updateBlocking        []string
	updateRemoveBlocking  []string
	updateBlockedBy       []string
	updateRemoveBlockedBy []string
	updateTag             []string
	updateRemoveTag       []string
	updateIfMatch         string
	updateJSON            bool
)

var updateCmd = &cobra.Command{
	Use:     "update <id>",
	Aliases: []string{"u"},
	Short:   "Update an issue's properties",
	Long:    `Updates one or more properties of an existing issue.`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		resolver := &graph.Resolver{Core: store}

		// Find the issue
		b, err := resolver.Query().Issue(ctx, args[0])
		if err != nil {
			return cmdError(updateJSON, output.ErrNotFound, "failed to find issue: %v", err)
		}

		// If not found, check the archive and unarchive if present
		wasArchived := false
		if b == nil {
			unarchived, unarchiveErr := store.LoadAndUnarchive(args[0])
			if unarchiveErr != nil {
				return cmdError(updateJSON, output.ErrNotFound, "issue not found: %s", args[0])
			}
			// Re-query to get the model.Issue
			b, err = resolver.Query().Issue(ctx, unarchived.ID)
			if err != nil || b == nil {
				return cmdError(updateJSON, output.ErrNotFound, "issue not found: %s", args[0])
			}
			wasArchived = true
		}

		// Track changes for output
		var changes []string

		// Prepare ifMatch for GraphQL mutations
		var ifMatch *string
		if updateIfMatch != "" {
			ifMatch = &updateIfMatch
		}

		// Build and validate field updates
		input, fieldChanges, err := buildUpdateInput(cmd, b.Tags, b.Body)
		if err != nil {
			return cmdError(updateJSON, output.ErrValidation, "%s", err)
		}
		changes = append(changes, fieldChanges...)

		// Add ifMatch to input if provided
		if ifMatch != nil {
			input.IfMatch = ifMatch
		}

		// Apply all updates atomically via single UpdateBean mutation
		// This includes field updates, body modifications, and relationship changes
		if hasFieldUpdates(input) {
			b, err = resolver.Mutation().UpdateIssue(ctx, b.ID, input)
			if err != nil {
				return mutationError(updateJSON, err)
			}
		}

		// Require at least one change
		if len(changes) == 0 {
			return cmdError(updateJSON, output.ErrValidation,
				"no changes specified (use --status, --type, --priority, --title, --due, --body, --parent, --blocking, --blocked-by, --tag, or their --remove-* variants)")
		}

		// Output result
		if updateJSON {
			msg := "Bean updated"
			if wasArchived {
				msg = "Bean unarchived and updated"
			}
			return output.Success(b, msg)
		}

		if wasArchived {
			fmt.Println(ui.Success.Render("Unarchived and updated ") + ui.ID.Render(b.ID) + " " + ui.Muted.Render(b.Path))
		} else {
			fmt.Println(ui.Success.Render("Updated ") + ui.ID.Render(b.ID) + " " + ui.Muted.Render(b.Path))
		}
		return nil
	},
}

// buildUpdateInput constructs the GraphQL input from flags and returns which fields changed.
func buildUpdateInput(cmd *cobra.Command, existingTags []string, currentBody string) (model.UpdateIssueInput, []string, error) {
	var input model.UpdateIssueInput
	var changes []string

	if cmd.Flags().Changed("status") {
		if !cfg.IsValidStatus(updateStatus) {
			return input, nil, fmt.Errorf("invalid status: %s (must be %s)", updateStatus, cfg.StatusList())
		}
		input.Status = &updateStatus
		changes = append(changes, "status")
	}

	if cmd.Flags().Changed("type") {
		if !cfg.IsValidType(updateType) {
			return input, nil, fmt.Errorf("invalid type: %s (must be %s)", updateType, cfg.TypeList())
		}
		input.Type = &updateType
		changes = append(changes, "type")
	}

	if cmd.Flags().Changed("priority") {
		if !cfg.IsValidPriority(updatePriority) {
			return input, nil, fmt.Errorf("invalid priority: %s (must be %s)", updatePriority, cfg.PriorityList())
		}
		input.Priority = &updatePriority
		changes = append(changes, "priority")
	}

	if cmd.Flags().Changed("title") {
		input.Title = &updateTitle
		changes = append(changes, "title")
	}

	if cmd.Flags().Changed("due") {
		input.Due = &updateDue
		changes = append(changes, "due")
	}

	// Handle body modifications
	if cmd.Flags().Changed("body") || cmd.Flags().Changed("body-file") {
		// Full body replacement
		body, err := resolveContent(updateBody, updateBodyFile)
		if err != nil {
			return input, nil, err
		}
		input.Body = &body
		changes = append(changes, "body")
	} else if cmd.Flags().Changed("body-replace-old") || cmd.Flags().Changed("body-append") {
		// Body modifications via bodyMod
		bodyMod := &model.BodyModification{}

		if cmd.Flags().Changed("body-replace-old") {
			// --body-replace-old requires --body-replace-new (enforced by MarkFlagsRequiredTogether)
			bodyMod.Replace = []*model.ReplaceOperation{
				{
					Old: updateBodyReplaceOld,
					New: updateBodyReplaceNew,
				},
			}
		}

		if cmd.Flags().Changed("body-append") {
			appendText, err := resolveAppendContent(updateBodyAppend)
			if err != nil {
				return input, nil, err
			}
			bodyMod.Append = &appendText
		}

		input.BodyMod = bodyMod
		changes = append(changes, "body")
	}

	// Handle tags using granular add/remove (consistent with relationships)
	if len(updateTag) > 0 {
		input.AddTags = updateTag
		changes = append(changes, "tags")
	}
	if len(updateRemoveTag) > 0 {
		input.RemoveTags = updateRemoveTag
		changes = append(changes, "tags")
	}

	// Handle parent relationship
	if cmd.Flags().Changed("parent") {
		input.Parent = &updateParent
		changes = append(changes, "parent")
	} else if updateRemoveParent {
		emptyParent := ""
		input.Parent = &emptyParent
		changes = append(changes, "parent")
	}

	// Handle blocking relationships
	if len(updateBlocking) > 0 {
		input.AddBlocking = updateBlocking
		changes = append(changes, "blocking")
	}
	if len(updateRemoveBlocking) > 0 {
		input.RemoveBlocking = updateRemoveBlocking
		changes = append(changes, "blocking")
	}

	// Handle blocked-by relationships
	if len(updateBlockedBy) > 0 {
		input.AddBlockedBy = updateBlockedBy
		changes = append(changes, "blocked-by")
	}
	if len(updateRemoveBlockedBy) > 0 {
		input.RemoveBlockedBy = updateRemoveBlockedBy
		changes = append(changes, "blocked-by")
	}

	return input, changes, nil
}

// hasFieldUpdates returns true if any field in the input is set.
func hasFieldUpdates(input model.UpdateIssueInput) bool {
	return input.Status != nil || input.Type != nil || input.Priority != nil ||
		input.Title != nil || input.Due != nil || input.Body != nil || input.BodyMod != nil || input.Tags != nil ||
		input.AddTags != nil || input.RemoveTags != nil ||
		input.Parent != nil || input.AddBlocking != nil || input.RemoveBlocking != nil ||
		input.AddBlockedBy != nil || input.RemoveBlockedBy != nil
}

// isConflictError returns true if the error is an ETag-related conflict error.
func isConflictError(err error) bool {
	_, isMismatch := errors.AsType[*core.ETagMismatchError](err)
	_, isRequired := errors.AsType[*core.ETagRequiredError](err)
	return isMismatch || isRequired
}

// mutationError returns a cmdError with the appropriate error code based on the error type.
func mutationError(jsonOutput bool, err error) error {
	if isConflictError(err) {
		return cmdError(jsonOutput, output.ErrConflict, "%s", err)
	}
	return cmdError(jsonOutput, output.ErrValidation, "%s", err)
}

func init() {
	// Build help text with allowed values from hardcoded config
	statusNames := config.DefaultStatusNames()
	typeNames := config.DefaultTypeNames()
	priorityNames := config.DefaultPriorityNames()

	updateCmd.Flags().StringVarP(&updateStatus, "status", "s", "", "New status ("+strings.Join(statusNames, ", ")+")")
	updateCmd.Flags().StringVarP(&updateType, "type", "t", "", "New type ("+strings.Join(typeNames, ", ")+")")
	updateCmd.Flags().StringVarP(&updatePriority, "priority", "p", "", "New priority ("+strings.Join(priorityNames, ", ")+", or empty to clear)")
	updateCmd.Flags().StringVar(&updateTitle, "title", "", "New title")
	updateCmd.Flags().StringVar(&updateDue, "due", "", "Due date (YYYY-MM-DD, empty to clear)")
	updateCmd.Flags().StringVarP(&updateBody, "body", "d", "", "New body (use '-' to read from stdin)")
	updateCmd.Flags().StringVar(&updateBodyFile, "body-file", "", "Read body from file")
	updateCmd.Flags().StringVar(&updateBodyReplaceOld, "body-replace-old", "", "Text to find and replace (requires --body-replace-new)")
	updateCmd.Flags().StringVar(&updateBodyReplaceNew, "body-replace-new", "", "Replacement text (requires --body-replace-old)")
	updateCmd.Flags().StringVar(&updateBodyAppend, "body-append", "", "Text to append to body (use '-' for stdin)")
	updateCmd.Flags().StringVar(&updateParent, "parent", "", "Set parent issue ID")
	updateCmd.Flags().BoolVar(&updateRemoveParent, "remove-parent", false, "Remove parent")
	updateCmd.Flags().StringArrayVar(&updateBlocking, "blocking", nil, "ID of issue this blocks (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateRemoveBlocking, "remove-blocking", nil, "ID of issue to unblock (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateBlockedBy, "blocked-by", nil, "ID of issue that blocks this one (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateRemoveBlockedBy, "remove-blocked-by", nil, "ID of blocker issue to remove (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateTag, "tag", nil, "Add tag (can be repeated)")
	updateCmd.Flags().StringArrayVar(&updateRemoveTag, "remove-tag", nil, "Remove tag (can be repeated)")
	updateCmd.Flags().StringVar(&updateIfMatch, "if-match", "", "Only update if etag matches (optimistic locking)")
	updateCmd.MarkFlagsMutuallyExclusive("parent", "remove-parent")
	updateCmd.Flags().BoolVar(&updateJSON, "json", false, "Output as JSON")
	// body and body-file are mutually exclusive with body modifications
	updateCmd.MarkFlagsMutuallyExclusive("body", "body-file", "body-replace-old")
	updateCmd.MarkFlagsMutuallyExclusive("body", "body-file", "body-append")
	// body-replace-old and body-append can now be used together!
	updateCmd.MarkFlagsRequiredTogether("body-replace-old", "body-replace-new")
	rootCmd.AddCommand(updateCmd)
}
