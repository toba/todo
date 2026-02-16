---
# 8gl-1cg
title: Make prime output token-efficient
status: completed
type: task
priority: normal
created_at: 2026-02-16T23:25:56Z
updated_at: 2026-02-16T23:26:58Z
---

Rewrite cmd/prompt.tmpl to reduce token count while preserving all behavioral instructions and CLI references. Current: ~1545 words.

## Summary of Changes\n\nRewrote cmd/prompt.tmpl to reduce token overhead:\n- Template file: 1545 → 561 words (64% reduction)\n- Rendered output: ~1656 → 678 words (59% reduction)\n- Replaced markdown tables with inline flag references\n- Deduplicated CLI examples and option docs\n- Compressed GraphQL, body modification, and concurrency sections\n- Preserved all behavioral rules, CLI flags, and template directives
