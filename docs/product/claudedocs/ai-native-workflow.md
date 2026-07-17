# Steloit — AI-Native Engineering Workflow (Proposal)

**Status:** Proposal for founder review · 2026-07-18
**Scope:** The engineering operating system for building Steloit over the next several years, on the assumption that AI coding agents perform most implementation work while humans hold architecture, product direction, review, and decisions.
**Inputs:** two deep research sweeps (2025–26 practice, ~40 primary sources), a re-review of the product corpus (handoff package, implementation plan, console repo, findings ledger, the GitHub Project built today), and the org's own prior art (the handoff's `22-agents/agent-guide.md`; the other Steloit workspace's wiki-as-spec pattern).

---

## 1 · Research findings (what actually works, with evidence)

Full sweeps are summarized here; the load-bearing findings:

**1.1 Two schools; pick per task size.** Ceremony-heavy spec-driven tooling (GitHub Spec Kit) measurably loses on small/medium work — Scott Logic benchmarked 4× agent time and 14× review time versus direct prompting, and called it "reinvented waterfall." The opposite school (Steinberger's OpenClaw: "just talk to it," 6,000+ agent commits/month) loses on multi-session, multi-agent, long-horizon work. The stable synthesis, visible across Kiro, Beads, Backlog.md, and Ralph loops: **small permanent steering layer + atomic in-repo tasks with explicit dependency edges + full specs only for work that crosses session boundaries.**

**1.2 Filesystem is agent memory; git is the database.** Every successful long-horizon pattern stores backlog, progress, and learned recipes in versioned files — never in chat context or a SaaS tracker. Beads (Yegge) is a git-backed DAG issue tracker whose core query is "ready work" (unblocked nodes); Ralph loops run one item per fresh context with `specs/*` + a plan file as the only memory.

**1.3 Context rot is the governing constraint.** Measured: effective context ≪ advertised (most models hold ≥85% quality only to ~2K tokens, NoLiMa); instruction adherence decays past a few hundred instructions (IFScale: 68% at 500); analysis of 2,500 AGENTS.md files found **~300–350 words median for well-performing files, >1,000 words negatively correlated**. Anthropic's own guidance: CLAUDE.md under ~200 lines, "would removing this line cause a mistake? If not, cut it." The winning doc architecture is a **router**: ~100-token metadata → <500-line entry file → one-level-deep references → executable scripts that never enter context.

**1.4 Issue content: hybrid beats both extremes.** An arXiv study of Copilot coding-agent merges found merge rates rise when issues name relevant files/modules (+6.4%), subsystems/functions (+7.2%), and a suggested approach (+6.1%). Pure pointer-issues underperform; monster spec-dumps don't scale. The winning unit is **atomic (one PR, ~1 hour human-equivalent, few hundred LOC)** — METR: agents succeed ~100% on tiny tasks, <10% on >4-hour tasks, and success decays exponentially with length, so N short independently-verified subtasks dominate one long run. Size to the 80% horizon, not the 50%.

**1.5 The interop standard is AGENTS.md.** 60k+ repos, Linux Foundation stewardship, read natively by Codex, Copilot, Cursor, Gemini CLI, Amp, Aider, Zed. Nesting: closest-file-wins; Codex caps combined size at 32 KiB. Common practice: AGENTS.md as source of truth, `CLAUDE.md`/`GEMINI.md` as symlinks/pointers — exactly the handoff's "one room, many doors."

**1.6 Verification must be structural, not asserted.** Codex runs tests listed in AGENTS.md before finishing; Claude Code Stop-hooks can block turn-end until checks pass; Kent Beck's warning ("the genie wants to write code and then tests that pass") argues for executable definitions of done. An ICSE 2026 study found a third of SWE-bench issues leaked their solutions — apparent agent success halves under clean evaluation. **Spec + harness quality is most of agent success.**

**1.7 GitHub is becoming the agent execution plane, not the planning plane.** Issues are what cloud agents consume (assign to Copilot, tag `@codex`); GitHub Projects is invisible to agents; plan artifacts belong in-repo. File→issue sync is still DIY (`gh` scripts, spec-kit's `taskstoissues`, mitsuhiko's gh-issue-sync) — which we've already half-built.

**1.8 Parallel agents partition by file ownership.** Worktrees isolate files but "do not make agents agree on architecture" — shared contracts (API schemas, types, migrations) must be fixed before fan-out; practical ceiling 3–5 concurrent agents before human review becomes the bottleneck.

---

## 2 · Review of the current system

What exists after today:

| Layer | State | Judgment |
|---|---|---|
| Design authority | `steloit-handoff` — 152 frames, openapi.yaml, models, F1–F13, RBAC, QA invariants, canon, ADR log, agent-guide | **Excellent and rare.** The console was already built by agents from it with high fidelity — proof the spec-driven core works for this product. Keep as authority; problem is *location and linkage*, not content. |
| Strategy | `claudedocs/implementation-plan.md` — modules, epics, 175 work items, DAG, sprints, estimates | Right content, right altitude. Wrong home (session-docs dir of the spec repo) and no machine linkage to execution. |
| Execution | 192 GitHub issues + 25 milestones + 20 labels + org Project #1 with 7 custom fields | Faithful mirror of the plan — for humans. See §3 for why it under-serves agents. |
| Code | `steloit/console` (frontend, complete vs mocks) · backend **homeless** · `steloit-go`? (not the product) | The backend's greenfield moment is *now* — the cheapest time this structure will ever be to set. |
| Org prior art | handoff `agent-guide.md` (authority order, hard rules, stop-and-ask) · other workspace's "wiki-as-spec, reference-by-path, CLAUDE.md→AGENTS.md symlink" | Both validate the proposed direction; neither covers *execution* (tasks, status, verification). |

---

## 3 · Critique: where today's system fails agents

Honest assessment, including of what I built this afternoon:

1. **Issue bodies are summaries, not closures.** Sample (US-3.2): 4 lines + one AC. An agent must independently rediscover feature-specs.md §F2, openapi.yaml, models.md, and QA scenario — several hundred thousand tokens of exploration *per task* that could be precomputed once.
2. **The context an issue needs lives in a different repo than the code will.** Issues sit in `steloit/console`; the spec corpus sits in `steloit-handoff`; the plan sits in `claudedocs/`. No checkout contains everything. Cloud agents (Codex, Copilot) clone *one* repo.
3. **Issue bodies are not version-controlled.** They were generated from `data/issues.json`, but edits on GitHub won't flow back. Two sources of truth from day one; drift guaranteed. (Stale specs "actively mislead agents" — the documented #1 failure mode of spec-driven repos.)
4. **Dependencies are comments, not data.** "⛔ Blocked by #12" is human-readable; no agent or script can compute the ready set from it.
5. **No executable definition of done.** Acceptance criteria are prose. Nothing tells an agent *which commands prove completion* — the single highest-leverage field per the research.
6. **No file paths, no read-lists.** Partly unavoidable (backend code doesn't exist), but the format has no slot for them, and named files are worth +6–7% merge rate.
7. **Sprints are a human cadence imposed on agents.** Agents don't need Sprint 9; they need the DAG. (Sprints stay — but as a *human forecast view*, not an execution input.)
8. **Estimates in engineer-weeks are meaningless to agents** and were already ±40% for humans. Keep for founder planning; agents ignore.
9. **The 175-item flat list buries the enrichment question.** Fully specifying all 175 upfront would be the Spec Kit waterfall trap (and specs for Sprint-14 work would rot before use). There is no concept of *when* a task graduates from stub to implementable.
10. **GitHub Projects is doing no work for agents** (and never will — research is unambiguous). It's fine as a human dashboard; it must not be load-bearing.
11. **What's right and must be kept:** the plan's DAG and epic decomposition; labels/milestones as metadata; the `key → issue-number` map in `.state/issue-map.json` (the migration's join table); the handoff's authority order and single-owner rule; canon + QA invariants as executable truth. Nothing done today is wasted — it becomes seed data (§11).

---

## 4 · The recommended system: one repo, three layers, one loop

**Design principle:** *the repository is the single source of truth; GitHub is a projection of it; agents read files, humans read dashboards.*

```
LAYER 1 — STEERING (always loaded, tiny)
  AGENTS.md (≤150 lines, root) + nested AGENTS.md per package
  = commands, authority order, task protocol, hard rules

LAYER 2 — KNOWLEDGE (load-on-demand, authoritative)
  docs/product/   ← the handoff package (frames, openapi, models, F-specs, canon, QA, ADRs)
  docs/adr/       ← engineering ADRs (stack, infra — cites INF-001 decision IDs)
  docs/plan/      ← implementation-plan.md (strategy; humans + spec-authoring agents read it)

LAYER 3 — EXECUTION (the working set)
  tasks/          ← one file per work item: YAML frontmatter (machine) + body (agent)
  scripts/spec-sync/  ← tasks ⇄ issues ⇄ project (evolved from today's gh-project scripts)

THE LOOP
  plan → task stubs → JIT enrichment (one wave ahead; agent-authored, founder-reviewed)
       → sync to issues/board → agent implements in a worktree (or cloud agent via issue)
       → PR (branch = task id; CI runs the task's verify block)
       → fresh-context agent review + founder review on protected paths
       → merge (task file's status flips to done in the same PR — status changes are commits)
```

What each party consumes:
- **A local agent** (Claude Code/Cursor/Gemini): `AGENTS.md` + one `tasks/**.md` file + the read-list inside it. Nothing else upfront.
- **A cloud agent** (Codex/Copilot): the issue — whose body is *generated from* the task file and contains it inline (cloud agents get the full closure without hunting).
- **A founder:** the Project board, the milestones, the plan doc.
- **CI:** the task's `verify:` commands + the standing suites (canon invariants, contract tests, RBAC sweep).

Why this passes the research bar: steering layer ≤150 lines (context-rot budget); tasks are atomic closures with named files and executable DoD (the +merge-rate factors); DAG in frontmatter (ready-set computable, Beads-style); enrichment is just-in-time (no waterfall, no spec rot); everything versioned (drift becomes diffs); GitHub is a projection (no second source of truth).

---

## 5 · Repository structure

**Recommendation (founder call #1): evolve `steloit/console` into the product monorepo and rename it `steloit/cloud`.** GitHub renames preserve issues, PRs, and redirects — today's 192 issues, milestones, and board survive untouched. The frontend moves to `apps/console` in one history-preserving commit. Rejected alternatives: a fresh repo (orphans today's issue infrastructure; splits history), staying multi-repo (agents can't atomically change contract + server + client; cloud agents clone one repo).

**Founder call #2: fold the handoff package in as `docs/product/` (git-subtree merge, history preserved), then archive the standalone repo.** Its reason to be separate — "no code exists yet" — expires now. The S-process (spec rulings) then amends `openapi.yaml` in the same repo the generated clients live in, and one checkout contains everything any agent needs. Governance survives via CODEOWNERS (below). *Lighter fallback if not folding:* vendor by subtree with a sync script; upstream stays canonical — workable but reintroduces two-repo drift.

```
steloit/cloud
├── AGENTS.md                     # root steering (≤150 lines) — see §9
├── CLAUDE.md → AGENTS.md         # symlink (org precedent)
├── GEMINI.md → AGENTS.md         # symlink; Gemini settings also accept fileName: ["AGENTS.md"]
├── .cursor/rules/steloit.mdc     # 5-line pointer to AGENTS.md, alwaysApply
├── .claude/
│   ├── skills/                   # task-pickup · spec-author · verify · release
│   ├── hooks/                    # protect docs/product/00-sources; Stop-hook verify gate
│   └── agents/reviewer.md        # fresh-context reviewer subagent
├── .github/
│   ├── workflows/                # ci.yml · spec-sync.yml · converge.yml (drift check)
│   └── CODEOWNERS                # docs/product/00-sources/** + 18-philosophy/** + docs/adr/** → founders
├── apps/console/                 # today's frontend, moved; keeps its own AGENTS.md
├── services/
│   ├── api/                      # ONE deployable: modular monolith (control-plane, billing,
│   │   │                         #   observe, assistant as internal modules — see Part II A5;
│   │   │                         #   split into services only on measured triggers, per
│   │   │                         #   INF-001 "cheap on capacity, never on shape")
│   │   └── src/{identity,orgs,provisioning,billing,observe,assistant}/
│   └── cell-agent/               # genuinely separate (runs inside the cell, D9 reconciler)
├── packages/
│   ├── contracts/                # types generated FROM docs/product/08-api/openapi.yaml (owner unchanged)
│   └── canon/                    # 19-canon fixtures + arithmetic invariants as importable test utilities
├── contexts/                     # reusable Context Packs (Part II A2) — cross-cutting domain knowledge
│   ├── README.md                 # pack index (one line each)
│   ├── provisioning.md  rbac.md  billing.md  events-spine.md  api-conventions.md
│   ├── canon-testing.md  frontend-console.md  ai-plane.md
├── infra/terraform/
├── docs/
│   ├── product/                  # ← the handoff package, whole — internal numbering (00–24)
│   │                             #   deliberately UNCHANGED: every cross-reference, frame id,
│   │                             #   and ADR citation in the corpus depends on those paths
│   ├── adr/                      # engineering ADRs (stack/infra); product ADRs STAY in docs/product/18-philosophy/decisions.md
│   └── plan/implementation-plan.md
├── tasks/                        # §6 — the execution layer
│   ├── _template.md
│   ├── _archive/                 # done tasks swept here by converge.yml after a sprint (Part II A4)
│   ├── e0-setup/ … e14-data-plane/, eqa/, eops/   (each with _epic.md for shared design)
└── scripts/spec-sync/            # evolved from scripts/gh-project; includes task.schema.json (CI-validated frontmatter)
```

Boundary rule (prevents the two-ADR-logs smell): **product/design decisions** → `docs/product/18-philosophy/decisions.md` (unchanged owner); **implementation decisions** (framework, library, infra shape — citing INF-001 D-IDs per §8 of the constitution) → `docs/adr/`. Each log cross-references the other; neither duplicates.

---

## 6 · Implementation specification format

One file per work item in `tasks/`. Two maturity levels, one format:

- **Stub** (~20–30 lines): frontmatter + goal + AC — what today's issues contain. All 175 exist from day one (generated, §11).
- **Ready** (~100–250 lines, hard cap 300): the full closure. A task must be `ready` before an agent may start it. Enrichment happens **one wave ahead** of implementation (JIT — never all upfront), authored by an agent running the `spec-author` skill against `docs/`, reviewed by a founder. Research basis: >1,000-word instruction files degrade performance; specs written months ahead rot; BMAD/ACE-FCA's "compile a story file per task" is the working pattern.

### Frontmatter (machine-readable; drives sync, scheduling, CI)

```yaml
id: US-3.2
title: Estimate before provision — impossible to skip at the API layer
epic: E3            # epic file: tasks/e3-provisioning/_epic.md
status: ready       # stub | ready | in-progress | in-review | done | blocked
phase: MVP          # MVP | V1 | Future
priority: critical  # critical | high | medium | low
sprint: 4           # human forecast only — agents ignore
estimate: 1ew       # human planning only — agents ignore
deps: [T3.1, S7]    # DAG edges; ready-set = status ready ∧ all deps done
issue: 141          # synced by spec-sync; never hand-edited
labels: [Backend, Billing]
module: M4-provisioning
contexts: [provisioning, api-conventions, canon-testing]   # Context Packs (Part II A2) — load before Read-first
files:              # planned touch-set → enables conflict-free parallel scheduling
  - services/api/src/estimates/**
  - services/api/src/services/create.ts
  - packages/contracts/        # regenerated, not hand-edited
apis: [POST /estimates, POST /envs/{env}/services]
tables: [estimates, services]
events: [estimate.created, service.created]
tests:
  - services/api/test/estimates.spec.ts
  - packages/canon/test/invariants.spec.ts
verify:             # the executable definition of done — CI runs these on the PR
  - pnpm --filter @steloit/api test estimates
  - pnpm --filter @steloit/canon check-invariants
  - pnpm typecheck
owner: agent        # agent | founder-a | founder-b
```

### Body sections (agent-readable; reference-by-path discipline throughout)

1. **Goal** — one sentence, outcome not method.
2. **Why** — 1–3 lines of business context (the "soul" sentence; agents write better code when they know what must not break).
3. **Read first** — 3–7 paths with a one-line *why* each, **plus a don't-read list**. This is the token-budget control: the spec routes; it never restates `feature-specs.md`.
4. **Approach** — expected implementation order, 5–10 steps. Named interfaces, state transitions, the pattern-example file to imitate ("follow `X.ts`" beats prose).
5. **Edge cases** — enumerated, each mapped to an AC or test.
6. **Security / performance notes** — only deltas from the standing rules in AGENTS.md.
7. **Acceptance criteria** — testable, WHEN/THEN-shaped, checkboxed.
8. **Validation** — restates `verify:` plus any manual evidence required (e.g., "paste the 402 response body in the PR").
9. **Common mistakes** — *task-specific* traps only (3–5); domain-level traps live in the referenced context packs' mistake banks (Part II A2/A4), so they're written once and loaded by every task in the domain.
10. **Out of scope** — explicit, with the task id that owns each excluded concern.
11. **Related** — sibling task ids, relevant ADRs, frame ids.
12. **Outcome** — *appended at completion* (Part II A4): 5–10 lines — what shipped, deviations from spec, follow-up tasks filed. The only per-task history kept; everything else lives in the PR.

The litmus test for every line (from Anthropic's guidance): *would removing it cause the agent to make a mistake? If not, cut it.* And for the whole file: *could a competent engineer who has never seen this repo complete and prove the task from this file plus its read-list alone?*

---

## 7 · GitHub Issue format

Issues become **projections of task files** — regenerated by spec-sync, never hand-edited (a bot comment on each issue says so).

```
Title: US-3.2 · Estimate before provision — impossible to skip at the API layer

Body:
📄 Spec: tasks/e3-provisioning/US-3.2-estimate-gate.md   ← the source of truth
Status: ready · Phase: MVP · Sprint 4 · Priority: critical · Epic: E3 (#4)
Depends on: #138 (T3.1) · #22 (S7)

<details><summary>Full specification (generated — do not edit here)</summary>
…task file body inlined…
</details>
```

Rationale for inlining under `<details>`: local agents read the file; **cloud agents (Codex/Copilot) read the issue** — inlining gives them the complete closure without repo archaeology, while the fold keeps the human view thin. Labels/milestone/assignee remain native metadata. Dependencies appear both as text (humans) and in frontmatter (machines). Closing: spec-sync closes the issue when the task file's status flips to `done` in a merged PR — never the reverse.

---

## 8 · GitHub Projects

**Keep — demoted to a pure human dashboard.** It already exists, costs nothing, and founders + five design partners need a visual answer to "where are we." Explicit contract:

- **Belongs there:** status rollup, sprint/milestone forecast views, phase filters, the founder's weekly review, partner-facing progress.
- **Never belongs there:** any information an agent needs. No agent reads or writes the board. All fields are written by spec-sync from frontmatter; a hand-edit on the board is overwritten on next sync (the board is a *view*).
- Watch GitHub's **Agent HQ / mission-control** as the successor surface for agent-run tracking; adopt when it stabilizes — the task-file layer is unaffected either way.

---

## 9 · AI metadata & steering files

**Root `AGENTS.md`** (≤150 lines; `CLAUDE.md` and `GEMINI.md` symlink to it) contains exactly five things:
1. What this repo is (3 lines) + the map (one line per top-level dir).
2. Commands (setup, per-app dev, test, `spec-sync`, verify).
3. **Authority order** — imported by reference from `docs/product/22-agents/agent-guide.md` §1 (one line: "conflicts are findings, not judgment calls").
4. **Task protocol:** how to pick work (`status: ready`, deps done, `files:` disjoint from in-flight tasks), status transitions, branch naming (`task/US-3.2`), the rule that the status flip to `done` ships in the same PR.
5. Hard rules (the handoff's §3 list, compressed) + don't-read list (`docs/product/99-history/` never; frame gallery via frame-id search only; fixtures via `packages/canon`, never raw).

**Nested `AGENTS.md`** per app/service, written at the service's birth (its first task's DoD includes "write this service's AGENTS.md ≤40 lines"). Closest-file-wins; keeps the root file small forever. Codex's 32 KiB combined cap is the enforcement budget.

**Skills** (`.claude/skills/`, mirrored as prompts for other tools):
- `task-pickup` — resolve ready set → claim task → create worktree/branch → load read-list → implement → run verify → open PR.
- `spec-author` — stub → ready enrichment procedure (which docs to mine per task type, the 11-section template, the 300-line cap).
- `verify` — run the active task's `verify:` block + standing suites; used by the Stop-hook.
- `release` — milestone-checkpoint procedure.

**Hooks** (deterministic where prose is advisory): PreToolUse block on `docs/product/00-sources/**` and `18-philosophy/decisions.md` (human-only paths); PostToolUse formatter; **Stop-hook that runs the active task's `verify:` commands and blocks completion until green** — the structural answer to "the genie writes tests that pass."

---

## 10 · Automation architecture

```
docs/plan/implementation-plan.md          (human-edited strategy)
        │  scripts/spec-sync/generate-stubs   (one-time seed done via data/issues.json;
        ▼                                      thereafter: new plan items → new stubs by hand/agent)
tasks/**.md                                (SOURCE OF TRUTH — status, deps, content)
        │  spec-sync.yml  (on push to main touching tasks/**)
        ├─→ GitHub Issues   create / update body / close   (idempotent; keyed by `issue:` field)
        ├─→ Project fields  Status, Sprint, Epic, Module, Priority, Estimate, Phase
        └─→ ready-set.json  (artifact: unblocked tasks + file-disjoint parallel sets)
        │
        │  execution (either path)
        ├─→ local: Claude Code / Cursor worktree per task (task-pickup skill)
        └─→ cloud: assign issue to Copilot / tag @codex  (issue carries the inlined closure)
        │
        ▼
Pull Request  (branch task/<id>; template auto-links the spec; includes the status→done flip)
        │  ci.yml: task's verify block + canon invariants + contract tests + RBAC sweep
        │  review: fresh-context agent reviewer (.claude/agents/reviewer.md) + CODEOWNERS gate
        ▼
merge → spec-sync closes issue, updates board
        │
converge.yml (weekly): frontmatter paths exist? orphaned issues? stale in-progress (>7 days)?
                        specs touched by no PR in 2 sprints → flag for re-validation (anti-rot)
```

Properties: everything reproducible from `tasks/**` + `docs/**` (delete all issues and the board; one sync rebuilds them — today's scripts already prove this pattern); status history is git history; prompt-injection surface shrinks because issue bodies are generated from reviewed files, not free-form input.

---

## 11 · Migration plan (≈1 day of agent work + 2 founder decisions)

Nothing built today is discarded; it becomes seed data.

| # | Step | Mechanics |
|---|---|---|
| 0 | **Founder calls:** (1) rename `console`→`cloud` + monorepo-ify; (2) fold handoff in vs. subtree-vendor | 10-minute decision; everything below works under either #2 outcome |
| 1 | Restructure: `git mv` frontend → `apps/console`; subtree-merge handoff → `docs/product/`; move plan → `docs/plan/` | History-preserving; one PR |
| 2 | Generate `tasks/**` stubs from `scripts/gh-project/data/{epics,issues}.json` + `.state/issue-map.json` (the key→issue join table) | Script; every stub born already linked to its live issue number |
| 3 | Evolve `scripts/gh-project` → `scripts/spec-sync` (read frontmatter instead of JSON; add body-regeneration + close-on-done) | Half the code already exists (04/05 scripts) |
| 4 | First sync: rewrite all 192 issue bodies into the §7 projection format | One run |
| 5 | Write root AGENTS.md + symlinks + hooks + the four skills; PR template; CODEOWNERS; ci/spec-sync/converge workflows | The §9 content |
| 6 | **Enrich wave 1 to `ready`:** Sprint-0/1 tasks only (E0 items, T1.0 spike, T1.1, E2 contract work) via `spec-author` + founder review | JIT discipline starts immediately — no bulk enrichment |
| 7 | Retire `claudedocs/` duplicates in the handoff repo (plan + this proposal move into the monorepo) | Housekeeping |

Risk note: the milestones/labels/Project all survive steps 1–7 untouched. Rollback at any step is `git revert` + one sync run.

---

## 12 · Example: US-3.2 as a ready implementation spec

*(Generated from the existing user story; would live at `tasks/e3-provisioning/US-3.2-estimate-gate.md`. Frontmatter as shown in §6 — omitted here to avoid duplication.)*

```markdown
## Goal
Service creation is impossible without an accepted estimate — enforced at the API
layer, so no client (console, CLI, SDK, future) can bypass it.

## Why
"Estimate before provision" is the product's founding promise (constitution:
"know before you deploy"). If this gate has a bypass, the first invoice that
surprises a design partner destroys the only story we have.

## Read first
- docs/product/05-features/feature-specs.md §F2 — the business rule and its edge cases (5 min)
- docs/product/08-api/openapi.yaml — `POST /estimates`, `createService` (requires estimate_id), x-error-catalog 402/409/422
- docs/product/09-data-models/models.md — `estimates` (transient) and `services` tables
- docs/product/18-philosophy/decisions.md ADR-024 (status vocabulary), ADR-025 (integer cents)
- tasks/e3-provisioning/T3.1-estimate-engine.md — the engine this gate consumes (must be `done`)
- services/api/src/projects/create.ts — pattern example: follow its handler/validation/problem+json shape
**Don't read:** the frame gallery (no UI in this task); 19-canon fixtures raw (use @steloit/canon imports);
99-history (never).

## Approach
1. Migration: `estimates` table — id (est_), org_id, cell_id, shape jsonb, line_items jsonb
   (integer cents), status (draft|accepted|expired), expires_at, created_at. Index (org_id, status).
2. `POST /estimates`: body = service shape(s) → engine (T3.1) → persisted line items. Estimate
   acceptance = the client passing its id to createService; add `accepted_at` stamp there.
3. `createService`: require `estimate_id`; load estimate; 422 if missing/expired/org-mismatch;
   409 with `reasons[]` if shape drifted from the estimated shape.
4. Persist the estimate's line grammar onto the service row (`estimate_line jsonb`) — the invoice
   generator (E11) will read it verbatim; this is how "estimate == invoice" survives repricing.
5. Emit `estimate.created` and stamp `estimate_id` on the `service.created` event (events spine, US-2.5).
6. Regenerate @steloit/contracts (`pnpm gen:contracts`) — never hand-edit generated types.

## Edge cases
- Estimate reused for a second creation → 409 `reasons:["estimate already consumed"]` (AC-3)
- Estimate expired (>24h) → 422 with remediation "request a new estimate" (AC-4)
- Estimate org ≠ caller org → 404 (not 403 — don't leak existence)
- Price table changed between estimate and accept → the *estimate's* numbers win (persisted line grammar)
- Shape mutated after estimate → 409 naming every drifted field in `reasons[]`

## Security & performance
- Estimate ids are unguessable (est_ + 24 rand); org-scoped on every read.
- The gate adds one indexed PK lookup to createService — no engine re-run on accept.

## Acceptance criteria
- [ ] WHEN createService is called without estimate_id THEN 422 problem+json with remediation
- [ ] WHEN called with a valid accepted estimate THEN service provisions and metering starts at `ready` only
- [ ] WHEN an estimate is consumed twice THEN 409 with reasons[]
- [ ] WHEN an estimate expired THEN 422 with remediation
- [ ] Estimate line grammar persisted on the service row equals the engine's output byte-for-byte
- [ ] Canon invariant: the $208-family arithmetic green against the engine + this gate

## Validation
Run: `pnpm --filter @steloit/api test estimates` · `pnpm --filter @steloit/canon check-invariants`
· `pnpm typecheck`. PR must include the 422 and 409 response bodies as evidence.

## Common mistakes
- Enforcing the gate in a middleware the console path bypasses — it must live in the createService handler.
- Re-running the pricing engine at accept time (breaks "the shown estimate is the contract").
- Float money anywhere (ADR-025: integer cents end-to-end).
- Hand-editing packages/contracts (generated only).
- Returning 403 for foreign-org estimates (leaks existence; use 404).

## Out of scope
- The estimate engine's pricing logic → T3.1 · Invoice generation → T11.3 · Console estimate UI → E8-S3
- Idempotency-Key handling → US-3.6 (dep: S7 ruling)

## Related
T3.1, US-3.3, US-3.6, T11.3 · ADR-024/025 · F2 · QA scenario 9
```

~120 lines. An agent reading this + its read-list starts implementing in minutes, with the traps pre-flagged from the console build's findings ledger.

---

## 13 · Per-tool recommendations

| Tool | Setup in this repo | Notes |
|---|---|---|
| **Claude Code** | `CLAUDE.md → AGENTS.md` symlink; the four skills; hooks incl. Stop-verify gate; `.claude/agents/reviewer.md`; worktree-per-task (`claude --worktree task-US-3.2`); headless fan-out (`claude -p`) for mechanical sweeps | Keep CLAUDE.md ≤150 lines; nested files load lazily — exploit for apps/services. `/clear` between tasks; one task per session. |
| **Codex** | Root + nested AGENTS.md (32 KiB combined cap — enforce in converge.yml); `## Review guidelines` section to tune `@codex review`; cloud env setup script installs *everything* (network is cut after setup) incl. pnpm install + docker-compose Postgres; secrets exist only during setup | Delegate by tagging `@codex` on a synced issue — the inlined closure (§7) is exactly what it needs. It runs tests named in AGENTS.md before finishing — list the standing suites there. |
| **Cursor** | `.cursor/rules/steloit.mdc` = 5 lines, `alwaysApply: true`, pointing at AGENTS.md; optionally glob-scoped rules per app later; `.cursor/environment.json` with idempotent `install` for background agents | Never grow rules past ~2 always-apply files; glob misconfig is the top "ignored rules" cause. |
| **Gemini CLI** | `GEMINI.md → AGENTS.md` symlink *or* `settings.json: context.fileName: ["AGENTS.md"]` | Gemini CLI is transitioning to Antigravity CLI (2026) — the AGENTS.md investment carries over; avoid Gemini-specific machinery. |
| **Copilot coding agent** | `.github/copilot-instructions.md` = pointer to AGENTS.md; assign synced issues directly | Issues-as-projection makes assignment safe: the body is reviewed repo content, not free-form text (prompt-injection surface reduced). |
| **All** | One task per session/container · evidence-based done (verify block) · fold recurring agent mistakes into the task template's Common-mistakes bank via retrospective | The repo, not the tool, is the system. |

---

## 14 · Additional improvements

1. **Contracts-first fan-out discipline:** before any parallel wave, the wave's shared artifacts (openapi.yaml amendments, migrations, event schemas) merge first as their own small tasks. This is already how E2→E3 is sequenced; make it a stated rule in AGENTS.md (research: worktrees isolate files, not architecture).
2. **`packages/canon` as an executable truth package:** the arithmetic invariants and the 10 QA scenarios become importable test utilities — every layer (estimate engine, invoice generator, console) imports the *same* assertions. The spec's strongest idea, industrialized.
3. **Findings-ledger continuity:** the console build's discipline (every spec conflict carried as an in-code finding, never silently fixed) becomes a PR-template checkbox: "Spec conflicts found: none / listed below." Feeds the S-process.
4. **Retro loop with a budget:** when an agent repeats a mistake, the fix goes into the *task template's* common-mistakes bank or a hook — never into the root AGENTS.md (which has a hard line-count budget enforced by converge.yml). This is how the steering layer stays small for years.
5. **Cache-friendly context:** keep the root AGENTS.md and skill prefixes stable (prompt-cache economics: ~99% of agent tokens are reads; stable prefixes cut cost dramatically). Volatile info (sprint number, current wave) lives in `ready-set.json`, never in steering files.
6. **Measure the system, not vibes:** track per-task agent wall-time, PR revision count, and reviewer minutes in the PR template. If a task class consistently needs >2 revisions, its spec template — not the agent — is the bug. (ICSE finding: harness quality explains most of "agent success.")

---

## Decision summary for founders

*(Superseded by the final table at the end of Part II — decision #1, the rename to `steloit/cloud`, was executed 2026-07-18.)*

---

# Part II — Adversarial re-review (2026-07-18)

Requested by the founder before migration: challenge the proposal, try to disprove it, and only then finalize. Method: each Part I claim re-tested against the two research sweeps' *measured* findings (not re-searched — the evidence base already covers Kiro steering, Cursor rules, skills, Beads archival, Ralph recipes, retro-into-AGENTS.md, spec-rot data) and against a 20-concurrent-agent stress scenario. **Verdict: the three-layer architecture survives; six amendments (A1–A6) were found, one of which removes real complexity Part I introduced.** Everything below is stamped into Part I where it changes it.

## II.1 · Challenge: is there a simpler architecture?

**Attack tried:** drop GitHub Issues entirely (tasks/ + a generated static dashboard; cloud agents get task text pasted directly). Verdict: *rejected, but it exposed real fat.* Issues earn their keep on exactly three jobs — cloud-agent assignment (`@codex`, Copilot), PR cross-linking, and partner-visible progress — and today's scripts already proved the sync. What does NOT earn its keep is sync *sophistication*:

> **A1 — Sync is strictly one-way and dumb.** Repo → GitHub, regenerate-and-overwrite, always. No reverse sync, no three-way merge, no conflict detection — ever. A hand-edit to an issue or board field is *defined* as lost on next sync (bot comment says so). Issues and the board are **disposable views**: delete them all and one `spec-sync` run rebuilds them. This deletes the hardest 60% of the pipeline code before it's written. (Precedent: mitsuhiko's gh-issue-sync needed three-way merge *because* GitHub was co-authoritative; we simply refuse co-authority.)

**Attack tried:** drop tasks/ and work straight from the plan doc. Rejected in one line: atomic files with frontmatter are what make claiming, status, DAG-computation, and 20-agent parallelism possible. The plan is strategy; tasks are the working set.

**What would the named teams do differently?**
- **Linear** would say "use Linear" (their MCP + Codex/Claude integrations are good). Rejected for now: it reintroduces a co-authoritative SaaS brain outside git, for a 2-founder team that doesn't need PM ergonomics. Revisit if the human team grows; the `tasks/` layer ports trivially (it's just files).
- **Anthropic/OpenAI** internal practice leans on *evals and CI as the arbiter* plus minimal markdown — which supports A1 (dumber sync) and A4 (executable checks over prose history).
- **Vercel** would add *preview deployments as review artifacts* — adopted as a cheap later addition: canon-mode console build per PR touching `apps/console`.
- **Cursor** would add semantic indexing — tool-side, not repo-side; nothing for us to build.

## II.2 · Challenge: 20 agents simultaneously — does it hold?

Bottleneck audit: **(a) task claiming** — races on frontmatter edits. Fix: **A6 — claim-by-branch**: pushing `task/<id>` *is* the claim (atomic at the git layer); the `status: in-progress` commit rides the branch; `ready-set.json` excludes ids with live branches. **(b) merge congestion** — fix: GitHub **merge queue** on `main` from day one of the monorepo; contracts-first waves (already a rule) keep rebases mechanical. **(c) CI minutes** — task `verify:` blocks run on PRs (scoped, fast); full suites run in the merge queue only. **(d) the honest ceiling: human review.** Research puts it at 3–5 concurrent agents per reviewer; two founders ≈ 6–10 in practice. Twenty agents is *architecturally* supported (the DAG, file-ownership partitioning, and claim protocol don't care) but *operationally* gated on review automation — fresh-context agent reviewers reduce, never remove, the founder gate on protected paths. Stated plainly so nobody buys 20 seats expecting 20× throughput.

## II.3 · Context Packs (A2)

**Problem confirmed:** with per-task Read-first lists alone, every E3 task repeats the same five paths and there is no home for *implementation* knowledge that emerges as code is written (the handoff owns product truth, not "how our API service is put together").

**Design — and the rule that prevents a fourth competing mechanism.** Three loading mechanisms already exist; packs must not overlap them:

| Knowledge shape | Home | Loaded by |
|---|---|---|
| Maps to one directory | nested `AGENTS.md` there | location (automatic, every tool) |
| A procedure ("how to do X") | a skill / prompt file | activity |
| **Spans directories** (a domain) | **`contexts/<pack>.md`** | task frontmatter `contexts:` |

Packs: ~8–12 files, **≤150 lines each**, capped as a set (converge.yml counts). Format: tiny frontmatter (`id`, `owns:` — the domain's file globs, `see:`) + body = the domain's *map*: key files and their roles, invariants, the pattern-file to imitate, and the **domain mistake bank** (moved here from per-task Common-mistakes — written once, loaded by every task in the domain). Initial set: `provisioning` (reconciler/estimate/drivers), `rbac` (two-layer eval — spans api+policies+console), `billing` (metering→quota→invoice), `events-spine`, `api-conventions` (problem+json, pagination, contract-first), `canon-testing` (invariants, scenarios, fault injection), `frontend-console` (four-state, slices), `ai-plane` (four laws mechanics).

**Alternatives tested and rejected:** packs-as-skills (Claude-only — packs must be tool-neutral plain markdown); semantic search (non-deterministic, tool-side); a single big ARCHITECTURE.md (recreates the monolith context-rot problem the packs exist to solve). Kiro's steering-files-with-inclusion-modes is the closest prior art and validates the shape.

**Token math (the point):** a ready task shrinks to ~80–200 lines; its context bill becomes AGENTS.md (~150) + 1–3 packs (≤450) + task (≤200) + targeted product-spec reads ≈ **a few thousand tokens of curated context instead of tens of thousands of exploratory ones** — inside the measured 2K-effective-context sweet spot for the always-loaded portion.

## II.4 · Task format re-verified (A3)

- **One markdown file + YAML frontmatter per task: confirmed.** Kiro's 3-file split is per-*feature* (a multi-task container) — our equivalent is `tasks/<epic>/_epic.md` for shared design. Splitting YAML from MD doubles reads and invites divergence; frontmatter is the ecosystem-wide pattern (Backlog.md, beads export, gh-issue-sync, every static-site parser).
- **Added: `scripts/spec-sync/task.schema.json`** — frontmatter validated in CI; a typo'd `deps:` id or unknown status fails the PR, not the scheduler.
- Tasks get *smaller* than Part I's cap (80–200 lines typical) because packs absorb the shared middle. The 300-line hard cap stays.

## II.5 · Implementation history (A4)

Research answer: successful agent-run projects keep history in **living files and the VCS, never in append-only logs** — Beads *prunes* old issues; Ralph regenerates its plan and folds learned recipes into AGENT.md; Codex's official guidance is retro-into-AGENTS.md; spec-rot data shows stale narrative docs actively mislead. A `history/` or `lessons-learned.md` directory is a write-only graveyard that some future agent will wrongly load. Design:

1. **The PR is the implementation record** — diff, evidence, review thread; durable, searchable (`gh pr list --search`), never loaded by default.
2. **`## Outcome` appended to the task file at completion** (5–10 lines: shipped, deviations, follow-ups filed) — then the file is swept to `tasks/_archive/` by converge.yml a sprint later (out of the ready-set scan, out of casual context, still greppable).
3. **Lessons distribute to living homes:** recurring mistake → the domain pack's mistake bank · repo recipe → nearest AGENTS.md · decision → the appropriate ADR log · rule agents keep violating → a hook. **"A lesson that didn't change a living file didn't happen."** Budgets (pack line-caps, AGENTS.md line-cap, enforced by converge.yml) keep ten years of lessons from becoming context rot.
4. **No devlog, no lessons-learned.md, no history/.** Architecture evolution *is* the ADR logs.

## II.6 · Repository structure re-challenged (A5)

Every folder attacked; two changes survived contact:

- **A5 — Modular monolith.** Part I's six `services/` dirs was premature fragmentation (my error — it copied 14-development's *logical* module list into *physical* deployables). Day one has **three deployables**: `apps/console`, `services/api` (control-plane + billing + observe + assistant as internal modules with enforced import boundaries), `services/cell-agent` (genuinely separate — it runs inside the cell, D9). This is constitutionally aligned — module boundaries are *shape* (day-one correct), separate deployables are *capacity* (split on measured triggers, e.g. observe ingest volume). Fewer packages = fewer cross-package PRs = better for agents.
- `packages/shared` deleted (YAGNI — create when two consumers exist). `docs/product/` internal numbering (00–24) **stays exactly as-is**: the corpus's cross-references, frame ids, and ADR citations all depend on those paths; renaming them would be pure churn. `contexts/` added at root (peer of `tasks/` — both are working-set layers). Everything else survived.

## II.7 · Final recommendation

**The Part I architecture stands** — repo as single source of truth; steering/knowledge/execution layers; JIT-enriched task closures; issues and Projects as generated views — now amended:

| # | Amendment | Effect |
|---|---|---|
| A1 | One-way, overwrite-always sync; issues/board formally disposable | Deletes the hardest pipeline code; kills drift by construction |
| A2 | Context Packs (`contexts/`, ≤150 lines, 8–12 max) for cross-cutting domains; directory→AGENTS.md, procedure→skill rule | Ends read-list duplication; per-domain mistake banks; big token cut |
| A3 | One MD+frontmatter file per task, JSON-schema-validated; `_epic.md` for shared design; tasks shrink to 80–200 lines | Confirmed optimal; adds CI validation |
| A4 | History = PR + `## Outcome` + lessons-to-living-files + archive sweep; no logs/graveyards | Future agents get lessons at the point of use, not in a pile |
| A5 | Three deployables (console, api-monolith, cell-agent); split on triggers per INF-001 | Removes Part I's own over-engineering |
| A6 | 20-agent readiness: claim-by-branch, merge queue, PR-scoped vs queue-scoped CI; honest review-ceiling note | Scales the mechanism; names the real bottleneck |

**Why the core survives its own cross-examination:** every attack either bounced off a measured finding (atomic files, hybrid issue content, small steering layers, JIT enrichment) or improved a part the evidence didn't actually support (six services, clever sync, per-task mistake banks). The remaining risk is discipline, not design — caps and sweeps are enforced by converge.yml precisely because prose rules decay.

## Updated decision summary

| # | Decision | Status |
|---|---|---|
| 1 | Rename `steloit/console` → `steloit/cloud` | **Done (2026-07-18)** |
| 2 | Fold `steloit-handoff` into `docs/product/` (subtree, history kept), archive standalone; CODEOWNERS protects human-only paths | Awaiting call |
| 3 | Approve formats as amended: task file (§6 + A2/A3/A4), issue projection (§7 + A1), Projects-as-dashboard (§8), Context Packs (II.3), monorepo shape (§5 + A5) | Awaiting call |
| 4 | Green-light migration (§11, now incl. contexts/ seeding + merge-queue setup; still ~1 agent-day, reversible) | Awaiting call |
```
