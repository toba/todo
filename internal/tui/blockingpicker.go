package tui

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/todo/internal/issue"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/ui"
)

// blockingConfirmedMsg is sent when blocking changes are confirmed
type blockingConfirmedMsg struct {
	beanID  string            // the issue we're modifying
	toAdd   []string          // IDs to add to blocking
	toRemove []string         // IDs to remove from blocking
}

// closeBlockingPickerMsg is sent when the blocking picker is cancelled
type closeBlockingPickerMsg struct{}

// openBlockingPickerMsg requests opening the blocking picker for an issue
type openBlockingPickerMsg struct {
	beanID          string
	beanTitle       string
	currentBlocking []string // IDs of beans currently being blocked
}

// blockingItem wraps an issue to implement list.Item for the blocking picker
type blockingItem struct {
	issue *issue.Issue
	cfg  *config.Config
}

func (i blockingItem) Title() string       { return i.issue.Title }
func (i blockingItem) Description() string { return i.issue.ID }
func (i blockingItem) FilterValue() string { return i.issue.Title + " " + i.issue.ID }

// blockingItemDelegate handles rendering of blocking picker items
type blockingItemDelegate struct {
	cfg             *config.Config
	pendingBlocking *map[string]bool // pointer to pending state for live updates
}

func (d blockingItemDelegate) Height() int                             { return 1 }
func (d blockingItemDelegate) Spacing() int                            { return 0 }
func (d blockingItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d blockingItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(blockingItem)
	if !ok {
		return
	}

	cursor := renderPickerCursor(index, &m)

	// Show blocking indicator - read from pending state for live updates
	isBlocking := (*d.pendingBlocking)[item.issue.ID]
	var blockingIndicator string
	if isBlocking {
		blockingIndicator = lipgloss.NewStyle().Foreground(ui.ColorDanger).Bold(true).Render("● ") // Red dot for blocking
	} else {
		blockingIndicator = lipgloss.NewStyle().Foreground(ui.ColorMuted).Render("○ ") // Empty circle for not blocking
	}

	// Get colors from config
	colors := d.cfg.GetIssueColors(item.issue.Status, item.issue.Type, item.issue.Priority)

	// Format: [indicator] [type] title (id)
	typeBadge := ui.RenderTypeText(item.issue.Type, colors.TypeColor)
	title := item.issue.Title
	if colors.IsArchive {
		title = ui.Muted.Render(title)
	}
	id := ui.Muted.Render(" (" + item.issue.ID + ")")

	fmt.Fprint(w, cursor+blockingIndicator+typeBadge+" "+title+id)
}

// blockingPickerModel is the model for the blocking picker view
type blockingPickerModel struct {
	list             list.Model
	beanID           string           // the issue we're setting blocking for
	beanTitle        string           // the issue's title
	originalBlocking map[string]bool  // original state (for computing diff)
	pendingBlocking  map[string]bool  // pending state (toggled by space)
	cfg              *config.Config
	width            int
	height           int
}

func newBlockingPickerModel(beanID, beanTitle string, currentBlocking []string, resolver *graph.Resolver, cfg *config.Config, width, height int) blockingPickerModel {
	// Fetch all issues
	allBeans, _ := resolver.Query().Issues(context.Background(), nil)

	// Create maps for original and pending state
	originalBlocking := make(map[string]bool)
	pendingBlocking := make(map[string]bool)
	for _, id := range currentBlocking {
		originalBlocking[id] = true
		pendingBlocking[id] = true
	}

	// Filter out the current bean and build items
	var eligibleBeans []*issue.Issue
	for _, b := range allBeans {
		if b.ID != beanID {
			eligibleBeans = append(eligibleBeans, b)
		}
	}

	// Sort by type order, then by title
	typeNames := cfg.TypeNames()
	typeOrder := make(map[string]int)
	for i, t := range typeNames {
		typeOrder[t] = i
	}
	sort.Slice(eligibleBeans, func(i, j int) bool {
		ti, tj := typeOrder[eligibleBeans[i].Type], typeOrder[eligibleBeans[j].Type]
		if ti != tj {
			return ti < tj
		}
		return strings.ToLower(eligibleBeans[i].Title) < strings.ToLower(eligibleBeans[j].Title)
	})

	// Build items list
	items := make([]list.Item, 0, len(eligibleBeans))
	for _, b := range eligibleBeans {
		items = append(items, blockingItem{
			issue: b,
			cfg:  cfg,
		})
	}

	// Calculate modal dimensions (60% width, 60% height, with min/max constraints)
	dims := calculatePickerDimensions(width, height, pickerDimensionConfig{
		WidthPct:      60,
		HeightPct:     60,
		MinWidth:      40,
		MaxWidth:      80,
		MinHeight:     10,
		MaxHeight:     20,
		WidthPadding:  6,
		HeightPadding: 9, // header(1) + subtitle(1) + blank(1) + blank(1) + description(1) + blank(1) + help(1) + border(2)
	})

	// Create delegate with pointer to pending state (so it can read live updates)
	delegate := blockingItemDelegate{cfg: cfg, pendingBlocking: &pendingBlocking}

	l := list.New(items, delegate, dims.ListWidth, dims.ListHeight)
	l.Title = "Manage Blocking"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Filter = substringFilter
	l.Styles.Title = listTitleStyle
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 0, 0)
	l.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ui.ColorPrimary)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary)

	return blockingPickerModel{
		list:             l,
		beanID:           beanID,
		beanTitle:        beanTitle,
		originalBlocking: originalBlocking,
		pendingBlocking:  pendingBlocking,
		cfg:              cfg,
		width:            width,
		height:           height,
	}
}

func (m blockingPickerModel) Init() tea.Cmd {
	return nil
}

func (m blockingPickerModel) Update(msg tea.Msg) (blockingPickerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		dims := calculatePickerDimensions(msg.Width, msg.Height, pickerDimensionConfig{
			WidthPct: 60, HeightPct: 60, MinWidth: 40, MaxWidth: 80,
			MinHeight: 10, MaxHeight: 20, WidthPadding: 6, HeightPadding: 9,
		})
		m.list.SetSize(dims.ListWidth, dims.ListHeight)

	case tea.KeyMsg:
		if m.list.FilterState() != list.Filtering {
			switch msg.String() {
			case " ":
				// Toggle the selected item's pending state
				// The delegate reads from pendingBlocking directly, so no need to update items
				if item, ok := m.list.SelectedItem().(blockingItem); ok {
					targetID := item.issue.ID
					if m.pendingBlocking[targetID] {
						delete(m.pendingBlocking, targetID)
					} else {
						m.pendingBlocking[targetID] = true
					}
				}
				return m, nil

			case "enter":
				// Confirm changes - compute diff and send message
				var toAdd, toRemove []string

				// Find additions (in pending but not in original)
				for id := range m.pendingBlocking {
					if !m.originalBlocking[id] {
						toAdd = append(toAdd, id)
					}
				}

				// Find removals (in original but not in pending)
				for id := range m.originalBlocking {
					if !m.pendingBlocking[id] {
						toRemove = append(toRemove, id)
					}
				}

				return m, func() tea.Msg {
					return blockingConfirmedMsg{
						beanID:   m.beanID,
						toAdd:    toAdd,
						toRemove: toRemove,
					}
				}

			case "esc", "backspace":
				// Cancel - discard changes
				return m, func() tea.Msg {
					return closeBlockingPickerMsg{}
				}
			}
		}
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m blockingPickerModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	return renderPickerModal(pickerModalConfig{
		Title:       "Manage Blocking",
		BeanTitle:   m.beanTitle,
		IssueID:      m.beanID,
		ListContent: m.list.View(),
		Description: "space toggle, enter confirm, esc cancel",
		Width:       m.width,
		WidthPct:    60,
		MaxWidth:    80,
	})
}

// ModalView returns the picker rendered as a centered modal overlay on top of the background
func (m blockingPickerModel) ModalView(bgView string, fullWidth, fullHeight int) string {
	modal := m.View()
	return overlayModal(bgView, modal, fullWidth, fullHeight)
}
