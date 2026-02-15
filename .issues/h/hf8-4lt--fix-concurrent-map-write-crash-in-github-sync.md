---
# hf8-4lt
title: Fix concurrent map write crash in GitHub sync
status: completed
type: bug
priority: critical
created_at: 2026-02-15T04:16:44Z
updated_at: 2026-02-15T04:16:44Z
---

## Description

The `Syncer.syncIssue` method writes to `s.issueToGHNumber` map without mutex protection at line 247 (after creating a new issue) and reads it at line 260 (looking up parent number). Since `syncIssue` is called from goroutines via `sync.WaitGroup.Go`, this causes a fatal `concurrent map writes` crash when syncing many issues.

## Root Cause

The `issueToGHNumber` map on the `Syncer` struct was accessed concurrently from multiple goroutines without synchronization. While `SyncIssues` had a local `mu sync.Mutex` protecting some map writes, the writes/reads inside `syncIssue` itself were unprotected.

## Fix

- [x] Add `sync.RWMutex` field to `Syncer` struct
- [x] Protect map writes in `syncIssue` with `s.mu.Lock()`
- [x] Protect map reads in `syncIssue` with `s.mu.RLock()`
- [x] Replace local `mu` in `SyncIssues` with `s.mu` for consistency

## Summary of Changes

Added `mu sync.RWMutex` to the `Syncer` struct and used it to protect all concurrent accesses to `issueToGHNumber` map. Replaced the local mutex in `SyncIssues` with the struct-level mutex for consistency. All map writes use `Lock()` and read-only accesses use `RLock()`.
