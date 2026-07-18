---
id: US-5.1
title: steloit init / create with the estimate-first safety grammar
epic: E5
status: stub
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: [T5.1, T5.2]
issue: 81
labels: [CLI]
module: M10 Clients
contexts: [api-conventions]
files: []
verify: []
owner: agent
---

## Goal

steloit init / create with the estimate-first safety grammar

## Summary

20-clients safety grammar: `--yes` accepts a *shown* estimate (no skip-seeing flag); context org/project/env echoed on every state-changing command. **Implicit-env rules (ADR-037):** at n=1, `--env` is never required or asked (echo still truthfully includes `· production`); at n≥2, resolution = flag → repo-link → profile default, then read-only commands default to production (env printed in header) and **state-changing commands never guess** — TTY prompts with the env list, non-TTY exits 2 with remediation `pass --env`.

## Acceptance criteria

- [ ] create without seeing an estimate is impossible.

> **Stub** — run the spec-author skill to enrich to `ready` before starting. Plan reference: docs/plan/implementation-plan.md §5 E5.
