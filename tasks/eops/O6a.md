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
  - "either kernel:* invocation is refused by configuration, or the absence of any subagent_type matcher is evidenced and the task closes [~]"
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

Resolve the Open question below **first** — it decides which branch applies.

- [ ] *If a subagent matcher exists:* invoking a `kernel:*` subagent is refused
  by `.claude/settings.json` configuration, not by convention; the refusal names
  ADR-0008 so the reader learns why, not just that; and
  `.claude/agents/README.md` drops its "nothing currently enforces this" caveat
  and points at the mechanism.
- [ ] *If no matcher exists:* close `[~]` with the evidence — what was tried,
  what the harness does instead — and leave the README caveat standing. Do not
  invent a mechanism, and do not mark the criterion met.
- [ ] Either way: `reviewer`, `qa`, `research`, `docs` are unaffected.

## Open question

Whether `permissions.deny` can match on a subagent invocation is **unverified**.
O6 confirmed only that every existing deny/ask rule is `Bash`-scoped. The
argument name to match on is likewise unconfirmed in-repo — establish the real
one from the harness rather than assuming `subagent_type`. If no such matcher
exists, that is a legitimate finding, not a failure: record it and close `[~]`.

## Related

ADR-0008 · O6 · docs/plan/kernel-workflow-review.md
