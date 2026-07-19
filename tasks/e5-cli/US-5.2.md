---
id: US-5.2
title: Output modes: human tables + six marks, --json verbatim, --quiet ids, exit codes 0–7
epic: E5
status: done
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: [T5.5]
issue: 82
labels: [CLI]
module: M10 Clients
contexts: [api-conventions]
files: []
verify:
  - cd apps/cli && go test ./...
owner: agent
---

## Goal

Output modes: human tables + six marks, --json verbatim, --quiet ids, exit codes 0–7

## Summary

**AC:** `--json` = raw API shapes (snake_case, *_cents, {data,next_cursor}); NO_COLOR/non-TTY degrade losslessly; exit-code map tested.

## Acceptance criteria

- [x] `--json` = raw API bytes verbatim (RawJSON re-emits; snake_case/*_cents/
      {data,next_cursor} untouched); `--quiet` = ids only; six marks; NO_COLOR strips
      escapes losslessly; exit-code map (0–7) pinned in `output_test.go`.

## Outcome

Carried by T5.5 (the shared output package) + T5.3 (every noun renders through it).
