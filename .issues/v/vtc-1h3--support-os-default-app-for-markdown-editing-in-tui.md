---
# vtc-1h3
title: Support OS default app for markdown editing in TUI
status: completed
type: feature
priority: normal
created_at: 2026-02-15T17:37:13Z
updated_at: 2026-02-15T17:38:21Z
---

When pressing 'e' in the TUI, support using the OS default markdown editor via a 'system' keyword and as a fallback before vi/nano. On macOS, use 'open -W -n -g' to open files with the registered .md handler.

## Summary of Changes\n\nModified `getEditor()` in `internal/tui/tui.go` to:\n- Add `systemEditor()` function that returns `open -W -n -g` on macOS\n- Recognize the `"system"` keyword (case-insensitive) from config or env vars\n- Insert OS default app opener as a fallback between `$EDITOR` and `vi`/`nano`\n- Added 6 new test cases covering the system keyword and fallback behavior
