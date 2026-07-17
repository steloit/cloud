---
id: X-0.0
title: One-line outcome
epic: E0
status: stub          # stub | ready | in-progress | in-review | done | blocked
phase: MVP            # MVP | V1 | Future
priority: medium      # critical | high | medium | low
sprint: 0             # human forecast only — agents ignore
estimate: 0.5ew       # human planning only — agents ignore
deps: []              # task ids; ready-set = status ready AND all deps done
issue: 0              # synced by spec-sync; never hand-edited
labels: []
module: Cross-cutting
contexts: []          # packs in contexts/ to load before Read-first
files: []             # intended touch-set (globs) — drives parallel scheduling; be honest
apis: []
tables: []
events: []
tests: []
verify: []            # executable definition of done — CI runs these on the PR
owner: agent
---

## Goal
<!-- One sentence, outcome not method. -->

## Why
<!-- 1–3 lines of business context. -->

## Read first
<!-- 3–7 paths, one-line why each. End with a Don't-read list. Packs come from contexts:, not here. -->

## Approach
<!-- 5–10 ordered steps. Name interfaces, state transitions, and the pattern-file to imitate. -->

## Edge cases
<!-- Enumerated; each maps to an AC or test. -->

## Security & performance
<!-- Deltas from AGENTS.md/pack rules only. -->

## Acceptance criteria
<!-- Testable, WHEN/THEN-shaped, checkboxed. -->

## Validation
<!-- Restate verify: plus any manual evidence required in the PR. -->

## Common mistakes
<!-- Task-specific only (3–5). Domain traps belong in the pack's mistake bank. -->

## Out of scope
<!-- Explicit, with the task id that owns each excluded concern. -->

## Related
<!-- Sibling tasks, ADRs, frame ids. -->
