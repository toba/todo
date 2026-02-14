package github

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]any
		wantOwner string
		wantRepo  string
		wantNil   bool
		wantErr   bool
	}{
		{
			name:      "valid config",
			input:     map[string]any{"repo": "owner/repo"},
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:    "nil config",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "missing repo",
			input:   map[string]any{"other": "value"},
			wantNil: true,
		},
		{
			name:    "empty repo",
			input:   map[string]any{"repo": ""},
			wantNil: true,
		},
		{
			name:    "non-string repo",
			input:   map[string]any{"repo": 123},
			wantNil: true,
		},
		{
			name:    "invalid repo format",
			input:   map[string]any{"repo": "noslash"},
			wantErr: true,
		},
		{
			name:    "empty owner",
			input:   map[string]any{"repo": "/repo"},
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   map[string]any{"repo": "owner/"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConfig(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if cfg != nil {
					t.Fatalf("expected nil config, got %+v", cfg)
				}
				return
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if cfg.Owner != tt.wantOwner {
				t.Errorf("Owner = %q, want %q", cfg.Owner, tt.wantOwner)
			}
			if cfg.Repo != tt.wantRepo {
				t.Errorf("Repo = %q, want %q", cfg.Repo, tt.wantRepo)
			}
		})
	}
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{"owner/repo", "owner", "repo", false},
		{"my-org/my-repo", "my-org", "my-repo", false},
		{"noslash", "", "", true},
		{"/repo", "", "", true},
		{"owner/", "", "", true},
		{"", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			owner, name, err := ParseRepo(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner = %q, want %q", owner, tt.wantOwner)
			}
			if name != tt.wantName {
				t.Errorf("name = %q, want %q", name, tt.wantName)
			}
		})
	}
}

func TestDefaultStatusMapping(t *testing.T) {
	// Verify all expected statuses are present
	expected := []string{"draft", "todo", "in-progress", "completed", "scrapped"}
	for _, status := range expected {
		if _, ok := DefaultStatusMapping[status]; !ok {
			t.Errorf("missing status mapping for %q", status)
		}
	}

	// Verify completed maps to closed with no label
	if m := DefaultStatusMapping["completed"]; m.State != "closed" || m.Label != "" {
		t.Errorf("completed mapping = %+v, want closed with no label", m)
	}

	// Verify scrapped maps to closed with label
	if m := DefaultStatusMapping["scrapped"]; m.State != "closed" || m.Label != "status:scrapped" {
		t.Errorf("scrapped mapping = %+v, want closed with status:scrapped label", m)
	}
}
