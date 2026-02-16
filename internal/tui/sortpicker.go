package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/ui"
)

// sortOrder represents a sort mode
type sortOrder string

const (
	sortDefault sortOrder = sortOrder(config.SortDefault)
	sortCreated sortOrder = "created"
	sortUpdated sortOrder = "updated"
	sortDue     sortOrder = "due"
)

// sortSelectedMsg is sent when a sort order is selected from the picker
type sortSelectedMsg struct {
	order sortOrder
}

// closeSortPickerMsg is sent when the sort picker is cancelled
type closeSortPickerMsg struct{}

// openSortPickerMsg requests opening the sort picker
type openSortPickerMsg struct {
	currentOrder sortOrder
}

// sortItem wraps a sort option to implement list.Item
type sortItem struct {
	name        string
	value       sortOrder
	description string
	isCurrent   bool
}

func (i sortItem) Title() string       { return i.name }
func (i sortItem) Description() string { return i.description }
func (i sortItem) FilterValue() string { return i.name + " " + i.description }

// sortItemDelegate handles rendering of sort picker items
type sortItemDelegate struct{}

func (d sortItemDelegate) Height() int                             { return 1 }
func (d sortItemDelegate) Spacing() int                            { return 0 }
func (d sortItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d sortItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	item, ok := listItem.(sortItem)
	if !ok {
		return
	}

	var cursor string
	if index == m.Index() {
		cursor = lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Render("▌") + " "
	} else {
		cursor = "  "
	}

	nameStyle := lipgloss.NewStyle().Bold(index == m.Index())
	nameText := nameStyle.Render(item.name)

	descText := ui.Muted.Render(" — " + item.description)

	// Add current indicator
	var currentIndicator string
	if item.isCurrent {
		currentIndicator = ui.Muted.Render(" (current)")
	}

	fmt.Fprint(w, cursor+nameText+descText+currentIndicator)
}

// sortPickerModel is the model for the sort picker view
type sortPickerModel struct {
	list         list.Model
	currentOrder sortOrder
	width        int
	height       int
}

func newSortPickerModel(currentOrder sortOrder, width, height int) sortPickerModel {
	delegate := sortItemDelegate{}

	options := []struct {
		name        string
		value       sortOrder
		description string
	}{
		{"Default", sortDefault, "Status, priority, type, then title"},
		{"Created", sortCreated, "Newest created first"},
		{"Updated", sortUpdated, "Last updated first"},
		{"Due", sortDue, "Soonest due first"},
	}

	items := make([]list.Item, 0, len(options))
	selectedIndex := 0

	for i, opt := range options {
		isCurrent := opt.value == currentOrder
		if isCurrent {
			selectedIndex = i
		}
		items = append(items, sortItem{
			name:        opt.name,
			value:       opt.value,
			description: opt.description,
			isCurrent:   isCurrent,
		})
	}

	// Calculate modal dimensions
	modalWidth := max(40, min(60, width*50/100))
	modalHeight := max(10, min(16, height*50/100))
	listWidth := modalWidth - 6
	listHeight := modalHeight - 7

	l := list.New(items, delegate, listWidth, listHeight)
	l.Title = "Sort Order"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.Styles.Title = listTitleStyle
	l.Styles.TitleBar = lipgloss.NewStyle().Padding(0, 0, 0, 0)

	// Select the current sort order
	if selectedIndex < len(items) {
		l.Select(selectedIndex)
	}

	return sortPickerModel{
		list:         l,
		currentOrder: currentOrder,
		width:        width,
		height:       height,
	}
}

func (m sortPickerModel) Init() tea.Cmd {
	return nil
}

func (m sortPickerModel) Update(msg tea.Msg) (sortPickerModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		modalWidth := max(40, min(60, msg.Width*50/100))
		modalHeight := max(10, min(16, msg.Height*50/100))
		listWidth := modalWidth - 6
		listHeight := modalHeight - 7
		m.list.SetSize(listWidth, listHeight)

	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(sortItem); ok {
				return m, func() tea.Msg {
					return sortSelectedMsg{order: item.value}
				}
			}
		case "esc", "backspace":
			return m, func() tea.Msg {
				return closeSortPickerMsg{}
			}
		}
	}

	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m sortPickerModel) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Calculate modal dimensions
	modalWidth := max(40, min(60, m.width*50/100))

	// Title
	title := lipgloss.NewStyle().Bold(true).Render("Sort Order")

	// Help footer
	help := helpKeyStyle.Render("enter") + " " + helpStyle.Render("select") + "  " +
		helpKeyStyle.Render("esc") + " " + helpStyle.Render("cancel")

	// Border style
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.ColorPrimary).
		Padding(0, 1).
		Width(modalWidth)

	content := title + "\n\n" + m.list.View() + "\n\n" + help

	return border.Render(content)
}

// ModalView returns the picker rendered as a centered modal overlay on top of the background
func (m sortPickerModel) ModalView(bgView string, fullWidth, fullHeight int) string {
	modal := m.View()
	return overlayModal(bgView, modal, fullWidth, fullHeight)
}
