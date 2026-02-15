package tui

import (
	"testing"

	"github.com/toba/todo/internal/issue"
)

func TestStripAnsi(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain string unchanged",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "red ANSI escape stripped",
			input: "\x1b[31mhello\x1b[0m",
			want:  "hello",
		},
		{
			name:  "bold ANSI escape stripped",
			input: "\x1b[1mbold text\x1b[0m",
			want:  "bold text",
		},
		{
			name:  "reset ANSI escape stripped",
			input: "before\x1b[0mafter",
			want:  "beforeafter",
		},
		{
			name:  "mixed escapes with surrounding text",
			input: "\x1b[31mhello\x1b[0m world",
			want:  "hello world",
		},
		{
			name:  "multiple escapes in sequence",
			input: "\x1b[1m\x1b[31m\x1b[4mstyledtext\x1b[0m",
			want:  "styledtext",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripAnsi(tt.input)
			if got != tt.want {
				t.Errorf("stripAnsi(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIntersectStrings(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want []string
	}{
		{
			name: "both empty",
			a:    []string{},
			b:    []string{},
			want: nil,
		},
		{
			name: "first empty",
			a:    []string{},
			b:    []string{"x", "y"},
			want: nil,
		},
		{
			name: "second empty",
			a:    []string{"x", "y"},
			b:    []string{},
			want: nil,
		},
		{
			name: "no overlap",
			a:    []string{"a", "b"},
			b:    []string{"c", "d"},
			want: nil,
		},
		{
			name: "full overlap",
			a:    []string{"a", "b", "c"},
			b:    []string{"c", "b", "a"},
			want: []string{"c", "b", "a"},
		},
		{
			name: "partial overlap",
			a:    []string{"a", "b", "c"},
			b:    []string{"b", "d", "c"},
			want: []string{"b", "c"},
		},
		{
			name: "order follows b slice",
			a:    []string{"z", "y", "x"},
			b:    []string{"x", "z"},
			want: []string{"x", "z"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := intersectStrings(tt.a, tt.b)
			if len(got) != len(tt.want) {
				t.Fatalf("intersectStrings(%v, %v) returned %v (len %d), want %v (len %d)",
					tt.a, tt.b, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("intersectStrings(%v, %v)[%d] = %q, want %q",
						tt.a, tt.b, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestCollectDescendants(t *testing.T) {
	tests := []struct {
		name     string
		issueID   string
		allIssues []*issue.Issue
		want     map[string]bool
	}{
		{
			name:   "no children",
			issueID: "root",
			allIssues: []*issue.Issue{
				{ID: "root"},
				{ID: "other", Parent: "someone-else"},
			},
			want: map[string]bool{},
		},
		{
			name:   "direct children only",
			issueID: "root",
			allIssues: []*issue.Issue{
				{ID: "root"},
				{ID: "child1", Parent: "root"},
				{ID: "child2", Parent: "root"},
			},
			want: map[string]bool{
				"child1": true,
				"child2": true,
			},
		},
		{
			name:   "grandchildren included",
			issueID: "root",
			allIssues: []*issue.Issue{
				{ID: "root"},
				{ID: "child1", Parent: "root"},
				{ID: "grandchild1", Parent: "child1"},
				{ID: "grandchild2", Parent: "child1"},
			},
			want: map[string]bool{
				"child1":      true,
				"grandchild1": true,
				"grandchild2": true,
			},
		},
		{
			name:   "no matching parent",
			issueID: "nonexistent",
			allIssues: []*issue.Issue{
				{ID: "a", Parent: "b"},
				{ID: "b"},
			},
			want: map[string]bool{},
		},
		{
			name:     "empty issue list",
			issueID:   "root",
			allIssues: []*issue.Issue{},
			want:     map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectDescendants(tt.issueID, tt.allIssues)
			if len(got) != len(tt.want) {
				t.Fatalf("collectDescendants(%q, ...) returned %v (len %d), want %v (len %d)",
					tt.issueID, got, len(got), tt.want, len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("collectDescendants(%q, ...)[%q] = %v, want %v",
						tt.issueID, k, got[k], v)
				}
			}
		})
	}
}
