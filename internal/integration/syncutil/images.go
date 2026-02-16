package syncutil

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// ImageRef represents a local image reference found in an issue body.
type ImageRef struct {
	FullMatch string // entire matched text (for replacement)
	AltText   string // alt text or image name
	LocalPath string // absolute file path
	Format    string // "markdown" or "custom"
}

var (
	// Matches ![alt text](/absolute/path/to/image.png)
	markdownImageRe = regexp.MustCompile(`!\[([^\]]*)\]\((/[^)]+)\)`)

	// Matches Image: Name → /path/to/file.png  or  Image: Name -> /path/to/file.png
	customImageRe = regexp.MustCompile(`Image:\s*(.+?)\s*(?:→|->)\s*(/\S+)`)
)

// FindLocalImages detects local image references in an issue body.
// It recognizes two formats:
//   - Markdown: ![alt](/absolute/path)
//   - Custom: Image: Name → /absolute/path
//
// Only absolute paths (starting with /) are matched; URLs are skipped.
func FindLocalImages(body string) []ImageRef {
	var refs []ImageRef
	seen := make(map[string]bool)

	for _, m := range markdownImageRe.FindAllStringSubmatch(body, -1) {
		path := m[2]
		if seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		refs = append(refs, ImageRef{
			FullMatch: m[0],
			AltText:   m[1],
			LocalPath: path,
			Format:    "markdown",
		})
	}

	for _, m := range customImageRe.FindAllStringSubmatch(body, -1) {
		path := m[2]
		if seen[m[0]] {
			continue
		}
		seen[m[0]] = true
		refs = append(refs, ImageRef{
			FullMatch: m[0],
			AltText:   strings.TrimSpace(m[1]),
			LocalPath: path,
			Format:    "custom",
		})
	}

	return refs
}

// ReplaceImages replaces local image references in a body with remote URLs.
// urlMap maps local file paths to remote URLs.
// Markdown refs become ![alt](url); custom refs are normalized to markdown format.
func ReplaceImages(body string, refs []ImageRef, urlMap map[string]string) string {
	for _, ref := range refs {
		url, ok := urlMap[ref.LocalPath]
		if !ok {
			continue
		}
		replacement := fmt.Sprintf("![%s](%s)", ref.AltText, url)
		body = strings.Replace(body, ref.FullMatch, replacement, 1)
	}
	return body
}

// ContentHash returns the first 12 hex characters of the SHA-256 hash of the file at filePath.
func ContentHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:12], nil
}

// ImageFileName returns a content-hashed filename for the image: {hash}_{basename}.
func ImageFileName(filePath string) (string, error) {
	hash, err := ContentHash(filePath)
	if err != nil {
		return "", err
	}
	return hash + "_" + filepath.Base(filePath), nil
}
