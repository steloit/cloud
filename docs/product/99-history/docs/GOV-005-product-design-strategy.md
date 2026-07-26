# Steloit Product Design Strategy

**Document class:** Internal — Product Design Phase Blueprint
**Document ID:** GOV-005 *(registered per GOV-004)*
**Status:** v1.0 — For approval
**Inputs (approved, immutable):**
- GOV-001 — Vision: *The Developer Cloud Platform*
- GOV-002 — Product Architecture Specification
- GOV-004-adjacent — Documentation Master Plan (waves, versions, EXP series)

**Audience:** Design, Product, and Engineering leadership; every designer who will ever open the Steloit Figma
**Purpose:** Define how Steloit transitions from architecture to design — the roadmap, sequencing, standards, file organization, and review machinery that will govern every future mockup, prototype, and interaction decision. This document designs the *design phase*; it deliberately contains no UI.

---

## 0. The Design Thesis

Steloit's architecture made a bet that most platforms lose: that N products can feel like one. GOV-002 backed that bet with mechanisms — nine primitives, one service grammar, one Binding fabric. **The design phase's job is to make those mechanisms *felt*.** A developer will never read FND-003; they will experience it as the eerie familiarity of the fifth service page they open.

This produces the phase's central inversion, and everything below follows from it:

> **On most products, designers design screens and consistency is hoped for. On Steloit, designers design *the system that generates screens*, and individual products are instantiations of it.**

Four principles govern the phase:

1. **The grammar is the client.** FND-003 (Service Grammar) is design's most important stakeholder. Every pattern must be an expression of a grammar rule; every grammar rule must have exactly one pattern. When a product's needs and the grammar conflict, the conflict is escalated as a *grammar question*, never resolved locally with a bespoke screen.
2. **Design one version ahead, one product deep.** Mirroring the documentation plan's pipeline: while engineering builds vN, design finishes vN+0.5 and explores vN+1. And at any moment, exactly one product is allowed to be *pioneering* (establishing new patterns); all others are *instantiating*. Two pioneers at once is how design systems fork.
3. **Journeys own screens; screens own nothing.** No screen is designed until the journey it serves is mapped and approved. A screen's quality is judged by its journey's success, not its own polish.
4. **CLI and Console are one design.** GOV-002 §2.1 makes all surfaces the same tree. Therefore the CLI is a *designed surface* inside this phase — its grammar, output, and error voice are design deliverables with the same review bar as Console screens. A flow is not "designed" until its Console, CLI, and API expressions are designed together.

---

# 1. The Design Phase Operating Model

## 1.1 Where design sits in the approved pipeline

The Documentation Master Plan's waves already reserve design's slots (EXP-001/002/003/006/008 in Waves 1–3). This strategy expands those slots into a full phase with its own internal stages, synchronized to the same clock:

```
Documentation waves:   W0 ──► W1 ──► W2 ──► W3 ──► W4 ...
                              │       │       │      │
Design stages:         D0 ────┴─ D1 ──┴─ D2 ──┴─ D3 ─┴─ D4 ...
                       │      │       │       │       │
                       │      │       │       │       └ v2 design (Compute, Previews)
                       │      │       │       └ Data-layer instantiation + Console GA design
                       │      │       └ Pioneer product design (Postgres) + system v1
                       │      └ Foundations: global decisions, journeys, design language
                       └ Design brief, research setup, terminology intake (GOV-003)
Engineering:                          v0 build ──► v0.5 build ──► v1 build ...
```

The invariant, inherited from the master plan: **design reaches "Build-Ready" for version vN before vN's build wave begins**, and design authorship of vN+1 overlaps vN's construction. Design is never the bottleneck and never the archaeologist.

## 1.2 Team shape (roles, not headcount)

- **Design Systems track** — owns the language, tokens, components, and the Service Page Kit; the guardians of FND-003's visual expression.
- **Platform Experience track** — owns cross-product journeys, IA (EXP-001), navigation, search, notifications, onboarding (EXP-006).
- **Product Design track(s)** — embedded per product wave; pioneers or instantiates per the sequencing in §5.
- **DX Design** — owns CLI (EXP-003), error voice, docs experience (EXP-007), SDK ergonomics review. Staffed by designer-engineers; this track is non-negotiable for a developer platform and chronically under-hired elsewhere — treat it as a differentiator.
- **Design Ops** — owns the Figma architecture (§8), tooling, review logistics, and the pattern registry (§9).
- **Research** — continuous developer research; owns the participant panel and the study cadence in §1.3.

## 1.3 Research spine (runs the entire phase)

Design decisions below are sequenced assuming a continuous research program, established in D0:

- **Foundational studies (D0–D1):** developer mental models of *project/environment/service* (validating the hierarchy's learnability — the architecture is immutable, but its *presentation* is design's to get right); competitive teardown of the reference set (Vercel, Neon, Railway, Render, Supabase, PlanetScale, Fly.io) focused on where their coherence breaks; terminology comprehension tests feeding GOV-003.
- **Pattern validation (D1–D2):** prototype tests of the provisioning flow, context bar, environment-switch-as-filter, and cost-estimate presentation — the four highest-risk interaction bets.
- **Journey benchmarks (D2+):** the five-minute test (GOV-002 §10) instrumented as a recurring usability benchmark with target task-times, run on every release candidate. Design's success metric is the same as the platform's.

---

# 2. Global UX Decisions — Before Any Product Is Designed

These decisions shape every screen. Made once, in D1, recorded in the **Platform Experience Standards** document (DES-002, §7), and treated as immutable-until-revised with the same discipline as architecture. Deciding them per-product later would fork the platform's feel; this list is the design phase's equivalent of the architecture's nine primitives.

**G1 — The context model.** How Org / Project / Environment context is displayed, switched, persisted across sessions, deep-linked, and recovered when a URL's context conflicts with the user's last state. Includes the signature interaction from GOV-002 §4.2: *environment switching is a filter, not a navigation* — its animation, its URL behavior, its handling of pages that don't exist in the target environment. This is the single most-used interaction on the platform; it is decided first.

**G2 — The status & health language.** One vocabulary of states (`provisioning, ready, degraded, suspended, deleting` per FND-003) with one visual system: colors, icons, motion (does "provisioning" pulse?), and the precise semantic of "green." Every product inherits it; no product may invent a status color.

**G3 — The danger model.** A single platform-wide standard for destructive and expensive actions, encoding GOV-002 §5's rule (*expensive explicit, reversible easy, irreversible slow*): confirmation tiers (click → typed-name → typed-name + delay), the Danger Zone placement, undo vs. grace-period presentation, and the visual grammar that distinguishes *reversible* from *irreversible* at a glance.

**G4 — The cost-visibility model.** Where and how money appears: the estimate panel in every provisioning flow (FND-010's contract), running-cost placement on service/environment/project pages, the visual treatment of estimate-vs-actual, and the rule that **no action with a recurring cost is ever confirmed without its cost shown**. Cost is a first-class UI citizen, not a billing-page exile — this is the pricing philosophy made visible.

**G5 — The form & configuration model.** One pattern for how anything is created or configured: progressive disclosure rules (defaults visible, advanced collapsed), inline validation voice, the relationship between form UI and its `--flag`/API equivalent (every form field maps to a named parameter, and complex forms offer a "view as CLI/Terraform" toggle — the platform teaching its own automation, a signature DX move).

**G6 — The empty, loading, and error triad.** Platform-wide standards: empty states always teach (what this is + first action + docs link), loading states never lie (skeletons for known shapes, progress for long operations via FND-002's async pattern), and errors follow one anatomy (what happened → why, if known → what to do → reference ID). The error-message voice guide lives here and covers Console *and* CLI *and* API error strings — one voice, three surfaces.

**G7 — The time model.** Timezone display rules (UTC vs local, and where each), timestamp formats, relative-time thresholds, and the time-range picker standard shared by all metrics/logs views. Trivial-seeming; observability dies without it being uniform.

**G8 — The identity of `production`.** How the UI treats the production environment differently: ambient signal (persistent, calm, not alarmist), heightened confirmation defaults, and policy-gated visual cues. Developers must always know when they're pointing at prod — without the UI crying wolf.

**G9 — The AI presence model.** Where intelligence appears (a consistent panel/affordance per product surface + ⌘K entry, per GOV-002 §3.6), how proposals render (the *proposal artifact* from PRD-012: recommendation → reasoning → evidence → apply-with-permission), how AI-generated content is always visually attributed, and the never-zones (no AI affordances in IAM, secrets values, deletion flows). Designed globally so PRD-012's four laws have one UI expression, not fourteen.

**G10 — Density, keyboard, and speed.** The platform's stance on information density (developer-dense, not marketing-airy), full keyboard navigability, ⌘K as universal escape hatch, and perceived-performance budgets (every navigation < 200ms to first meaningful paint or it gets a skeleton). Speed is a design requirement with numbers, reviewed like any other.

**G11 — Theming and accessibility floor.** Dark mode as first-class from day one (developers live there; retrofitting is misery), WCAG 2.1 AA as the hard floor (EXP-002), and the token architecture that makes both cheap (§4).

---
# 3. Journeys Before Screens

No product screen enters high fidelity until its governing journey is mapped, reviewed, and approved. Journeys are the design phase's equivalent of the foundation specs: products cite them. Eleven **canonical journeys** are designed in D1–D2, in this order:

| # | Journey | Why it precedes screens |
|---|---|---|
| J1 | **First five minutes** — signup → org → guided project creation (incl. the AI-recommendation variant from GOV-001) → first provisioned service → first successful connection from code | The platform's front door and its benchmark metric (EXP-006). Every onboarding screen serves this map. |
| J2 | **The daily loop** — open project → check health → view a service → tail logs → make a change | The most-repeated journey; optimizing it justifies the density and speed decisions (G10). |
| J3 | **Add a capability** — existing project → add service → bind to consumer → config arrives in code | The Binding fabric as experienced; establishes the create-and-connect pattern every service reuses. |
| J4 | **Something is wrong** — alert → notification → correlated timeline → culprit service → logs at timestamp → (AI explanation) → fix → verify | GOV-002 §6's worked example, designed end-to-end. The pivot-don't-switch interaction is born here. |
| J5 | **Protect and recover** — configure retention → disaster strikes → restore-to-branch → verify → promote | Restore as rehearsable workflow (PRD-006). Designed as a journey because its comfort under stress *is* the product. |
| J6 | **Promote a change** — staging → diff → confirm → production, with rollback path visible | Environment promotion (GOV-002 §5.9); establishes the diff-presentation pattern later reused by deployments and AI proposals. |
| J7 | **Bring the team** — invite → role assignment → teammate's first session in an existing project | IAM as experience; also defines the *second* user's onboarding, which J1 does not cover. |
| J8 | **Understand the bill** — estimate at provisioning → month passes → bill arrives → "what changed" → act on a recommendation | Cost transparency as a journey, not a page (G4, PRD-009). |
| J9 | **Retire safely** — archive vs destroy decision → grace period → gone | The danger model (G3) exercised end-to-end. |
| J10 | **CLI-first parallel** — J1 through J6 executed entirely in the terminal | Not a separate product: each journey map carries a CLI lane. J10 is the audit that the lanes are real. |
| J11 | **Ship a change** *(designed in D3 for v2)* — push → build → preview env with branched DB → review → merge → production | The v2 flagship (PRD-011). Journey-mapped early in D3 so v2 screens instantiate rather than improvise. |

Each journey map includes: personas in context, entry points, the Console lane and the CLI lane side by side, emotional temperature (where anxiety peaks — J4, J5, J9 — design spends its care there), instrumentation points, and the pattern obligations it creates.

---

# 4. The Design System: What Gets Built First, and in What Order

The design system (EXP-002's deliverable) is constructed in four deliberate layers. The ordering rule: **each layer is validated by the layer above it before it expands** — tokens are proven by components, components by patterns, patterns by the pioneer product. Building layer 1 exhaustively before touching layer 3 is the classic design-system death spiral; Steloit builds vertically thin slices instead.

**Layer 1 — Foundations (D1).** Design tokens: color (semantic-first: `status-ready`, `cost-emphasis`, `danger-irreversible` — not `green-500`), typography (including the monospace system for IDs, logs, and code — chosen with the same care as the brand face; developers read mono all day), spacing, elevation, motion durations, and the dark/light theme architecture (G11). Plus the **status language** (G2) and **iconography system** (service-type icons are the topology map's alphabet — designed as a family, extensible for every future service GOV-002 §8 promises).

**Layer 2 — Core components (D1–D2).** Only what the pioneer product needs, built properly: context bar, navigation shell, tables (the workhorse: sortable, dense, keyboard-navigable), forms per G5, buttons/menus/dialogs with the G3 danger tiers built in, status badges, toasts/inline alerts per G6, tabs, the time-range picker (G7), sparklines and the chart primitives, code/connection-string display (with reveal/copy/rotate affordances — a component other platforms treat as an afterthought and developers touch daily), log line renderer, empty-state frame, cost display components (estimate panel, running-cost chip).

**Layer 3 — Platform patterns (D2).** Compositions that encode the workflows of §5: the Provisioning Wizard, the Service Page Anatomy template, the Binding visualization (node + edge, list + graph), the Diff Presenter (J6's child, reused by deployments and AI proposals), the Restore Flow frame, the Danger Zone, the Proposal Artifact card (G9), the correlated timeline, the ⌘K palette.

**Layer 4 — The Service Page Kit (D2, the system's capstone).** A Figma-native template implementing FND-003's page anatomy: Overview/Metrics/Logs/*Type tabs*/Backups/Settings, pre-wired with layers 1–3. **This kit is the mechanism by which "a developer fluent in one service is fluent in all" becomes cheap to maintain**: designing a new service type means instantiating the kit and designing only the type-specific tabs. The kit is versioned, published as a Figma library, and changing it requires the same review as changing FND-003 — because it *is* FND-003, rendered.

Component engineering (the coded library) begins in D2 against layers 1–2, so that v0's build wave receives components, not PNGs. The Figma library and the coded library share token sources and names — one dictionary, two renderings (§9).

---

# 5. Platform-Wide Workflow Standards

Seven workflows are designed once, as standards, and instantiated everywhere. Each gets a named pattern spec (§7) and a Figma pattern page (§8). These are the design phase's constitution, ratified in D2 by the pioneer product's proof:

1. **Create & Provision** — the wizard: name → select/describe (AI variant) → configure-with-defaults → *cost estimate* → confirm → async progress → ready-with-next-step. Every service type, and project creation itself, is this wizard with different middles.
2. **Connect (Bind)** — pick consumer/target → scope declaration → preview of what will be injected → confirm → visible on both services and on the topology map. Includes the external-runtime variant (Steloit-as-data-layer mode, flagged in the master plan's ambiguity list — design resolves its ergonomics in D2 with FND-004's authors).
3. **Observe & Pivot** — the correlated timeline and the four-click pivot (alert → metrics → logs-at-timestamp → cause). One interaction model whether the trouble is a database, a queue, or (later) a deployment.
4. **Change Safely** — the Diff Presenter governing environment promotion, config changes, deployment promotion (v2), and AI proposals: *what will change, what it costs, how to undo* — always those three, always in that order.
5. **Protect & Restore** — backup status as ambient signal; restore as guided, rehearsable, branch-first flow.
6. **Scale** — current shape → proposed shape → cost delta → confirm; identical for a Postgres resize, a Valkey memory bump, or (later) compute autoscale ranges.
7. **Retire** — archive/destroy per the danger model, with the automatic-retirement variant (preview environments) designed as visible lifecycle, not silent garbage collection.

The test for any future screen: *which standard workflow is this an instance of?* A screen that answers "none" is either a new standard (rare, escalated, added here) or a design error.

---

# 6. Product Design Order: Pioneers and Instantiators

The sequencing principle from §0: one pioneer at a time; everyone else instantiates. The order below interleaves with the D-stages and mirrors the documentation waves — design authors alongside spec authors, by intent.

| Order | Product | Role | Establishes / Reuses |
|---|---|---|---|
| 1 | **Project & Environment shell** (creation flow, project overview, topology map, context bar) | **Pioneer** | Establishes: G1 context model in practice, provisioning wizard (workflow 1), topology visualization, project overview anatomy. Designed first because it is every product's container. |
| 2 | **Managed PostgreSQL** | **Pioneer** (the *canonical* service) | Establishes: the Service Page Kit's first real instantiation, connection-credential UX, backup/restore surfaces, scale flow, branching UX (type tab), query-insights surface, the danger zone in anger. PRD-001's designer and spec author work as a pair — the design *is* part of the spec's review. |
| 3 | **Observability** | **Pioneer** (the *correlation* surface) | Establishes: metrics dashboards grammar, log query UX, alert configuration, the correlated timeline, workflow 3. Designed immediately after Postgres so its dashboards have a real service to observe. |
| 4 | **IAM & onboarding** (with J1, J7) | Instantiator + small pioneer | Reuses forms, tables, danger model; establishes invitation and API-key patterns (small, contained). |
| 5 | **Valkey** | **Pure instantiator — deliberately** | The system's first stress test: target is ≥80% kit reuse, design time a small fraction of Postgres's. If Valkey needs new patterns beyond its type tabs, the kit failed — fix the kit, not the mockup. This checkpoint is a formal gate (§10). |
| 6 | **Object Storage** | Instantiator + one pattern | Reuses the kit; establishes the object/bucket browser (the one genuinely novel surface in the data layer) and public-exposure warnings per G8's cousin logic. |
| 7 | **Queue** | Instantiator + one pattern | Reuses the kit; establishes the message/DLQ inspector and the consumer-binding view (interface for v2's Workers designed now as a stub). |
| 8 | **Secrets, Cost & Usage, Backup-as-workflow** | Instantiators | Secrets reuses forms/tables/reveal patterns; Cost instantiates G4 at page scale (J8); Backup elevates workflow 5 to its cross-service home. |
| 9 | **AI surfaces** (provisioning recommendations first, per PRD-012's scope) | Instantiator of G9 | The proposal-artifact card designed globally in D1–D2 gets its first live instantiation; per-product AI panels follow the capability registry, never ahead of it. |
| 10 | **Compute + Deployments + Previews** (D3, for v2) | **Pioneer** (the last big one) | Establishes: deployment timeline, build logs, rollout visualization, domain/TLS setup, J11. Reuses: the kit, diff presenter, danger model, scale flow — v2's design cost is a fraction of its engineering cost *because* of everything above. |
| 11+ | Vector/Search, enterprise surfaces (SSO, policies, private networking), BYOC console | Instantiators | Enterprise arrives as *intensified existing patterns* — the design mirror of GOV-002 §9.4. A v3 that needs a new design language is the same alarm as a v3 that needs new specs. |

**Explicit reuse economics:** products 1–3 consume roughly 60% of the phase's original design effort; products 4–9 should consume under 30% combined. If the curve doesn't bend at product 5, the system layer is underinvested — reallocate immediately. This curve is tracked and reported (§10).

---
# 7. Design Documents Required Before High-Fidelity UI

Registered in the master plan's registry as a **DES series** (design's parallel to FND/PRD). High-fidelity work on any product may not begin until the documents its screens depend on are Approved. The set is deliberately small — eight documents, not a bureaucracy:

| ID | Document | Contents | Gate it unlocks |
|---|---|---|---|
| DES-001 | **Design Principles & Experience Charter** | The design thesis (§0) expanded: the platform's personality (calm, dense, honest, fast), voice & tone foundations, the "expensive explicit / reversible easy / irreversible slow" rule as design law, anti-patterns register (no dark patterns, no fake urgency, no engagement tricks — echoing PRD-012's trust stance). | All design work |
| DES-002 | **Platform Experience Standards** | The G1–G11 decisions (§2), each with rationale, examples, and its CLI expression. The design phase's constitution. | Any pattern work |
| DES-003 | **Canonical Journey Maps** | The eleven journeys of §3, with lanes, metrics, and pattern obligations. | Any product's hi-fi |
| DES-004 | **Design Token & Theme Spec** | Layer-1 foundations: token dictionary, theme architecture, accessibility floor evidence. Shared source of truth with the coded library. | Component build |
| DES-005 | **Component & Pattern Specifications** | Living spec per component/pattern (anatomy, states, behavior, keyboard, a11y, content rules, do/don't). Grows with the library; a component without its spec page is not "done." | Kit assembly |
| DES-006 | **Service Page Kit Spec** | FND-003's anatomy as normative design contract: required tabs, permitted type-tab patterns, extension rules for future services, versioning policy. Co-signed by FND-003's owner. | All service-product design (products 2, 5–7, 10+) |
| DES-007 | **Content & Voice Standard** | Terminology (imports GOV-003 wholesale), error-message anatomy and voice, CLI output style, empty-state copy patterns, microcopy rules for danger/cost/AI attribution. Writing is design; this doc has the same authority as visual specs. | Any screen with words — i.e., all of them |
| DES-008 | **Research Playbook & Benchmark Definitions** | The study cadence (§1.3), the five-minute-test protocol, journey benchmark task lists and target times, participant panel standards. | D2 validation studies |

Each product then gets a lightweight **Product Design Brief** (2–3 pages, not a new document class): the PRD it serves, journeys touched, patterns reused, the *one* thing it may pioneer (if pioneer-designated), open questions, success measures. Briefs are reviewed at the Journey Gate (§10) — the anti-scope-creep instrument.

---

# 8. Figma Organization

One workspace, mirrored to the platform's own hierarchy so that navigating the design mirrors navigating the product. Structure is Design Ops-owned; deviation is a review finding.

```
Steloit (Figma Organization)
│
├── 🏛 00 — Foundations                      [libraries: published]
│   ├── Tokens & Themes            (DES-004 rendered; dark + light)
│   ├── Icons & Illustration       (incl. the service-type icon family)
│   └── Core Components            (Layer 2; one file per component group)
│
├── 🧩 01 — Patterns                         [library: published]
│   ├── Platform Patterns          (Layer 3: wizard, diff, danger, proposal card…)
│   └── Service Page Kit           (Layer 4; versioned pages: Kit v1.0, v1.1…)
│
├── 🗺 02 — Journeys                         [FigJam + Figma]
│   └── J1 … J11                   (one file each: map, lanes, prototypes)
│
├── 📦 03 — Products                         [one project per product]
│   ├── Shell & Projects
│   ├── PostgreSQL
│   ├── Observability
│   ├── IAM & Onboarding
│   ├── Valkey / Storage / Queue / …
│   └── (each: 🔍 Exploration → 🎯 Proposed → ✅ Build-Ready → 🚢 Shipped pages)
│
├── ⌨️ 04 — DX Surfaces
│   ├── CLI Design                 (command grammar boards, output specs, error voice)
│   └── Docs & Onboarding Content
│
├── 🔬 05 — Research
│   └── Studies, benchmark recordings, synthesis boards
│
└── 🗄 99 — Archive                          (superseded work; nothing is deleted, nothing stale stays visible)
```

**File discipline (the rules that keep 50 designers coherent):**
- **Page states are workflow states.** Every product file uses the same four pages (Exploration → Proposed → Build-Ready → Shipped). Engineering may only build from *Build-Ready*; anything else linked in a ticket is a process violation. "Shipped" pages are updated to match reality — the file is the as-built record.
- **Libraries flow one way.** Products consume Foundations and Patterns; they never define local components for anything the libraries cover. A needed-but-missing component is a *library request*, triaged weekly by the Systems track — the design mirror of "escalate grammar questions, don't solve locally."
- **Detach is a four-letter word.** Detached instances are lintable violations (automated via Figma analytics/plugins); each one is either a library gap (fix the library) or an error (fix the file).
- **Naming mirrors GOV-003.** Frames, components, and files use canonical terminology; the terminology standard is loaded as a shared glossary. A frame named "DB dashboard" for what GOV-003 calls a "service overview" fails review.
- **Every Build-Ready frame links its authority.** A pinned annotation citing the PRD/FND sections and DES patterns it implements. Design that can't cite its spec is exploration, not delivery.
- **Branching for system changes.** Foundations and Patterns libraries use Figma branching with required review by the Systems track before merge — library changes are the design phase's schema migrations and get migration notes (what instances are affected, what updates auto-propagate).

---

# 9. Consistency Machinery

Consistency is not a review comment; it is a system with five interlocking mechanisms:

1. **One dictionary, three renderings.** GOV-003's terminology + DES-004's tokens exist as a single machine-readable source consumed by the Figma libraries, the coded component library, and the docs. A color, a term, or a state name changed in the dictionary propagates everywhere or nowhere.
2. **The Pattern Registry.** A living index (owned by Design Ops, public to the company) listing every approved pattern, its spec (DES-005), its Figma source, its coded counterpart, and every product instance consuming it. New screens declare their patterns at review; the registry makes reuse *visible* and divergence *auditable*. This is also where the §6 reuse-economics curve is measured.
3. **The Kit as enforcement.** Because service pages are Kit instantiations, consistency across services is structural. The Kit's version history *is* the consistency history; products on old Kit versions appear on a drift dashboard with scheduled upgrades.
4. **Cross-surface parity audits.** Quarterly (and before each version gate), the DX track audits Console↔CLI↔API↔Docs expression of every shipped flow against the journey maps' dual lanes. Parity findings are defects, tracked like bugs — this is how "one design, all surfaces" survives contact with deadlines.
5. **Design linting in CI.** The coded library ships with lint rules (token-only colors, approved components only, a11y assertions); Figma-side plugins check token usage, naming, and detachment. Machines catch drift so human review can spend itself on judgment.

---

# 10. Design Reviews, Gates, and the Handoff to Engineering

## 10.1 The review cadence (continuous)

- **Weekly Open Crit** — any work, any stage, advisory only. Culture-building; keeps exploration cheap and visible.
- **Systems Review (weekly)** — the Systems track triages library requests, reviews pattern changes, merges library branches. The grammar's design court.
- **DX Review (weekly)** — CLI grammar, error voice, docs experience. Engineers sit in this one by default.
- **Research Readout (bi-weekly)** — findings routed to owning tracks with required written responses (accepted / rejected-with-reason). Research that doesn't change decisions is theater; the response requirement prevents it.

## 10.2 The gates (per product, sequential, each with named approvers)

| Gate | Question answered | Approvers | Exit artifact |
|---|---|---|---|
| **G-A · Journey Gate** | Is the journey right? | Platform Experience lead + PM + PRD owner | Approved journey map (DES-003 entry) + Product Design Brief |
| **G-B · Pattern Gate** | Does it compose from approved patterns — and is any pioneering justified? | Design Systems lead (+ FND-003 owner if grammar-adjacent) | Pattern declaration in the Registry; library requests filed |
| **G-C · Fidelity Gate** | Is the Build-Ready design correct, complete, consistent, accessible — on *all* surfaces? | VP Design or delegate + Eng lead + PRD owner; DX lead for the CLI lane | Build-Ready pages; annotated states (loading/empty/error per G6); a11y checklist; content per DES-007 |
| **G-D · Build Gate** *(joint with PRC-002)* | Can engineering start — and does everyone agree what "matching the design" means? | Eng lead + designer + PM | Handoff package: Build-Ready link, component/token manifest, interaction specs, instrumentation plan, open-questions register (empty or accepted) |

**Rules of the gates:** G-A before any hi-fi (Principle 3, enforced). G-C requires the *triad* — every state, every surface, every word — because "the happy path in light mode" is where inconsistency breeds. Approvals are recorded in the file (pinned) and the registry; verbal approval does not exist. A gate may be failed *fast* — same-week re-review — so rigor never becomes latency.

## 10.3 Design during and after build

The designer stays paired through construction: implementation review against Build-Ready (visual QA is the designer's, not QA's), deviation decisions recorded (either the build changes or the file does — never silent drift), and post-ship benchmark runs (DES-008) feeding the next revision. Shipped pages update within the sprint. The design corpus, like the spec corpus, is living or it is lying.

---

# 11. The Design Roadmap (consolidated)

| Stage | Calendar shape | Deliverables | Exit criteria |
|---|---|---|---|
| **D0 — Setup** | ~2 wks, with doc Wave 0 | DES-001 draft, DES-008, research panel live, Figma org scaffolded, GOV-003 terminology intake, competitive teardown started | Charter approved; foundational studies fielded |
| **D1 — Foundations** | ~4–6 wks, with Wave 1 | G1–G11 decided (DES-002), J1–J9 mapped (DES-003), Layer 1 tokens + Layer 2 core components underway (DES-004/005 begun), DES-007 v1 | DES-002/003 approved; pattern-validation prototypes tested |
| **D2 — Pioneers & System v1** | ~6–8 wks, with Wave 2 | Shell + PostgreSQL + Observability designed through G-C; Layers 3–4 (patterns + Service Page Kit v1, DES-006); coded components begin; CLI grammar boards through DX review; **Valkey instantiation checkpoint** | v0 surfaces Build-Ready before v0 build; kit-reuse gate passed |
| **D3 — The Data Layer & GA** | ~6–8 wks, with Wave 3 | Storage/Queue/Secrets/Cost/Backup/AI-recommendations instantiated; J8/J9 shipped; Console GA polish; EXP-006 onboarding built; J11 journey-mapped; parity audit #1 | v0.5 and v1 surfaces Build-Ready ahead of their builds; reuse curve bent per §6 |
| **D4 — The Developer Cloud** | with Wave 4 | Compute/Deployments/Previews pioneered (product 10), Kit v2 if needed, J11 through all gates | v2 surfaces Build-Ready before v2 build |
| **D5+ — Enterprise & beyond** | with v2.5/v3 | Enterprise surfaces as pattern intensifications; BYOC console parity audit (Cloud↔BYOC must be indistinguishable per GOV-002) | v3 ships with zero new design-language elements — the health check itself |

**Phase-level success metrics** (reported monthly to leadership): five-minute-test time and completion rate; journey benchmark times (J2, J4 especially); kit-reuse percentage and the §6 effort curve; parity-audit defect count trending to zero; detach-violation count; and one qualitative bar — *unprompted user language describing the platform as "consistent" or "it just feels like one thing."* That sentence, appearing in research transcripts, is this entire phase succeeding.

---

# 12. Closing: The Contract This Phase Signs

The architecture promised that every product would reinforce the others. The documentation plan promised the contracts would be written before the products. This strategy signs design's side of the same contract:

- **Decide globally, instantiate locally** — eleven global decisions, eleven journeys, seven workflow standards, one Kit; products inherit before they invent.
- **One pioneer at a time** — Postgres teaches the system; Valkey proves it; Compute completes it.
- **All surfaces, one design** — a flow exists when its Console, CLI, and error messages exist, together.
- **Gates over vibes** — four gates, named approvers, pinned records, fast failure.
- **The system is the product of this phase.** Screens are its evidence.

When implementation begins, engineering should receive not a folder of mockups but a *language* — and every future designer, opening the Figma for the first time, should be able to design a Steloit product they have never seen, correctly, because the platform's coherence lives in the system and not in anyone's memory.

*— End of strategy —*
