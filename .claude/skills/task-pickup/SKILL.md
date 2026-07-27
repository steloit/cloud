---
name: task-pickup
description: Pick up and execute a Steloit work item from tasks/. Use when asked to "take the next task", "work on <task-id>", or start implementation work in this repo.
---

# Task pickup

1. **Resolve ready work.** A task is ready when its file has `status: ready` AND every id in
   `deps:` has `status: done` AND no branch `task/<id>` exists on origin. If the user named a
   task, verify it is ready — if not, stop and say which dep or status blocks it.
2. **Claim.** Create branch `task/<id>` (worktree recommended: `git worktree add ../wt-<id> -b task/<id>`),
   flip `status: in-progress` in the task file, commit, push. The pushed branch is the claim.
3. **Load context — in this order, nothing more:** the task file → each pack in `contexts:` →
   each Read-first entry. Respect the task's and AGENTS.md's don't-read lists.
4. **Implement** within the task's `files:` globs. Follow the task's Approach order. Anything
   discovered outside scope becomes a finding or follow-up task, not a detour.
5. **Verify.** Run every command in the task's `verify:` block. All must pass. Capture output.
6. **Close.** Append `## Outcome` (5–10 lines: shipped, deviations, follow-ups) and flip
   `status: done` in the same branch. Open the PR with the template: id in title, evidence pasted,
   spec-conflicts section honest.

When you request review (ADR-0008), give the reviewers two things beyond the diff:

- **The blast radius.** Which endpoints, contracts or clients change behaviour —
  *including outside the task's stated scope*. A validation added to a shared path applies
  everywhere that path is called: US-3.7 added an intent check inside `resolve()`, which
  `Price`/`PriceAll` also call, making it a breaking change to `POST /v1/estimates` that
  the builder did not notice and QA had to surface. If you cannot name the radius, you do
  not yet know what you changed.
- **Where you ran your fault injection.** AGENTS.md requires a throwaway copy; if you
  nonetheless mutated the tree, say so here. A mutate-then-restore leaves no diff, so this
  self-report is the only detector that exists.
- **The mutations you already ran**, and what each covers. This is CONTEXT, not a
  skip-list: E3's most serious defect was found by a reviewer re-running the builder's own
  plaintext-scan against a DIFFERENT representation of the same data (the body, which
  base64-encodes, where the builder had only covered the header). Re-running a mutation
  against a representation the builder did not consider is a known-productive move —
  saying which representation each of yours covered is what makes that cheap to spot.

**When a reviewer fails to return.** A stalled or errored review agent is NOT an approval —
it produced no verdict, so ADR-0008's gate has not been satisfied. Retry twice; if it still
does not complete, surface it as a blocked merge and say which reviewer is missing. In
US-3.7 four consecutive agent failures (one API drop, two watchdog stalls, one mid-run drop)
made this a live question, and the one time a merge on a single reviewer was nearly taken,
the late report contained the most serious defect of the session. Note this pipeline is not
cheap — seven review dimensions, per-representation mutations, blast radius — so stalls are
likelier than they were; that is a reason to write the rule down, not to relax it.
