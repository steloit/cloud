---
id: E13
title: E13 · AI plane (four laws)
epic: E13
status: stub
phase: V1
priority: high
sprint: 7
estimate: 8ew
deps: [E2, E3, E6]
issue: 14
labels: [AI, Backend]
module: M9 AI
contexts: []
owner: founders
---

## Scope

Composer core (T13.1/13.2/13.6/13.7) pulled to S7–8 (2026-07-18 re-rating: ADR-039/040 critical path; founder-approved); the assistant surfaces (threads/insights/proposals, E8-S7) remain S13–15 after E10/E12 — Law 4 is a product property (whole without AI), not a build order. 8 read-only tools, id-only context envelopes + resolver, threads, insights, proposals. No auto-apply path exists in the API — architecture, not policy.

**Exit:** QA scenario 7 (AI disable) green; apply re-runs full two-layer RBAC. (implementation-plan §5 E13)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
