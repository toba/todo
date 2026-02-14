package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/graph/model"
	"github.com/toba/todo/internal/ui"
)

// substringFilter is a custom filter function that matches contiguous substrings
// instead of the default fuzzy matching. It performs case-insensitive matching.
func substringFilter(term string, targets []string) []list.Rank {
	term = strings.ToLower(term)
	var ranks []list.Rank
	for i, t := range targets {
		lower := strings.ToLower(t)
		idx := strings.Index(lower, term)
		if idx >= 0 {
			matchedIndexes := make([]int, len(term))
			for j := range term {
				matchedIndexes[j] = idx + j
			}
			ranks = append(ranks, list.Rank{
				Index:          i,
				MatchedIndexes: matchedIndexes,
			})
		}
	}
	return ranks
}

// beanItem wraps an issue to implement list.Item, with tree context
type beanItem struct {
	issue       *issue.Issue
	cfg        *config.Config
	treePrefix string // tree prefix for rendering (e.g., "├─" or "  └─")
	matched    bool   // true if issue matched filter (vs. ancestor shown for context)
	deepSearch *bool  // pointer to listModel.deepSearch
}

func (i beanItem) Title() string       { return i.issue.Title }
func (i beanItem) Description() string { return i.issue.ID + " · " + i.issue.Status }
func (i beanItem) FilterValue() string {
	if i.deepSearch != nil && *i.deepSearch {
		return i.issue.Title + " " + i.issue.ID + " " + i.issue.Body
	}
	return i.issue.Title + " " + i.issue.ID
}

// itemDelegate handles rendering of list items
type itemDelegate struct {
	cfg           *config.Config
	hasTags       bool
	width         int
	cols          ui.ResponsiveColumns // cached responsive columns
	idColWidth    int                  // ID column width (accounts for tree prefix)
	selectedBeans *map[string]bool     // pointer to marked beans for multi-select
}

func newItemDelegate(cfg *config.Config) itemDelegate {
	return itemDelegate{cfg: cfg, hasTags: false, width: 0}
}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(beanItem)
	if !ok {
		return
	}

	// Get colors from config
	colors := d.cfg.GetIssueColors(item.issue.Status, item.issue.Type, item.issue.Priority)

	// Calculate max title width using responsive columns
	idWidth := d.cols.ID
	if d.idColWidth > 0 {
		idWidth = d.idColWidth
	}
	baseWidth := idWidth + d.cols.Status + d.cols.Type + 4 // 4 for cursor + padding
	if d.cols.ShowTags {
		baseWidth += d.cols.Tags
	}
	maxTitleWidth := max(0, m.Width()-baseWidth)

	// Check if bean is marked for multi-select
	var isMarked bool
	if d.selectedBeans != nil {
		isMarked = (*d.selectedBeans)[item.issue.ID]
	}

	str := ui.RenderIssueRow(
		item.issue.ID,
		item.issue.Status,
		item.issue.Type,
		item.issue.Title,
		ui.IssueRowConfig{
			StatusColor:   colors.StatusColor,
			TypeColor:     colors.TypeColor,
			PriorityColor: colors.PriorityColor,
			Priority:      item.issue.Priority,
			IsArchive:     colors.IsArchive,
			MaxTitleWidth: maxTitleWidth,
			ShowCursor:    true,
			IsSelected:    index == m.Index(),
			IsMarked:      isMarked,
			Tags:          item.issue.Tags,
			ShowTags:      d.cols.ShowTags,
			TagsColWidth:  d.cols.Tags,
			MaxTags:       d.cols.MaxTags,
			TreePrefix:    item.treePrefix,
			Dimmed:        !item.matched,
			IDColWidth:    d.idColWidth,
		},
	)

	fmt.Fprint(w, str)
}

// listModel is the model for the issue list view
type listModel struct {
	list     list.Model
	resolver *graph.Resolver
	config   *config.Config
	width    int
	height   int
	err      error

	// Responsive column state
	hasTags    bool                 // whether any beans have tags
	cols       ui.ResponsiveColumns // calculated responsive columns
	idColWidth int                  // ID column width (accounts for tree depth)

	// Active filters
	tagFilter string // if set, only show beans with this tag

	// Sort order
	sortOrder sortOrder // current sort mode

	// Deep search mode (heap-allocated so beanItem pointers survive value copies)
	deepSearch *bool // when true, filter also searches issue body

	// Multi-select state
	selectedBeans map[string]bool // IDs of beans marked for multi-edit

	// Status message to display in footer
	statusMessage string
}

func newListModel(resolver *graph.Resolver, cfg *config.Config) listModel {
	selectedBeans := make(map[string]bool)
	delegate := itemDelegate{cfg: cfg, selectedBeans: &selectedBeans}

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Beans"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.Filter = substringFilter
	l.Styles.Title = listTitleStyle
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 1, 1)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	deepSearch := false
	return listModel{
		list:          l,
		resolver:      resolver,
		config:        cfg,
		sortOrder:     sortOrder(cfg.GetDefaultSort()),
		deepSearch:    &deepSearch,
		selectedBeans: selectedBeans,
	}
}

// beansLoadedMsg is sent when beans are loaded
type beansLoadedMsg struct {
	items      []ui.FlatItem // flattened tree items
	idColWidth int           // calculated ID column width for tree
}

// errMsg is sent when an error occurs
type errMsg struct {
	err error
}

// selectIssueMsg is sent when an issue is selected
type selectIssueMsg struct {
	issue *issue.Issue
}

func (m listModel) Init() tea.Cmd {
	return m.loadBeans
}

func (m listModel) loadBeans() tea.Msg {
	// Build filter if tag filter is set
	var filter *model.IssueFilter
	if m.tagFilter != "" {
		filter = &model.IssueFilter{Tags: []string{m.tagFilter}}
	}

	// Query filtered beans
	filteredBeans, err := m.resolver.Query().Issues(context.Background(), filter)
	if err != nil {
		return errMsg{err}
	}

	// Query all issues for tree context (ancestors)
	allBeans, err := m.resolver.Query().Issues(context.Background(), nil)
	if err != nil {
		return errMsg{err}
	}

	// Sort function for tree building
	var sortFn func([]*issue.Issue)
	switch m.sortOrder {
	case sortCreated:
		effectiveDates := issue.ComputeEffectiveDates(allBeans, "created_at")
		sortFn = func(beans []*issue.Issue) {
			issue.SortByCreatedAt(beans, effectiveDates)
		}
	case sortUpdated:
		effectiveDates := issue.ComputeEffectiveDates(allBeans, "updated_at")
		sortFn = func(beans []*issue.Issue) {
			issue.SortByUpdatedAt(beans, effectiveDates)
		}
	case sortDue:
		sortFn = func(beans []*issue.Issue) {
			issue.SortByDueDate(beans)
		}
	default:
		sortFn = func(beans []*issue.Issue) {
			issue.SortByStatusPriorityAndType(beans, m.config.StatusNames(), m.config.PriorityNames(), m.config.TypeNames())
		}
	}

	// Build tree and flatten it
	tree := ui.BuildTree(filteredBeans, allBeans, sortFn)
	items := ui.FlattenTree(tree)

	// Calculate ID column width based on max ID length and tree depth
	maxIDLen := 0
	for _, b := range allBeans {
		if len(b.ID) > maxIDLen {
			maxIDLen = len(b.ID)
		}
	}
	maxDepth := ui.MaxTreeDepth(items)
	// ID column = base ID width + tree indent (3 chars per depth level)
	idColWidth := maxIDLen + 2 // base padding
	if maxDepth > 0 {
		idColWidth += maxDepth * 3 // 3 chars per depth level (├─ + space)
	}

	return beansLoadedMsg{items: items, idColWidth: idColWidth}
}

// setTagFilter sets the tag filter
func (m *listModel) setTagFilter(tag string) {
	m.tagFilter = tag
}

// clearFilter clears all active filters
func (m *listModel) clearFilter() {
	m.tagFilter = ""
}

// hasActiveFilter returns true if any filter is active
func (m *listModel) hasActiveFilter() bool {
	return m.tagFilter != ""
}

func (m listModel) Update(msg tea.Msg) (listModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Reserve space for border and footer
		m.list.SetSize(msg.Width-2, msg.Height-4)
		// Recalculate responsive columns
		m.cols = ui.CalculateResponsiveColumns(msg.Width, m.hasTags)
		m.updateDelegate()

	case beansLoadedMsg:
		items := make([]list.Item, len(msg.items))
		// Check if any beans have tags
		m.hasTags = false
		for i, flatItem := range msg.items {
			items[i] = beanItem{
				issue:       flatItem.Issue,
				cfg:        m.config,
				treePrefix: flatItem.TreePrefix,
				matched:    flatItem.Matched,
				deepSearch: m.deepSearch,
			}
			if len(flatItem.Issue.Tags) > 0 {
				m.hasTags = true
			}
		}
		m.list.SetItems(items)
		m.idColWidth = msg.idColWidth
		// Calculate responsive columns based on hasTags and width
		m.cols = ui.CalculateResponsiveColumns(m.width, m.hasTags)
		m.updateDelegate()
		return m, nil

	case errMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// Toggle deep search: "/" while filtering with empty input
		if m.list.FilterState() == list.Filtering && msg.String() == "/" {
			if m.list.FilterInput.Value() == "" {
				*m.deepSearch = !*m.deepSearch
				if *m.deepSearch {
					m.list.FilterInput.Prompt = "Search: "
				} else {
					m.list.FilterInput.Prompt = "Filter: "
				}
				return m, nil
			}
		}

		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case " ":
				// Toggle selection for multi-select, then move to next item
				if item, ok := m.list.SelectedItem().(beanItem); ok {
					if m.selectedBeans[item.issue.ID] {
						delete(m.selectedBeans, item.issue.ID)
					} else {
						m.selectedBeans[item.issue.ID] = true
					}
					m.list.CursorDown()
				}
				return m, nil
			case "enter":
				if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return selectIssueMsg{issue: item.issue}
					}
				}
			case "p":
				// Open parent picker for selected bean(s)
				if len(m.selectedBeans) > 0 {
					// Multi-select mode
					ids := make([]string, 0, len(m.selectedBeans))
					types := make([]string, 0, len(m.selectedBeans))
					for id := range m.selectedBeans {
						ids = append(ids, id)
						// Find the issue to get its type
						for _, item := range m.list.Items() {
							if bi, ok := item.(beanItem); ok && bi.issue.ID == id {
								types = append(types, bi.issue.Type)
								break
							}
						}
					}
					return m, func() tea.Msg {
						return openParentPickerMsg{
							beanIDs:   ids,
							beanTitle: fmt.Sprintf("%d selected beans", len(ids)),
							beanTypes: types,
						}
					}
				} else if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return openParentPickerMsg{
							beanIDs:       []string{item.issue.ID},
							beanTitle:     item.issue.Title,
							beanTypes:     []string{item.issue.Type},
							currentParent: item.issue.Parent,
						}
					}
				}
			case "s":
				// Open status picker for selected bean(s)
				if len(m.selectedBeans) > 0 {
					// Multi-select mode
					ids := make([]string, 0, len(m.selectedBeans))
					for id := range m.selectedBeans {
						ids = append(ids, id)
					}
					return m, func() tea.Msg {
						return openStatusPickerMsg{
							beanIDs:   ids,
							beanTitle: fmt.Sprintf("%d selected beans", len(ids)),
						}
					}
				} else if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return openStatusPickerMsg{
							beanIDs:       []string{item.issue.ID},
							beanTitle:     item.issue.Title,
							currentStatus: item.issue.Status,
						}
					}
				}
			case "t":
				// Open type picker for selected bean(s)
				if len(m.selectedBeans) > 0 {
					// Multi-select mode
					ids := make([]string, 0, len(m.selectedBeans))
					for id := range m.selectedBeans {
						ids = append(ids, id)
					}
					return m, func() tea.Msg {
						return openTypePickerMsg{
							beanIDs:   ids,
							beanTitle: fmt.Sprintf("%d selected beans", len(ids)),
						}
					}
				} else if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return openTypePickerMsg{
							beanIDs:     []string{item.issue.ID},
							beanTitle:   item.issue.Title,
							currentType: item.issue.Type,
						}
					}
				}
			case "P":
				// Open priority picker for selected bean(s)
				if len(m.selectedBeans) > 0 {
					// Multi-select mode
					ids := make([]string, 0, len(m.selectedBeans))
					for id := range m.selectedBeans {
						ids = append(ids, id)
					}
					return m, func() tea.Msg {
						return openPriorityPickerMsg{
							beanIDs:   ids,
							beanTitle: fmt.Sprintf("%d selected beans", len(ids)),
						}
					}
				} else if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return openPriorityPickerMsg{
							beanIDs:         []string{item.issue.ID},
							beanTitle:       item.issue.Title,
							currentPriority: item.issue.Priority,
						}
					}
				}
			case "b":
				// Open blocking picker for selected bean
				if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return openBlockingPickerMsg{
							beanID:          item.issue.ID,
							beanTitle:       item.issue.Title,
							currentBlocking: item.issue.Blocking,
						}
					}
				}
			case "o":
				// Open sort order picker
				return m, func() tea.Msg {
					return openSortPickerMsg{currentOrder: m.sortOrder}
				}
			case "c":
				// Open create modal
				return m, func() tea.Msg {
					return openCreateModalMsg{}
				}
			case "e":
				// Open editor for selected bean
				if item, ok := m.list.SelectedItem().(beanItem); ok {
					return m, func() tea.Msg {
						return openEditorMsg{
							beanID:   item.issue.ID,
							beanPath: item.issue.Path,
						}
					}
				}
			case "y":
				// Copy issue ID(s) to clipboard
				if len(m.selectedBeans) > 0 {
					// Multi-select mode: copy all selected IDs
					ids := make([]string, 0, len(m.selectedBeans))
					for id := range m.selectedBeans {
						ids = append(ids, id)
					}
					return m, func() tea.Msg {
						return copyBeanIDMsg{ids: ids}
					}
				} else if item, ok := m.list.SelectedItem().(beanItem); ok {
					// Single bean mode
					return m, func() tea.Msg {
						return copyBeanIDMsg{ids: []string{item.issue.ID}}
					}
				}
			case "esc", "backspace":
				// First clear selection if any beans are selected
				if len(m.selectedBeans) > 0 {
					clear(m.selectedBeans)
					return m, nil
				}
				// Then clear active filter if any
				if m.hasActiveFilter() {
					return m, func() tea.Msg {
						return clearFilterMsg{}
					}
				}
			}
		}
	}

	// Always forward to the list component
	m.list, cmd = m.list.Update(msg)

	// Reset deep search if filtering ended
	if m.list.FilterState() == list.Unfiltered && *m.deepSearch {
		*m.deepSearch = false
		m.list.FilterInput.Prompt = "Filter: "
	}

	return m, cmd
}

// updateDelegate updates the list delegate with current responsive columns
func (m *listModel) updateDelegate() {
	delegate := itemDelegate{
		cfg:           m.config,
		hasTags:       m.hasTags,
		width:         m.width,
		cols:          m.cols,
		idColWidth:    m.idColWidth,
		selectedBeans: &m.selectedBeans,
	}
	m.list.SetDelegate(delegate)
}

func (m listModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	if m.width == 0 {
		return "Loading..."
	}

	// Update title based on active filter
	if m.tagFilter != "" {
		m.list.Title = fmt.Sprintf("Beans [tag: %s]", m.tagFilter)
	} else {
		m.list.Title = "Beans"
	}

	// Simple bordered container
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorMuted).
		Width(m.width - 2).
		Height(m.height - 4)

	content := border.Render(m.list.View())

	// Footer - show different help based on filter/selection state
	var help string

	// Show selection count if any beans are selected
	var selectionPrefix string
	if len(m.selectedBeans) > 0 {
		selectionStyle := lipgloss.NewStyle().Foreground(ui.ColorWarning).Bold(true)
		selectionPrefix = selectionStyle.Render(fmt.Sprintf("(%d selected) ", len(m.selectedBeans)))
	}

	if len(m.selectedBeans) > 0 {
		// When beans are selected, show esc to clear selection
		help = helpKeyStyle.Render("space") + " " + helpStyle.Render("toggle") + "  " +
			helpKeyStyle.Render("o") + " " + helpStyle.Render("sort") + "  " +
			helpKeyStyle.Render("P") + " " + helpStyle.Render("priority") + "  " +
			helpKeyStyle.Render("s") + " " + helpStyle.Render("status") + "  " +
			helpKeyStyle.Render("t") + " " + helpStyle.Render("type") + "  " +
			helpKeyStyle.Render("y") + " " + helpStyle.Render("copy id") + "  " +
			helpKeyStyle.Render("esc") + " " + helpStyle.Render("clear selection") + "  " +
			helpKeyStyle.Render("?") + " " + helpStyle.Render("help") + "  " +
			helpKeyStyle.Render("q") + " " + helpStyle.Render("quit")
	} else if m.hasActiveFilter() {
		help = helpKeyStyle.Render("space") + " " + helpStyle.Render("select") + "  " +
			helpKeyStyle.Render("enter") + " " + helpStyle.Render("view") + "  " +
			helpKeyStyle.Render("b") + " " + helpStyle.Render("blocking") + "  " +
			helpKeyStyle.Render("c") + " " + helpStyle.Render("create") + "  " +
			helpKeyStyle.Render("e") + " " + helpStyle.Render("edit") + "  " +
			helpKeyStyle.Render("o") + " " + helpStyle.Render("sort") + "  " +
			helpKeyStyle.Render("p") + " " + helpStyle.Render("parent") + "  " +
			helpKeyStyle.Render("P") + " " + helpStyle.Render("priority") + "  " +
			helpKeyStyle.Render("s") + " " + helpStyle.Render("status") + "  " +
			helpKeyStyle.Render("t") + " " + helpStyle.Render("type") + "  " +
			helpKeyStyle.Render("y") + " " + helpStyle.Render("copy id") + "  " +
			helpKeyStyle.Render("esc") + " " + helpStyle.Render("clear filter") + "  " +
			helpKeyStyle.Render("?") + " " + helpStyle.Render("help") + "  " +
			helpKeyStyle.Render("q") + " " + helpStyle.Render("quit")
	} else {
		help = helpKeyStyle.Render("space") + " " + helpStyle.Render("select") + "  " +
			helpKeyStyle.Render("enter") + " " + helpStyle.Render("view") + "  " +
			helpKeyStyle.Render("b") + " " + helpStyle.Render("blocking") + "  " +
			helpKeyStyle.Render("c") + " " + helpStyle.Render("create") + "  " +
			helpKeyStyle.Render("e") + " " + helpStyle.Render("edit") + "  " +
			helpKeyStyle.Render("o") + " " + helpStyle.Render("sort") + "  " +
			helpKeyStyle.Render("p") + " " + helpStyle.Render("parent") + "  " +
			helpKeyStyle.Render("P") + " " + helpStyle.Render("priority") + "  " +
			helpKeyStyle.Render("s") + " " + helpStyle.Render("status") + "  " +
			helpKeyStyle.Render("t") + " " + helpStyle.Render("type") + "  " +
			helpKeyStyle.Render("y") + " " + helpStyle.Render("copy id") + "  " +
			helpKeyStyle.Render("/") + " " + helpStyle.Render("filter") + "  " +
			helpKeyStyle.Render("//") + " " + helpStyle.Render("search") + "  " +
			helpKeyStyle.Render("?") + " " + helpStyle.Render("help") + "  " +
			helpKeyStyle.Render("q") + " " + helpStyle.Render("quit")
	}

	// Show status message if present, otherwise show help
	footer := selectionPrefix
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(ui.ColorSuccess).Bold(true)
		footer += statusStyle.Render(m.statusMessage)
	} else {
		footer += help
	}

	return content + "\n" + footer
}

