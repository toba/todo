---
# zdq-djq
title: TUI auto-refresh when issues change on disk
status: completed
type: bug
priority: normal
created_at: 2026-02-16T02:23:01Z
updated_at: 2026-02-16T02:24:50Z
---

## Problem

The TUI does not reliably refresh when issues change on disk.

## Plan

- [x] Add polling fallback in watcher (2s interval, mtime-based change detection)
- [x] Watch new directories as they are created
- [x] Add periodic tick in TUI as safety net for dropped events
- [x] Add unit tests for pollForChanges

## Summary of Changes

- Added `pollForChanges` method to watcher that walks `.issues/`, compares mtimes, and detects creates/writes/removes every 2 seconds
- Added `snapshotMtimes` to seed the mtime cache on startup
- New subdirectories are now watched both via fsnotify Create events and during polling walks
- Added `tickMsg`/`tickCmd` to TUI that refreshes the list and detail view every 2 seconds as a safety net
- Added unit tests for `pollForChanges` and `snapshotMtimes`
