package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
)

// TreeNode represents a node in the issue tree hierarchy.
type TreeNode struct {
	Issue     *issue.Issue
	Children []*TreeNode
	Matched  bool // true if this issue matched the filter (vs. shown for context)
}

// TreeNodeJSON is the JSON-serializable version of TreeNode.
type TreeNodeJSON struct {
	ID       string          `json:"id"`
	Slug     string          `json:"slug,omitempty"`
	Path     string          `json:"path"`
	Title    string          `json:"title"`
	Status   string          `json:"status"`
	Type     string          `json:"type,omitempty"`
	Priority string          `json:"priority,omitempty"`
	Tags     []string        `json:"tags,omitempty"`
	Body     string          `json:"body,omitempty"`
	Matched  bool            `json:"matched"`
	Children []*TreeNodeJSON `json:"children,omitempty"`
}

// ToJSON converts a TreeNode to its JSON-serializable form.
func (n *TreeNode) ToJSON(includeFull bool) *TreeNodeJSON {
	json := &TreeNodeJSON{
		ID:       n.Issue.ID,
		Slug:     n.Issue.Slug,
		Path:     n.Issue.Path,
		Title:    n.Issue.Title,
		Status:   n.Issue.Status,
		Type:     n.Issue.Type,
		Priority: n.Issue.Priority,
		Tags:     n.Issue.Tags,
		Matched:  n.Matched,
	}
	if includeFull {
		json.Body = n.Issue.Body
	}
	if len(n.Children) > 0 {
		json.Children = make([]*TreeNodeJSON, len(n.Children))
		for i, child := range n.Children {
			json.Children[i] = child.ToJSON(includeFull)
		}
	}
	return json
}

// BuildTree builds a tree structure from filtered issues, including ancestors for context.
// matchedIssues: issues that matched the filter
// allIssues: all issues (needed to find ancestors)
// sortFn: function to sort issues at each level
func BuildTree(matchedIssues []*issue.Issue, allIssues []*issue.Issue, sortFn func([]*issue.Issue)) []*TreeNode {
	// Build index of all issues by ID
	issueByID := make(map[string]*issue.Issue)
	for _, b := range allIssues {
		issueByID[b.ID] = b
	}

	// Build set of matched issue IDs
	matchedSet := make(map[string]bool)
	for _, b := range matchedIssues {
		matchedSet[b.ID] = true
	}

	// Find all ancestors needed for context
	// Start with matched issues, then walk up parent links
	neededIssues := make(map[string]*issue.Issue)
	for _, b := range matchedIssues {
		neededIssues[b.ID] = b
	}

	// Add ancestors of matched issues
	for _, b := range matchedIssues {
		addAncestors(b, issueByID, neededIssues)
	}

	// Build children index (parent ID -> children)
	children := make(map[string][]*issue.Issue)
	for _, b := range neededIssues {
		if b.Parent != "" {
			// Only add as child if parent is in our needed set
			if _, ok := neededIssues[b.Parent]; ok {
				children[b.Parent] = append(children[b.Parent], b)
			}
		}
	}

	// Sort children at each level
	for parentID := range children {
		sortFn(children[parentID])
	}

	// Find root issues (no parent or parent not in needed set)
	var roots []*issue.Issue
	for _, b := range neededIssues {
		if b.Parent == "" {
			roots = append(roots, b)
		} else {
			// Check if parent is in the tree
			if _, ok := neededIssues[b.Parent]; !ok {
				roots = append(roots, b)
			}
		}
	}
	sortFn(roots)

	// Build tree nodes recursively
	return buildNodes(roots, children, matchedSet)
}

// addAncestors recursively adds all ancestors of an issue to the needed set.
func addAncestors(b *issue.Issue, issueByID map[string]*issue.Issue, needed map[string]*issue.Issue) {
	if b.Parent == "" {
		return
	}
	parent, ok := issueByID[b.Parent]
	if !ok {
		return // parent doesn't exist (broken link)
	}
	if _, alreadyNeeded := needed[b.Parent]; alreadyNeeded {
		return // already processed
	}
	needed[b.Parent] = parent
	addAncestors(parent, issueByID, needed)
}

// buildNodes recursively builds TreeNodes from issues.
func buildNodes(issues []*issue.Issue, children map[string][]*issue.Issue, matchedSet map[string]bool) []*TreeNode {
	nodes := make([]*TreeNode, len(issues))
	for i, b := range issues {
		nodes[i] = &TreeNode{
			Issue:     b,
			Matched:  matchedSet[b.ID],
			Children: buildNodes(children[b.ID], children, matchedSet),
		}
	}
	return nodes
}

// Tree rendering constants
const (
	treeBranch     = "├─ "
	treeLastBranch = "└─ "
	treePipe       = "│  " // vertical line for ongoing branches
	treeSpace      = "   " // empty space for completed branches
	treeIndent     = 3     // width of connector
)

// calculateMaxDepth returns the maximum depth of the tree.
func calculateMaxDepth(nodes []*TreeNode) int {
	maxDepth := 0
	for _, node := range nodes {
		depth := 1 + calculateMaxDepth(node.Children)
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	return maxDepth
}

// RenderTree renders the tree as an ASCII tree with styled columns.
// termWidth is used to calculate responsive column widths.
func RenderTree(nodes []*TreeNode, cfg *config.Config, maxIDWidth int, hasTags bool, termWidth int) string {
	var sb strings.Builder

	// Calculate max depth to determine ID column width
	maxDepth := calculateMaxDepth(nodes)
	// ID column needs: indent (3 chars per level beyond depth 1) + connector (3 chars) + ID width
	// depth 0: 0 extra chars
	// depth 1: 3 chars (connector only)
	// depth 2: 6 chars (3 indent + 3 connector)
	// depth N: (N-1)*3 + 3 = N*3 chars
	treeColWidth := maxIDWidth
	if maxDepth > 0 {
		treeColWidth = maxIDWidth + maxDepth*treeIndent
	}

	// Calculate responsive columns based on terminal width
	// Adjust for tree column width vs default ID column width
	adjustedWidth := termWidth - treeColWidth + ColWidthID
	cols := CalculateResponsiveColumns(adjustedWidth, hasTags)

	// Calculate title width from remaining space
	// Account for: tree/ID col, type col, status col, priority symbol (2), space before tags (1)
	titleWidth := termWidth - treeColWidth - ColWidthType - ColWidthStatus - 3
	if cols.ShowTags {
		titleWidth -= cols.Tags
	}
	if titleWidth < 20 {
		titleWidth = 20
	}

	// Header with manual padding (lipgloss Width doesn't handle styled strings well)
	headerCol := lipgloss.NewStyle().Foreground(ColorMuted)
	idHeader := headerCol.Render("ID") + strings.Repeat(" ", treeColWidth-2)
	typeHeader := headerCol.Render("TYPE") + strings.Repeat(" ", ColWidthType-4)
	statusHeader := headerCol.Render("STATUS") + strings.Repeat(" ", ColWidthStatus-6)

	header := idHeader + typeHeader + statusHeader + headerCol.Render("TITLE")
	if cols.ShowTags && titleWidth > 5 {
		header += strings.Repeat(" ", titleWidth-5+3) + headerCol.Render("TAGS") // +3 for priority/spacing
	}
	dividerWidth := termWidth - 1 // -1 to avoid wrapping on exact terminal width
	sb.WriteString(header)
	sb.WriteString("\n")
	sb.WriteString(Muted.Render(strings.Repeat("─", dividerWidth)))
	sb.WriteString("\n")

	// Build render config from responsive columns
	renderCfg := treeRenderConfig{
		treeColWidth: treeColWidth,
		titleWidth:   titleWidth,
		cols:         cols,
	}

	// Render nodes (depth 0 = root level, no ancestry yet)
	renderNodes(&sb, nodes, 0, nil, cfg, renderCfg)

	return sb.String()
}

// treeRenderConfig holds computed rendering configuration for tree output
type treeRenderConfig struct {
	treeColWidth int
	titleWidth   int
	cols         ResponsiveColumns
}

// renderNodes recursively renders tree nodes with proper indentation.
// depth 0 = root level (no connector), depth 1+ = nested (has connector)
// ancestry tracks whether each parent level was a last child (true = last, no continuation line needed)
func renderNodes(sb *strings.Builder, nodes []*TreeNode, depth int, ancestry []bool, cfg *config.Config, renderCfg treeRenderConfig) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		renderNode(sb, node, depth, isLast, ancestry, cfg, renderCfg)
		// Only add to ancestry when depth > 0 (roots have no connectors to continue)
		if len(node.Children) > 0 {
			var newAncestry []bool
			if depth > 0 {
				newAncestry = append(ancestry, isLast)
			}
			renderNodes(sb, node.Children, depth+1, newAncestry, cfg, renderCfg)
		}
	}
}

// renderNode renders a single tree node with tree connectors.
// depth 0 = root (no connector), depth 1+ = nested (has connector)
// ancestry tracks whether each parent level was a last child (true = last, no continuation line needed)
func renderNode(sb *strings.Builder, node *TreeNode, depth int, isLast bool, ancestry []bool, cfg *config.Config, renderCfg treeRenderConfig) {
	b := node.Issue

	// Build tree prefix from ancestry
	var prefix strings.Builder
	if depth > 0 {
		for _, wasLast := range ancestry {
			if wasLast {
				prefix.WriteString(treeSpace)
			} else {
				prefix.WriteString(treePipe)
			}
		}
		if isLast {
			prefix.WriteString(treeLastBranch)
		} else {
			prefix.WriteString(treeBranch)
		}
	}

	// Get colors from config
	colors := cfg.GetIssueColors(b.Status, b.Type, b.Priority)

	// Use shared RenderIssueRow function with responsive columns
	row := RenderIssueRow(b.ID, b.Status, b.Type, b.Title, IssueRowConfig{
		StatusColor:   colors.StatusColor,
		TypeColor:     colors.TypeColor,
		PriorityColor: colors.PriorityColor,
		Priority:      b.Priority,
		IsArchive:     colors.IsArchive,
		MaxTitleWidth: renderCfg.titleWidth,
		ShowCursor:    false,
		Tags:          b.Tags,
		ShowTags:      renderCfg.cols.ShowTags,
		TagsColWidth:  renderCfg.cols.Tags,
		MaxTags:       renderCfg.cols.MaxTags,
		TreePrefix:    prefix.String(),
		Dimmed:        !node.Matched,
		IDColWidth:    renderCfg.treeColWidth,
	})

	sb.WriteString(row)
	sb.WriteString("\n")
}

// FlatItem represents a flattened tree node with rendering context.
// Used by TUI to render tree structure in a flat list.
type FlatItem struct {
	Issue       *issue.Issue
	Depth      int    // 0 = root, 1+ = nested
	IsLast     bool   // last child at this level
	Matched    bool   // true if issue matched filter (vs. shown for context)
	TreePrefix string // pre-computed tree prefix (e.g., "  └─")
}

// FlattenTree converts a tree into a flat slice with tree context preserved.
// Each item includes the pre-computed tree prefix for rendering.
func FlattenTree(nodes []*TreeNode) []FlatItem {
	var items []FlatItem
	flattenNodes(nodes, 0, nil, &items)
	return items
}

// flattenNodes recursively flattens tree nodes.
// ancestry tracks whether each parent level was a last child (true = last, no continuation line needed)
func flattenNodes(nodes []*TreeNode, depth int, ancestry []bool, items *[]FlatItem) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1

		// Compute tree prefix
		var prefix strings.Builder
		if depth > 0 {
			// Build prefix from ancestry - each level adds either │ or space
			for _, wasLast := range ancestry {
				if wasLast {
					prefix.WriteString(treeSpace) // parent was last child, no continuation line
				} else {
					prefix.WriteString(treePipe) // parent has more siblings, show continuation line
				}
			}
			// Add connector for this node
			if isLast {
				prefix.WriteString(treeLastBranch)
			} else {
				prefix.WriteString(treeBranch)
			}
		}

		*items = append(*items, FlatItem{
			Issue:       node.Issue,
			Depth:      depth,
			IsLast:     isLast,
			Matched:    node.Matched,
			TreePrefix: prefix.String(),
		})

		// Recurse into children, passing updated ancestry
		// Only add to ancestry when depth > 0 (roots have no connectors to continue)
		if len(node.Children) > 0 {
			var newAncestry []bool
			if depth > 0 {
				newAncestry = append(ancestry, isLast)
			}
			flattenNodes(node.Children, depth+1, newAncestry, items)
		}
	}
}

// MaxTreeDepth returns the maximum depth of the flattened tree.
func MaxTreeDepth(items []FlatItem) int {
	maxDepth := 0
	for _, item := range items {
		if item.Depth > maxDepth {
			maxDepth = item.Depth
		}
	}
	return maxDepth
}
