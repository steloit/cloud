# Kernel (devkernel) Workflow Review — should it run Steloit's implementation?

**Status:** Founder-requested tooling review · 2026-07-18 · Evaluated from primary sources: the plugin source (`~/Code/tools/kernel`, v2.0.2), its 22+ ADRs/doctrine, settings/hooks surface, and its production evidence on OpsCore + squad-platform. Web/community research is answered factually below.
**Verdict up front:** **Do not adopt for steloit/cloud now** (option: "Do not use it for Steloit"), with one cherry-pick (the push-protection guardrails) and one pre-registered revisit trigger. The decisive fact is not any weakness of Kernel — it is that **this repo already ships a ratified, frozen, purpose-built Engineering OS (ADR-0002) that has never yet run a single implementation task**, and both constitutions (Steloit's ADR-040 stability clause and Kernel's own adoption charter) forbid replacing an unfalsified incumbent on speculation.

## 1 · What Kernel actually is (verified, not marketed)

**Identity:** the founder's own tool — private repo `hashirventhodi/kernel`, created **2026-07-15** (three days old), v2.0.2 after 33 PRs and 6 schema migrations, 0 stars, **no license file**, no public footprint. The research brief's "community adoption / HN / Reddit / independent reviews" have a factual answer: **none exist and none can** — every public "Kernel" hit (onkernel.com browser infra, Jupyter kernels, Linux) is an unrelated name collision. Maintenance = the founder; roadmap = its own evidence-gated protocol; marketing-vs-verified is unusually clean because the docs are governed by a doctrine that marks unvalidated claims as unvalidated.

**Architecture (verified from source):** 7 skills (`setup/intake/refine/plan/implement/review/land`) + 1 clean-context reviewer subagent (**Read/Grep/Glob only — shell-free**) + 1 SessionStart resume-brief hook + a settings fragment (deny-lists for push-to-main/force-push; `.devk/runtime` sealed to runtime-owned writes) + a `tasks` shim over Beads + file-based `.devk/` workspace, **machine-local by design** (`.git/info/exclude`; product PRs ship with zero Kernel files). No binary, no server, no database. 54/54 tests. Migration system for workspace schema.

**Production evidence (real):** M0 measured 35s ceremony per S-task; M1 ran real Jira tickets on OpsCore (multi-repo umbrella, ADR 0007); 10 tickets landed on squad-platform (PRs #88–91 et al.) with the reviewer catching real blockers and two consecutive evidence reviews concluding "no Kernel change justified." Its own maturity label: *"Production-ready, not Mature."*

**Known risks in the tool itself:** Beads upstream churn (deleted its SQLite backend within the usage window; org moved) — mitigated by the shim; 0.1.0 → 2.0.2 in 72 hours (high early churn); one recorded misfire (a Kernel snippet appended into a target repo's AGENTS.md tail); the settings fragment merges into the target repo's `.claude/settings.json` (a real, reviewable footprint).

## 2 · The Steloit-specific evaluation

The incumbent: ADR-0002's Engineering OS — 181 pre-authored spec-driven tasks with 12-section bodies and executable `verify:` blocks, claim-by-branch locking, contexts packs, authority-ordered citations, one-way GitHub projection (issues + board, byte-verified), and three repo-native skills (`task-pickup`, `spec-author`, `verify`) that are exactly Kernel-shaped, specialized to this repo. **AGENTS.md freezes it.**

| Question | Answer |
|---|---|
| Faster implementation? | **No** for planned tasks — every Kernel stage has a working incumbent equivalent (intake≈task file+claim · refine/plan≈spec-author enrichment to `ready` · implement≈task-pickup · review≈verify+/code-review · land≈PR+Outcome+lessons-to-living-files); adding Kernel means two rulebooks per item |
| Fewer mistakes? | Kernel's genuine deltas — mandated independent review, hard plan gate, push-protection deny rules, resume brief — are **separable**; the guardrails can be cherry-picked without Kernel, and /code-review already provides clean-context review |
| Multi-agent collaboration? | **Worse fit**: `.devk` is machine-local, invisible to other agents/machines; Steloit's branch+repo state *is* the shared lock and audit trail |
| ADR/implementation consistency? | Steloit-native machinery (authority order, contexts packs, S-process, spec-sync validator) owns this; Kernel is generic and unaware of GOV-002/INF-001/canon |
| Regretted dependency? | Technical exit is clean (delete `.devk`, uninstall — zero repo contamination, proven). The real cost is **founder attention** (single maintainer) and two-workflow ambiguity for agents |
| Contributors vs agents? | Moot — solo founder + agents; Kernel is agent-facing by construction |
| Required or optional? | Its machine-local design means it can only ever be **personal/optional** here; it cannot be the repo's required workflow without contradicting its own architecture |
| Workflows that would benefit | None today. The plausible future fit is **unplanned work** (post-alpha bugs/incidents/partner reports) — but the repo's own protocol routes discovered work into tasks/ |
| Workflows it must not touch | Founder rulings (S-process), 00-sources passes, canon migrations, decision-log stamps — all governed by repo-specific human-only machinery Kernel doesn't know |

## 3 · Why both constitutions say "not now"

- **Steloit (ADR-040):** architecture and workflow change only on implementation/performance/customer/security evidence; *a cleaner abstraction is never a trigger*. Zero implementation tasks have run — the incumbent is unfalsified; there is no evidence of the failure modes Kernel exists to fix (lost context, skipped verification, dropped discovered work) *in this repo*.
- **Kernel's own charter:** the team's tracker owns the backlog (here: tasks/ + spec-sync **is** the tracker); adoption/changes require production evidence with the default answer "no"; the Feature Test generalizes — a repo that already ships the capability shouldn't gain a second implementation; and "time spent on Kernel instead of delivering is itself a purpose violation."

**Risk of adopting now:** undefined precedence between AGENTS.md's task protocol and Kernel's gates (the worst possible state for agents); attention split across two workflow codebases during the highest-leverage build phase; coupling Sprint 0 to a 72-hour-old schema. **Risk of not adopting:** losing the guardrails (cherry-picked below); founder context-switching between Kernel repos and this one (real, tolerable, revisited on evidence).

## 4 · Decision

1. **Do not use Kernel for steloit/cloud.** The native Engineering OS is the standard implementation workflow. Kernel remains the founder's standard elsewhere (OpsCore, squad-platform) — unaffected.
2. **Cherry-pick (no Kernel dependency): the push-protection permission rules** (deny push-to-main/force-push forms + `ask: *git push*` backstop) into this repo's `.claude/settings.json` — pure safety, aligned with claim-by-branch, reversible.
3. **Pre-registered revisit trigger** (mirrors Kernel's own 10-ticket protocol and the M4-checkpoint pattern): after **10 completed implementation tasks or the end of Sprint 1**, whichever first, run a native-OS friction review using Kernel's seven questions (friction? review effective? validation effective? rules violated? repos? lessons? simplify?). Adopt Kernel *per-item* only if recurring failures land in Kernel-owned territory (session amnesia, skipped verification, dropped discovered work, plan-less implementation) — integration shape pre-approved for that case: `/kernel:intake` reads from tasks/ (backlog authority unchanged), `.devk` stays machine-local, PRs stay clean.
4. This review is the record; no ADR needed unless the revisit trigger fires (adoption then = an engineering ADR in docs/adr/).
