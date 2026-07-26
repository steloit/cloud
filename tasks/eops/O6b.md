---
id: O6b
title: Make the by-name review invocation discoverable in CLAUDE.md
epic: EOPS
status: done
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
  - "git ls-files -s CLAUDE.md shows mode 120000 — the index is the right authority here, since this task only needs to know the edit was committed to both entry points; O6c's standing check reads the working tree instead, because that is what a session actually loads"
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

- [x] CLAUDE.md step 5a states the reviewers are invoked as
  `subagent_type: "reviewer"` and `subagent_type: "qa"`, with a pointer to
  `.claude/agents/README.md`.
- [x] The line is short enough to survive the file's token discipline — one
  clause, not a section (AGENTS.md 93 → 96 lines, cap 150).
- [~] **AGENTS.md and CLAUDE.md cannot fork.** *Premise corrected twice.
  `CLAUDE.md` is a **symlink** to `AGENTS.md` (git mode `120000`), not a copy, so
  a content `diff` can never detect a fork — it returns empty either way, which
  is what fooled both the original filing and me. But "structural" only holds
  while the mode does, and **nothing asserts the mode**: QA reproduced
  `rm CLAUDE.md && echo forked > CLAUDE.md && git add`, flipping it to `100644`
  and forking the content completely, with `validate.mjs` still printing `OK`.
  The real check is a mode assertion; it belongs in `validate.mjs`, outside this
  task's glob, and is **delivered in O6c** — not closed here.*
- [ ] No change to review POLICY, which is ADR-0008's and stays there.

## Out of scope

Rewriting the eight historical Outcomes. They are accurate records of what was
done at the time and must not be retconned.

## Related

ADR-0008 · O6

## Outcome

Added one clause to step 5a naming `subagent_type: "reviewer"` and
`subagent_type: "qa"`, pointing at `.claude/agents/README.md`, and stating that
the generic-runner workaround is retired even though older Outcomes still show
it. AGENTS.md grew 3 lines (93 → 96, cap 150).

The mirror premise was wrong twice, and the second correction matters more than
the first. `CLAUDE.md` is a symlink to `AGENTS.md` (mode `120000`), not a copy —
so the `diff`-based sync check filed from O6's review was worthless, since a
symlink diffs empty exactly as identical copies do. That is what fooled both of
us. But I then over-corrected to "the invariant is structural, so no check is
needed or wanted," and QA caught it: structural only holds *while the mode
does*, and nothing asserts the mode. QA reproduced the failure —
`rm CLAUDE.md && echo forked > CLAUDE.md && git add` flips it to `100644`, forks
the content, and `validate.mjs` still prints `OK`.

The degraded case is worse than a fork: under a `core.symlinks=false` checkout,
`CLAUDE.md` materializes as a **9-byte regular file containing the string
`AGENTS.md`**. A session auto-loading it gets nine bytes instead of the authority
order, hard rules, and task protocol — silently, with no error. So the criterion
is `[~]`, not closed: the real check is a git-mode assertion in `validate.mjs`,
outside this task's glob, and O6c now owns it.

Follow-ups: **O6c** gains `entrypoint-symlink` and `agents-readme-exists`, since
step 5a now points at a file no gate protects. Two corrections came out of the
re-review and are worth carrying: the check must assert the **working tree**
(`lstatSync`/`readlinkSync`, verified working) not `git ls-files -s`, because the
index stays `120000` under `core.symlinks=false` while the working tree holds the
9-byte file — the mechanism I first specified missed the exact outage that
justified it. And the delegated checks were added to O6c's *acceptance criteria*
only, while Done is defined as its `verify:` block passing, so O6c could have
closed green with the work unwritten; its `verify:` block now gates all of it.
**O6d** filed for the root of it — ADR-0008 still asserts reviewers are read-only
as unqualified fact, which O6 disproved; the ADR is the source every restatement
inherits, and step 5a's pre-existing "read-only" wording was left untouched here
rather than qualifying an ADR-owned claim from a steering file.

Evidence that this task was needed, beyond the eight historical Outcomes: while
O6 was still open the founder's own directive instructed running reviews through
`general-purpose` "until repository agents become first-class" — the retired
pattern, quoted from the only place it was written down. That is the third
independent instance of the drift, and the first from outside the task loop.
