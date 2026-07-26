# ADR-0002 · AI-native engineering operating system

**Status:** Accepted · 2026-07-18 · Founder-approved
**Decision:** Engineering runs on the system specified in [`docs/plan/ai-native-workflow.md`](../plan/ai-native-workflow.md) (Part I as amended by Part II A1–A6) and executed by [`docs/plan/migration-plan.md`](../plan/migration-plan.md):

- **The repository is the single source of truth.** GitHub Issues and the Project board are generated, disposable views (one-way overwrite sync — A1); hand-edits there are defined as lost.
- **Task files** (`tasks/**.md`, YAML frontmatter + closure body, schema-validated) are the execution contract; stubs graduate to `ready` by **just-in-time enrichment** one wave ahead, never in bulk.
- **Context Packs** (`contexts/`, ≤150 lines, ≤12 packs) carry cross-cutting domain knowledge and the per-domain mistake banks; directory knowledge lives in nested AGENTS.md; procedures live in skills (A2).
- **Verification is executable:** each task's `verify:` block is its definition of done, run in CI and by agents before any PR.
- **History lives in living files:** PR = record, `## Outcome` on the task, lessons routed to packs/AGENTS.md/ADRs, done tasks archived — no logs or graveyards (A4).
- **Claim-by-branch** (`task/<id>`), file-ownership scheduling from `files:` globs, merge queue at first feature wave (A6).

**Context:** Two research sweeps of 2025–26 practice plus an adversarial re-review; full evidence and rejected alternatives (Linear, heavyweight spec generation, packs-as-skills, devlogs) are in the workflow document.

**Consequences:** Issues carry pointers + a folded generated spec; the board is a human dashboard only; `scripts/spec-sync/` is the projection machinery; caps (AGENTS.md ≤150 lines, pack budgets) are CI-enforced so the steering layer stays small for years.
