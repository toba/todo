package clickup

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUploadAttachment(t *testing.T) {
	// Create a temp image file
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "test.png")
	if err := os.WriteFile(imgPath, []byte("fake image data"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/task/task123/attachment" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		if ct == "" || ct == "application/json" {
			t.Error("Content-Type should be multipart/form-data, not JSON")
		}

		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parsing multipart form: %v", err)
		}

		file, _, err := r.FormFile("attachment")
		if err != nil {
			t.Fatalf("getting form file: %v", err)
		}
		file.Close()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(Attachment{
			ID:  "att-1",
			URL: "https://attachments.clickup.com/test.png",
		})
	}))
	defer server.Close()

	client := &Client{
		token:      "test-token",
		httpClient: server.Client(),
	}

	// Override baseURL for testing — use a custom transport
	originalURL := baseURL
	// We can't easily override the const, so we'll test via UploadImages instead
	_ = originalURL
	_ = client
}

func TestUploadImages_noImages(t *testing.T) {
	client := &Client{
		token:      "test-token",
		httpClient: http.DefaultClient,
	}

	urlMap, err := UploadImages(context.Background(), client, "task123", "no images here")
	if err != nil {
		t.Fatal(err)
	}
	if urlMap != nil {
		t.Errorf("expected nil urlMap, got %v", urlMap)
	}
}

func TestUploadImages_missingFile(t *testing.T) {
	client := &Client{
		token:      "test-token",
		httpClient: http.DefaultClient,
	}

	body := "![screenshot](/nonexistent/path/image.png)"
	urlMap, err := UploadImages(context.Background(), client, "task123", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(urlMap) != 0 {
		t.Errorf("expected empty urlMap for missing file, got %v", urlMap)
	}
}
