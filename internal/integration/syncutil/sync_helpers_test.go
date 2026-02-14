package syncutil

import (
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
)

func TestGetSyncString(t *testing.T) {
	tests := []struct {
		name     string
		issue    *issue.Issue
		syncName string
		key      string
		want     string
	}{
		{
			name:     "nil Sync map returns empty string",
			issue:    &issue.Issue{Sync: nil},
			syncName: "clickup",
			key:      "id",
			want:     "",
		},
		{
			name: "missing syncName returns empty string",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"github": {"id": "123"},
				},
			},
			syncName: "clickup",
			key:      "id",
			want:     "",
		},
		{
			name: "missing key returns empty string",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"id": "123"},
				},
			},
			syncName: "clickup",
			key:      "url",
			want:     "",
		},
		{
			name: "existing key with string value returns string",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"id": "abc-123"},
				},
			},
			syncName: "clickup",
			key:      "id",
			want:     "abc-123",
		},
		{
			name: "existing key with non-string value returns empty string",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"count": 42},
				},
			},
			syncName: "clickup",
			key:      "count",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSyncString(tt.issue, tt.syncName, tt.key)
			if got != tt.want {
				t.Errorf("GetSyncString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetSyncTime(t *testing.T) {
	validTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	validRFC3339 := validTime.Format(time.RFC3339)

	tests := []struct {
		name     string
		issue    *issue.Issue
		syncName string
		key      string
		wantNil  bool
		wantTime time.Time
	}{
		{
			name:     "nil Sync map returns nil",
			issue:    &issue.Issue{Sync: nil},
			syncName: "clickup",
			key:      "synced_at",
			wantNil:  true,
		},
		{
			name: "missing key returns nil",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"id": "123"},
				},
			},
			syncName: "clickup",
			key:      "synced_at",
			wantNil:  true,
		},
		{
			name: "valid RFC3339 string returns parsed time",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"synced_at": validRFC3339},
				},
			},
			syncName: "clickup",
			key:      "synced_at",
			wantNil:  false,
			wantTime: validTime,
		},
		{
			name: "invalid time string returns nil",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"synced_at": "not-a-time"},
				},
			},
			syncName: "clickup",
			key:      "synced_at",
			wantNil:  true,
		},
		{
			name: "empty string value returns nil",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"synced_at": ""},
				},
			},
			syncName: "clickup",
			key:      "synced_at",
			wantNil:  true,
		},
		{
			name: "non-string value returns nil",
			issue: &issue.Issue{
				Sync: map[string]map[string]any{
					"clickup": {"synced_at": 12345},
				},
			},
			syncName: "clickup",
			key:      "synced_at",
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetSyncTime(tt.issue, tt.syncName, tt.key)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetSyncTime() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("GetSyncTime() = nil, want non-nil time")
			}
			if !got.Equal(tt.wantTime) {
				t.Errorf("GetSyncTime() = %v, want %v", got, tt.wantTime)
			}
		})
	}
}
