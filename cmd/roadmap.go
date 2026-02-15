package cmd

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"text/template"

	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/spf13/cobra"
)

//go:embed roadmap.tmpl
var roadmapTemplateContent string

var (
	roadmapJSON        bool
	roadmapIncludeDone bool
	roadmapStatus      []string
	roadmapNoStatus    []string
	roadmapNoLinks     bool
	roadmapLinkPrefix  string
)

// roadmapData holds the structured roadmap for JSON output.
type roadmapData struct {
	Milestones  []milestoneGroup `json:"milestones"`
	Unscheduled *unscheduledGroup `json:"unscheduled,omitempty"`
}

// unscheduledGroup represents items not assigned to any milestone.
type unscheduledGroup struct {
	Epics []epicGroup  `json:"epics,omitempty"`
	Other []*issue.Issue `json:"other,omitempty"`
}

// milestoneGroup represents a milestone and its contents.
type milestoneGroup struct {
	Milestone *issue.Issue   `json:"milestone"`
	Epics     []epicGroup  `json:"epics,omitempty"`
	Other     []*issue.Issue `json:"other,omitempty"`
}

// epicGroup represents an epic and its child items.
type epicGroup struct {
	Epic  *issue.Issue   `json:"epic"`
	Items []*issue.Issue `json:"items,omitempty"`
}


var roadmapCmd = &cobra.Command{
	Use:   "roadmap",
	Short: "Generate a Markdown roadmap from milestones and epics",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Query all issues via GraphQL resolver
		resolver := &graph.Resolver{Core: store}
		allIssues, err := resolver.Query().Issues(context.Background(), nil)
		if err != nil {
			return fmt.Errorf("querying issues: %w", err)
		}

		// Build the roadmap
		data := buildRoadmap(allIssues, roadmapIncludeDone, roadmapStatus, roadmapNoStatus)

		// JSON output
		if roadmapJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(data)
		}

		// Markdown output
		links := !roadmapNoLinks
		linkPrefix := roadmapLinkPrefix
		if links && linkPrefix == "" {
			// Default: relative path from cwd to data directory
			linkPrefix = defaultLinkPrefix()
		}
		md := renderRoadmapMarkdown(data, links, linkPrefix)
		fmt.Print(md)
		return nil
	},
}

// buildRoadmap constructs the roadmap data structure from issues.
func buildRoadmap(allIssues []*issue.Issue, includeDone bool, statusFilter, noStatusFilter []string) *roadmapData {
	// Index all issues by ID for lookups
	byID := make(map[string]*issue.Issue)
	for _, b := range allIssues {
		byID[b.ID] = b
	}

	// Build children index: parent ID -> children
	// This maps each issue ID to the issues that have it as a parent
	children := make(map[string][]*issue.Issue)
	for _, b := range allIssues {
		if b.Parent != "" {
			children[b.Parent] = append(children[b.Parent], b)
		}
	}

	// Find milestones, applying status filters
	var milestones []*issue.Issue
	for _, b := range allIssues {
		if b.Type != config.TypeMilestone {
			continue
		}
		// Apply status filters to milestones
		if len(statusFilter) > 0 && !containsStatus(statusFilter, b.Status) {
			continue
		}
		if len(noStatusFilter) > 0 && containsStatus(noStatusFilter, b.Status) {
			continue
		}
		milestones = append(milestones, b)
	}

	// Sort milestones by status order, then by created date
	sortByStatusThenCreated(milestones, cfg)

	// Build milestone groups
	var milestoneGroups []milestoneGroup
	for _, m := range milestones {
		group := buildMilestoneGroup(m, children, includeDone)
		// Only include milestones that have visible content
		if len(group.Epics) > 0 || len(group.Other) > 0 {
			milestoneGroups = append(milestoneGroups, group)
		}
	}

	// Build unscheduled group: items not under any milestone
	// Track which issues are under a milestone (directly or via epic)
	underMilestone := make(map[string]bool)
	for _, m := range milestones {
		underMilestone[m.ID] = true
		for _, child := range children[m.ID] {
			underMilestone[child.ID] = true
			// Also mark children of epics under this milestone
			if child.Type == config.TypeEpic {
				for _, epicChild := range children[child.ID] {
					underMilestone[epicChild.ID] = true
				}
			}
		}
	}

	// Find unscheduled epics (epics not under a milestone)
	var unscheduledEpics []epicGroup
	for _, b := range allIssues {
		if b.Type != config.TypeEpic {
			continue
		}
		if underMilestone[b.ID] {
			continue
		}
		// Build epic group if it has visible children
		epicItems := filterChildren(children[b.ID], includeDone)
		if len(epicItems) > 0 {
			sortByTypeThenStatus(epicItems, cfg)
			unscheduledEpics = append(unscheduledEpics, epicGroup{Epic: b, Items: epicItems})
		}
	}

	// Sort unscheduled epics by title
	sort.Slice(unscheduledEpics, func(i, j int) bool {
		return unscheduledEpics[i].Epic.Title < unscheduledEpics[j].Epic.Title
	})

	// Find orphan items (not milestone, not epic, no parent or parent is not milestone/epic)
	var orphanItems []*issue.Issue
	for _, b := range allIssues {
		// Skip milestones and epics
		if b.Type == config.TypeMilestone || b.Type == config.TypeEpic {
			continue
		}
		// Skip if already under a milestone
		if underMilestone[b.ID] {
			continue
		}
		// Skip if has a parent (it's under an unscheduled epic, handled above)
		if b.Parent != "" {
			continue
		}
		// Apply done filter
		if !includeDone && cfg.IsArchiveStatus(b.Status) {
			continue
		}
		orphanItems = append(orphanItems, b)
	}

	// Sort orphan items
	sortByTypeThenStatus(orphanItems, cfg)

	// Build unscheduled group if there's content
	var unscheduled *unscheduledGroup
	if len(unscheduledEpics) > 0 || len(orphanItems) > 0 {
		unscheduled = &unscheduledGroup{
			Epics: unscheduledEpics,
			Other: orphanItems,
		}
	}

	return &roadmapData{
		Milestones:  milestoneGroups,
		Unscheduled: unscheduled,
	}
}

// buildMilestoneGroup builds a milestone group with its epics and other items.
func buildMilestoneGroup(m *issue.Issue, children map[string][]*issue.Issue, includeDone bool) milestoneGroup {
	group := milestoneGroup{Milestone: m}

	// Get direct children of this milestone
	directChildren := children[m.ID]

	// Separate epics from other items
	var epics []*issue.Issue

	for _, child := range directChildren {
		if child.Type == config.TypeEpic {
			epics = append(epics, child)
		}
	}

	// Build epic groups
	for _, epic := range epics {
		epicItems := filterChildren(children[epic.ID], includeDone)
		// Only include epics that have visible children
		if len(epicItems) > 0 {
			sortByTypeThenStatus(epicItems, cfg)
			group.Epics = append(group.Epics, epicGroup{Epic: epic, Items: epicItems})
		}
	}

	// Build "Other" list: direct children that are not epics
	// (With single parent enforcement, items can't be both under an epic and directly under the milestone)
	var other []*issue.Issue
	for _, child := range directChildren {
		if child.Type == config.TypeEpic {
			continue
		}
		if includeDone || !cfg.IsArchiveStatus(child.Status) {
			other = append(other, child)
		}
	}

	// Sort epics by their epic's title
	sort.Slice(group.Epics, func(i, j int) bool {
		return group.Epics[i].Epic.Title < group.Epics[j].Epic.Title
	})

	// Sort other items
	sortByTypeThenStatus(other, cfg)
	group.Other = other

	return group
}

// filterChildren filters children based on done status.
func filterChildren(children []*issue.Issue, includeDone bool) []*issue.Issue {
	if includeDone {
		// Return a copy to avoid modifying the original
		result := make([]*issue.Issue, len(children))
		copy(result, children)
		return result
	}

	var filtered []*issue.Issue
	for _, b := range children {
		if !cfg.IsArchiveStatus(b.Status) {
			filtered = append(filtered, b)
		}
	}
	return filtered
}

// containsStatus checks if a status is in the list.
func containsStatus(statuses []string, status string) bool {
	return slices.Contains(statuses, status)
}

// sortByStatusThenCreated sorts issues by status order, then by created date.
func sortByStatusThenCreated(issues []*issue.Issue, cfg interface{ StatusNames() []string }) {
	statusOrder := make(map[string]int)
	for i, s := range cfg.StatusNames() {
		statusOrder[s] = i
	}

	sort.Slice(issues, func(i, j int) bool {
		oi, oj := statusOrder[issues[i].Status], statusOrder[issues[j].Status]
		if oi != oj {
			return oi < oj
		}
		// Then by created date (oldest first for milestones)
		if issues[i].CreatedAt != nil && issues[j].CreatedAt != nil {
			return issues[i].CreatedAt.Before(*issues[j].CreatedAt)
		}
		return issues[i].ID < issues[j].ID
	})
}

// sortByTypeThenStatus sorts issues by type order, then status order, then by ID.
func sortByTypeThenStatus(issues []*issue.Issue, cfg interface {
	StatusNames() []string
	TypeNames() []string
}) {
	statusOrder := make(map[string]int)
	for i, s := range cfg.StatusNames() {
		statusOrder[s] = i
	}
	typeOrder := make(map[string]int)
	for i, t := range cfg.TypeNames() {
		typeOrder[t] = i
	}

	sort.Slice(issues, func(i, j int) bool {
		// First by type
		ti, tj := typeOrder[issues[i].Type], typeOrder[issues[j].Type]
		if ti != tj {
			return ti < tj
		}
		// Then by status
		si, sj := statusOrder[issues[i].Status], statusOrder[issues[j].Status]
		if si != sj {
			return si < sj
		}
		return issues[i].ID < issues[j].ID
	})
}

// renderRoadmapMarkdown renders the roadmap as Markdown using the template.
func renderRoadmapMarkdown(data *roadmapData, links bool, linkPrefix string) string {
	// Create template with closures that capture link settings
	tmpl := template.Must(
		template.New("roadmap").Funcs(template.FuncMap{
			"firstParagraph": firstParagraph,
			"typeBadge":      typeBadge,
			"issueRef": func(b *issue.Issue) string {
				return renderIssueRef(b, links, linkPrefix)
			},
		}).Parse(roadmapTemplateContent),
	)

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		panic(err)
	}
	return sb.String()
}

// renderIssueRef renders an issue ID, optionally as a markdown link.
func renderIssueRef(b *issue.Issue, asLink bool, linkPrefix string) string {
	if !asLink {
		return "(" + b.ID + ")"
	}
	if linkPrefix == "" {
		return fmt.Sprintf("([%s](%s))", b.ID, b.Path)
	}
	// Ensure prefix ends with / for clean concatenation
	if !strings.HasSuffix(linkPrefix, "/") {
		linkPrefix += "/"
	}
	return fmt.Sprintf("([%s](%s%s))", b.ID, linkPrefix, b.Path)
}

// typeBadge returns a shields.io badge markdown for the issue type.
func typeBadge(b *issue.Issue) string {
	if b.Type == "" {
		return ""
	}
	// Map types to colors
	colors := map[string]string{
		config.TypeBug:       "d73a4a",
		config.TypeFeature:   "0e8a16",
		config.TypeTask:      "1d76db",
		config.TypeEpic:      "5319e7",
		config.TypeMilestone: "fbca04",
	}
	color := colors[b.Type]
	if color == "" {
		color = "gray"
	}
	return fmt.Sprintf("![%s](https://img.shields.io/badge/%s-%s?style=flat-square)", b.Type, b.Type, color)
}

// defaultLinkPrefix returns the relative path from cwd to the data directory.
func defaultLinkPrefix() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(cwd, store.Root())
	if err != nil {
		return ""
	}
	// Convert to forward slashes for URL compatibility
	return filepath.ToSlash(rel)
}

// firstParagraph extracts the first paragraph from a body text.
func firstParagraph(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	// Find the first blank line (paragraph separator)
	lines := strings.Split(body, "\n")
	var para []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			break
		}
		// Skip markdown headers
		if strings.HasPrefix(line, "#") {
			continue
		}
		para = append(para, strings.TrimSpace(line))
	}

	const maxDescriptionLen = 200

	result := strings.Join(para, " ")
	if len(result) > maxDescriptionLen {
		result = result[:maxDescriptionLen-3] + "..."
	}
	return result
}

func init() {
	roadmapCmd.Flags().BoolVar(&roadmapJSON, "json", false, "Output as JSON")
	roadmapCmd.Flags().BoolVar(&roadmapIncludeDone, "include-done", false, "Include completed items")
	roadmapCmd.Flags().StringArrayVar(&roadmapStatus, "status", nil, "Filter milestones by status (can be repeated)")
	roadmapCmd.Flags().StringArrayVar(&roadmapNoStatus, "no-status", nil, "Exclude milestones by status (can be repeated)")
	roadmapCmd.Flags().BoolVar(&roadmapNoLinks, "no-links", false, "Don't render issue IDs as markdown links")
	roadmapCmd.Flags().StringVar(&roadmapLinkPrefix, "link-prefix", "", "URL prefix for links")
	rootCmd.AddCommand(roadmapCmd)
}
