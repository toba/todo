---
# 4g0-3b1
title: TUI auto-refresh when issues change on disk
status: in-progress
type: bug
priority: normal
created_at: 2026-02-16T02:22:56Z
updated_at: 2026-02-16T17:47:55Z
---

## Problem

The TUI doesn't refresh (or refreshes very slowly) when issues change on disk — e.g., when running `todo update` or `todo create` in another terminal.

## Root Causes

1. macOS kqueue may miss file content modifications when only the directory is watched
2. New subdirectories created after watcher starts aren't watched
3. Non-blocking fan-out drops events with no retry

## Plan

- [ ] Add polling fallback in watcher (2s interval, mtime-based change detection)
- [ ] Watch new directories as they're created
- [ ] Add periodic tick in TUI as safety net for dropped events
- [ ] Add unit tests for pollForChanges

## Fix: Filtered list disappears after auto-refresh

The `issuesLoadedMsg` handler in `list.go` was discarding the `tea.Cmd` returned by `list.SetItems()`. This command re-applies the active text filter to the new items. Without it, filtered results vanished after each 2-second tick refresh.

Fixed by capturing and returning the command: `cmd = m.list.SetItems(items)` and `return m, cmd`.
