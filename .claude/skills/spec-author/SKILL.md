---
name: spec-author
description: Enrich a Steloit task stub into a ready implementation spec. Use when asked to "enrich", "spec out", or "make ready" a task, or to prepare the next wave of tasks.
---

# Spec authoring (stub → ready)

Enrichment is just-in-time: only enrich tasks in the next wave (deps done or one wave out).
Never bulk-enrich — distant specs rot.

1. **Mine the owners** for this task's concern (authority order in AGENTS.md): the F-spec section,
   openapi.yaml paths, models.md tables, relevant ADRs, frame ids, QA scenarios.
2. **Fill the template** (`tasks/_template.md`) — every section. Rules:
   - Reference by path; never paste spec content that has an owner.
   - `contexts:` before Read-first: if three tasks need the same explanation, it belongs in a pack,
     not in this file (extend the pack in the same PR).
   - `files:` globs = the intended touch-set (drives parallel scheduling — be honest, not broad).
   - `verify:` = commands that fail before the work and pass after. No unverifiable ACs.
   - Common mistakes: task-specific only (3–5); domain traps go to the pack's mistake bank.
3. **Budgets:** body 80–250 lines, hard cap 300. Litmus: would removing this line cause a mistake?
4. **Validate:** `node scripts/spec-sync/validate.mjs` must pass. Flip `status: ready`.
5. **PR it** — enrichment is reviewable work like any other; a founder approves specs for
   critical-path tasks.
