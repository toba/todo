package syncutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindLocalImages(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []ImageRef
	}{
		{
			name: "markdown image with absolute path",
			body: "Here is a screenshot: ![screenshot](/Users/jason/Desktop/image.png)",
			want: []ImageRef{
				{FullMatch: "![screenshot](/Users/jason/Desktop/image.png)", AltText: "screenshot", LocalPath: "/Users/jason/Desktop/image.png", Format: "markdown"},
			},
		},
		{
			name: "markdown image with URL is skipped",
			body: "![logo](https://example.com/logo.png)",
			want: nil,
		},
		{
			name: "markdown image with relative path is skipped",
			body: "![pic](images/pic.png)",
			want: nil,
		},
		{
			name: "custom format with arrow",
			body: "Image: Screenshot → /tmp/screenshot.png",
			want: []ImageRef{
				{FullMatch: "Image: Screenshot → /tmp/screenshot.png", AltText: "Screenshot", LocalPath: "/tmp/screenshot.png", Format: "custom"},
			},
		},
		{
			name: "custom format with ascii arrow",
			body: "Image: My Image -> /home/user/photo.jpg",
			want: []ImageRef{
				{FullMatch: "Image: My Image -> /home/user/photo.jpg", AltText: "My Image", LocalPath: "/home/user/photo.jpg", Format: "custom"},
			},
		},
		{
			name: "multiple images mixed formats",
			body: "![a](/path/a.png)\nSome text\nImage: B → /path/b.jpg\n![c](/path/c.gif)",
			want: []ImageRef{
				{FullMatch: "![a](/path/a.png)", AltText: "a", LocalPath: "/path/a.png", Format: "markdown"},
				{FullMatch: "![c](/path/c.gif)", AltText: "c", LocalPath: "/path/c.gif", Format: "markdown"},
				{FullMatch: "Image: B → /path/b.jpg", AltText: "B", LocalPath: "/path/b.jpg", Format: "custom"},
			},
		},
		{
			name: "no images",
			body: "Just some plain text with no images.",
			want: nil,
		},
		{
			name: "empty body",
			body: "",
			want: nil,
		},
		{
			name: "duplicate references deduplicated",
			body: "![same](/path/img.png) and ![same](/path/img.png)",
			want: []ImageRef{
				{FullMatch: "![same](/path/img.png)", AltText: "same", LocalPath: "/path/img.png", Format: "markdown"},
			},
		},
		{
			name: "markdown image with empty alt text",
			body: "![](/path/to/file.png)",
			want: []ImageRef{
				{FullMatch: "![](/path/to/file.png)", AltText: "", LocalPath: "/path/to/file.png", Format: "markdown"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindLocalImages(tt.body)
			if len(got) != len(tt.want) {
				t.Fatalf("FindLocalImages() returned %d refs, want %d", len(got), len(tt.want))
			}
			for i, g := range got {
				w := tt.want[i]
				if g.FullMatch != w.FullMatch {
					t.Errorf("ref[%d].FullMatch = %q, want %q", i, g.FullMatch, w.FullMatch)
				}
				if g.AltText != w.AltText {
					t.Errorf("ref[%d].AltText = %q, want %q", i, g.AltText, w.AltText)
				}
				if g.LocalPath != w.LocalPath {
					t.Errorf("ref[%d].LocalPath = %q, want %q", i, g.LocalPath, w.LocalPath)
				}
				if g.Format != w.Format {
					t.Errorf("ref[%d].Format = %q, want %q", i, g.Format, w.Format)
				}
			}
		})
	}
}

func TestReplaceImages(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		refs   []ImageRef
		urlMap map[string]string
		want   string
	}{
		{
			name: "replace markdown image",
			body: "See ![screenshot](/Users/jason/Desktop/img.png) here",
			refs: []ImageRef{
				{FullMatch: "![screenshot](/Users/jason/Desktop/img.png)", AltText: "screenshot", LocalPath: "/Users/jason/Desktop/img.png", Format: "markdown"},
			},
			urlMap: map[string]string{"/Users/jason/Desktop/img.png": "https://cdn.example.com/img.png"},
			want:   "See ![screenshot](https://cdn.example.com/img.png) here",
		},
		{
			name: "replace custom format normalizes to markdown",
			body: "Image: My Screenshot → /tmp/shot.png",
			refs: []ImageRef{
				{FullMatch: "Image: My Screenshot → /tmp/shot.png", AltText: "My Screenshot", LocalPath: "/tmp/shot.png", Format: "custom"},
			},
			urlMap: map[string]string{"/tmp/shot.png": "https://cdn.example.com/shot.png"},
			want:   "![My Screenshot](https://cdn.example.com/shot.png)",
		},
		{
			name: "skip refs not in urlMap",
			body: "![a](/path/a.png) ![b](/path/b.png)",
			refs: []ImageRef{
				{FullMatch: "![a](/path/a.png)", AltText: "a", LocalPath: "/path/a.png", Format: "markdown"},
				{FullMatch: "![b](/path/b.png)", AltText: "b", LocalPath: "/path/b.png", Format: "markdown"},
			},
			urlMap: map[string]string{"/path/a.png": "https://cdn.example.com/a.png"},
			want:   "![a](https://cdn.example.com/a.png) ![b](/path/b.png)",
		},
		{
			name:   "empty refs no-op",
			body:   "no images here",
			refs:   nil,
			urlMap: nil,
			want:   "no images here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceImages(tt.body, tt.refs, tt.urlMap)
			if got != tt.want {
				t.Errorf("ReplaceImages() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContentHash(t *testing.T) {
	// Create a temp file with known content
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	if err := os.WriteFile(path, []byte("test image content"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := ContentHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hash1) != 12 {
		t.Errorf("ContentHash() returned %d chars, want 12", len(hash1))
	}

	// Same content should produce same hash
	hash2, err := ContentHash(path)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Error("ContentHash() not deterministic")
	}

	// Different content should produce different hash
	path2 := filepath.Join(dir, "test2.png")
	if err := os.WriteFile(path2, []byte("different content"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash3, err := ContentHash(path2)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 == hash3 {
		t.Error("ContentHash() returned same hash for different content")
	}
}

func TestContentHash_fileNotFound(t *testing.T) {
	_, err := ContentHash("/nonexistent/file.png")
	if err == nil {
		t.Error("ContentHash() should return error for nonexistent file")
	}
}

func TestImageFileName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "screenshot.png")
	if err := os.WriteFile(path, []byte("image data"), 0o644); err != nil {
		t.Fatal(err)
	}

	name, err := ImageFileName(path)
	if err != nil {
		t.Fatal(err)
	}

	// Should be {12-char-hash}_screenshot.png
	if len(name) < 14 { // at least 12 + _ + 1
		t.Errorf("ImageFileName() = %q, too short", name)
	}
	if name[12] != '_' {
		t.Errorf("ImageFileName() = %q, expected '_' at position 12", name)
	}
	if filepath.Base(name) != name {
		t.Error("ImageFileName() should not contain path separators")
	}
	if !contains(name, "screenshot.png") {
		t.Errorf("ImageFileName() = %q, should contain original filename", name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}
