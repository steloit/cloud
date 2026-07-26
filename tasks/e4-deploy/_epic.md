---
id: E4
title: E4 · Deploy & branched previews
epic: E4
status: stub
phase: MVP
priority: critical
sprint: 4
estimate: 9ew
deps: [E3]
issue: 5
labels: [Platform, Backend]
module: M5 Deploy
contexts: []
owner: founders
---

## Scope

The certainty demo's mechanism (wedge review 2026-07-18: branching is implementation, never positioning): push → build → deploy; PR → preview env with masked production data (F14) at a guaranteed, capped price; rollback <60s; deploy markers. Custom domains & TLS (T4.8) and masking-by-policy depth (T4.9) follow in V1.

**Exit:** push → estimate → approve → live service + Postgres → PR → preview URL with branched DB → merge → teardown (= milestone M4). (implementation-plan §5 E4)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
