---
id: O6a
title: Enforce the kernel prohibition, don't just document it
epic: EOPS
status: ready
phase: V1
priority: medium
sprint: 1
estimate: 0.25ew
deps: [O6]
issue: 0
labels: [DevOps, Tooling]
module: Engineering OS
contexts: []
files:
  - .claude/settings.json
  - .claude/agents/README.md
verify:
  - "invoking subagent_type 'kernel:reviewer' is refused by configuration, not by convention"
  - "invoking 'reviewer' and 'qa' still succeeds unchanged"
owner: agent
---

## Goal

ADR-0008 forbids any external, plugin, or machine-level reviewer standing in for
`reviewer`/`qa`, and names `kernel:*` as explicitly rejected. Today that
prohibition is **documented and nothing more** — `kernel:reviewer` remains
listed and invokable, because it is installed at the machine level and O6 could
only reach `.claude/agents/**`. Close the gap with repo-level configuration.

## Why

O6 verified that bare names resolve to repo agents, so a plugin cannot *silently*
substitute. But nothing stops a session from invoking `kernel:reviewer`
deliberately — which already happened once and is the drift O6 was filed about.
Visibility is not permission, and a prohibition enforced only by prose is one
distracted session away from being violated again.

## Acceptance criteria

- [ ] Invoking a `kernel:*` subagent is refused by `.claude/settings.json`
  configuration (deny rule or equivalent), not by convention.
- [ ] The refusal names ADR-0008 so the reader learns why, not just that.
- [ ] `reviewer`, `qa`, `research`, `docs` are unaffected.
- [ ] `.claude/agents/README.md` drops the "nothing currently enforces this"
  caveat and points at the mechanism.

## Open question

Whether `permissions.deny` matches on the `Agent` tool's `subagent_type`
argument is **unverified** — O6 confirmed only that the existing deny rules are
`Bash`-scoped. Establish that first; if the harness offers no such matcher, that
is a legitimate finding and this task closes as "not achievable from repo
config", recording the evidence rather than inventing a mechanism.

## Related

ADR-0008 · O6 · docs/plan/kernel-workflow-review.md
