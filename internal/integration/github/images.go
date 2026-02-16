package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/toba/todo/internal/integration/syncutil"
)

// contentsResponse represents the GitHub Contents API response.
type contentsResponse struct {
	SHA         string `json:"sha"`
	DownloadURL string `json:"download_url"`
}

// contentsRequest represents the GitHub Contents API PUT request body.
type contentsRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	SHA     string `json:"sha,omitempty"`
}

// contentsCreateResponse represents the response from creating/updating content.
type contentsCreateResponse struct {
	Content contentsResponse `json:"content"`
}

// GetContents retrieves the SHA of a file in the repository via the Contents API.
// Returns empty string and nil error if the file does not exist (404).
func (c *Client) GetContents(ctx context.Context, path string) (sha string, err error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, c.owner, c.repo, path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}

	var result contentsResponse
	if err := c.doRequest(req, &result); err != nil {
		// 404 means file doesn't exist, which is not an error for our use case
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return "", nil
		}
		return "", fmt.Errorf("getting contents: %w", err)
	}

	return result.SHA, nil
}

// UploadContents creates or updates a file in the repository via the Contents API.
// If sha is non-empty, the file is updated; otherwise it is created.
// Returns the download URL of the uploaded file.
func (c *Client) UploadContents(ctx context.Context, path string, content []byte, message, sha string) (downloadURL string, err error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", baseURL, c.owner, c.repo, path)

	body := contentsRequest{
		Message: message,
		Content: base64.StdEncoding.EncodeToString(content),
	}
	if sha != "" {
		body.SHA = sha
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var result contentsCreateResponse
	if err := c.doRequest(req, &result); err != nil {
		return "", fmt.Errorf("uploading contents: %w", err)
	}

	return result.Content.DownloadURL, nil
}

// UploadImages detects local image references in the body, uploads them to
// the repository via the Contents API, and returns a map of local paths to
// download URLs. Images are stored at .github/todo-images/{hash}_{filename}.
// Missing files or upload failures are logged to stderr and skipped.
func UploadImages(ctx context.Context, client *Client, body string) (map[string]string, error) {
	refs := syncutil.FindLocalImages(body)
	if len(refs) == 0 {
		return nil, nil
	}

	urlMap := make(map[string]string)
	for _, ref := range refs {
		if _, ok := urlMap[ref.LocalPath]; ok {
			continue // already processed
		}

		if _, err := os.Stat(ref.LocalPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping image %s: %v\n", ref.LocalPath, err)
			continue
		}

		imageName, err := syncutil.ImageFileName(ref.LocalPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to hash image %s: %v\n", ref.LocalPath, err)
			continue
		}

		repoPath := ".github/todo-images/" + imageName

		// Check if already uploaded
		sha, err := client.GetContents(ctx, repoPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to check image %s: %v\n", ref.LocalPath, err)
			continue
		}

		if sha != "" {
			// File already exists — construct the download URL without re-uploading
			downloadURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/%s", client.owner, client.repo, repoPath)
			urlMap[ref.LocalPath] = downloadURL
			continue
		}

		content, err := os.ReadFile(ref.LocalPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to read image %s: %v\n", ref.LocalPath, err)
			continue
		}

		commitMsg := fmt.Sprintf("chore: upload issue image %s", imageName)
		downloadURL, err := client.UploadContents(ctx, repoPath, content, commitMsg, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to upload image %s: %v\n", ref.LocalPath, err)
			continue
		}

		urlMap[ref.LocalPath] = downloadURL
	}

	return urlMap, nil
}
