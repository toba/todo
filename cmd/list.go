package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/graph/model"
	"github.com/toba/todo/internal/output"
	"github.com/toba/todo/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	listJSON       bool
	listSearch     string
	listStatus     []string
	listNoStatus   []string
	listType       []string
	listNoType     []string
	listPriority   []string
	listNoPriority []string
	listTag        []string
	listNoTag      []string
	listHasParent   bool
	listNoParent    bool
	listParentID    string
	listHasBlocking bool
	listNoBlocking  bool
	listIsBlocked   bool
	listReady      bool
	listQuiet      bool
	listSort       string
	listFull       bool
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all issues",
	Long: `Lists all issues in the data directory.

Search Syntax (--search/-S):
  The search flag supports Bleve query string syntax:

  login          Exact term match
  login~         Fuzzy match (1 edit distance, finds "loggin", "logins")
  login~2        Fuzzy match (2 edit distance)
  log*           Wildcard prefix match
  "user login"   Exact phrase match
  user AND login Both terms required
  user OR login  Either term matches
  slug:auth      Search only in slug field
  title:login    Search only in title field
  body:auth      Search only in body field`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build GraphQL filter from CLI flags
		filter := &model.IssueFilter{
			Status:          listStatus,
			ExcludeStatus:   listNoStatus,
			Type:            listType,
			ExcludeType:     listNoType,
			Priority:        listPriority,
			ExcludePriority: listNoPriority,
			Tags:            listTag,
			ExcludeTags:     listNoTag,
		}

		// Add search filter if provided
		if listSearch != "" {
			filter.Search = &listSearch
		}

		// Add parent/blocks filters
		if listHasParent {
			filter.HasParent = &listHasParent
		}
		if listNoParent {
			filter.NoParent = &listNoParent
		}
		if listParentID != "" {
			filter.ParentID = &listParentID
		}
		if listHasBlocking {
			filter.HasBlocking = &listHasBlocking
		}
		if listNoBlocking {
			filter.NoBlocking = &listNoBlocking
		}
		// --ready and --is-blocked are mutually exclusive
		if listReady && listIsBlocked {
			return fmt.Errorf("--ready and --is-blocked are mutually exclusive")
		}

		if listIsBlocked {
			filter.IsBlocked = &listIsBlocked
		}

		// --ready: issues available to start (not blocked, excludes in-progress/completed/scrapped/draft)
		if listReady {
			isBlocked := false
			filter.IsBlocked = &isBlocked
			filter.ExcludeStatus = append(filter.ExcludeStatus, config.StatusInProgress, config.StatusCompleted, config.StatusScrapped, config.StatusDraft)
		}

		// Execute query via GraphQL resolver
		resolver := &graph.Resolver{Core: store}
		issues, err := resolver.Query().Issues(context.Background(), filter)
		if err != nil {
			return fmt.Errorf("querying issues: %w", err)
		}

		// Sort issues
		sortIssues(issues, listSort, cfg)

		// JSON output (flat list)
		if listJSON {
			if !listFull {
				for _, b := range issues {
					b.Body = ""
				}
			}
			return output.SuccessMultiple(issues)
		}

		// Quiet mode: just IDs (flat)
		if listQuiet {
			for _, b := range issues {
				fmt.Println(b.ID)
			}
			return nil
		}

		// Default: tree view
		// We need all issues to find ancestors for context
		allIssues, err := resolver.Query().Issues(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("querying all issues for tree: %w", err)
		}

		// Create sort function for tree building
		sortFn := func(b []*issue.Issue) {
			sortIssues(b, listSort, cfg)
		}

		// Build tree
		tree := ui.BuildTree(issues, allIssues, sortFn)

		if len(tree) == 0 {
			fmt.Println(ui.Muted.Render("No issues found. Create one with: todo new <title>"))
			return nil
		}

		// Calculate max ID width from all issues in tree
		maxIDWidth := 2
		for _, b := range allIssues {
			if len(b.ID) > maxIDWidth {
				maxIDWidth = len(b.ID)
			}
		}
		maxIDWidth += 2

		// Check if any issues have tags
		hasTags := false
		for _, b := range issues {
			if len(b.Tags) > 0 {
				hasTags = true
				break
			}
		}

		// Detect terminal width (default to 80 if not a terminal)
		termWidth := 80
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
			termWidth = w
		}

		fmt.Print(ui.RenderTree(tree, cfg, maxIDWidth, hasTags, termWidth))
		return nil
	},
}

func sortIssues(issues []*issue.Issue, sortBy string, cfg *config.Config) {
	statusNames := cfg.StatusNames()
	priorityNames := cfg.PriorityNames()
	typeNames := cfg.TypeNames()

	switch sortBy {
	case "created":
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].CreatedAt == nil && issues[j].CreatedAt == nil {
				return issues[i].ID < issues[j].ID
			}
			if issues[i].CreatedAt == nil {
				return false
			}
			if issues[j].CreatedAt == nil {
				return true
			}
			return issues[i].CreatedAt.After(*issues[j].CreatedAt)
		})
	case "updated":
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].UpdatedAt == nil && issues[j].UpdatedAt == nil {
				return issues[i].ID < issues[j].ID
			}
			if issues[i].UpdatedAt == nil {
				return false
			}
			if issues[j].UpdatedAt == nil {
				return true
			}
			return issues[i].UpdatedAt.After(*issues[j].UpdatedAt)
		})
	case "status":
		// Build status order from configured statuses
		statusOrder := make(map[string]int)
		for i, s := range statusNames {
			statusOrder[s] = i
		}
		sort.Slice(issues, func(i, j int) bool {
			oi, oj := statusOrder[issues[i].Status], statusOrder[issues[j].Status]
			if oi != oj {
				return oi < oj
			}
			return issues[i].ID < issues[j].ID
		})
	case "priority":
		// Build priority order from configured priorities
		priorityOrder := make(map[string]int)
		for i, p := range priorityNames {
			priorityOrder[p] = i
		}
		// Find normal priority index for issues without priority
		normalIdx := len(priorityNames)
		for i, p := range priorityNames {
			if p == config.PriorityNormal {
				normalIdx = i
				break
			}
		}
		sort.Slice(issues, func(i, j int) bool {
			pi := normalIdx
			if issues[i].Priority != "" {
				if order, ok := priorityOrder[issues[i].Priority]; ok {
					pi = order
				}
			}
			pj := normalIdx
			if issues[j].Priority != "" {
				if order, ok := priorityOrder[issues[j].Priority]; ok {
					pj = order
				}
			}
			if pi != pj {
				return pi < pj
			}
			return issues[i].ID < issues[j].ID
		})
	case "due":
		issue.SortByDueDate(issues)
	case "id":
		sort.Slice(issues, func(i, j int) bool {
			return issues[i].ID < issues[j].ID
		})
	default:
		// Default: sort by status order, then priority, then type order, then title (same as TUI)
		issue.SortByStatusPriorityAndType(issues, statusNames, priorityNames, typeNames)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	listCmd.Flags().StringVarP(&listSearch, "search", "S", "", "Full-text search in title and body")
	listCmd.Flags().StringArrayVarP(&listStatus, "status", "s", nil, "Filter by status (can be repeated)")
	listCmd.Flags().StringArrayVar(&listNoStatus, "no-status", nil, "Exclude by status (can be repeated)")
	listCmd.Flags().StringArrayVarP(&listType, "type", "t", nil, "Filter by type (can be repeated)")
	listCmd.Flags().StringArrayVar(&listNoType, "no-type", nil, "Exclude by type (can be repeated)")
	listCmd.Flags().StringArrayVarP(&listPriority, "priority", "p", nil, "Filter by priority (can be repeated)")
	listCmd.Flags().StringArrayVar(&listNoPriority, "no-priority", nil, "Exclude by priority (can be repeated)")
	listCmd.Flags().StringArrayVar(&listTag, "tag", nil, "Filter by tag (can be repeated, OR logic)")
	listCmd.Flags().StringArrayVar(&listNoTag, "no-tag", nil, "Exclude issues with tag (can be repeated)")
	listCmd.Flags().BoolVar(&listHasParent, "has-parent", false, "Filter issues with a parent")
	listCmd.Flags().BoolVar(&listNoParent, "no-parent", false, "Filter issues without a parent")
	listCmd.Flags().StringVar(&listParentID, "parent", "", "Filter by parent ID")
	listCmd.Flags().BoolVar(&listHasBlocking, "has-blocking", false, "Filter issues that are blocking others")
	listCmd.Flags().BoolVar(&listNoBlocking, "no-blocking", false, "Filter issues that aren't blocking others")
	listCmd.Flags().BoolVar(&listIsBlocked, "is-blocked", false, "Filter issues that are blocked by others")
	listCmd.Flags().BoolVar(&listReady, "ready", false, "Filter issues available to start (not blocked, excludes in-progress/completed/scrapped/draft)")
	listCmd.Flags().BoolVarP(&listQuiet, "quiet", "q", false, "Only output IDs (one per line)")
	listCmd.Flags().StringVar(&listSort, "sort", "", "Sort by: created, updated, due, status, priority, id (default: status, priority, type, title)")
	listCmd.Flags().BoolVar(&listFull, "full", false, "Include issue body in JSON output")
	rootCmd.AddCommand(listCmd)
}
