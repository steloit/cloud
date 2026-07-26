---
id: E8
title: E8 · Console integration (MSW → real API)
epic: E8
status: stub
phase: V1
priority: high
sprint: 6
estimate: 10ew
deps: [E7]
issue: 9
labels: [Frontend]
module: M10 Clients
contexts: []
owner: founders
---

## Scope

Swap MSW → real API surface-by-surface behind a per-family flag. Canon mode is retained permanently as the demo world (ADR-026) and E2E fixture harness. Each slice = flag flip + Playwright suite green.

**Exit:** session seam removed; all slices live; four-state truth verified per slice via fault injection. (implementation-plan §5 E8)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
