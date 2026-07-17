# Steloit PDS Phase — Master Execution Plan

**Document class:** Internal — Design Execution
**Document ID:** DES-009 *(registered per GOV-004)*
**Status:** v1.0 — For approval
**Inputs (approved, immutable):** GOV-001 Vision · GOV-002 Architecture · Documentation Master Plan · GOV-005 Design Strategy · DES-000 PDS System
**Purpose:** The operating plan for the PDS phase: the complete PDS inventory (validated, not assumed), the dependency graph, the execution waves, the first-product decision with full justification, the authoring blueprint for that first PDS, and the go/no-go readiness assessment.

---

# Deliverable 1 — The PDS Inventory (Validated)

## 1.1 Validating the hierarchy before listing it

DES-000 §6.2 rules that PDS numbers mirror PRD numbers. Auditing that rule against the approved corpus exposes **three gaps and two over-allocations** — surfaces that need design but have no PRD, and PRDs that should not each own a screen-owning PDS. This plan proposes the following registry amendments (submitted through GOV-004's revision process; nothing here contradicts an approved decision, it completes the mapping):

**Additions (design surfaces without a PRD):**

1. **PDS-SHELL — Platform Shell & Project Experience.** GOV-005 §6 names the "Project & Environment shell" as pioneer #1 and DES-000 §9 references "PDS-SHELL," but no PRD exists for it — correctly, because it is the rendering of FND-001/EXP-001, not a product. It needs a formal PDS ID and a defined scope (below). Its authority chain cites FND/EXP specs directly, which DES-000's traceability model permits (FND → PDS is already a binding edge).
2. **PDS-ACC — Account & Authentication Surfaces.** Signup, login, MFA, recovery (SEC-008's customer-facing flows). No PRD covers them; J1 cannot start without them. Small, but skipping it means the platform's literal first screen is designed by accident.
3. **PDS-TPL — Templates Experience.** EXP-008 defines the template *format*; the gallery, template-detail, and template-driven creation variant need a design home. Thin, and deliberately scheduled after the creation wizard it decorates.

**Consolidations (PRDs that should not own standalone screen sets):**

4. **PDS-012 (AI) is scoped down, not out.** Per G9 and DES-000 §12, AI behavior is specified *inside each product's PDS*, and the proposal-artifact card is a Layer-3 global pattern (DES-005). PDS-012 therefore covers only what is genuinely its own surface: the ⌘K assistant conversation view and the capability-registry's cross-product presentation. Making PDS-012 a broad "AI design" document would recreate the "AI tab" GOV-002 §3.6 explicitly forbids.
5. **Onboarding is merged into PDS-SHELL, not a separate PDS.** EXP-006's first-run flow *is* J1, and J1's spine is project creation — Shell territory. A standalone onboarding PDS would put the two halves of the platform's most important journey in two documents with two owners. EXP-006 remains the spec authority; PDS-SHELL is its design instrument. (Same logic keeps ⌘K search, notifications, and settings frames inside PDS-SHELL: they are the shell, per EXP-001.)

**Confirmed non-PDSs:** the CLI (a mandatory *lane* in every PDS §7 plus EXP-003's grammar boards — a standalone CLI PDS would fork the one-design-all-surfaces rule); public docs (EXP-007's content architecture, not a screen product); and everything on the never-build and v4-deferred lists.

## 1.2 The register — fifteen active PDSs, two reserved

Complexity scale: **S** (≤1 designer-week of authoring) · **M** (2–3) · **L** (4–6) · **XL** (6+, pioneer-grade). "Related DES" always includes DES-000/002/003/005/007 implicitly; only distinctive additions are listed.

| ID | Product | Purpose (one line) | Primary users | Role | Cx | Related PRD | Key FND | Distinctive DES/EXP |
|---|---|---|---|---|---|---|---|---|
| **PDS-SHELL** | Platform Shell & Project Experience | Nav, context bar, project/env lifecycle, creation wizard, topology map, ⌘K, notifications, settings frames, first-run (J1) | Every user, every session | **Pioneer** | **XL** | — (FND-direct) + PRD-012 (recommendation flow) | FND-001, 002, 007, 009, 010 (estimates) | EXP-001, 006; G1, G4, G8, G9, G10 |
| **PDS-001** | Managed PostgreSQL | The canonical service: Kit instantiation #1, connection UX, branching, backup/restore surfaces, query insights | App developers | **Pioneer** | **XL** | PRD-001 | FND-003, 004, 006, 010, 011 | DES-006 (Kit, co-created); G2, G3, G5 |
| **PDS-005** | Observability | Dashboards grammar, log UX, alert config, correlated timeline, pivot flow (J4) | Developers on-call / debugging | **Pioneer** | **L** | PRD-005 | FND-008 | G6, G7; workflow 3 |
| **PDS-007** | IAM & Team | Invitations, roles, API keys, teams; J7 | Admins, team leads | Instantiator (+micro-pioneer: invitation & key patterns) | **M** | PRD-007 | FND-005 | G3 (key revocation) |
| **PDS-ACC** | Account & Auth | Signup, login, MFA, recovery | Everyone, pre-context | Instantiator | **S** | — (SEC-008) | FND-005 | EXP-006 (J1 step 0) |
| **PDS-002** | Managed Valkey | Kit instantiation stress test; mode selection, eviction, memory surfaces | App developers | **Pure instantiator (gate)** | **S** *(by mandate — §8.3 thinness test)* | PRD-002 | FND-003/004 | Kit v1 |
| **PDS-003** | Object Storage | Kit + the object/bucket browser; public-exposure warnings | App developers | Instantiator +1 pattern | **M** | PRD-003 | FND-009 (exposure) | G3-cousin (public warnings) |
| **PDS-004** | Queue | Kit + message/DLQ inspector; consumer-binding stub for v2 | App developers | Instantiator +1 pattern | **M** | PRD-004 | FND-004 | Workflow 2 |
| **PDS-008** | Secrets | Versioned values, reveal/rotate UX, access views | Developers, admins | Instantiator | **S** | PRD-008 | FND-006 | Reveal/copy component |
| **PDS-006** | Backup & Restore | The cross-service restore workflow home; rehearsal flow (J5) | Developers, admins | Instantiator (elevates workflow 5) | **M** | PRD-006 | FND-007 | G3; Diff/confirm patterns |
| **PDS-009** | Cost & Usage | Rollups, estimate-vs-actual, budgets, "what changed" (J8) | Owners, admins, developers | Instantiator of G4 at page scale | **M** | PRD-009 | FND-010, 011 | G4; BIZ-001 inputs |
| **PDS-012** | AI Assistant Surfaces | ⌘K assistant view; capability-registry presentation | All users (opt-in per policy) | Instantiator of G9 | **S/M** | PRD-012 | FND-005, 007 | G9; proposal card (DES-005) |
| **PDS-TPL** | Templates | Gallery, detail, template-driven creation variant | New-project creators | Instantiator | **S** | — (EXP-008) | FND-001 | Wizard variant |
| **PDS-010** | Compute & Deployments | Deploy timeline, build logs, rollout viz, domains/TLS; J11 spine | App developers | **Pioneer** (last major) | **XL** | PRD-010 | FND-012, 009 | Diff presenter reuse; G2 extension |
| **PDS-011** | Git & Preview Environments | Repo connect, push-to-deploy, PR previews with DB branches | App developers | Instantiator over 010+001 | **L** | PRD-011 | FND-012 | J11; workflow 7 auto-retire |
| *PDS-013* | Vector & Search | *Reserved — written at PRD-001 graduation trigger* | — | Instantiator | — | PRD-013 | — | Kit |
| *PDS-014* | Private Networking / Enterprise | *Reserved — v3 wave, with SSO/policy revisions landing as PDS-007/SHELL revs* | — | Instantiator | — | PRD-014 | — | Pattern intensification |

**Why each exists** is carried in the table's authority columns plus one sentence each where non-obvious: PDS-SHELL exists because the container is a product-grade experience nobody's PRD owns; PDS-ACC because first impressions are designed or they are defaults; PDS-006 because restore must feel identical across services and per-service PDSs would fragment it; PDS-012 in reduced form because AI lives inside products, not beside them; PDS-TPL because template-driven creation is a wizard *variant* and variants designed without their base become forks. Reserved IDs exist so the future has addresses without documents — the same discipline the Master Plan applies to v4 specs.

---

# Deliverable 2 — The Design Dependency Graph

## 2.1 The graph

```
                       GLOBAL LAYER (not PDSs, but the graph's roots)
   DES-002 (G1–G11) ── DES-003 (Journeys) ── DES-004 (Tokens) ── DES-005/006 (Patterns/Kit)
        │                                                            ▲
        ▼                                                            │ (Kit is CREATED by
 ┌─────────────────── WAVE P0: THE PAIRED PIONEERS ────────────┐     │  the P0 pair, then
 │                                                             │     │  feeds everything)
 │   PDS-SHELL  ◄════ co-designed, one artifact-step ════►  PDS-001  │
 │   (container: context, wizard,        (tenant: Kit anatomy,       │
 │    topology, ⌘K, J1 spine)             service patterns, J3/J5)   │
 └───────┬──────────────────────────────────────┬────────────────────┘
         │                                      │
         │ context/wizard/nav patterns          │ Service Page Kit v1, connection UX,
         │                                      │ danger zone, scale flow, restore surfaces
         ▼                                      ▼
   ┌─ WAVE P1 ────────────────────────────────────────────┐
   │  PDS-005 Observability ◄─ needs a real service       │
   │  PDS-007 IAM  ·  PDS-ACC (parallel, low coupling)    │
   └───────┬──────────────────────────────────────────────┘
           │ timeline/log/alert patterns → every later service page's Metrics/Logs tabs
           ▼
   ┌─ WAVE P2 ────────────────────────────────────────────┐
   │  PDS-002 Valkey (GATE: pure instantiation)           │
   │  then PDS-003 Storage ∥ PDS-004 Queue ∥ PDS-008      │
   └───────┬──────────────────────────────────────────────┘
           │ Kit proven at scale; +2 contained patterns
           ▼
   ┌─ WAVE P3 ────────────────────────────────────────────┐
   │  PDS-009 Cost ∥ PDS-006 Backup ∥ PDS-012 AI ∥ PDS-TPL│
   └───────┬──────────────────────────────────────────────┘
           │ diff/confirm + proposal-card patterns mature
           ▼
   ┌─ WAVE P4 (v2 horizon) ───────────────────────────────┐
   │  PDS-010 Compute ──► PDS-011 Git & Previews          │
   └──────────────────────────────────────────────────────┘
```

## 2.2 Reading the graph — the five edge types

**Hard blocks (must exist first):** PDS-SHELL's context model and wizard block *every* service PDS (all live inside the shell and instantiate its wizard). PDS-001 blocks all Kit consumers (002/003/004, and effectively 005's dashboards need a service to dashboard). PDS-010 blocks PDS-011 (previews deploy compute). PDS-009 depends on FND-010's estimate contract *and* on the wizard's estimate panel existing (Shell) before it can design the page-scale story.

**Pattern establishment (who mints what):** Shell mints context bar, provisioning wizard, topology map, ⌘K, notification inbox. Postgres mints the Kit itself, connection-credential UX, danger zone in anger, scale flow, restore surfaces, branching UX. Observability mints dashboard grammar, log query UX, alert config, correlated timeline, pivot. Storage and Queue mint exactly one contained pattern each (browser; inspector). Compute mints deployment timeline, build-log view, rollout visualization. **Nobody else mints anything** — that is the plan, enforced at Gate G-B by the §3 pioneer declarations.

**Pattern inheritance:** every Wave P2+ PDS inherits the Kit plus Shell patterns; PDS-006 inherits Postgres's restore surfaces and generalizes them; PDS-011 inherits the diff presenter (born in J6/Shell promotion, matured in P3) and workflow 7's auto-retire variant (stubbed in PDS-004's consumer view).

**Legal parallelism:** within-wave parallel tracks are safe where coupling is low: IAM ∥ ACC ∥ Observability (P1); Storage ∥ Queue ∥ Secrets after the Valkey gate clears (P2); all of P3. The P0 pair is *deliberately not parallel-independent* — it is one braided effort (Deliverable 4).

**Intentional waits:** PDS-010/011 wait for v2's window even though designers will itch to start — J11 is journey-mapped in P3 (per GOV-005 D3) but its screens wait for FND-012's full spec, or the PDS would be fiction. PDS-013/014 wait for their triggers. PDS-TPL waits for the wizard it decorates. PDS-012's assistant view waits until at least two products have registered capabilities, so the surface is designed against real proposals, not hypothetical ones.

---
# Deliverable 3 — Design Waves

Waves are execution units with entry/exit gates, synchronized to GOV-005's D-stages and the Master Plan's build clock (design Build-Ready always precedes the corresponding build wave).

## Wave P0 — The Paired Pioneers *(D2, ~6–8 weeks)*

- **Contents:** PDS-SHELL + PDS-001, braided (Deliverable 4); Kit v1 (DES-006) produced as their joint by-product; global patterns minted and registered.
- **Objective:** produce the platform's *grammar in pixels-to-be*: the container and the canonical tenant, designed as one experience so J1 and J3 are seamless across their boundary.
- **Why together:** designing the Shell without a real service violates the content-first law (A7) — the wizard would have a fake middle, the topology map a fake node, the overview fake cards. Designing Postgres without the Shell means designing a service page with no context model, no wizard to instantiate, no home. Every past platform that designed "the frame" and "the first app" separately shipped a visible seam between them; the seam *is* the failure mode, so the wave is the pair.
- **Risks:** scope gravity (Shell's XL scope swallowing the wave — mitigated by the §5 slicing plan); the Kit hardening too early around Postgres-only needs (mitigated: Kit review explicitly rehearses Valkey/Storage/Queue against every anatomy decision, on paper, before v1 locks); two XL pioneers competing for the Systems track (mitigated: the one-pioneer rule is preserved because the pair shares one pattern budget and one G-B review).
- **Deliverables:** both PDSs Approved (G-A + G-B passed); Kit v1 published; pattern registry seeded; validation studies run on the four highest-risk bets (G1 context, wizard+estimate, environment-switch filter, branching mental model).
- **Reusable patterns expected:** context bar · provisioning wizard · cost-estimate panel · topology map · ⌘K palette · notification inbox · Service Page Kit v1 · connection-credential block · danger zone · scale flow · restore surface v1.
- **Success criteria:** study results within DES-008 benchmarks for J1's first half; zero unresolved grammar escalations; PDS-001 delta statement contains no Shell-domain items (the boundary held).
- **Exit criteria:** Kit v1 Approved; both PDSs at G-B pass; hi-fi begins on both; **Valkey thinness pre-check** — a designer not on the P0 team drafts PDS-002 §§4–6 in two days using only the Kit and registry; if they can't, P1 does not start until the gap is fixed.

## Wave P1 — Sight and People *(late D2, overlapping P0 hi-fi, ~4–5 weeks)*

- **Contents:** PDS-005 Observability (pioneer #3) · PDS-007 IAM · PDS-ACC.
- **Objective:** complete the v0 experience: something to observe with (J4's machinery), someone to invite (J7), a way in (J1 step 0).
- **Why together:** Observability needs Postgres shipped-in-design to have real dashboards to define; IAM and ACC are low-coupling, small, and staff the wave's parallel lanes without competing for the pioneer's pattern budget.
- **Risks:** Observability's log/metrics UX quietly becoming a second design language (mitigated: G6/G7 are already global; the correlated timeline is its *only* licensed pioneer pattern beyond dashboard/log/alert grammar); ACC drifting toward marketing-site aesthetics (mitigated: DES-001's personality applies from the first screen).
- **Reusable patterns expected:** dashboard grammar · log line/query UX · alert-rule form (a G5 instantiation, not a new pattern) · correlated timeline + pivot · invitation flow · API-key lifecycle block.
- **Success / exit:** J4 wireflow walks end-to-end across PDS-005+001 with zero seams; every later PDS's Metrics/Logs tabs are now pure inheritance; v0 design surface fully Build-Ready before v0 build wave starts (the phase's hard external deadline).

## Wave P2 — The Instantiation Proof *(D3 first half, ~3–4 weeks)*

- **Contents:** PDS-002 Valkey **first and alone for its first week (the gate)**, then PDS-003 Storage ∥ PDS-004 Queue ∥ PDS-008 Secrets.
- **Objective:** prove the system's economics: the reuse curve bends here or the whole strategy is re-costed.
- **Why together:** the three data services are the Kit's target population; Secrets is a small pure consumer riding along.
- **Risks:** the gate failing (Valkey needs real pattern work) — treated not as schedule slip but as *system defect*: Kit revs, P0 pair reconvenes, P2 restarts; the plan says this out loud so nobody is tempted to wave it through. Storage's browser and Queue's inspector creeping beyond their one-pattern licenses (mitigated: their §3 declarations name exactly one pattern each).
- **Reusable patterns expected:** none from Valkey (that's the point); object browser (Storage); message inspector (Queue).
- **Success criteria:** PDS-002 ≤20% of PDS-001's length (DES-000 §8.3's number, now live); combined P2 authoring effort <30% of P0's (GOV-005 §6's curve, measured at the document layer); zero new library components requested by Valkey.
- **Exit criteria:** all four Approved; Kit v1.x published with any (minor) amendments; drift dashboard empty.

## Wave P3 — Money, Safety, and Intelligence *(D3 second half, ~4–5 weeks)*

- **Contents:** PDS-009 Cost ∥ PDS-006 Backup ∥ PDS-012 AI surfaces ∥ PDS-TPL; **J11 journey-mapping runs here too** (design's advance scout for v2, per GOV-005 D3).
- **Objective:** complete v1 GA's experience: the bill you can trust (J8), the recovery you've rehearsed (J5), the assistant with manners, the fast start.
- **Why together:** all four are instantiators over now-mature patterns; all four are what GA means experientially beyond the services themselves.
- **Risks:** PDS-006 discovering that per-service restore surfaces (minted in P0) don't generalize — the likeliest late grammar escalation in the plan; budgeted for explicitly. PDS-012 designing ahead of registered capabilities (mitigated: its entry criterion is ≥2 products' §12s Approved — satisfied by SHELL's recommendation flow and 001's query assistance).
- **Reusable patterns expected:** "what changed" explainer · budget/policy form · restore-rehearsal flow (generalized) · proposal-artifact card in situ · template-creation wizard variant.
- **Success / exit:** J5, J8 walk end-to-end; v0.5+v1 design surfaces Build-Ready ahead of their builds; parity audit #1 (GOV-005 §9.4) passes with findings only in the "tracked defect" class.

## Wave P4 — The Developer Cloud *(D4, v2 window)*

- **Contents:** PDS-010 Compute (final pioneer) → PDS-011 Git & Previews.
- **Objective:** J11 — the flagship — designed as the reward for three waves of discipline: Compute's PDS should be XL in engineering-novelty but *moderate* in design-novelty, because the Kit, diff presenter, danger model, scale flow, and timeline grammar all pre-exist.
- **Risks:** FND-012 spec slippage starving the wave (mitigated: J11 map and PDS-010 §§1–5 can author against FND-012's stub; §§6+ wait for the full spec); the deployment timeline tempting a new visual language (mitigated: it is declared as the wave's *single* net-new pattern family).
- **Success / exit:** J11 gates passed; v2 Build-Ready before v2 build; the phase-level metric achieved — Compute's design effort demonstrably a fraction of its engineering effort, the strategy's promised payoff.

**Deferred beyond waves:** PDS-013/014 (triggers per register), enterprise revisions (v3 arrives as PDS-007/SHELL/009 *revisions* — the design mirror of the Master Plan's "v3 is revisions" doctrine).

---

# Deliverable 4 — The First Product

## 4.1 The decision

**The first design unit is the braided pair — PDS-SHELL leading PDS-001 by exactly one artifact step — and if one document must be called "first," it is PDS-SHELL.** This confirms GOV-005 §6's ordering by analysis rather than by citation, and sharpens it with the pairing discipline the strategy implied but did not operationalize.

## 4.2 Why the analysis lands here (not assumed)

The instruction was to test the assumption, so here is the honest case for each alternative and why it loses:

**The case for PostgreSQL first:** it is the anchor product, the hardest engineering, the Kit's source material, and the revenue wedge; designing it first front-loads the highest-value learning. **Why it loses as sole first:** a service PDS's §§4–7 are structurally unwritable without the container's decisions — its objects sit in *a* navigation (whose?), its wizard instantiates *a* pattern (which?), its URL lives in *a* scheme, its cost panel renders in *a* placement. Postgres-first means inventing a provisional shell inside PDS-001 and refactoring it later — the exact "local solution to a global question" DES-000 §1 forbids.

**The case for Observability first:** it is the most cross-cutting surface and the "no setup required" promise. **Why it loses:** dashboards of nothing. It has no objects of its own until a service exists; it is correctly pioneer #3.

**The case for Shell *alone* first (the naive reading of GOV-005):** it is the container; finish it, then fill it. **Why it loses:** three of the Shell's four signature artifacts are *contentless without a tenant* — the wizard's middle, the topology map's first node, the project overview's cards, and above all J1, which does not end at "project created" but at "first service provisioned and connected" (GOV-001's five-minute promise). A Shell designed against placeholder services violates A7 (content-first) and would be re-opened the week Postgres design starts. The seam between "frame team" and "first-app team" is the most reliably shipped defect in platform history; Steloit's whole thesis is the absence of seams.

**Therefore: the pair, braided.** Shell leads by one artifact step because its decisions are the container's: Shell's A1–A2 (object map: Organization, Project, Environment, Folder, Notification, plus Service-as-card and Binding-as-edge; IA placement) complete first and hand PDS-001 its coordinate system; then both proceed in lockstep — Shell's A5 wizard wireflow is drawn *with* Postgres as the concrete middle; Postgres's A5 service-page wireflows are drawn *inside* Shell's frame. One combined G-A review; one combined G-B review; one pattern budget; two documents.

## 4.3 What this first unit establishes, reduces, and unlocks

- **Patterns defined (the reusable dividend):** the eleven listed in Wave P0 — between them touching G1, G3, G4, G5, G9, and G10 in their first concrete form, plus the Kit itself.
- **Dependencies served:** every subsequent PDS inherits from this pair; nothing inherits from anything else that doesn't itself inherit from this pair. It is the graph's root by construction.
- **Risks retired:** the four highest-risk interaction bets (context model, environment-switch filter, wizard+estimate, branching mental model) are validated here, where changing course costs days; discovered any later, each costs a wave. It also retires the phase's biggest process risk — whether the DES-000 machinery works — on the two documents with the most senior staffing and attention.
- **The five-minute test becomes designable:** J1 end-to-end exists only when this pair does; the platform's benchmark metric gets its first measurement inside P0's validation studies, not after GA.

---
# Deliverable 5 — Authoring Plan for the First Unit (PDS-SHELL, braided with PDS-001)

The blueprint below follows DES-000's A1–A8 pipeline and fourteen-section anatomy exactly; it is the pipeline *scheduled and staffed* for the specific first case. Duration target: 6–8 weeks to G-B for the pair. Team: 1 staff designer (Shell lead), 1 staff/senior designer (Postgres lead), Design Systems pair (Kit), DX designer (CLI lanes), content designer (A7), researcher (studies), with the PRD-001 author and FND-001/003 owners as standing reviewers.

## 5.1 Required inputs (the entry checklist — nothing starts without them)

| Input | Needed state | Consumed by |
|---|---|---|
| GOV-003 Terminology | **Approved** (esp. the `steloit db` vs `postgres` noun, ID/slug scheme) | Every artifact; A7 especially |
| FND-001 Resource Model | Approved or late Review | A1, §4 |
| FND-002 API Conventions | Review (URL shapes, async op pattern, error taxonomy) | §4 URLs, §6 states, §9 errors |
| FND-003 Service Grammar | Review — *co-evolving*: PDS-001 is its designated pressure test | Kit, §6 |
| FND-004 Bindings · FND-010 Metering (estimate contract) | Draft minimum | J3 lane; wizard estimate panel |
| DES-001/002 | Approved (G1–G11 decided) | Everything |
| DES-003 | J1–J7 mapped | §5 |
| DES-004 tokens · DES-007 voice · DES-008 playbook | v1 available | A7–A8, studies |
| PDS-000-EXEMPLAR | Published | Authors' onboarding |
| PRD-001 | Approved or late Review | PDS-001 throughout |
| PRD-012 (recommendation-flow scope) | Draft | Shell §12 |
| D0 research | Mental-model & terminology studies read out | A1, §1 |

## 5.2 Discovery (week 1, parallel with A1)

Competitive teardown *focused for this unit*: project/app creation flows and first-run across the reference set (where do they lose the user before first success?); Postgres provisioning and branching UX across Neon/Supabase/PlanetScale (what mental models already exist for branches — inherit the good ones); a card-sort/terminology study on Project/Environment/Service with target developers (the hierarchy is immutable; its labels and first-encounter explanations are ours to get right); and an internal FND-walkthrough where the pair's designers interrogate FND-001/003 authors clause by clause — the ritual that makes design the spec's first pressure test rather than its surprised consumer.

## 5.3 The artifact schedule (A1→A8, braided)

**A1 — Object maps (wk 1).** Shell: Organization, Project, Environment, Folder/Label, Notification, Settings scopes; Service-as-card and Binding-as-edge as *rendered* objects (owned elsewhere, displayed here). Postgres: instance, branch, replica, backup/snapshot, connection credential, migration artifact (stub) — each mapped to FND-001 identities and FND-003 states, with branching's parent/child semantics drawn until they stop generating questions. *Exit: both maps reviewed against FND-001 with zero unmapped nouns.*

**A2 — IA placement (wk 1–2).** Shell settles: the nav tree instance, URL grammar in practice, breadcrumb behavior, ⌘K index plan, notification categories, the three settings scopes' page frames. Postgres slots in: its service page address, type tabs declared (**Branches**, **Query Insights** — and the anti-sprawl argument for why only these), branch URLs. *Exit: EXP-001 skeleton and this placement are the same drawing.*

**A3 — Journey overlays (wk 2).** J1 end-to-end across ACC-stub→Shell→Postgres→connected code, Console and CLI lanes side by side, with the AI-recommendation variant drawn as a first-class path (not an alternate footnote); J2 daily loop; J3 add-and-bind (including the **external-runtime variant** — see readiness item R2); J5 and J9 for Postgres's protect/retire participation; J7 stub. Emotional-temperature marks per DES-003. *Exit: G-A material complete.*

**A4 — Workflow instantiation (wk 2).** The seven-standards table for both documents. Expected shape: Shell instantiates workflows 1 and 7 and *defines their first concrete middles*; Postgres instantiates 1–7 minus 3 (Observability's, next wave). Pioneer declarations finalized here — Shell's license: context bar, wizard, topology, ⌘K, inbox; Postgres's license: Kit anatomy, connection block, danger-in-anger, scale flow, restore surface, branching UX. *Anything else that looks novel gets reformulated or escalated now.*

**A5 — Wireflows (wk 3–4).** The named-box maps, unhappy paths mandatory: creation wizard (empty org → project → select/describe → configure → estimate → provisioning → ready-with-next-step; failure and quota branches); environment create/switch/settings; topology interactions; ⌘K flows; Postgres provision-within-wizard, service page tab flows, branch create/promote/retire, credential reveal/rotate, scale, restore-to-branch, destroy (G3 tier 3 walked fully). CLI ribbon under every flow. *Exit: screen inventory born; every box passes the six-line declaration.*

**A6 — State tables (wk 3–4, trailing A5 by days).** Every screen and async object: the wizard's provisioning progress against FND-002's async pattern; a branch's states while storage copies (the known PRD pressure-point — expect PRD-001 change requests from this table, route them per DES-000 §6.1); topology nodes across all FND-003 states; connection-loss behavior for live regions. *Exit: zero cells reading "TBD."*

**A7 — Content-first drafts (wk 4–5).** Microcopy inventories for every A5 screen in DES-007's voice; the error catalogs mapped to FND-002 codes; the wizard's explanatory copy (where the platform teaches its own model — the highest-leverage words in the product); confirmation copy for every G3-tiered action; terminology submissions to GOV-003. *Exit: content review passed; no lorem exists to ever be typed.*

**A8 — Low-fi wireframes (wk 5–6).** Per §4.3 fidelity rules, annotated, pattern-cited, built *with* the Systems pair so Kit v1 (DES-006) is extracted from Postgres's frames as they form — the Kit is discovered here, not decreed. *Exit: PDS appendices complete; Kit v1 draft circulating with the Valkey/Storage/Queue paper-rehearsal attached.*

## 5.4 Section-by-section notes (where this unit needs special care)

- **§1 Intent:** Shell's sentence targets *"I understood the whole platform from one screen"*; Postgres's targets *"branching a database felt like branching code."* Write these first; every dispute returns to them.
- **§7 Parity:** the unit's parity tables are the CLI's real birth — `steloit project create`, `steloit db create/branch/scale/restore` specified to output-shape and exit-code level with the DX designer as co-author, not reviewer.
- **§10 A11y:** topology map (graph a11y: focus order through nodes/edges, non-visual traversal), wizard focus management across async waits, credential-reveal screen-reader behavior.
- **§12 AI:** Shell registers the recommendation flow (evidence display: *why these services*, per GOV-001's example); Postgres registers nothing yet (query assistance is a v1.5 revision) — writing "N/A, deferred to rev" is the discipline on display.
- **§13 Cost & danger:** the wizard estimate panel is G4's first rendering — treat its review as a BIZ-001-owner conversation too; Postgres's danger map is the platform's hardest (destroy-with-branches semantics).
- **§14:** instrument J1 stepwise (the five-minute test's funnel), branch-creation time-to-comprehension, wizard abandonment points.

## 5.5 Validation & gates

Studies (DES-008): wizard + estimate comprehension prototype (required — pioneer pattern); environment-switch filter prototype (required — feel-critical); branching mental-model study on A5-level flows; terminology follow-up. Gates: **combined G-A** end of wk 2 (§§1–5 both docs); **combined G-B** end of wk 6 (§§6–14, Kit v1, pattern registry entries) — hi-fi unlocks for both on one decision. Reviewers per GOV-005 §10.2 plus FND-003's owner at both gates (the co-evolution seat). Fail-fast: same-week re-review.

## 5.6 Expected outputs

Two Approved PDSs; Kit v1 + DES-006; ~11 registered patterns; EXP-001/003 skeletons made concrete; a set of PRD-001/FND-003 change requests (expected, healthy, counted); study readouts; the Valkey pre-check result; and the template shakedown — a DES-000 amendment list from the first real usage.

---

# Deliverable 6 — Readiness Assessment

**Verdict: GO — the PDS phase may begin the moment the Wave-P0 entry checklist (§5.1) is green — with four named questions to resolve *inside* the first unit's early weeks, and one process condition.**

## 6.1 Why the foundation is sufficient

Every layer the PDS system requires now exists and is load-bearing: the architecture supplies the primitives and hierarchy the object maps will render; the Master Plan supplies the PRD/FND authorities and their delivery waves (aligned so specs land just ahead of the PDSs that cite them); GOV-005 supplies the global decisions, journeys, workflow standards, gates, and Figma machinery; DES-000 supplies the anatomy, artifacts, notation, and review material. Nothing in Deliverables 1–5 required inventing a rule — only applying them. That is the definition of ready.

## 6.2 Resolve-inside-P0 (not blockers to start; blockers to specific artifacts)

- **R1 — The CLI noun for PostgreSQL** (`db` vs `postgres`) and the ID/slug scheme: GOV-003 items, needed by A2/A7. *Owner: GOV-003; deadline: end of week 1.*
- **R2 — External-runtime binding ergonomics** (the Master Plan's flagged ambiguity, FND-004): J3's external variant is v0's *primary* variant (no Steloit compute exists yet) — the design cannot defer what the first users will do first. *Owner: FND-004 author + Shell designer, jointly; deadline: before A3 completes.*
- **R3 — Branch cost & lifecycle presentation:** what a branch costs, whether it sleeps, who is warned (FND-010 × PRD-001) — the wizard/estimate and branch UX need the *model*, not the prices. *Deadline: before A6.*
- **R4 — Recommendation-flow evidence contract** (PRD-012 draft → the minimum viable proposal-artifact fields for Shell §12). *Deadline: before A5's wizard AI-path is drawn.*

## 6.3 Watch items (risks, not questions)

The FND-003/PDS-001 co-evolution loop must be timeboxed (spec churn is healthy for two weeks and pathological after four — the Systems Review owns the clock); the Valkey pre-check at P0 exit is the system's first falsifiable claim and must be run honestly by a non-P0 designer; and staffing — the plan assumes the DX and content designers exist on day one, because their absence is invisible until A7 and catastrophic then.

## 6.4 The process condition

PDS-000-EXEMPLAR (DES-000 §9's teaching example) is published *before* P0's A5 begins — a one-week task that repays itself on every document after the first two.

---

**The plan in one paragraph:** fifteen PDSs, two reserved; a dependency graph rooted in one braided pair; four waves whose exit gates measure the system's own promises (the Valkey thinness test, the bending effort curve, the seam-free journeys); a first unit — Shell leading PostgreSQL by one artifact step — chosen because every alternative either designs containers without content or content without a container; a week-by-week authoring blueprint that is DES-000's pipeline made concrete; and a GO verdict with four named questions on a two-to-four-week fuse. The next artifact anyone produces is Shell's object map, in week one, with the terminology standard open beside it.

*— End of execution plan —*
