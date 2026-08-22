---
name: verify
description: Run the active Steloit task's executable definition of done. Use before opening or updating any PR, or when asked to verify work.
---

# Verify

1. Identify the active task (branch `task/<id>` → `tasks/**/<id>*.md`).
2. Run **every** command in its `verify:` block. Then `node scripts/spec-sync/validate.mjs`.
3. All green → paste the output into the PR's Evidence section.
4. Any red → fix and re-run. Never weaken a check, skip a test, or edit an AC to make it pass —
   if a check is genuinely wrong, that's a finding for the PR + a spec change, not a local edit.
5. Frontend tasks additionally keep the console suites green: `pnpm --filter console test` and
   the UI-consistency checks.
6. **Negative evidence — state which mutation kills each new guard.** A green suite proves
   the code runs; it does not prove a test DISCRIMINATES. For every guard the task
   introduces, record in the PR the mutation you ran and the failure message you observed.
   A guard with no recorded mutation is exercised, not verified.

   One mutation is evidence about one mutation. If the guard covers data with more than
   one representation (raw/encoded, present/absent/dropped, two rendered surfaces), run
   one per representation — see the mutation-class rule in `.claude/agents/qa.md`. State
   WHERE you ran them: AGENTS.md requires a throwaway copy, and since a mutate-then-restore
   leaves no diff, this line is the only place a tree mutation becomes visible.
7. **A skipped test is not a pass.** If a verify command can print `ok` while its tests
   skip or match nothing, it is not a gate. In US-1.3 integration tests were reported as
   "PASSED against real Postgres" when they had **silently skipped** — `DOCKER_HOST` was
   unset for colima — and the suite still printed `ok`. The same shape hides a renamed
   test: `go test -run X` exits 0 with `[no tests to run]`. CK-M3 carries the pattern that
   closes both: make the skip fatal under an env var the verify block sets, and grep for
   the pass line rather than trusting the exit code:

   ```
   STELOIT_CHECKPOINT=1 go test -count=1 -run X -v ./pkg/ 2>&1 | grep -q '^--- PASS: TestX'
   ```
8. **Say what the evidence needs to reproduce.** If a suite depends on a container runtime,
   record the invocation (`DOCKER_HOST=…`) alongside the result. "Zero skips" is not
   checkable by a reader who cannot reproduce the environment that produced it.
