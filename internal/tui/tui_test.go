package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/toba/todo/internal/config"
)

func TestGetEditor(t *testing.T) {
	// Save and restore env vars
	origVisual := os.Getenv("VISUAL")
	origEditor := os.Getenv("EDITOR")
	t.Cleanup(func() {
		os.Setenv("VISUAL", origVisual)
		os.Setenv("EDITOR", origEditor)
	})

	t.Run("config editor takes priority over env", func(t *testing.T) {
		os.Setenv("VISUAL", "emacs")
		os.Setenv("EDITOR", "nano")

		cfg := config.Default()
		cfg.Issues.Editor = "vim"

		cmd, args := getEditor(cfg)
		if cmd != "vim" {
			t.Errorf("cmd = %q, want \"vim\"", cmd)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})

	t.Run("falls back to VISUAL", func(t *testing.T) {
		os.Setenv("VISUAL", "emacs")
		os.Setenv("EDITOR", "nano")

		cfg := config.Default()
		// no editor set in config

		cmd, args := getEditor(cfg)
		if cmd != "emacs" {
			t.Errorf("cmd = %q, want \"emacs\"", cmd)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})

	t.Run("falls back to EDITOR", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Setenv("EDITOR", "nano")

		cfg := config.Default()

		cmd, args := getEditor(cfg)
		if cmd != "nano" {
			t.Errorf("cmd = %q, want \"nano\"", cmd)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})

	t.Run("multi-word editor splits into cmd and args", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()
		cfg.Issues.Editor = "code --wait"

		cmd, args := getEditor(cfg)
		if cmd != "code" {
			t.Errorf("cmd = %q, want \"code\"", cmd)
		}
		if len(args) != 1 || args[0] != "--wait" {
			t.Errorf("args = %v, want [\"--wait\"]", args)
		}
	})

	t.Run("relative path resolved against config dir", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()
		cfg.SetConfigDir("/project/root")
		cfg.Issues.Editor = "./scripts/my-editor"

		cmd, args := getEditor(cfg)
		want := filepath.Join("/project/root", "scripts/my-editor")
		if cmd != want {
			t.Errorf("cmd = %q, want %q", cmd, want)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})

	t.Run("relative path with args", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()
		cfg.SetConfigDir("/project/root")
		cfg.Issues.Editor = "../bin/editor --flag"

		cmd, args := getEditor(cfg)
		want := filepath.Join("/project/root", "../bin/editor")
		if cmd != want {
			t.Errorf("cmd = %q, want %q", cmd, want)
		}
		if len(args) != 1 || args[0] != "--flag" {
			t.Errorf("args = %v, want [\"--flag\"]", args)
		}
	})

	t.Run("absolute path not modified", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()
		cfg.SetConfigDir("/project/root")
		cfg.Issues.Editor = "/usr/local/bin/nvim"

		cmd, args := getEditor(cfg)
		if cmd != "/usr/local/bin/nvim" {
			t.Errorf("cmd = %q, want \"/usr/local/bin/nvim\"", cmd)
		}
		if len(args) != 0 {
			t.Errorf("args = %v, want empty", args)
		}
	})

	t.Run("system keyword resolves to OS default opener", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()
		cfg.Issues.Editor = "system"

		cmd, args := getEditor(cfg)
		if runtime.GOOS == "darwin" {
			if cmd != "open" {
				t.Errorf("cmd = %q, want \"open\"", cmd)
			}
			wantArgs := []string{"-W", "-n", "-g"}
			if len(args) != len(wantArgs) {
				t.Fatalf("args = %v, want %v", args, wantArgs)
			}
			for i, a := range wantArgs {
				if args[i] != a {
					t.Errorf("args[%d] = %q, want %q", i, args[i], a)
				}
			}
		} else {
			// On non-darwin, "system" falls through to vi/nano
			if cmd != "vi" && cmd != "nano" {
				t.Errorf("cmd = %q, want \"vi\" or \"nano\" on non-darwin", cmd)
			}
		}
	})

	t.Run("system keyword is case-insensitive", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()
		cfg.Issues.Editor = "System"

		cmd, _ := getEditor(cfg)
		if runtime.GOOS == "darwin" {
			if cmd != "open" {
				t.Errorf("cmd = %q, want \"open\"", cmd)
			}
		}
	})

	t.Run("VISUAL=system resolves to OS default opener", func(t *testing.T) {
		os.Setenv("VISUAL", "system")
		os.Unsetenv("EDITOR")

		cfg := config.Default()

		cmd, args := getEditor(cfg)
		if runtime.GOOS == "darwin" {
			if cmd != "open" {
				t.Errorf("cmd = %q, want \"open\"", cmd)
			}
			wantArgs := []string{"-W", "-n", "-g"}
			if len(args) != len(wantArgs) {
				t.Fatalf("args = %v, want %v", args, wantArgs)
			}
			for i, a := range wantArgs {
				if args[i] != a {
					t.Errorf("args[%d] = %q, want %q", i, args[i], a)
				}
			}
		}
	})

	t.Run("no env vars falls back to system editor before vi/nano", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Unsetenv("EDITOR")

		cfg := config.Default()

		cmd, _ := getEditor(cfg)
		if runtime.GOOS == "darwin" {
			if cmd != "open" {
				t.Errorf("cmd = %q, want \"open\" on darwin", cmd)
			}
		} else {
			if cmd != "vi" && cmd != "nano" {
				t.Errorf("cmd = %q, want \"vi\" or \"nano\" on non-darwin", cmd)
			}
		}
	})

	t.Run("explicit editor takes priority over system fallback", func(t *testing.T) {
		os.Unsetenv("VISUAL")
		os.Setenv("EDITOR", "vim")

		cfg := config.Default()

		cmd, _ := getEditor(cfg)
		if cmd != "vim" {
			t.Errorf("cmd = %q, want \"vim\"", cmd)
		}
	})
}
