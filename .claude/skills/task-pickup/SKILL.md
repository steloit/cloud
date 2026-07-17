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
