---
id: O6b
title: Make the by-name review invocation discoverable in CLAUDE.md
epic: EOPS
status: ready
phase: V1
priority: high
sprint: 1
estimate: 0.25ew
deps: [O6]
issue: 0
labels: [DevOps, Tooling, Docs]
module: Engineering OS
contexts: []
files:
  - CLAUDE.md
  - AGENTS.md
verify:
  - "CLAUDE.md step 5a names subagent_type 'reviewer' and 'qa' explicitly"
  - "diff AGENTS.md CLAUDE.md is empty — the two files are byte-identical mirrors"
  - "a session reading only CLAUDE.md invokes the pipeline by name without re-deriving it"
owner: agent
---

## Goal

CLAUDE.md §5a mandates the ADR-0008 review pipeline but never says **how** to
invoke it. Name the two `subagent_type`s (`reviewer`, `qa`) and point at
`.claude/agents/README.md`.

## Why

O6 retired the workaround of running the reviewer/qa prompts through a generic
`general-purpose` runner. But the retirement is undiscoverable: a session
grepping `tasks/` for review precedent finds the **old pattern modelled eight
times** (T12.5, T12.7, US-10.2, US-11.1/.2/.5/.6/.7) and its retirement zero
times, because those Outcomes are permanent history. Documentation that the
reader never reaches has not shipped, and precedent beats prose every time.

This is the same failure mode O6 itself fixed one level down — the contract
existed in the harness but nowhere in writing, so every session re-derived it.

## Acceptance criteria

- [ ] CLAUDE.md step 5a states the reviewers are invoked as
  `subagent_type: "reviewer"` and `subagent_type: "qa"`, with a pointer to
  `.claude/agents/README.md`.
- [ ] The line is short enough to survive the file's token discipline — one
  sentence, not a section.
- [ ] **AGENTS.md receives the identical edit.** The two files are currently
  byte-identical mirrors (`diff AGENTS.md CLAUDE.md` is empty, 93 lines each) and
  nothing pins that — `validate.mjs` checks only AGENTS.md's line cap. Editing
  one and not the other silently forks the repo's two entry points, and this task
  is the first to edit either.
- [ ] No change to review POLICY, which is ADR-0008's and stays there.

## Out of scope

Rewriting the eight historical Outcomes. They are accurate records of what was
done at the time and must not be retconned.

## Related

ADR-0008 · O6
