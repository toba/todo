package clickup

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/toba/todo/internal/integration/syncutil"
)

// Attachment represents a ClickUp attachment response.
type Attachment struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// UploadAttachment uploads a file as an attachment to a ClickUp task.
// It uses POST /task/{taskID}/attachment with multipart/form-data.
func (c *Client) UploadAttachment(ctx context.Context, taskID, filePath string) (*Attachment, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("attachment", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}

	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	if _, err := part.Write(fileBytes); err != nil {
		return nil, fmt.Errorf("writing file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("closing multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/task/%s/attachment", baseURL, taskID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &buf)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var result struct {
		Attachment
	}
	if err := c.doRequest(req, &result); err != nil {
		return nil, fmt.Errorf("uploading attachment: %w", err)
	}

	return &result.Attachment, nil
}

// UploadImages detects local image references in the body, uploads them as
// ClickUp attachments, and returns a map of local paths to remote URLs.
// Missing files or upload failures are logged to stderr and skipped.
func UploadImages(ctx context.Context, client *Client, taskID, body string) (map[string]string, error) {
	refs := syncutil.FindLocalImages(body)
	if len(refs) == 0 {
		return nil, nil
	}

	urlMap := make(map[string]string)
	for _, ref := range refs {
		if _, ok := urlMap[ref.LocalPath]; ok {
			continue // already uploaded
		}

		if _, err := os.Stat(ref.LocalPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping image %s: %v\n", ref.LocalPath, err)
			continue
		}

		att, err := client.UploadAttachment(ctx, taskID, ref.LocalPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to upload image %s: %v\n", ref.LocalPath, err)
			continue
		}

		urlMap[ref.LocalPath] = att.URL
	}

	return urlMap, nil
}
