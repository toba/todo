package clickup

import (
	"errors"
	"testing"
	"time"

	"github.com/toba/todo/internal/issue"
)

func TestIsTransientNetworkError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"stream error", errors.New("stream error"), true},
		{"INTERNAL_ERROR", errors.New("INTERNAL_ERROR"), true},
		{"connection reset", errors.New("connection reset"), true},
		{"connection refused", errors.New("connection refused"), true},
		{"EOF", errors.New("EOF"), true},
		{"timeout", errors.New("timeout"), true},
		{"Timeout", errors.New("Timeout"), true},
		{"some other error", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientNetworkError(tt.err)
			if got != tt.want {
				t.Errorf("isTransientNetworkError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsTransientHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       bool
	}{
		{"500 empty body", 500, []byte{}, true},
		{"502 empty body", 502, []byte{}, true},
		{"503 empty body", 503, []byte{}, true},
		{"504 empty body", 504, []byte{}, true},
		{"400 with CloudFront", 400, []byte("CloudFront error"), true},
		{"400 with cloudfront", 400, []byte("cloudfront error"), true},
		{"400 with try again", 400, []byte("please try again later"), true},
		{"400 with Try again", 400, []byte("Try again later"), true},
		{"400 with bad request", 400, []byte("bad request"), false},
		{"200 empty body", 200, []byte{}, false},
		{"429 empty body", 429, []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTransientHTTPError(tt.statusCode, tt.body)
			if got != tt.want {
				t.Errorf("isTransientHTTPError(%d, %q) = %v, want %v", tt.statusCode, tt.body, got, tt.want)
			}
		})
	}
}

func TestIntPtrEqual(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name string
		a    *int
		b    *int
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil b non-nil", nil, intPtr(1), false},
		{"a non-nil b nil", intPtr(1), nil, false},
		{"both equal", intPtr(42), intPtr(42), true},
		{"both different", intPtr(1), intPtr(2), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intPtrEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("intPtrEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestInt64PtrEqual(t *testing.T) {
	int64Ptr := func(v int64) *int64 { return &v }

	tests := []struct {
		name string
		a    *int64
		b    *int64
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil b non-nil", nil, int64Ptr(1), false},
		{"a non-nil b nil", int64Ptr(1), nil, false},
		{"both equal", int64Ptr(42), int64Ptr(42), true},
		{"both different", int64Ptr(1), int64Ptr(2), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := int64PtrEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("int64PtrEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCustomFieldDateEqual(t *testing.T) {
	tests := []struct {
		name    string
		current any
		target  int64
		want    bool
	}{
		{"nil current", nil, 1700000000000, false},
		{"string matching", "1700000000000", 1700000000000, true},
		{"string not matching", "1700000000000", 1700000000001, false},
		{"float64 matching", float64(1700000000000), 1700000000000, true},
		{"float64 not matching", float64(1700000000000), 1700000000001, false},
		{"int64 matching", int64(1700000000000), 1700000000000, true},
		{"int64 not matching", int64(1700000000000), 1700000000001, false},
		{"unsupported type bool", true, 1700000000000, false},
		{"unparseable string", "not-a-number", 1700000000000, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := customFieldDateEqual(tt.current, tt.target)
			if got != tt.want {
				t.Errorf("customFieldDateEqual(%v, %d) = %v, want %v", tt.current, tt.target, got, tt.want)
			}
		})
	}
}

func TestPriorityEqual(t *testing.T) {
	intPtr := func(v int) *int { return &v }
	s := &Syncer{}

	tests := []struct {
		name    string
		current *TaskPriority
		target  *int
		want    bool
	}{
		{"both nil", nil, nil, true},
		{"current nil target non-nil", nil, intPtr(1), false},
		{"current non-nil target nil", &TaskPriority{ID: 1}, nil, false},
		{"matching IDs", &TaskPriority{ID: 3}, intPtr(3), true},
		{"different IDs", &TaskPriority{ID: 3}, intPtr(4), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.priorityEqual(tt.current, tt.target)
			if got != tt.want {
				t.Errorf("priorityEqual(%v, %v) = %v, want %v", tt.current, tt.target, got, tt.want)
			}
		})
	}
}

func TestFilterIssuesNeedingSync(t *testing.T) {
	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	tests := []struct {
		name     string
		issues   []*issue.Issue
		setup    func(*memorySyncProvider)
		force    bool
		wantIDs  []string
	}{
		{
			name:    "empty list",
			issues:  nil,
			setup:   func(_ *memorySyncProvider) {},
			force:   false,
			wantIDs: nil,
		},
		{
			name: "force returns all",
			issues: []*issue.Issue{
				{ID: "a", UpdatedAt: &now},
				{ID: "b", UpdatedAt: &now},
			},
			setup: func(s *memorySyncProvider) {
				s.SetSyncedAt("a", now)
				s.SetSyncedAt("b", now)
			},
			force:   true,
			wantIDs: []string{"a", "b"},
		},
		{
			name: "never synced included",
			issues: []*issue.Issue{
				{ID: "new-issue", UpdatedAt: &now},
			},
			setup:   func(_ *memorySyncProvider) {},
			force:   false,
			wantIDs: []string{"new-issue"},
		},
		{
			name: "synced but updated after included",
			issues: []*issue.Issue{
				{ID: "updated", UpdatedAt: &now},
			},
			setup: func(s *memorySyncProvider) {
				s.SetSyncedAt("updated", hourAgo)
			},
			force:   false,
			wantIDs: []string{"updated"},
		},
		{
			name: "synced and not updated since excluded",
			issues: []*issue.Issue{
				{ID: "up-to-date", UpdatedAt: &twoHoursAgo},
			},
			setup: func(s *memorySyncProvider) {
				s.SetSyncedAt("up-to-date", hourAgo)
			},
			force:   false,
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemorySyncProvider()
			tt.setup(store)

			got := FilterIssuesNeedingSync(tt.issues, store, tt.force)

			var gotIDs []string
			for _, b := range got {
				gotIDs = append(gotIDs, b.ID)
			}

			if len(gotIDs) == 0 && len(tt.wantIDs) == 0 {
				return
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("got %d issues %v, want %d issues %v", len(gotIDs), gotIDs, len(tt.wantIDs), tt.wantIDs)
			}
			for i := range gotIDs {
				if gotIDs[i] != tt.wantIDs[i] {
					t.Errorf("issue[%d] = %q, want %q", i, gotIDs[i], tt.wantIDs[i])
				}
			}
		})
	}
}
