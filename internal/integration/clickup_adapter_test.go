package integration

import (
	"testing"

	"github.com/toba/todo/internal/config"
	"github.com/toba/todo/internal/core"
	"github.com/toba/todo/internal/integration/clickup"
)

func TestCheckMappingKeys(t *testing.T) {
	cfg := config.Default()
	c := core.New(t.TempDir(), cfg)

	tests := []struct {
		name           string
		clickupCfg     *clickup.Config
		wantFail       bool   // expect a CheckFail result
		wantWarn       bool   // expect a CheckWarn result
		wantFailSubstr string // substring in fail message
		wantWarnSubstr string // substring in warn message
	}{
		{
			name: "valid status mapping with full coverage",
			clickupCfg: &clickup.Config{
				ListID: "123",
				StatusMapping: map[string]string{
					"draft":       "backlog",
					"ready":       "to do",
					"in-progress": "in progress",
					"completed":   "complete",
					"scrapped":    "closed",
				},
				PriorityMapping: clickup.DefaultPriorityMapping,
			},
			wantFail: false,
			wantWarn: false,
		},
		{
			name: "unknown status key in mapping",
			clickupCfg: &clickup.Config{
				ListID: "123",
				StatusMapping: map[string]string{
					"todo":        "not started", // "todo" is not a valid status
					"draft":       "not started",
					"in-progress": "in progress",
					"completed":   "completed",
					"scrapped":    "completed",
				},
				PriorityMapping: clickup.DefaultPriorityMapping,
			},
			wantFail:       true,
			wantFailSubstr: "todo",
			wantWarn:       true,        // "ready" is unmapped
			wantWarnSubstr: "ready",
		},
		{
			name: "unmapped status",
			clickupCfg: &clickup.Config{
				ListID: "123",
				StatusMapping: map[string]string{
					"draft":       "not started",
					"in-progress": "in progress",
					"completed":   "completed",
					"scrapped":    "completed",
					// missing "ready"
				},
				PriorityMapping: clickup.DefaultPriorityMapping,
			},
			wantWarn:       true,
			wantWarnSubstr: "ready",
		},
		{
			name: "unknown type key in mapping",
			clickupCfg: &clickup.Config{
				ListID: "123",
				StatusMapping: map[string]string{
					"draft":       "backlog",
					"ready":       "to do",
					"in-progress": "in progress",
					"completed":   "complete",
					"scrapped":    "closed",
				},
				TypeMapping: map[string]int{
					"bug":         1001,
					"nonexistent": 1002, // invalid type
				},
				PriorityMapping: clickup.DefaultPriorityMapping,
			},
			wantWarn:       true,
			wantWarnSubstr: "nonexistent",
		},
		{
			name: "unknown priority key in mapping",
			clickupCfg: &clickup.Config{
				ListID: "123",
				StatusMapping: map[string]string{
					"draft":       "backlog",
					"ready":       "to do",
					"in-progress": "in progress",
					"completed":   "complete",
					"scrapped":    "closed",
				},
				PriorityMapping: map[string]int{
					"critical": 1,
					"urgent":   1, // not a valid priority
				},
			},
			wantWarn:       true,
			wantWarnSubstr: "urgent",
		},
		{
			name: "invalid sync filter exclude_status",
			clickupCfg: &clickup.Config{
				ListID: "123",
				StatusMapping: map[string]string{
					"draft":       "backlog",
					"ready":       "to do",
					"in-progress": "in progress",
					"completed":   "complete",
					"scrapped":    "closed",
				},
				PriorityMapping: clickup.DefaultPriorityMapping,
				SyncFilter: &clickup.SyncFilter{
					ExcludeStatus: []string{"completed", "todo"}, // "todo" is invalid
				},
			},
			wantWarn:       true,
			wantWarnSubstr: "todo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cu := &clickUpIntegration{cfg: tt.clickupCfg, core: c}
			results := cu.checkMappingKeys()

			var hasFail, hasWarn bool
			var failMsg, warnMsg string
			for _, r := range results {
				switch r.Status {
				case CheckFail:
					hasFail = true
					failMsg = r.Message
				case CheckWarn:
					hasWarn = true
					warnMsg = r.Message
				}
			}

			if tt.wantFail && !hasFail {
				t.Errorf("expected a CheckFail result but got none; results: %+v", results)
			}
			if !tt.wantFail && hasFail {
				t.Errorf("unexpected CheckFail: %s", failMsg)
			}
			if tt.wantWarn && !hasWarn {
				t.Errorf("expected a CheckWarn result but got none; results: %+v", results)
			}
			if !tt.wantWarn && hasWarn {
				t.Errorf("unexpected CheckWarn: %s", warnMsg)
			}
			if tt.wantFailSubstr != "" && hasFail {
				if !contains(failMsg, tt.wantFailSubstr) {
					t.Errorf("fail message %q does not contain %q", failMsg, tt.wantFailSubstr)
				}
			}
			if tt.wantWarnSubstr != "" && hasWarn {
				if !contains(warnMsg, tt.wantWarnSubstr) {
					t.Errorf("warn message %q does not contain %q", warnMsg, tt.wantWarnSubstr)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
