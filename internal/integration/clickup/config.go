package clickup

import (
	"fmt"
)

// Sync metadata constants
const (
	SyncName       = "clickup"
	SyncKeyTaskID  = "task_id"
	SyncKeySyncedAt = "synced_at"
)

// Config holds ClickUp-specific settings parsed from cfg.SyncConfig("clickup").
type Config struct {
	ListID          string
	Assignee        *int
	StatusMapping   map[string]string
	PriorityMapping map[string]int
	TypeMapping     map[string]int
	CustomFields    *CustomFieldsMap
	SyncFilter      *SyncFilter
}

// CustomFieldsMap maps issue fields to ClickUp custom field UUIDs.
type CustomFieldsMap struct {
	IssueID   string
	CreatedAt string
	UpdatedAt string
}

// SyncFilter defines which issues to sync.
type SyncFilter struct {
	ExcludeStatus []string
}

// DefaultStatusMapping provides standard issue status → ClickUp status mapping.
var DefaultStatusMapping = map[string]string{
	"draft":       "backlog",
	"ready":       "to do",
	"in-progress": "in progress",
	"completed":   "complete",
	"scrapped":    "closed",
}

// DefaultPriorityMapping provides standard issue priority → ClickUp priority mapping.
// ClickUp priorities: 1=Urgent, 2=High, 3=Normal, 4=Low
var DefaultPriorityMapping = map[string]int{
	"critical": 1,
	"high":     2,
	"normal":   3,
	"low":      4,
	"deferred": 4,
}

// ParseConfig parses ClickUp configuration from a map[string]any (from cfg.SyncConfig("clickup")).
// Returns nil if the map is nil or has no list_id.
func ParseConfig(m map[string]any) (*Config, error) {
	if m == nil {
		return nil, nil
	}

	listID, _ := m["list_id"].(string)
	if listID == "" {
		return nil, nil
	}

	cfg := &Config{
		ListID:          listID,
		StatusMapping:   DefaultStatusMapping,
		PriorityMapping: DefaultPriorityMapping,
	}

	// Parse assignee
	if v, ok := m["assignee"]; ok {
		switch a := v.(type) {
		case int:
			cfg.Assignee = &a
		case float64:
			i := int(a)
			cfg.Assignee = &i
		}
	}

	// Parse status_mapping
	if v, ok := m["status_mapping"]; ok {
		if sm, ok := v.(map[string]any); ok {
			mapping := make(map[string]string, len(sm))
			for k, val := range sm {
				if s, ok := val.(string); ok {
					mapping[k] = s
				}
			}
			if len(mapping) > 0 {
				cfg.StatusMapping = mapping
			}
		}
	}

	// Parse priority_mapping
	if v, ok := m["priority_mapping"]; ok {
		if pm, ok := v.(map[string]any); ok {
			mapping := make(map[string]int, len(pm))
			for k, val := range pm {
				switch n := val.(type) {
				case int:
					mapping[k] = n
				case float64:
					mapping[k] = int(n)
				}
			}
			if len(mapping) > 0 {
				cfg.PriorityMapping = mapping
			}
		}
	}

	// Parse type_mapping
	if v, ok := m["type_mapping"]; ok {
		if tm, ok := v.(map[string]any); ok {
			mapping := make(map[string]int, len(tm))
			for k, val := range tm {
				switch n := val.(type) {
				case int:
					mapping[k] = n
				case float64:
					mapping[k] = int(n)
				}
			}
			if len(mapping) > 0 {
				cfg.TypeMapping = mapping
			}
		}
	}

	// Parse custom_fields
	if v, ok := m["custom_fields"]; ok {
		if cf, ok := v.(map[string]any); ok {
			fields := &CustomFieldsMap{}
			fields.IssueID, _ = cf["issue_id"].(string)
			fields.CreatedAt, _ = cf["created_at"].(string)
			fields.UpdatedAt, _ = cf["updated_at"].(string)
			if fields.IssueID != "" || fields.CreatedAt != "" || fields.UpdatedAt != "" {
				cfg.CustomFields = fields
			}
		}
	}

	// Parse sync_filter
	if v, ok := m["sync_filter"]; ok {
		if sf, ok := v.(map[string]any); ok {
			filter := &SyncFilter{}
			if es, ok := sf["exclude_status"]; ok {
				switch s := es.(type) {
				case []any:
					for _, item := range s {
						if str, ok := item.(string); ok {
							filter.ExcludeStatus = append(filter.ExcludeStatus, str)
						}
					}
				}
			}
			if len(filter.ExcludeStatus) > 0 {
				cfg.SyncFilter = filter
			}
		}
	}

	return cfg, nil
}

// GetStatusMapping returns the effective status mapping.
func (c *Config) GetStatusMapping() map[string]string {
	if c.StatusMapping != nil {
		return c.StatusMapping
	}
	return DefaultStatusMapping
}

// GetPriorityMapping returns the effective priority mapping.
func (c *Config) GetPriorityMapping() map[string]int {
	if c.PriorityMapping != nil {
		return c.PriorityMapping
	}
	return DefaultPriorityMapping
}

// Validate checks the config for issues.
func (c *Config) Validate() error {
	if c.ListID == "" {
		return fmt.Errorf("list_id is required")
	}
	return nil
}
