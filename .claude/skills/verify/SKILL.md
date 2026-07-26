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
