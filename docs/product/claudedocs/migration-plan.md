# Migration Plan — Steloit AI-Native Engineering OS

**Status:** Executing · 2026-07-18 · Governed by `ai-native-workflow.md` (Part I + Part II amendments A1–A6)
**Repos:** `steloit/cloud` (target monorepo; local `~/Downloads/steloit-console`) · `steloit/steloit-handoff` (source of `docs/product/`; local `~/Downloads/steloit-handoff`)

## Principles (binding for every phase)
Small commits, one logical change each · verify after every step · restructuring commits never contain feature work · `git mv`/subtree to preserve history · all 192 issues, 25 milestones, 20 labels, and Project #1 metadata preserved (they are disposable *views*, but we don't discard them — we re-point them) · automation idempotent (every script re-runnable) · a `pre-phase-N` tag before each phase = the rollback point · a phase is entered only after the previous phase's exit criteria pass.

## Phase 0 — Safety net
- **Objective:** snapshot both repos; establish the working-state baseline.
- **Expected state:** unchanged trees; handoff's long-pending work committed as curated commits (its uncommitted state cannot ride a subtree); tags `pre-migration` on both.
- **Files:** none new (commits of existing work + tags).
- **Validation:** `git status` clean in both repos; `pnpm install && pnpm build && pnpm test` green in cloud (baseline proof); tags pushed.
- **Rollback:** tags are the rollback destination for everything later; Phase 0 itself is additive-only.
- **Exit criteria:** both repos clean, pushed, tagged; baseline build/test recorded.

## Phase 1 — Monorepo shape (`apps/console`)
- **Objective:** move the frontend to `apps/console/` under a pnpm workspace root, app fully working from its new home.
- **Expected state:** `cloud/` root = workspace files only; app builds/tests from `apps/console`.
- **Files:** `git mv` of all app files → `apps/console/`; new root `package.json` + `pnpm-workspace.yaml`; path fixes (vite/tsconfig/scripts) as their own commit.
- **Validation:** `pnpm install` at root; `pnpm --filter console build` + unit tests green; canon-mode dev server boots (spot check).
- **Rollback:** `git reset --hard pre-phase-1` (nothing else depends on it yet).
- **Exit criteria:** build + tests green from new layout; pushed.

## Phase 2 — Knowledge layer (`docs/`)
- **Objective:** fold the handoff package in with history; establish `docs/{product,plan,adr}`.
- **Expected state:** `docs/product/` = the handoff repo verbatim (internal 00–24 numbering untouched); plan + workflow + this file moved to `docs/plan/`; `docs/adr/README.md` seeded with the product-vs-engineering ADR boundary rule; `scripts/gh-project` relocated to `scripts/spec-sync/` (still v1).
- **Files:** `git subtree add --prefix=docs/product` + follow-up `git mv` commits.
- **Validation:** `git log --follow docs/product/README.md` shows handoff history; frame-id grep works (`grep -c 'U6' docs/product/00-sources/Steloit-Console-Screens.html` > 0); no path in `docs/product/` altered except the claudedocs/scripts extractions.
- **Rollback:** revert the subtree merge commit + the mv commits.
- **Exit criteria:** history-preserving fold verified; pushed. (Handoff repo is archived in Phase 6, not now.)

## Phase 3 — Steering layer
- **Objective:** the always-loaded contract: root `AGENTS.md` (≤150 lines) + `CLAUDE.md`/`GEMINI.md` symlinks; `apps/console/AGENTS.md` (from its existing CLAUDE.md); `.cursor/rules/steloit.mdc`; `.github/CODEOWNERS` (00-sources, decisions.md, docs/adr → founders); PR template; `.claude/` hooks (protect human-only paths; verify gate) and the three v1 skills (task-pickup, spec-author, verify).
- **Validation:** `wc -l AGENTS.md` ≤150; symlinks resolve; CODEOWNERS paths exist; a scripted hook dry-run blocks a write to `docs/product/00-sources/`.
- **Rollback:** revert commits (additive phase).
- **Exit criteria:** all steering files present and within budgets; pushed.

## Phase 4 — Execution layer (`tasks/` + `contexts/`)
- **Objective:** generate all task stubs from today's issue data; seed the eight Context Packs.
- **Expected state:** `tasks/` = `_template.md`, per-epic dirs with `_epic.md` + one stub per work item (frontmatter incl. `issue:` number from `.state/issue-map.json`, `contexts:` assignments); `contexts/` = README index + 8 packs (v1 content, ≤150 lines each); `scripts/spec-sync/task.schema.json`.
- **Validation:** stub count = 175 + 17 epics; schema validation green over every file; every `deps:` id resolves to an existing task; every `contexts:` entry resolves to a pack; ready-set computation runs and matches the known Sprint-0/1 unblocked set.
- **Rollback:** revert commits (additive).
- **Exit criteria:** all validations green; pushed.

## Phase 5 — Sync v2 + CI
- **Objective:** the one-way projection pipeline (A1) live.
- **Expected state:** `scripts/spec-sync/sync.mjs` (single-file Node, no deps: parse frontmatter → upsert issue bodies in §7 projection format → set Project fields → close on `status: done`); `validate.mjs` (schema + deps + caps); workflows `ci.yml` (console tests + validate), `spec-sync.yml` (push-to-main on `tasks/**`), `converge.yml` (weekly caps/drift/archive sweep).
- **Validation:** dry-run diff on 3 issues → full run regenerates all 192 bodies; spot-check US-3.2's issue shows spec-path + `<details>` closure; **idempotency: immediate second run = zero writes**; board fields unchanged where frontmatter agrees.
- **Rollback:** issues are disposable views (A1) — any bad sync is fixed by re-running; worst case `run-all.sh` (v1) still exists at the same commit.
- **Exit criteria:** idempotent sync proven; CI green on a no-op PR; pushed.

## Phase 6 — Cutover
- **Objective:** make the new system the only system.
- **Steps:** enable merge queue + branch protection on `cloud@main` (for feature work; migration is done) · archive `steloit/steloit-handoff` on GitHub (read-only; final commit points to `steloit/cloud/docs/product/`) · update memory/vault notes · Wave-1 enrichment kickoff (Sprint-0/1 tasks → `ready` via spec-author; **post-migration work, tracked as tasks, not part of this plan**).
- **Validation:** full checklist — clean clone of `cloud` + `pnpm install` + build + validate green; a test branch PR shows CI + template + CODEOWNERS behavior; archived handoff rejects pushes.
- **Rollback:** un-archive is one API call; merge queue toggles off; everything before is tagged.
- **Exit criteria:** checklist green → migration complete; Sprint 0 proceeds inside the new system.
