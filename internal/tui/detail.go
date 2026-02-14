package tui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/ui"
)

// getGlamourRenderer returns a cached glamour renderer, initialized once.
// Uses DarkStyle instead of WithAutoStyle() to avoid slow terminal detection
// that can cause multi-second delays in some terminals.
var getGlamourRenderer = sync.OnceValue(func() *glamour.TermRenderer {
	r, err := glamour.NewTermRenderer(glamour.WithStylePath("dark"))
	if err != nil {
		return nil
	}
	return r
})

// backToListMsg signals navigation back to the list
type backToListMsg struct{}

// resolvedLink represents a link with the target bean resolved
type resolvedLink struct {
	linkType string
	issue     *issue.Issue
	incoming bool // true if another issue links TO this one
}

// linkItem wraps a resolvedLink to implement list.Item
type linkItem struct {
	link  resolvedLink
	cfg   *config.Config
	width int
	cols  ui.ResponsiveColumns
	label string // pre-computed label like "Blocks:" or "Blocked by:"
}

func (i linkItem) Title() string       { return i.link.issue.Title }
func (i linkItem) Description() string { return i.link.issue.ID }
func (i linkItem) FilterValue() string {
	return i.link.issue.Title + " " + i.link.issue.ID + " " + i.label
}

// linkDelegate handles rendering of link list items
type linkDelegate struct {
	cfg   *config.Config
	width int
	cols  ui.ResponsiveColumns
}

func (d linkDelegate) Height() int                             { return 1 }
func (d linkDelegate) Spacing() int                            { return 0 }
func (d linkDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d linkDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(linkItem)
	if !ok {
		return
	}

	link := item.link

	// Cursor indicator
	cursor := "  "
	if index == m.Index() {
		cursor = ui.Primary.Render("▸ ")
	}

	// Format the link type label
	labelCol := lipgloss.NewStyle().Width(12).Render(ui.Muted.Render(item.label + ":"))

	// Get colors from config
	colors := d.cfg.GetIssueColors(link.issue.Status, link.issue.Type, link.issue.Priority)

	// Calculate max title width using responsive columns
	baseWidth := d.cols.ID + d.cols.Status + d.cols.Type + 12 + 4 // label + cursor + padding
	if d.cols.ShowTags {
		baseWidth += d.cols.Tags
	}
	maxTitleWidth := max(10, d.width-baseWidth-8) // 8 for border padding

	// Use shared issue row rendering (without cursor, we handle it separately)
	row := ui.RenderIssueRow(
		link.issue.ID,
		link.issue.Status,
		link.issue.Type,
		link.issue.Title,
		ui.IssueRowConfig{
			StatusColor:   colors.StatusColor,
			TypeColor:     colors.TypeColor,
			PriorityColor: colors.PriorityColor,
			Priority:      link.issue.Priority,
			IsArchive:     colors.IsArchive,
			MaxTitleWidth: maxTitleWidth,
			ShowCursor:    false,
			IsSelected:    false,
			Tags:          link.issue.Tags,
			ShowTags:      d.cols.ShowTags,
			TagsColWidth:  d.cols.Tags,
			MaxTags:       d.cols.MaxTags,
		},
	)

	fmt.Fprint(w, cursor+labelCol+row)
}

// detailModel displays a single bean's details
type detailModel struct {
	viewport      viewport.Model
	issue          *issue.Issue
	resolver      *graph.Resolver
	config        *config.Config
	width         int
	height        int
	ready         bool
	links         []resolvedLink       // combined outgoing + incoming links
	linkList      list.Model           // list component for links (supports filtering)
	linksActive   bool                 // true = links section focused
	cols          ui.ResponsiveColumns // responsive column widths for links
	statusMessage string               // Status message to display in footer
}

func newDetailModel(b *issue.Issue, resolver *graph.Resolver, cfg *config.Config, width, height int) detailModel {
	m := detailModel{
		issue:        b,
		resolver:    resolver,
		config:      cfg,
		width:       width,
		height:      height,
		ready:       true,
		linksActive: false,
	}

	// Resolve all links
	m.links = m.resolveAllLinks()

	// Check if any linked beans have tags
	hasTags := false
	for _, link := range m.links {
		if len(link.issue.Tags) > 0 {
			hasTags = true
			break
		}
	}

	// Calculate responsive columns for links section
	// Account for the label column (12 chars) + cursor (2 chars) + border padding
	linkAreaWidth := width - 12 - 2 - 8
	m.cols = ui.CalculateResponsiveColumns(linkAreaWidth, hasTags)

	// Initialize link list with items
	m.linkList = m.createLinkList()

	// If there are links, select first one and focus links by default
	if len(m.links) > 0 {
		m.linksActive = true
	}

	// Calculate header height dynamically
	headerHeight := m.calculateHeaderHeight()
	footerHeight := 2
	vpWidth := width - 4
	vpHeight := height - headerHeight - footerHeight

	m.viewport = viewport.New(vpWidth, vpHeight)
	m.viewport.SetContent(m.renderBody(vpWidth))

	return m
}

// createLinkList creates a new list.Model for the links
func (m detailModel) createLinkList() list.Model {
	delegate := linkDelegate{
		cfg:   m.config,
		width: m.width,
		cols:  m.cols,
	}

	// Convert links to list items
	items := make([]list.Item, len(m.links))
	for i, link := range m.links {
		items[i] = linkItem{
			link:  link,
			cfg:   m.config,
			width: m.width,
			cols:  m.cols,
			label: m.formatLinkLabel(link.linkType, link.incoming),
		}
	}

	// Calculate list height: show all links up to 1/3 of screen height
	// Add 2 for the title row and padding
	maxHeight := max(3, m.height/3)
	listHeight := min(len(m.links), maxHeight) + 2

	l := list.New(items, delegate, m.width-8, listHeight)
	l.Title = "Linked Beans"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(true)
	l.Filter = substringFilter

	// Style the title bar similar to the detail header title (badge style) but with different color
	l.Styles.Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#fff")).
		Background(ui.ColorBlue).
		Padding(0, 1)
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 0, 1) // Left padding to align with header title
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	l.Styles.NoItems = lipgloss.NewStyle()

	return l
}

func (m detailModel) Init() tea.Cmd {
	return nil
}

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Recalculate responsive columns for links
		hasTags := false
		for _, link := range m.links {
			if len(link.issue.Tags) > 0 {
				hasTags = true
				break
			}
		}
		linkAreaWidth := msg.Width - 12 - 2 - 8
		m.cols = ui.CalculateResponsiveColumns(linkAreaWidth, hasTags)

		// Update link list delegate with new dimensions
		m.updateLinkListDelegate()

		// Update link list size: show all links up to 1/3 of screen height
		// Add 2 for the title row and padding
		maxHeight := max(3, msg.Height/3)
		listHeight := min(len(m.links), maxHeight) + 2
		m.linkList.SetSize(msg.Width-8, listHeight)

		headerHeight := m.calculateHeaderHeight()
		footerHeight := 2
		vpWidth := msg.Width - 4
		vpHeight := max(
			// Ensure vpHeight doesn't go negative
			msg.Height-headerHeight-footerHeight, 1)

		if !m.ready {
			m.viewport = viewport.New(vpWidth, vpHeight)
			m.viewport.SetContent(m.renderBody(vpWidth))
			m.ready = true
		} else {
			m.viewport.Width = vpWidth
			m.viewport.Height = vpHeight
			m.viewport.SetContent(m.renderBody(vpWidth))
		}

	case tea.KeyMsg:
		// If links list is filtering, let it handle all keys except quit
		if m.linksActive && m.linkList.FilterState() == list.Filtering {
			m.linkList, cmd = m.linkList.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "esc", "backspace":
			return m, func() tea.Msg {
				return backToListMsg{}
			}

		case "tab":
			// Toggle focus between links and body
			if len(m.links) > 0 {
				m.linksActive = !m.linksActive
			}
			return m, nil

		case "enter":
			// Navigate to selected link
			if m.linksActive {
				if item, ok := m.linkList.SelectedItem().(linkItem); ok {
					targetBean := item.link.issue
					return m, func() tea.Msg {
						return selectIssueMsg{issue: targetBean}
					}
				}
			}

		case "p":
			// Open parent picker
			return m, func() tea.Msg {
				return openParentPickerMsg{
					beanIDs:       []string{m.issue.ID},
					beanTitle:     m.issue.Title,
					beanTypes:     []string{m.issue.Type},
					currentParent: m.issue.Parent,
				}
			}

		case "s":
			// Open status picker
			return m, func() tea.Msg {
				return openStatusPickerMsg{
					beanIDs:       []string{m.issue.ID},
					beanTitle:     m.issue.Title,
					currentStatus: m.issue.Status,
				}
			}

		case "t":
			// Open type picker
			return m, func() tea.Msg {
				return openTypePickerMsg{
					beanIDs:     []string{m.issue.ID},
					beanTitle:   m.issue.Title,
					currentType: m.issue.Type,
				}
			}

		case "P":
			// Open priority picker
			return m, func() tea.Msg {
				return openPriorityPickerMsg{
					beanIDs:         []string{m.issue.ID},
					beanTitle:       m.issue.Title,
					currentPriority: m.issue.Priority,
				}
			}

		case "b":
			// Open blocking picker
			return m, func() tea.Msg {
				return openBlockingPickerMsg{
					beanID:          m.issue.ID,
					beanTitle:       m.issue.Title,
					currentBlocking: m.issue.Blocking,
				}
			}

		case "e":
			// Open editor for this issue
			return m, func() tea.Msg {
				return openEditorMsg{
					beanID:   m.issue.ID,
					beanPath: m.issue.Path,
				}
			}

		case "y":
			// Copy issue ID to clipboard
			return m, func() tea.Msg {
				return copyBeanIDMsg{ids: []string{m.issue.ID}}
			}
		}
	}

	// Forward updates to the appropriate component
	if m.linksActive && len(m.links) > 0 {
		m.linkList, cmd = m.linkList.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// updateLinkListDelegate updates the link list delegate with current dimensions
func (m *detailModel) updateLinkListDelegate() {
	delegate := linkDelegate{
		cfg:   m.config,
		width: m.width,
		cols:  m.cols,
	}
	m.linkList.SetDelegate(delegate)
}

func (m detailModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header (bean info only, no links)
	header := m.renderHeader()

	// Links section (if any)
	var linksSection string
	if len(m.links) > 0 {
		linksBorderColor := ui.ColorMuted
		if m.linksActive {
			linksBorderColor = ui.ColorPrimary
		}
		linksBorder := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(linksBorderColor).
			Width(m.width - 4)
		linksSection = linksBorder.Render(m.linkList.View()) + "\n"
	}

	// Body
	bodyBorderColor := ui.ColorMuted
	if !m.linksActive {
		bodyBorderColor = ui.ColorPrimary
	}
	bodyBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(bodyBorderColor).
		Width(m.width - 4)
	body := bodyBorder.Render(m.viewport.View())

	// Footer
	scrollPct := int(m.viewport.ScrollPercent() * 100)
	footer := helpStyle.Render(fmt.Sprintf("%d%%", scrollPct)) + "  "
	if len(m.links) > 0 {
		footer += helpKeyStyle.Render("tab") + " " + helpStyle.Render("switch") + "  "
		if m.linksActive {
			footer += helpKeyStyle.Render("/") + " " + helpStyle.Render("filter") + "  "
		}
		footer += helpKeyStyle.Render("enter") + " " + helpStyle.Render("go to") + "  "
	}
	footer += helpKeyStyle.Render("b") + " " + helpStyle.Render("blocking") + "  " +
		helpKeyStyle.Render("e") + " " + helpStyle.Render("edit") + "  " +
		helpKeyStyle.Render("p") + " " + helpStyle.Render("parent") + "  " +
		helpKeyStyle.Render("P") + " " + helpStyle.Render("priority") + "  " +
		helpKeyStyle.Render("s") + " " + helpStyle.Render("status") + "  " +
		helpKeyStyle.Render("t") + " " + helpStyle.Render("type") + "  " +
		helpKeyStyle.Render("y") + " " + helpStyle.Render("copy id") + "  " +
		helpKeyStyle.Render("j/k") + " " + helpStyle.Render("scroll") + "  " +
		helpKeyStyle.Render("?") + " " + helpStyle.Render("help") + "  " +
		helpKeyStyle.Render("esc") + " " + helpStyle.Render("back") + "  " +
		helpKeyStyle.Render("q") + " " + helpStyle.Render("quit")

	// Prepend status message if present
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(ui.ColorSuccess).Bold(true)
		footer = statusStyle.Render(m.statusMessage) + "  " + footer
	}

	return header + "\n" + linksSection + body + "\n" + footer
}

func (m detailModel) calculateHeaderHeight() int {
	// Base: title line + ID/status line + borders/padding = ~6
	baseHeight := 6

	// Add height for links section (separate bordered box)
	if len(m.links) > 0 {
		// Links list height + borders (matches createLinkList calculation)
		// +2 for title row and padding, +3 for borders and spacing
		maxHeight := max(3, m.height/3)
		listHeight := min(len(m.links), maxHeight) + 2
		baseHeight += listHeight + 3
	}

	return baseHeight
}

func (m detailModel) renderHeader() string {
	// Title
	title := detailTitleStyle.Render(m.issue.Title)

	// ID
	id := ui.ID.Render(m.issue.ID)

	// Status badge
	statusCfg := m.config.GetStatus(m.issue.Status)
	statusColor := "gray"
	if statusCfg != nil {
		statusColor = statusCfg.Color
	}
	isArchive := m.config.IsArchiveStatus(m.issue.Status)
	status := ui.RenderStatusWithColor(m.issue.Status, statusColor, isArchive)

	// Build header content
	var headerContent strings.Builder
	headerContent.WriteString(title)
	headerContent.WriteString("\n")
	headerContent.WriteString(id + "  " + status)

	// Add tags if present
	if len(m.issue.Tags) > 0 {
		headerContent.WriteString("  ")
		headerContent.WriteString(ui.RenderTags(m.issue.Tags))
	}

	// Header box style - always muted border (not focused, links section is separate)
	headerBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorMuted).
		Padding(0, 1).
		Width(m.width - 4)

	return headerBox.Render(headerContent.String())
}

// formatLinkLabel returns a human-readable label for the link type
func (m detailModel) formatLinkLabel(linkType string, incoming bool) string {
	if incoming {
		switch linkType {
		case issue.LinkTypeBlocking:
			return "Blocked by"
		case issue.LinkTypeParent:
			return "Child"
		default:
			return linkType + " (incoming)"
		}
	}

	// Outgoing labels - capitalize first letter
	switch linkType {
	case issue.LinkTypeBlocking:
		return "Blocking"
	case issue.LinkTypeParent:
		return "Parent"
	default:
		return linkType
	}
}

func (m detailModel) resolveAllLinks() []resolvedLink {
	var links []resolvedLink
	ctx := context.Background()
	beanResolver := m.resolver.Issue()

	// Resolve outgoing links via GraphQL resolvers
	if blocking, _ := beanResolver.Blocking(ctx, m.issue, nil); blocking != nil {
		for _, b := range blocking {
			links = append(links, resolvedLink{linkType: issue.LinkTypeBlocking, issue: b, incoming: false})
		}
	}
	if parent, _ := beanResolver.Parent(ctx, m.issue); parent != nil {
		links = append(links, resolvedLink{linkType: issue.LinkTypeParent, issue: parent, incoming: false})
	}

	// Resolve incoming links via GraphQL resolvers
	if blockedBy, _ := beanResolver.BlockedBy(ctx, m.issue, nil); blockedBy != nil {
		for _, b := range blockedBy {
			links = append(links, resolvedLink{linkType: issue.LinkTypeBlocking, issue: b, incoming: true})
		}
	}
	if children, _ := beanResolver.Children(ctx, m.issue, nil); children != nil {
		for _, b := range children {
			links = append(links, resolvedLink{linkType: issue.LinkTypeParent, issue: b, incoming: true})
		}
	}

	// Sort all links by link type label first, then by bean status/type/title
	// This keeps link categories together while ordering beans consistently with the main list
	statusNames := m.config.StatusNames()
	typeNames := m.config.TypeNames()
	sort.Slice(links, func(i, j int) bool {
		// First: group by link label (e.g., "Child", "Parent", "Blocks", etc.)
		labelI := m.formatLinkLabel(links[i].linkType, links[i].incoming)
		labelJ := m.formatLinkLabel(links[j].linkType, links[j].incoming)
		if labelI != labelJ {
			return labelI < labelJ
		}
		// Within same link type: sort by status, priority, type, then title
		priorityNames := m.config.PriorityNames()
		return compareBeansByStatusPriorityAndType(links[i].issue, links[j].issue, statusNames, priorityNames, typeNames)
	})

	return links
}

// compareBeansByStatusPriorityAndType compares two beans using the same ordering as issue.SortByStatusPriorityAndType.
func compareBeansByStatusPriorityAndType(a, b *issue.Issue, statusNames, priorityNames, typeNames []string) bool {
	return issue.CompareByStatusPriorityAndType(a, b, statusNames, priorityNames, typeNames)
}

func (m detailModel) renderBody(_ int) string {
	if m.issue.Body == "" {
		return lipgloss.NewStyle().
			Foreground(ui.ColorMuted).
			Padding(0, 1).
			Render("No description")
	}

	renderer := getGlamourRenderer()
	if renderer == nil {
		return m.issue.Body
	}

	rendered, err := renderer.Render(m.issue.Body)
	if err != nil {
		return m.issue.Body
	}

	return strings.TrimSpace(rendered)
}
