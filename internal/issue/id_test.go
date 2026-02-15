package issue

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"basic spaces", "Hello World", "hello-world"},
		{"underscores", "hello_world", "hello-world"},
		{"special chars", "Hello! World?", "hello-world"},
		{"multiple spaces", "hello   world", "hello-world"},
		{"multiple dashes", "hello---world", "hello-world"},
		{"leading trailing dashes", "--hello--", "hello"},
		{"empty string", "", ""},
		{"numbers", "Test 123", "test-123"},
		{"mixed special chars", "Hello, World! How's it going?", "hello-world-hows-it-going"},
		{"only special chars", "!@#$%^&*()", ""},
		{"unicode letters", "Café Résumé", "café-résumé"},
		{"already lowercase", "already-slugified", "already-slugified"},
		{"all caps", "ALL CAPS", "all-caps"},
		{"spaces and underscores mixed", "hello world_test", "hello-world-test"},
		{
			"truncation at 50 chars",
			"this is a very long title that should be truncated to fifty characters",
			"this-is-a-very-long-title-that-should-be-truncated",
		},
		{
			"truncation removes trailing dash",
			"this is a very long title that should be truncated-at dash",
			"this-is-a-very-long-title-that-should-be-truncated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseFilename(t *testing.T) {
	tests := []struct {
		name         string
		filename     string
		expectedID   string
		expectedSlug string
	}{
		// New format with double-dash
		{"new format basic", "abc--my-slug.md", "abc", "my-slug"},
		{"new format with prefix", "todo-z5r9--add-unit-tests.md", "todo-z5r9", "add-unit-tests"},
		{"new format long slug", "xyz--this-is-a-longer-slug.md", "xyz", "this-is-a-longer-slug"},

		// Dot format
		{"dot format basic", "abc.my-slug.md", "abc", "my-slug"},
		{"dot format with prefix", "todo-z5r9.add-unit-tests.md", "todo-z5r9", "add-unit-tests"},

		// Legacy format with single dash
		{"legacy format basic", "abc-my-slug.md", "abc", "my-slug"},
		{"legacy format multi-part slug", "abc-my-multi-part-slug.md", "abc", "my-multi-part-slug"},

		// ID only
		{"id only with md", "abc.md", "abc", ""},
		{"id only with prefix", "todo-z5r9.md", "todo", "z5r9"}, // legacy format interpretation
		{"id only no extension", "abc", "abc", ""},

		// Edge cases
		{"empty string", "", "", ""},
		{"just md extension", ".md", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotID, gotSlug := ParseFilename(tt.filename)
			if gotID != tt.expectedID || gotSlug != tt.expectedSlug {
				t.Errorf("ParseFilename(%q) = (%q, %q), want (%q, %q)",
					tt.filename, gotID, gotSlug, tt.expectedID, tt.expectedSlug)
			}
		})
	}
}

func TestBuildFilename(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		slug     string
		expected string
	}{
		{"with slug", "abc", "my-slug", "abc--my-slug.md"},
		{"empty slug", "abc", "", "abc.md"},
		{"with prefix id", "todo-z5r9", "add-tests", "todo-z5r9--add-tests.md"},
		{"long slug", "xyz", "this-is-a-longer-slug", "xyz--this-is-a-longer-slug.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildFilename(tt.id, tt.slug)
			if got != tt.expected {
				t.Errorf("BuildFilename(%q, %q) = %q, want %q",
					tt.id, tt.slug, got, tt.expected)
			}
		})
	}
}

func TestNewID(t *testing.T) {
	t.Run("format is xxx-xxx", func(t *testing.T) {
		id := NewID()
		if len(id) != 7 {
			t.Errorf("NewID() length = %d, want 7", len(id))
		}
		if id[3] != '-' {
			t.Errorf("NewID() = %q, want hyphen at position 3", id)
		}
	})

	t.Run("uses valid alphabet", func(t *testing.T) {
		for range 100 {
			id := NewID()
			for i, r := range id {
				if i == 3 {
					continue // skip the hyphen
				}
				if !strings.ContainsRune(idAlphabet, r) {
					t.Errorf("NewID contains invalid character %q at position %d, should only use %q", r, i, idAlphabet)
				}
			}
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		seen := make(map[string]bool)
		for range 100 {
			id := NewID()
			if seen[id] {
				t.Errorf("NewID generated duplicate: %q", id)
			}
			seen[id] = true
		}
	})
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		slug     string
		expected string
	}{
		{"with slug", "abc-def", "my-slug", "a/abc-def--my-slug.md"},
		{"empty slug", "abc-def", "", "a/abc-def.md"},
		{"numeric prefix", "9xy-abc", "test", "9/9xy-abc--test.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPath(tt.id, tt.slug)
			if got != tt.expected {
				t.Errorf("BuildPath(%q, %q) = %q, want %q",
					tt.id, tt.slug, got, tt.expected)
			}
		})
	}
}

func TestParseFilenameAndBuildFilenameRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		id   string
		slug string
	}{
		{"basic", "abc-def", "my-slug"},
		{"long slug", "xyz-123", "this-is-a-longer-slug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := BuildFilename(tt.id, tt.slug)
			gotID, gotSlug := ParseFilename(filename)
			if gotID != tt.id || gotSlug != tt.slug {
				t.Errorf("Roundtrip failed: BuildFilename(%q, %q) = %q, ParseFilename = (%q, %q)",
					tt.id, tt.slug, filename, gotID, gotSlug)
			}
		})
	}
}
