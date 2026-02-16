package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/graph"
	"github.com/toba/todo/internal/graph/model"
)

// viewState represents which view is currently active
type viewState int

const (
	viewList viewState = iota
	viewDetail
	viewTagPicker
	viewParentPicker
	viewStatusPicker
	viewTypePicker
	viewBlockingPicker
	viewPriorityPicker
	viewSortPicker
	viewCreateModal
	viewHelpOverlay
)

// issuesChangedMsg is sent when issues change on disk (via file watcher)
type issuesChangedMsg struct{}

// tickMsg is sent periodically to refresh the TUI as a safety net
type tickMsg time.Time

// openTagPickerMsg requests opening the tag picker
type openTagPickerMsg struct{}

// tagSelectedMsg is sent when a tag is selected from the picker
type tagSelectedMsg struct {
	tag string
}

// clearFilterMsg is sent to clear any active filter
type clearFilterMsg struct{}

// copyIssueIDMsg requests copying issue ID(s) to clipboard
type copyIssueIDMsg struct {
	ids []string
}

// openEditorMsg requests opening the editor for an issue
type openEditorMsg struct {
	issueID   string
	issuePath string
}

// editorFinishedMsg is sent when the editor closes
type editorFinishedMsg struct {
	err error
}

// openParentPickerMsg requests opening the parent picker for issue(s)
type openParentPickerMsg struct {
	issueIDs      []string // IDs of issues to update
	issueTitle    string   // Display title (single title or "N selected issues")
	issueTypes    []string // Types of the issues (to filter eligible parents)
	currentParent string   // Only meaningful for single issue
}

// App is the main TUI application model
type App struct {
	state          viewState
	list           listModel
	detail         detailModel
	tagPicker      tagPickerModel
	parentPicker   parentPickerModel
	statusPicker   statusPickerModel
	typePicker     typePickerModel
	blockingPicker blockingPickerModel
	priorityPicker priorityPickerModel
	sortPicker     sortPickerModel
	createModal    createModalModel
	helpOverlay    helpOverlayModel
	history        []detailModel // stack of previous detail views for back navigation
	core           *core.Core
	resolver       *graph.Resolver
	config         *config.Config
	width          int
	height         int
	program        *tea.Program // reference to program for sending messages from watcher

	// Key chord state - tracks partial key sequences like "g" waiting for "t"
	pendingKey string

	// Modal state - tracks view behind modal pickers
	previousState viewState

	// Editor state - tracks issue being edited to update updated_at on save
	editingIssueID      string
	editingIssueModTime time.Time
}

// New creates a new TUI application
func New(core *core.Core, cfg *config.Config) *App {
	resolver := &graph.Resolver{Core: core}
	return &App{
		state:    viewList,
		core:     core,
		resolver: resolver,
		config:   cfg,
		list:     newListModel(resolver, cfg),
	}
}

// tickCmd returns a command that sends a tickMsg after 2 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.list.Init(), tickCmd())
}

// Update handles messages
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height

	case tea.KeyMsg:
		// Clear status messages on any keypress
		a.list.statusMessage = ""
		a.detail.statusMessage = ""

		// Handle key chord sequences
		if a.state == viewList && a.list.list.FilterState() != 1 {
			if a.pendingKey == "g" {
				a.pendingKey = ""
				switch msg.String() {
				case "t":
					// "g t" - go to tags
					return a, func() tea.Msg { return openTagPickerMsg{} }
				default:
					// Invalid second key, ignore the chord
				}
				// Don't forward this key since it was part of a chord attempt
				return a, nil
			}

			// Start of potential chord
			if msg.String() == "g" {
				a.pendingKey = "g"
				return a, nil
			}
		}

		// Clear pending key on any other key press
		a.pendingKey = ""

		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "?":
			// Open help overlay if not already showing it (and not in a picker/modal)
			if a.state == viewList || a.state == viewDetail {
				a.previousState = a.state
				a.helpOverlay = newHelpOverlayModel(a.width, a.height)
				a.state = viewHelpOverlay
				return a, a.helpOverlay.Init()
			}
		case "q":
			if a.state == viewDetail || a.state == viewTagPicker || a.state == viewParentPicker || a.state == viewStatusPicker || a.state == viewTypePicker || a.state == viewBlockingPicker || a.state == viewPriorityPicker || a.state == viewSortPicker || a.state == viewHelpOverlay {
				return a, tea.Quit
			}
			// For list, only quit if not filtering
			if a.state == viewList && a.list.list.FilterState() != 1 {
				return a, tea.Quit
			}
		}

	case issuesChangedMsg:
		// Issues changed on disk - refresh
		if a.state == viewDetail {
			// Try to reload the current issue via GraphQL
			updatedIssue, err := a.resolver.Query().Issue(context.Background(), a.detail.issue.ID)
			if err != nil || updatedIssue == nil {
				// Issue was deleted - return to list
				a.state = viewList
				a.history = nil
			} else {
				// Recreate detail view with fresh issue data
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		// Trigger list refresh
		return a, a.list.loadIssues

	case tickMsg:
		// Periodic refresh as safety net for dropped fsnotify events
		if a.state == viewDetail {
			updatedIssue, err := a.resolver.Query().Issue(context.Background(), a.detail.issue.ID)
			if err != nil || updatedIssue == nil {
				a.state = viewList
				a.history = nil
			} else {
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		return a, tea.Batch(tickCmd(), a.list.loadIssues)

	case openTagPickerMsg:
		// Collect all tags with their counts
		tags := a.collectTagsWithCounts()
		if len(tags) == 0 {
			// No tags in system, don't open picker
			return a, nil
		}
		a.tagPicker = newTagPickerModel(tags, a.width, a.height)
		a.state = viewTagPicker
		return a, a.tagPicker.Init()

	case tagSelectedMsg:
		a.state = viewList
		a.list.setTagFilter(msg.tag)
		return a, a.list.loadIssues

	case openParentPickerMsg:
		// Check if all issue types can have parents
		for _, issueType := range msg.issueTypes {
			if core.ValidParentTypes(issueType) == nil {
				// At least one issue type (e.g., milestone) cannot have parents - don't open the picker
				return a, nil
			}
		}
		a.previousState = a.state // Remember where we came from for the modal background
		a.parentPicker = newParentPickerModel(msg.issueIDs, msg.issueTitle, msg.issueTypes, msg.currentParent, a.resolver, a.config, a.width, a.height)
		a.state = viewParentPicker
		return a, a.parentPicker.Init()

	case closeParentPickerMsg:
		// Return to previous view and refresh in case issues changed while picker was open
		a.state = a.previousState
		return a, a.list.loadIssues

	case openStatusPickerMsg:
		a.previousState = a.state
		a.statusPicker = newStatusPickerModel(msg.issueIDs, msg.issueTitle, msg.currentStatus, a.config, a.width, a.height)
		a.state = viewStatusPicker
		return a, a.statusPicker.Init()

	case closeStatusPickerMsg:
		// Return to previous view and refresh in case issues changed while picker was open
		a.state = a.previousState
		return a, a.list.loadIssues

	case statusSelectedMsg:
		// Update all issues' status via GraphQL mutations
		for _, issueID := range msg.issueIDs {
			_, err := a.resolver.Mutation().UpdateIssue(context.Background(), issueID, model.UpdateIssueInput{
				Status: &msg.status,
			})
			if err != nil {
				// Continue with other issues even if one fails
				continue
			}
		}
		// Return to the previous view and refresh
		a.state = a.previousState
		// Clear selection after batch edit
		clear(a.list.selectedIssues)
		if a.state == viewDetail && len(msg.issueIDs) == 1 {
			updatedIssue, _ := a.resolver.Query().Issue(context.Background(), msg.issueIDs[0])
			if updatedIssue != nil {
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		return a, a.list.loadIssues

	case openTypePickerMsg:
		a.previousState = a.state
		a.typePicker = newTypePickerModel(msg.issueIDs, msg.issueTitle, msg.currentType, a.config, a.width, a.height)
		a.state = viewTypePicker
		return a, a.typePicker.Init()

	case closeTypePickerMsg:
		// Return to previous view and refresh in case issues changed while picker was open
		a.state = a.previousState
		return a, a.list.loadIssues

	case typeSelectedMsg:
		// Update all issues' type via GraphQL mutations
		for _, issueID := range msg.issueIDs {
			_, err := a.resolver.Mutation().UpdateIssue(context.Background(), issueID, model.UpdateIssueInput{
				Type: &msg.issueType,
			})
			if err != nil {
				// Continue with other issues even if one fails
				continue
			}
		}
		// Return to the previous view and refresh
		a.state = a.previousState
		// Clear selection after batch edit
		clear(a.list.selectedIssues)
		if a.state == viewDetail && len(msg.issueIDs) == 1 {
			updatedIssue, _ := a.resolver.Query().Issue(context.Background(), msg.issueIDs[0])
			if updatedIssue != nil {
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		return a, a.list.loadIssues

	case openPriorityPickerMsg:
		a.previousState = a.state
		a.priorityPicker = newPriorityPickerModel(msg.issueIDs, msg.issueTitle, msg.currentPriority, a.config, a.width, a.height)
		a.state = viewPriorityPicker
		return a, a.priorityPicker.Init()

	case closePriorityPickerMsg:
		// Return to previous view and refresh in case issues changed while picker was open
		a.state = a.previousState
		return a, a.list.loadIssues

	case prioritySelectedMsg:
		// Update all issues' priority via GraphQL mutations
		for _, issueID := range msg.issueIDs {
			_, err := a.resolver.Mutation().UpdateIssue(context.Background(), issueID, model.UpdateIssueInput{
				Priority: &msg.priority,
			})
			if err != nil {
				// Continue with other issues even if one fails
				continue
			}
		}
		// Return to the previous view and refresh
		a.state = a.previousState
		// Clear selection after batch edit
		clear(a.list.selectedIssues)
		if a.state == viewDetail && len(msg.issueIDs) == 1 {
			updatedIssue, _ := a.resolver.Query().Issue(context.Background(), msg.issueIDs[0])
			if updatedIssue != nil {
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		return a, a.list.loadIssues

	case openSortPickerMsg:
		a.previousState = a.state
		a.sortPicker = newSortPickerModel(msg.currentOrder, a.width, a.height)
		a.state = viewSortPicker
		return a, a.sortPicker.Init()

	case closeSortPickerMsg:
		a.state = a.previousState
		return a, nil

	case sortSelectedMsg:
		a.list.sortOrder = msg.order
		a.state = a.previousState
		return a, a.list.loadIssues

	case openHelpMsg:
		a.previousState = a.state
		a.helpOverlay = newHelpOverlayModel(a.width, a.height)
		a.state = viewHelpOverlay
		return a, a.helpOverlay.Init()

	case closeHelpMsg:
		a.state = a.previousState
		return a, nil

	case openBlockingPickerMsg:
		a.previousState = a.state
		a.blockingPicker = newBlockingPickerModel(msg.issueID, msg.issueTitle, msg.currentBlocking, a.resolver, a.config, a.width, a.height)
		a.state = viewBlockingPicker
		return a, a.blockingPicker.Init()

	case closeBlockingPickerMsg:
		// Return to previous view and refresh in case issues changed while picker was open
		a.state = a.previousState
		return a, a.list.loadIssues

	case blockingConfirmedMsg:
		// Apply all blocking changes via updateIssue mutation
		if len(msg.toAdd) > 0 || len(msg.toRemove) > 0 {
			input := model.UpdateIssueInput{
				AddBlocking:    msg.toAdd,
				RemoveBlocking: msg.toRemove,
			}
			a.resolver.Mutation().UpdateIssue(context.Background(), msg.issueID, input)
		}
		// Return to previous view and refresh
		a.state = a.previousState
		if a.state == viewDetail {
			updatedIssue, _ := a.resolver.Query().Issue(context.Background(), msg.issueID)
			if updatedIssue != nil {
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		return a, a.list.loadIssues

	case openCreateModalMsg:
		a.previousState = a.state
		a.createModal = newCreateModalModel(a.width, a.height)
		a.state = viewCreateModal
		return a, a.createModal.Init()

	case closeCreateModalMsg:
		a.state = a.previousState
		return a, nil

	case issueCreatedMsg:
		// Create the issue via GraphQL mutation with draft status
		draftStatus := "draft"
		createdIssue, err := a.resolver.Mutation().CreateIssue(context.Background(), model.CreateIssueInput{
			Title:  msg.title,
			Status: &draftStatus,
		})
		if err != nil {
			// TODO: Show error to user
			a.state = a.previousState
			return a, nil
		}
		// Return to list and open the new issue in editor
		a.state = viewList
		return a, tea.Batch(
			a.list.loadIssues,
			func() tea.Msg {
				return openEditorMsg{issueID: createdIssue.ID, issuePath: createdIssue.Path}
			},
		)

	case openEditorMsg:
		// Launch editor for the issue file
		editorCmd, editorArgs := getEditor(a.config)
		fullPath := filepath.Join(a.core.Root(), msg.issuePath)

		// Record the issue ID and file mod time before editing
		a.editingIssueID = msg.issueID
		if info, err := os.Stat(fullPath); err == nil {
			a.editingIssueModTime = info.ModTime()
		}

		args := append(editorArgs, fullPath)
		c := exec.Command(editorCmd, args...)
		return a, tea.ExecProcess(c, func(err error) tea.Msg {
			return editorFinishedMsg{err: err}
		})

	case editorFinishedMsg:
		// Editor closed - check if file was modified and update updated_at if so
		if a.editingIssueID != "" {
			if b, err := a.core.Get(a.editingIssueID); err == nil {
				fullPath := filepath.Join(a.core.Root(), b.Path)
				if info, err := os.Stat(fullPath); err == nil {
					if info.ModTime().After(a.editingIssueModTime) {
						// File was modified - reload from disk first to get user's changes,
						// then call Update to set updated_at
						_ = a.core.Load()
						if b, err = a.core.Get(a.editingIssueID); err == nil {
							_ = a.core.Update(b, nil)
						}
					}
				}
			}
			// Clear editing state
			a.editingIssueID = ""
			a.editingIssueModTime = time.Time{}
		}
		return a, nil

	case parentSelectedMsg:
		// Set the new parent via updateIssue mutation for all issues
		parentValue := msg.parentID
		input := model.UpdateIssueInput{
			Parent: &parentValue,
		}
		for _, issueID := range msg.issueIDs {
			_, err := a.resolver.Mutation().UpdateIssue(context.Background(), issueID, input)
			if err != nil {
				// Continue with other issues even if one fails
				continue
			}
		}
		// Return to the previous view and refresh
		a.state = a.previousState
		// Clear selection after batch edit
		clear(a.list.selectedIssues)
		if a.state == viewDetail && len(msg.issueIDs) == 1 {
			// Refresh the issue to show updated parent
			updatedIssue, _ := a.resolver.Query().Issue(context.Background(), msg.issueIDs[0])
			if updatedIssue != nil {
				a.detail = newDetailModel(updatedIssue, a.resolver, a.config, a.width, a.height)
			}
		}
		return a, a.list.loadIssues

	case clearFilterMsg:
		a.list.clearFilter()
		return a, a.list.loadIssues

	case copyIssueIDMsg:
		var statusMsg string
		text := strings.Join(msg.ids, ", ")
		if err := clipboard.WriteAll(text); err != nil {
			statusMsg = fmt.Sprintf("Failed to copy: %v", err)
		} else if len(msg.ids) == 1 {
			statusMsg = fmt.Sprintf("Copied %s to clipboard", msg.ids[0])
		} else {
			statusMsg = fmt.Sprintf("Copied %d issue IDs to clipboard", len(msg.ids))
		}

		// Set status on current view
		if a.state == viewList {
			a.list.statusMessage = statusMsg
		} else if a.state == viewDetail {
			a.detail.statusMessage = statusMsg
		}

		return a, nil

	case selectIssueMsg:
		// Push current detail view to history if we're already viewing an issue
		if a.state == viewDetail {
			a.history = append(a.history, a.detail)
		}
		a.state = viewDetail
		a.detail = newDetailModel(msg.issue, a.resolver, a.config, a.width, a.height)
		return a, a.detail.Init()

	case backToListMsg:
		// Pop from history if available, otherwise go to list
		if len(a.history) > 0 {
			a.detail = a.history[len(a.history)-1]
			a.history = a.history[:len(a.history)-1]
			// Stay in viewDetail state
		} else {
			a.state = viewList
			// Force list to pick up any size changes that happened while in detail view
			a.list, cmd = a.list.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			return a, cmd
		}
		return a, nil
	}

	// Forward all messages to the current view
	switch a.state {
	case viewList:
		a.list, cmd = a.list.Update(msg)
	case viewDetail:
		a.detail, cmd = a.detail.Update(msg)
	case viewTagPicker:
		a.tagPicker, cmd = a.tagPicker.Update(msg)
	case viewParentPicker:
		a.parentPicker, cmd = a.parentPicker.Update(msg)
	case viewStatusPicker:
		a.statusPicker, cmd = a.statusPicker.Update(msg)
	case viewTypePicker:
		a.typePicker, cmd = a.typePicker.Update(msg)
	case viewPriorityPicker:
		a.priorityPicker, cmd = a.priorityPicker.Update(msg)
	case viewSortPicker:
		a.sortPicker, cmd = a.sortPicker.Update(msg)
	case viewBlockingPicker:
		a.blockingPicker, cmd = a.blockingPicker.Update(msg)
	case viewCreateModal:
		a.createModal, cmd = a.createModal.Update(msg)
	case viewHelpOverlay:
		a.helpOverlay, cmd = a.helpOverlay.Update(msg)
	}

	return a, cmd
}

// collectTagsWithCounts returns all tags with their usage counts
func (a *App) collectTagsWithCounts() []tagWithCount {
	issues, _ := a.resolver.Query().Issues(context.Background(), nil)
	tagCounts := make(map[string]int)
	for _, b := range issues {
		for _, tag := range b.Tags {
			tagCounts[tag]++
		}
	}

	tags := make([]tagWithCount, 0, len(tagCounts))
	for tag, count := range tagCounts {
		tags = append(tags, tagWithCount{tag: tag, count: count})
	}

	return tags
}

// View renders the current view
func (a *App) View() string {
	switch a.state {
	case viewList:
		return a.list.View()
	case viewDetail:
		return a.detail.View()
	case viewTagPicker:
		return a.tagPicker.View()
	case viewParentPicker:
		return a.parentPicker.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewStatusPicker:
		return a.statusPicker.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewTypePicker:
		return a.typePicker.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewPriorityPicker:
		return a.priorityPicker.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewSortPicker:
		return a.sortPicker.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewBlockingPicker:
		return a.blockingPicker.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewCreateModal:
		return a.createModal.ModalView(a.getBackgroundView(), a.width, a.height)
	case viewHelpOverlay:
		return a.helpOverlay.ModalView(a.getBackgroundView(), a.width, a.height)
	}
	return ""
}

// getBackgroundView returns the view to show behind modal pickers
func (a *App) getBackgroundView() string {
	switch a.previousState {
	case viewList:
		return a.list.View()
	case viewDetail:
		return a.detail.View()
	default:
		return a.list.View()
	}
}

// systemEditor returns the OS default application opener command and flags.
// On macOS, it returns "open" with flags to wait for the app to close (-W),
// open a new instance (-n), and keep the GUI app focused (-g).
// On other platforms, it returns empty values (e.g. xdg-open doesn't support blocking).
func systemEditor() (cmd string, args []string, ok bool) {
	if runtime.GOOS == "darwin" {
		return "open", []string{"-W", "-n", "-g"}, true
	}
	return "", nil, false
}

// getEditor returns the user's preferred editor command and any extra arguments.
// Fallback chain: config editor -> $VISUAL -> $EDITOR -> OS default -> vi -> nano.
// The special value "system" resolves to the OS default application opener.
// Multi-word editor values (e.g. "code --wait") are split on whitespace.
// Relative paths from config are resolved relative to the config directory.
func getEditor(cfg *config.Config) (string, []string) {
	editor := cfg.GetEditor()
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}

	// Handle the "system" keyword — resolve to OS default app opener
	if strings.EqualFold(editor, "system") {
		if cmd, args, ok := systemEditor(); ok {
			return cmd, args
		}
		// Unsupported platform — fall through to vi/nano
		editor = ""
	}

	if editor == "" {
		// Try OS default app opener before falling back to terminal editors
		if cmd, args, ok := systemEditor(); ok {
			return cmd, args
		}
		if _, err := exec.LookPath("vi"); err == nil {
			editor = "vi"
		} else {
			editor = "nano"
		}
	}

	parts := strings.Fields(editor)
	cmd := parts[0]
	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	// Resolve relative paths from config against config directory
	if (strings.HasPrefix(cmd, "./") || strings.HasPrefix(cmd, "../")) && cfg.ConfigDir() != "" {
		cmd = filepath.Join(cfg.ConfigDir(), cmd)
	}

	return cmd, args
}

// Run starts the TUI application with file watching
func Run(core *core.Core, cfg *config.Config) error {
	app := New(core, cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Store reference to program for sending messages from watcher
	app.program = p

	// Start file watching
	if err := core.StartWatching(); err != nil {
		return err
	}
	defer core.Unwatch()

	// Subscribe to issue events
	eventCh, unsubscribe := core.Subscribe()
	defer unsubscribe()

	// Forward events to TUI in a goroutine
	go func() {
		for range eventCh {
			// Send message to TUI when issues change
			if app.program != nil {
				app.program.Send(issuesChangedMsg{})
			}
		}
	}()

	_, err := p.Run()
	return err
}
