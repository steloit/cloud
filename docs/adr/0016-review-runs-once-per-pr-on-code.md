# ADR-0016 — The review pipeline runs ONCE per PR, on code

**Status:** Accepted (founder, 2026-08-24) · Amends ADR-0008

## Decision

ADR-0008's two independent reviewers (`reviewer`, `qa`) stay mandatory for every
significant PR. Two things change:

1. **Once per PR, on the final diff** — not after every round. ADR-0008 already
   says "run on the branch diff *before* merge"; the practice had drifted into a
   pass after each round, which is what made a PR cost fifteen of them.
2. **Code and behavioural claims only.** Task-file narrative, mutation counts,
   citation numbering and section restoration are OUT of review scope. Those get
   one self-checked pass by the implementer before the review runs.

Blocking findings are still fixed and re-verified with measurement, and the
evidence still goes in the PR. What is dropped is re-reviewing the fix to a
finding the reviewer already named.

## Why

Measured over one session of E3 work. The reviewers found, in code that was
already green under its own tests:

- a status mapping that made the agent declare a generation converged **mid-apply**,
  so a failed upgrade would never be observed (found independently by both);
- a migration whose unguarded cast **aborts** on a legacy row — dirty
  `schema_migrations`, deploy stopped;
- the same migration never bumping `generation`, so the fleet is corrected in the
  database and not on the cells;
- a storage quota that made the API return **201** for a service the cell can
  never admit — priced and billed;
- an invoice total frozen at `MaxInt64`, permanently, behind `ON CONFLICT DO NOTHING`;
- a CI gate neuterable by inserting one line, **twice**, one line apart;
- a text pin bypassable by parking the expression in a comment.

Several are money or a stopped deploy. None was caught by a green suite. That is
the case for keeping both reviewers.

The case for scoping them: whole rounds went on counts, citations and restoring
task-file sections — real accuracy, near-zero risk — and four rounds went on
undoing a change that was scope creep past the task in the first place. Neither
is review's cost; both are cheaper to fix by not generating them.

## Consequences

- A PR gets one review pass. If the fixes to its findings are themselves
  substantial and behavioural, that is a judgement call for a second pass — not
  a default.
- Implementers own record accuracy. A count restated rather than re-derived, or
  a citation to something that does not exist, is still a defect; it is just not
  a reviewer's job to find it.
- **Scope discipline is the cheapest lever.** The worst defect of the session was
  an improvement nobody asked for. Do the task.
