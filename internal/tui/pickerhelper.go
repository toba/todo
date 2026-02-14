package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/toba/todo/internal/ui"
)

// pickerDimensions holds the calculated dimensions for a picker modal.
type pickerDimensions struct {
	ModalWidth  int
	ModalHeight int
	ListWidth   int
	ListHeight  int
}

// pickerDimensionConfig holds the configuration for calculating picker dimensions.
type pickerDimensionConfig struct {
	// WidthPct is the percentage of screen width for the modal (default 50).
	WidthPct int
	// HeightPct is the percentage of screen height for the modal (default 50).
	HeightPct int
	// MinWidth is the minimum modal width (default 40).
	MinWidth int
	// MaxWidth is the maximum modal width (default 60).
	MaxWidth int
	// MinHeight is the minimum modal height (default 10).
	MinHeight int
	// MaxHeight is the maximum modal height (default 16).
	MaxHeight int
	// WidthPadding is subtracted from modal width to get list width (default 6).
	WidthPadding int
	// HeightPadding is subtracted from modal height to get list height (default 7).
	HeightPadding int
}

// defaultPickerDimensionConfig returns the default picker dimension config
// used by status, type, and priority pickers.
func defaultPickerDimensionConfig() pickerDimensionConfig {
	return pickerDimensionConfig{
		WidthPct:      50,
		HeightPct:     50,
		MinWidth:      40,
		MaxWidth:      60,
		MinHeight:     10,
		MaxHeight:     16,
		WidthPadding:  6,
		HeightPadding: 7,
	}
}

// calculatePickerDimensions computes modal and list dimensions from screen size.
func calculatePickerDimensions(screenWidth, screenHeight int, cfg pickerDimensionConfig) pickerDimensions {
	modalWidth := max(cfg.MinWidth, min(cfg.MaxWidth, screenWidth*cfg.WidthPct/100))
	modalHeight := max(cfg.MinHeight, min(cfg.MaxHeight, screenHeight*cfg.HeightPct/100))
	return pickerDimensions{
		ModalWidth:  modalWidth,
		ModalHeight: modalHeight,
		ListWidth:   modalWidth - cfg.WidthPadding,
		ListHeight:  modalHeight - cfg.HeightPadding,
	}
}

// renderPickerCursor renders the cursor indicator for picker list items.
// Returns the cursor string (with trailing space) for the given index.
func renderPickerCursor(index int, m interface{ Index() int }) string {
	if index == m.Index() {
		return lipgloss.NewStyle().Foreground(ui.ColorPrimary).Bold(true).Render("▌") + " "
	}
	return "  "
}

// renderPickerItem renders a standard picker item with cursor, text, and optional current indicator.
func renderPickerItem(w io.Writer, cursor, text string, isCurrent bool) {
	var currentIndicator string
	if isCurrent {
		currentIndicator = ui.Muted.Render(" (current)")
	}
	fmt.Fprint(w, cursor+text+currentIndicator)
}
