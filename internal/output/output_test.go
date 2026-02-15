package output

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/toba/todo/internal/issue"
)

// captureStdout captures stdout output from a function.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}

	origStdout := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	r.Close()
	return string(buf[:n])
}

func TestJSON(t *testing.T) {
	output := captureStdout(t, func() {
		_ = JSON(Response{
			Success: true,
			Message: "hello",
		})
	})

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Message != "hello" {
		t.Errorf("message = %q, want %q", resp.Message, "hello")
	}
}

func TestSuccess(t *testing.T) {
	b := &issue.Issue{ID: "test-1", Title: "Test Issue", Status: "todo"}

	output := captureStdout(t, func() {
		_ = Success(b, "Issue created")
	})

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Message != "Issue created" {
		t.Errorf("message = %q, want %q", resp.Message, "Issue created")
	}
	if resp.Issue == nil {
		t.Fatal("expected issue to be present")
	}
	if resp.Issue.ID != "test-1" {
		t.Errorf("issue.ID = %q, want %q", resp.Issue.ID, "test-1")
	}
}

func TestSuccessWithWarnings(t *testing.T) {
	b := &issue.Issue{ID: "test-1", Title: "Test", Status: "todo"}
	warnings := []string{"warning 1", "warning 2"}

	output := captureStdout(t, func() {
		_ = SuccessWithWarnings(b, "Updated", warnings)
	})

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if len(resp.Warnings) != 2 {
		t.Fatalf("warnings count = %d, want 2", len(resp.Warnings))
	}
	if resp.Warnings[0] != "warning 1" {
		t.Errorf("warnings[0] = %q, want %q", resp.Warnings[0], "warning 1")
	}
}

func TestSuccessSingle(t *testing.T) {
	b := &issue.Issue{ID: "test-1", Title: "Direct Issue", Status: "todo"}

	output := captureStdout(t, func() {
		_ = SuccessSingle(b)
	})

	// SuccessSingle outputs an issue directly, not wrapped in Response
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if result["id"] != "test-1" {
		t.Errorf("id = %v, want %q", result["id"], "test-1")
	}
	if result["title"] != "Direct Issue" {
		t.Errorf("title = %v, want %q", result["title"], "Direct Issue")
	}
	// Should NOT have the Response wrapper fields
	if _, ok := result["success"]; ok {
		t.Error("SuccessSingle should not have 'success' wrapper field")
	}
}

func TestSuccessMultiple(t *testing.T) {
	issues := []*issue.Issue{
		{ID: "b1", Title: "First", Status: "todo"},
		{ID: "b2", Title: "Second", Status: "todo"},
	}

	output := captureStdout(t, func() {
		_ = SuccessMultiple(issues)
	})

	// SuccessMultiple outputs a JSON array directly
	var result []map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("count = %d, want 2", len(result))
	}
	if result[0]["id"] != "b1" {
		t.Errorf("result[0].id = %v, want %q", result[0]["id"], "b1")
	}
	if result[1]["id"] != "b2" {
		t.Errorf("result[1].id = %v, want %q", result[1]["id"], "b2")
	}
}

func TestSuccessMultipleEmpty(t *testing.T) {
	output := captureStdout(t, func() {
		_ = SuccessMultiple([]*issue.Issue{})
	})

	var result []map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

func TestSuccessMessage(t *testing.T) {
	output := captureStdout(t, func() {
		_ = SuccessMessage("All done")
	})

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Message != "All done" {
		t.Errorf("message = %q, want %q", resp.Message, "All done")
	}
	if resp.Issue != nil {
		t.Error("expected no issue in message-only response")
	}
}

func TestSuccessInit(t *testing.T) {
	output := captureStdout(t, func() {
		_ = SuccessInit("/path/to/.issues")
	})

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Path != "/path/to/.issues" {
		t.Errorf("path = %q, want %q", resp.Path, "/path/to/.issues")
	}
	if resp.Message != "Initialized data directory" {
		t.Errorf("message = %q, want %q", resp.Message, "Initialized data directory")
	}
}

func TestError(t *testing.T) {
	var output string
	var returnedErr error

	output = captureStdout(t, func() {
		returnedErr = Error(ErrNotFound, "issue not found")
	})

	// Should return an error
	if returnedErr == nil {
		t.Fatal("Error() should return a non-nil error")
	}
	if returnedErr.Error() != "issue not found" {
		t.Errorf("returned error = %q, want %q", returnedErr.Error(), "issue not found")
	}

	// Should output JSON with success=false
	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\noutput: %s", err, output)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Code != ErrNotFound {
		t.Errorf("code = %q, want %q", resp.Code, ErrNotFound)
	}
	if resp.Error != "issue not found" {
		t.Errorf("error = %q, want %q", resp.Error, "issue not found")
	}
}

func TestErrorFrom(t *testing.T) {
	var output string
	var returnedErr error

	output = captureStdout(t, func() {
		returnedErr = ErrorFrom(ErrValidation, errors.New("invalid status"))
	})

	if returnedErr == nil {
		t.Fatal("ErrorFrom() should return a non-nil error")
	}
	if returnedErr.Error() != "invalid status" {
		t.Errorf("returned error = %q, want %q", returnedErr.Error(), "invalid status")
	}

	var resp Response
	if err := json.Unmarshal([]byte(output), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.Code != ErrValidation {
		t.Errorf("code = %q, want %q", resp.Code, ErrValidation)
	}
}

func TestResponseOmitsEmptyFields(t *testing.T) {
	output := captureStdout(t, func() {
		_ = SuccessMessage("test")
	})

	// Verify that empty fields are omitted (not present in JSON)
	var raw map[string]any
	if err := json.Unmarshal([]byte(output), &raw); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := raw["issue"]; ok {
		t.Error("expected 'issue' field to be omitted when nil")
	}
	if _, ok := raw["issues"]; ok {
		t.Error("expected 'issues' field to be omitted when nil")
	}
	if _, ok := raw["warnings"]; ok {
		t.Error("expected 'warnings' field to be omitted when nil")
	}
	if _, ok := raw["error"]; ok {
		t.Error("expected 'error' field to be omitted when empty")
	}
	if _, ok := raw["code"]; ok {
		t.Error("expected 'code' field to be omitted when empty")
	}
}
