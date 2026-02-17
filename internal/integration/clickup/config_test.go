package clickup

import (
	"testing"
)

func TestParseConfig_Nil(t *testing.T) {
	cfg, err := ParseConfig(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for nil input")
	}
}

func TestParseConfig_NoListID(t *testing.T) {
	cfg, err := ParseConfig(map[string]any{
		"assignee": 123,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config when list_id is missing")
	}
}

func TestParseConfig_MinimalConfig(t *testing.T) {
	cfg, err := ParseConfig(map[string]any{
		"list_id": "901234567890",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.ListID != "901234567890" {
		t.Errorf("ListID = %q, want %q", cfg.ListID, "901234567890")
	}
	// Should have default mappings
	if cfg.StatusMapping == nil {
		t.Fatal("expected default status mapping")
	}
	if cfg.StatusMapping["ready"] != "to do" {
		t.Errorf("StatusMapping[ready] = %q, want %q", cfg.StatusMapping["ready"], "to do")
	}
	if cfg.PriorityMapping == nil {
		t.Fatal("expected default priority mapping")
	}
	if cfg.PriorityMapping["critical"] != 1 {
		t.Errorf("PriorityMapping[critical] = %d, want 1", cfg.PriorityMapping["critical"])
	}
}

func TestParseConfig_FullConfig(t *testing.T) {
	cfg, err := ParseConfig(map[string]any{
		"list_id":  "901234567890",
		"assignee": float64(42),
		"status_mapping": map[string]any{
			"draft": "backlog",
			"ready": "open",
		},
		"priority_mapping": map[string]any{
			"critical": float64(1),
			"high":     float64(2),
		},
		"type_mapping": map[string]any{
			"bug":     float64(100),
			"feature": float64(200),
		},
		"custom_fields": map[string]any{
			"issue_id":   "cf-1",
			"created_at": "cf-2",
			"updated_at": "cf-3",
		},
		"sync_filter": map[string]any{
			"exclude_status": []any{"completed", "scrapped"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	// Assignee
	if cfg.Assignee == nil || *cfg.Assignee != 42 {
		t.Errorf("Assignee = %v, want 42", cfg.Assignee)
	}

	// Status mapping (custom overrides default)
	if cfg.StatusMapping["ready"] != "open" {
		t.Errorf("StatusMapping[ready] = %q, want %q", cfg.StatusMapping["ready"], "open")
	}
	if cfg.StatusMapping["draft"] != "backlog" {
		t.Errorf("StatusMapping[draft] = %q, want %q", cfg.StatusMapping["draft"], "backlog")
	}

	// Priority mapping (custom overrides default)
	if cfg.PriorityMapping["critical"] != 1 {
		t.Errorf("PriorityMapping[critical] = %d, want 1", cfg.PriorityMapping["critical"])
	}

	// Type mapping
	if cfg.TypeMapping["bug"] != 100 {
		t.Errorf("TypeMapping[bug] = %d, want 100", cfg.TypeMapping["bug"])
	}
	if cfg.TypeMapping["feature"] != 200 {
		t.Errorf("TypeMapping[feature] = %d, want 200", cfg.TypeMapping["feature"])
	}

	// Custom fields
	if cfg.CustomFields == nil {
		t.Fatal("expected non-nil custom fields")
	}
	if cfg.CustomFields.IssueID != "cf-1" {
		t.Errorf("CustomFields.IssueID = %q, want %q", cfg.CustomFields.IssueID, "cf-1")
	}
	if cfg.CustomFields.CreatedAt != "cf-2" {
		t.Errorf("CustomFields.CreatedAt = %q, want %q", cfg.CustomFields.CreatedAt, "cf-2")
	}
	if cfg.CustomFields.UpdatedAt != "cf-3" {
		t.Errorf("CustomFields.UpdatedAt = %q, want %q", cfg.CustomFields.UpdatedAt, "cf-3")
	}

	// Sync filter
	if cfg.SyncFilter == nil {
		t.Fatal("expected non-nil sync filter")
	}
	if len(cfg.SyncFilter.ExcludeStatus) != 2 {
		t.Fatalf("SyncFilter.ExcludeStatus len = %d, want 2", len(cfg.SyncFilter.ExcludeStatus))
	}
	if cfg.SyncFilter.ExcludeStatus[0] != "completed" {
		t.Errorf("SyncFilter.ExcludeStatus[0] = %q, want %q", cfg.SyncFilter.ExcludeStatus[0], "completed")
	}
}

func TestParseConfig_IntAssignee(t *testing.T) {
	// YAML sometimes deserializes integers as int, not float64
	cfg, err := ParseConfig(map[string]any{
		"list_id":  "123",
		"assignee": 99,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Assignee == nil || *cfg.Assignee != 99 {
		t.Errorf("Assignee = %v, want 99", cfg.Assignee)
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := &Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for empty list_id")
	}

	cfg.ListID = "123"
	if err := cfg.Validate(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfig_GetStatusMapping_Default(t *testing.T) {
	cfg := &Config{}
	m := cfg.GetStatusMapping()
	if m["ready"] != "to do" {
		t.Errorf("GetStatusMapping()[ready] = %q, want %q", m["ready"], "to do")
	}
}

func TestConfig_GetPriorityMapping_Default(t *testing.T) {
	cfg := &Config{}
	m := cfg.GetPriorityMapping()
	if m["critical"] != 1 {
		t.Errorf("GetPriorityMapping()[critical] = %d, want 1", m["critical"])
	}
}
