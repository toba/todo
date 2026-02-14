package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/graph/model"
	"github.com/toba/todo/internal/output"
	"github.com/toba/todo/internal/ui"
	"github.com/spf13/cobra"
)

var (
	createStatus    string
	createType      string
	createPriority  string
	createBody      string
	createBodyFile  string
	createTag       []string
	createDue       string
	createParent    string
	createBlocking  []string
	createBlockedBy []string
	createPrefix    string
	createJSON      bool
)

var createCmd = &cobra.Command{
	Use:     "create [title]",
	Aliases: []string{"c", "new"},
	Short:   "Create a new issue",
	Long:    `Creates a new issue (issue) with a generated ID and optional title.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")
		if title == "" {
			title = "Untitled"
		}

		// Validate inputs
		if createStatus != "" && !cfg.IsValidStatus(createStatus) {
			return cmdError(createJSON, output.ErrInvalidStatus, "invalid status: %s (must be %s)", createStatus, cfg.StatusList())
		}
		if createType != "" && !cfg.IsValidType(createType) {
			return cmdError(createJSON, output.ErrValidation, "invalid type: %s (must be %s)", createType, cfg.TypeList())
		}
		if createPriority != "" && !cfg.IsValidPriority(createPriority) {
			return cmdError(createJSON, output.ErrValidation, "invalid priority: %s (must be %s)", createPriority, cfg.PriorityList())
		}

		body, err := resolveContent(createBody, createBodyFile)
		if err != nil {
			return cmdError(createJSON, output.ErrFileError, "%s", err)
		}

		// Build GraphQL input
		input := model.CreateIssueInput{Title: title}
		if createStatus != "" {
			input.Status = &createStatus
		} else {
			defaultStatus := cfg.GetDefaultStatus()
			input.Status = &defaultStatus
		}
		if createType != "" {
			input.Type = &createType
		} else {
			defaultType := cfg.GetDefaultType()
			input.Type = &defaultType
		}
		if createPriority != "" {
			input.Priority = &createPriority
		}
		if body != "" {
			input.Body = &body
		}
		if len(createTag) > 0 {
			input.Tags = createTag
		}
		if createDue != "" {
			input.Due = &createDue
		}

		// Add parent
		if createParent != "" {
			input.Parent = &createParent
		}

		// Add blocking
		if len(createBlocking) > 0 {
			input.Blocking = createBlocking
		}

		// Add blocked_by
		if len(createBlockedBy) > 0 {
			input.BlockedBy = createBlockedBy
		}

		// Add custom prefix
		if createPrefix != "" {
			input.Prefix = &createPrefix
		}

		// Create via GraphQL mutation
		resolver := &graph.Resolver{Core: store}
		b, err := resolver.Mutation().CreateIssue(context.Background(), input)
		if err != nil {
			return cmdError(createJSON, output.ErrFileError, "failed to create issue: %v", err)
		}

		if createJSON {
			return output.Success(b, "Bean created")
		}

		fmt.Println(ui.Success.Render("Created ") + ui.ID.Render(b.ID) + " " + ui.Muted.Render(b.Path))
		return nil
	},
}

func init() {
	// Build help text with allowed values from hardcoded config
	statusNames := config.DefaultStatusNames()
	typeNames := config.DefaultTypeNames()
	priorityNames := config.DefaultPriorityNames()

	createCmd.Flags().StringVarP(&createStatus, "status", "s", "", "Initial status ("+strings.Join(statusNames, ", ")+")")
	createCmd.Flags().StringVarP(&createType, "type", "t", "", "issue type ("+strings.Join(typeNames, ", ")+")")
	createCmd.Flags().StringVarP(&createPriority, "priority", "p", "", "Priority level ("+strings.Join(priorityNames, ", ")+")")
	createCmd.Flags().StringVarP(&createBody, "body", "d", "", "Body content (use '-' to read from stdin)")
	createCmd.Flags().StringVar(&createBodyFile, "body-file", "", "Read body from file")
	createCmd.Flags().StringArrayVar(&createTag, "tag", nil, "Add tag (can be repeated)")
	createCmd.Flags().StringVar(&createDue, "due", "", "Due date (YYYY-MM-DD)")
	createCmd.Flags().StringVar(&createParent, "parent", "", "Parent issue ID")
	createCmd.Flags().StringArrayVar(&createBlocking, "blocking", nil, "ID of issue this blocks (can be repeated)")
	createCmd.Flags().StringArrayVar(&createBlockedBy, "blocked-by", nil, "ID of issue that blocks this one (can be repeated)")
	createCmd.Flags().StringVar(&createPrefix, "prefix", "", "Custom ID prefix (overrides config prefix)")
	createCmd.Flags().BoolVar(&createJSON, "json", false, "Output as JSON")
	createCmd.MarkFlagsMutuallyExclusive("body", "body-file")
	rootCmd.AddCommand(createCmd)
}
