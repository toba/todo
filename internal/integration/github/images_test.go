package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetContents_exists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(contentsResponse{
			SHA:         "abc123",
			DownloadURL: "https://raw.githubusercontent.com/owner/repo/main/test.png",
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)

	sha, err := client.GetContents(context.Background(), ".github/todo-images/test.png")
	if err != nil {
		t.Fatal(err)
	}
	if sha != "abc123" {
		t.Errorf("got sha %q, want %q", sha, "abc123")
	}
}

func TestGetContents_notFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errorResponse{Message: "Not Found"})
	}))
	defer server.Close()

	client := newTestClient(t, server)

	sha, err := client.GetContents(context.Background(), ".github/todo-images/test.png")
	if err != nil {
		t.Fatalf("404 should not return error, got: %v", err)
	}
	if sha != "" {
		t.Errorf("got sha %q, want empty string", sha)
	}
}

func TestUploadContents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "contents/") {
			t.Errorf("expected contents path, got %s", r.URL.Path)
		}

		json.NewEncoder(w).Encode(contentsCreateResponse{
			Content: contentsResponse{
				SHA:         "def456",
				DownloadURL: "https://raw.githubusercontent.com/owner/repo/main/.github/todo-images/test.png",
			},
		})
	}))
	defer server.Close()

	client := newTestClient(t, server)

	downloadURL, err := client.UploadContents(context.Background(), ".github/todo-images/test.png", []byte("image data"), "chore: upload image", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(downloadURL, "test.png") {
		t.Errorf("got downloadURL %q, expected it to contain test.png", downloadURL)
	}
}

func TestUploadImages_noImages(t *testing.T) {
	client := &Client{
		owner:      "owner",
		repo:       "repo",
		token:      "test-token",
		httpClient: http.DefaultClient,
	}

	urlMap, err := UploadImages(context.Background(), client, "no images here")
	if err != nil {
		t.Fatal(err)
	}
	if urlMap != nil {
		t.Errorf("expected nil urlMap, got %v", urlMap)
	}
}

func TestUploadImages_missingFile(t *testing.T) {
	client := &Client{
		owner:      "owner",
		repo:       "repo",
		token:      "test-token",
		httpClient: http.DefaultClient,
	}

	body := "![screenshot](/nonexistent/path/image.png)"
	urlMap, err := UploadImages(context.Background(), client, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(urlMap) != 0 {
		t.Errorf("expected empty urlMap for missing file, got %v", urlMap)
	}
}

func TestUploadImages_fullFlow(t *testing.T) {
	// Create a temp image file
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "screenshot.png")
	if err := os.WriteFile(imgPath, []byte("fake image data"), 0o644); err != nil {
		t.Fatal(err)
	}

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if r.Method == http.MethodGet {
			// GetContents - file doesn't exist yet
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(errorResponse{Message: "Not Found"})
			return
		}
		if r.Method == http.MethodPut {
			// UploadContents
			json.NewEncoder(w).Encode(contentsCreateResponse{
				Content: contentsResponse{
					SHA:         "newsha",
					DownloadURL: "https://raw.githubusercontent.com/owner/repo/main/.github/todo-images/screenshot.png",
				},
			})
			return
		}
		t.Errorf("unexpected method: %s", r.Method)
	}))
	defer server.Close()

	client := newTestClient(t, server)

	body := "![screenshot](" + imgPath + ")"
	urlMap, err := UploadImages(context.Background(), client, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(urlMap) != 1 {
		t.Fatalf("expected 1 entry in urlMap, got %d", len(urlMap))
	}
	if url, ok := urlMap[imgPath]; !ok || !strings.Contains(url, "screenshot.png") {
		t.Errorf("unexpected urlMap entry: %v", urlMap)
	}
}

// newTestClient creates a Client that uses the test server URL as its base.
// Since baseURL is a const, we replace the httpClient with a custom transport
// that redirects requests to the test server.
func newTestClient(t *testing.T, server *httptest.Server) *Client {
	t.Helper()
	return &Client{
		owner: "owner",
		repo:  "repo",
		token: "test-token",
		httpClient: &http.Client{
			Transport: &testTransport{
				server: server,
			},
		},
	}
}

// testTransport redirects requests from the real API URL to the test server.
type testTransport struct {
	server *httptest.Server
}

func (t *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Redirect to test server
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.server.URL, "http://")
	return http.DefaultTransport.RoundTrip(req)
}
