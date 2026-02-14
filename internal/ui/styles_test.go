package ui

import (
	"testing"

	"github.com/toba/todo/internal/config"
)

func TestRenderIssueRow_NarrowWidth(t *testing.T) {
	// Test that RenderIssueRow doesn't panic with very small MaxTitleWidth values
	// This was a bug where MaxTitleWidth < 4 caused a slice bounds panic

	tests := []struct {
		name          string
		maxTitleWidth int
		title         string
	}{
		{"zero width", 0, "Test Title"},
		{"width 1", 1, "Test Title"},
		{"width 2", 2, "Test Title"},
		{"width 3", 3, "Test Title"},
		{"width 4", 4, "Test Title"},
		{"width 5", 5, "Test Title"},
		{"short title fits", 10, "Hi"},
		{"exact fit", 10, "0123456789"},
		{"needs truncation", 10, "This is a longer title"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("RenderIssueRow panicked with MaxTitleWidth=%d: %v", tt.maxTitleWidth, r)
				}
			}()

			cfg := IssueRowConfig{
				MaxTitleWidth: tt.maxTitleWidth,
				StatusColor:   "green",
				TypeColor:     "blue",
			}

			result := RenderIssueRow("abc123", "todo", "task", tt.title, cfg)
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestRenderIssueRow_NarrowWidthWithPriority(t *testing.T) {
	// Priority symbol takes 2 extra chars, which reduces available title width
	// This tests that the adjustment doesn't cause negative slice bounds

	tests := []struct {
		name          string
		maxTitleWidth int
		priority      string
	}{
		{"width 1 with priority", 1, "high"},
		{"width 2 with priority", 2, "high"},
		{"width 3 with priority", 3, "critical"},
		{"width 4 with priority", 4, "high"},
		{"width 5 with priority", 5, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("RenderIssueRow panicked with MaxTitleWidth=%d and priority=%s: %v",
						tt.maxTitleWidth, tt.priority, r)
				}
			}()

			cfg := IssueRowConfig{
				MaxTitleWidth: tt.maxTitleWidth,
				Priority:      tt.priority,
				PriorityColor: "red",
				StatusColor:   "green",
				TypeColor:     "blue",
			}

			result := RenderIssueRow("abc123", "todo", "task", "Long title that needs truncation", cfg)
			if result == "" {
				t.Error("expected non-empty result")
			}
		})
	}
}

func TestIsValidColor(t *testing.T) {
	tests := []struct {
		name  string
		color string
		want  bool
	}{
		{"valid hex #RGB", "#f00", true},
		{"valid hex #RRGGBB", "#ff0000", true},
		{"too short hex", "#ff", false},
		{"wrong length hex 4 chars after #", "#ff00", false},
		{"too long hex", "#ff00000", false},
		{"named red", "red", true},
		{"named green", "green", true},
		{"named cyan", "cyan", true},
		{"named uppercase RED", "RED", true},
		{"unknown name", "unknown", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidColor(tt.color)
			if got != tt.want {
				t.Errorf("IsValidColor(%q) = %v, want %v", tt.color, got, tt.want)
			}
		})
	}
}

func TestGetPrioritySymbol(t *testing.T) {
	tests := []struct {
		name     string
		priority string
		want     string
	}{
		{"critical", config.PriorityCritical, "‼"},
		{"high", config.PriorityHigh, "!"},
		{"low", config.PriorityLow, "↓"},
		{"deferred", config.PriorityDeferred, "→"},
		{"normal", config.PriorityNormal, ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPrioritySymbol(tt.priority)
			if got != tt.want {
				t.Errorf("GetPrioritySymbol(%q) = %q, want %q", tt.priority, got, tt.want)
			}
		})
	}
}

func TestCalculateResponsiveColumns(t *testing.T) {
	tests := []struct {
		name       string
		totalWidth int
		hasTags    bool
		wantID     int
		wantStatus int
		wantType   int
		wantShow   bool
		checkMax   bool // whether to check MaxTags > 0
		wantMaxGE  int  // MaxTags should be >= this value (when checkMax is true)
	}{
		{
			name:       "narrow with tags",
			totalWidth: 80,
			hasTags:    true,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   false,
		},
		{
			name:       "narrow without tags",
			totalWidth: 80,
			hasTags:    false,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   false,
		},
		{
			name:       "at boundary 139 with tags",
			totalWidth: 139,
			hasTags:    true,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   false,
		},
		{
			name:       "at boundary 140 with tags",
			totalWidth: 140,
			hasTags:    true,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   true,
			checkMax:   true,
			wantMaxGE:  1,
		},
		{
			name:       "wide 200 with tags",
			totalWidth: 200,
			hasTags:    true,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   true,
			checkMax:   true,
			wantMaxGE:  1,
		},
		{
			name:       "very wide 250 with tags",
			totalWidth: 250,
			hasTags:    true,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   true,
			checkMax:   true,
			wantMaxGE:  3,
		},
		{
			name:       "wide 200 without tags",
			totalWidth: 200,
			hasTags:    false,
			wantID:     ColWidthID,
			wantStatus: ColWidthStatus,
			wantType:   ColWidthType,
			wantShow:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateResponsiveColumns(tt.totalWidth, tt.hasTags)

			if got.ID != tt.wantID {
				t.Errorf("ID = %d, want %d", got.ID, tt.wantID)
			}
			if got.Status != tt.wantStatus {
				t.Errorf("Status = %d, want %d", got.Status, tt.wantStatus)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %d, want %d", got.Type, tt.wantType)
			}
			if got.ShowTags != tt.wantShow {
				t.Errorf("ShowTags = %v, want %v", got.ShowTags, tt.wantShow)
			}
			if tt.checkMax && got.MaxTags < tt.wantMaxGE {
				t.Errorf("MaxTags = %d, want >= %d", got.MaxTags, tt.wantMaxGE)
			}
			if !tt.wantShow && got.MaxTags != 0 {
				t.Errorf("MaxTags = %d, want 0 when ShowTags is false", got.MaxTags)
			}
		})
	}
}
