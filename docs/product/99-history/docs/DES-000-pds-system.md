# Steloit Product Design Specification System

**Document class:** Internal — Design Governance
**Document ID:** DES-000 *(the root of the DES series; registered per GOV-004)*
**Status:** v1.0 — For approval
**Inputs (approved, immutable):**
- GOV-001 Vision · GOV-002 Product Architecture · Documentation Master Plan · GOV-005 Product Design Strategy

**Audience:** Every designer, PM, and engineering lead who will author, review, or consume a design specification
**Purpose:** Define the Product Design Specification (PDS) — the document class that bridges approved architecture and high-fidelity Figma. After this document is approved, no Steloit product enters high-fidelity design without a PDS, and no PDS is valid unless it conforms to this system.

---

## 0. What a PDS Is, and the Gap It Closes

The approved corpus leaves exactly one gap. GOV-002 defines what the platform *is*. The PRDs define what each product *must do*. The DES series (per GOV-005) defines how the platform *behaves globally* — G1–G11, journeys, workflow standards, the Service Page Kit. Figma will define what each product *looks like*.

Between "what the product must do" and "what it looks like" sits an unwritten layer: **what the product's experience *is*** — its objects, its screens' existence and purpose, its states, its words, its instantiation of the global standards. Today that layer would live in designers' heads and Slack threads. The PDS makes it a document, because GOV-005 §10 forbids high-fidelity work from resting on anything unreviewable.

**Definition:** A PDS is the per-product design contract that translates a PRD into a fully specified experience — structure, behavior, content, and standards-instantiation — at *below-visual* fidelity, such that high-fidelity design becomes rendering rather than inventing.

Three properties follow, and they are the system's laws:

1. **A PDS is a delta document.** Exactly as PRDs are thin deltas over FND specs (Master Plan §2.1), a PDS is a thin delta over the DES series. It never restates a global decision; it *cites and instantiates* it. A PDS that re-explains the danger model is malformed. The corollary: **PDS thinness is a health metric of the global layer** — a fat PDS means DES-002 or the Kit is missing something.
2. **A PDS specifies experience, not pixels.** Its native fidelities are object maps, wireflows, state tables, content inventories, and annotated low-fi wireframes. Visual design decisions (exact layout, spacing, color application) belong to Figma under the token system. The boundary is precise: *the PDS decides that a screen exists, what is on it, and how it behaves; Figma decides how it looks.*
3. **A PDS is testable before Figma.** Every requirement in it is written so a reviewer can answer pass/fail at Gate G-A/G-B (GOV-005 §10.2) — and so the eventual build can be audited against it, not against memory.

---

# 1. The Two-Tier Specification Model

Everything specifiable is either **global** (specified once, in the DES series, inherited by all) or **product** (specified per product, in a PDS). The sorting rule:

> **If two products could reasonably answer the question differently and the platform would still feel coherent, it is a product decision. Otherwise it is global.**

Applying the rule produces this division — the definitive table; disputes are resolved against it:

| Concern | Global (DES series) | Per product (PDS) |
|---|---|---|
| Context & navigation model | G1, EXP-001 | Where this product's objects sit *within* the model |
| Status & health | G2 vocabulary/visuals | Which states this product's type-specific resources add (rare; needs G-B approval) |
| Danger & confirmation | G3 tiers and grammar | Which of this product's actions map to which tier |
| Cost visibility | G4 placements and rules | This product's billable dimensions rendered in those placements |
| Forms & configuration | G5 pattern | This product's fields, defaults, validation rules |
| Empty/loading/error | G6 anatomy and voice | This product's specific empty states, error catalog entries |
| Time | G7 | Nothing (no product may vary it) |
| Production identity | G8 | Nothing |
| AI presence | G9 + PRD-012 laws | This product's registered AI capabilities and their evidence displays |
| Density/keyboard/speed | G10 budgets | Screen-specific keyboard maps within the global scheme |
| Theming/a11y floor | G11, DES-004 | Product-specific a11y risks beyond the floor (e.g., log color-coding) |
| Journeys | DES-003 canonical eleven | Which journeys this product participates in, and its lane details |
| Workflow standards | GOV-005 §5 seven workflows | The product's instantiation table: which standard, with what middles |
| Components & patterns | DES-005, the Kit (DES-006) | Consumption declaration + type-tab designs + library requests |
| Content voice | DES-007, GOV-003 | The product's terminology entries, microcopy inventory, error strings |
| Motion | Global motion spec (§5.7, an addition to DES-004) | Only type-specific data-viz motion, if any |

Two structural consequences:

- **Nothing is specified twice.** If a PDS author feels the need to write something that belongs in the left column, the correct action is a *library/standards request* (GOV-005 §8's mechanism), never local text. Reviewers enforce this mechanically (§7).
- **The global tier is versioned independently.** PDSs pin the versions of DES documents they were written against (like dependency lockfiles); when a global standard revises, the drift dashboard (GOV-005 §9.3's mechanism, extended) lists PDSs requiring reconciliation.

---

# 2. The Anatomy of a PDS

Every PDS uses one template, fourteen sections. Sections marked **[M]** are mandatory for all products; **[C]** are conditional with the condition stated — and a conditional section that doesn't apply is not deleted but marked *"N/A because…"* so absence is always a decision, never an oversight. The GOV-005 "Product Design Brief" is hereby absorbed: it becomes PDS §§1–3 in draft state, which is what Gate G-A reviews; the brief and the PDS are one document at two stages of completeness.

### §1 — Design Intent **[M]**
One page. What this product's experience must *feel like* and accomplish, in the platform's voice. The single sentence a developer should say after first use ("branching a database felt like branching code"). The one thing this design must get right, and the one failure it must avoid. This section is the taste anchor every later dispute returns to.

### §2 — Authorities & Delta Statement **[M]**
The citation spine: the governing PRD (with section references), the FND specs whose contracts surface here, the DES versions pinned, the journeys joined (DES-003), the workflow standards instantiated. Then the **delta statement**: an explicit list of everything this PDS specifies *that is not already determined by an authority* — typically one short list for an instantiator, a longer one for a pioneer. The delta statement is the review's table of contents and the thinness metric made visible.

### §3 — Scope, Non-Goals & Pioneer Declaration **[M]**
Which product surfaces this PDS covers, which version horizon (per the Master Plan's version applicability convention), what is explicitly out. If the product is pioneer-designated (GOV-005 §6), the declaration names the *specific* patterns it is licensed to establish — and nothing else. An instantiator's declaration reads "none"; any pioneering it later attempts is a gate failure by construction.

### §4 — Object Model & Information Architecture **[M]**
The product's nouns, mapped into the platform:
- **Object map:** the product's objects (primary/secondary per GOV-002 §4.1), their relationships, their FND-001 identities, their lifecycle states (importing FND-003's standard states; declaring any type-specific additions).
- **Placement:** where each object lives in navigation (EXP-001 tree position), its URL shape (mirroring FND-002 paths), its breadcrumb, its ⌘K search behavior (indexed fields, result rendering, actions offered).
- **The type-tab plan:** for Kit-based products, which type tabs exist, what each is *for* (one sentence each), and why no more are needed — the anti-sprawl clause.

### §5 — Journeys & Workflow Instantiation **[M]**
Not new journey design — *participation mapping*. For each canonical journey the product joins: its entry and exit points, its lane specifics (Console and CLI, always both), and any product-specific sub-flows (e.g., PostgreSQL's branch-create inside J3). Then the **workflow instantiation table**: each applicable standard workflow (GOV-005 §5's seven) with this product's "middle" — the Provisioning Wizard's product-specific steps, the Scale flow's product-specific shape options, the Retire flow's product-specific consequences copy. Anything not expressible as a standard-workflow instance is either a declared pioneer pattern (§3) or a defect.

### §6 — Interaction Model **[M]** *(the PDS's center of mass; standards in §4 of this document)*
- **Screen inventory:** every screen/panel/dialog, each with: purpose (one sentence), governing pattern, wireframe reference, and the *reason it exists* (which journey step or grammar obligation demands it). Screens without a reason don't survive review.
- **Wireflows:** the screens connected as flows, in the standard notation (§4.2), covering happy paths *and* the unhappy ones (failure, timeout, permission-denied, quota-hit — the FND-002/011 error contracts made navigable).
- **State specification:** for every screen and every async object on it, the state table — empty / loading / partial / ready / degraded / error / forbidden — with content or behavior per state (G6 instantiated with real words, not "TBD").
- **Input & keyboard map:** per-screen keyboard behavior within G10's global scheme; focus order; the destructive-action reachability rules (G3 actions are never a single keystroke).
- **Real-time behavior:** what updates live (log tails, provisioning progress, metric refresh), at what cadence, and what happens on connection loss — the honesty rules of G6 applied to liveness.

### §7 — Surface Parity Specification **[M]**
The table that makes Principle 4 of GOV-005 auditable: every capability in §6, one row, four columns — Console / CLI / API / Docs — each cell naming the expression (command grammar, endpoint, doc type) or an explicit, justified "Console-only"/"CLI-only" (rare; requires DX-lead sign-off, because GOV-002 §3.5's "no Console-only capabilities" is the default law). CLI cells specify: command form, key flags, output shape (human + `--json`), and exit-code mapping. This section is co-authored with the DX Design track; it is where "one design, all surfaces" stops being a slogan.

### §8 — Component & Pattern Consumption **[M]**
The declaration reviewed at Gate G-B: every pattern and component consumed (by registry name and version), every Kit instantiation, and the **library request list** — components or pattern variants the product needs that don't exist. Requests carry a proposed owner (Systems track) and a blocking/non-blocking flag for the product's schedule. The section's discipline: *this list is the only legal channel for novelty.*

### §9 — Content Specification **[M]**
Words are specified before pixels render them:
- **Terminology entries:** the product's nouns/verbs as GOV-003 submissions (new terms enter the standard, or the product uses existing ones — no private vocabulary).
- **Microcopy inventory:** every label, button, tooltip, empty-state, and confirmation string for §6's screens, drafted per DES-007. Placeholder text ("Lorem", "TBD") in a Build-Ready design is a gate failure; the inventory is where real words come from.
- **Error catalog:** the product's user-facing errors, each with the G6 anatomy (what/why/next/reference), mapped to FND-002 error codes — one entry serving Console, CLI, and API messages.
- **Docs obligations:** the concept/task/reference pages this product owes EXP-007, named now so docs ship with the product, not after it.

### §10 — Accessibility Specification **[M]**
Beyond the G11 floor (which is inherited, not restated): the product's specific risks and their resolutions — data-viz color independence (metrics, status-dense tables), log readability, focus management in its long-running flows, screen-reader narration for its live regions (provisioning progress, log tails), and the reduced-motion behavior of any §11 motion. Each item phrased as a testable assertion for the G-C checklist.

### §11 — Motion Specification **[C — required if the product introduces any motion beyond the global system]**
Global motion (durations, easings, the provisioning pulse, transition grammar) lives in DES-004's motion chapter and is inherited silently. This section exists only for type-specific motion — e.g., topology-map edge animation, chart transitions — and must specify purpose (motion communicates state change or spatial continuity; never decoration), tokens used, and the reduced-motion fallback. Most instantiator PDSs mark this *N/A*.

### §12 — AI Behavior Specification **[C — required if the product registers any capability in PRD-012's registry]**
The G9/PRD-012 laws instantiated: each AI capability, its trigger surface, the proposal artifact it emits (recommendation/plan/diff), the *evidence display* (what data the AI examined, rendered how), the apply-path (which existing permissioned action it routes through), the failure/uncertainty presentation, and the explicit list of this product's AI never-zones. If the product has no registered capabilities, this section is one line: *N/A — no capabilities registered*; unregistered AI affordances discovered in design are a G-B failure.

### §13 — Cost & Danger Map **[M]**
Two short tables that operationalize G3/G4 for this product: (a) every action with a recurring or significant one-time cost, and where its estimate/actual renders; (b) every destructive or expensive action, its assigned danger tier, its undo/grace story, and its confirmation copy (cross-referenced to §9). Reviewers read this section against GOV-002 §5's law — *expensive explicit, reversible easy, irreversible slow* — line by line.

### §14 — Instrumentation, Assumptions & Open Questions **[M]**
The measurement plan (journey benchmark hooks per DES-008, funnel events for this product's wizard, error-rate signals); the assumptions register (design decisions resting on unvalidated beliefs, each with its validation plan); and the open-questions register, which must be **empty or explicitly accepted by the approvers** at Gate G-C — the same rule the Master Plan applies to specs entering build.

---
# 3. The Pre-Figma Artifact Pipeline

A PDS is built from artifacts produced in a fixed order, each cheap to change until the next hardens it. High-fidelity Figma is the *eighth* step, not the first. The pipeline (per product):

```
A1 Object Map ─► A2 IA Placement ─► A3 Journey Overlay ─► A4 Workflow Instantiation
      ─► A5 Wireflows ─► A6 State Tables ─► A7 Content-First Drafts ─► [Gates A/B]
      ─► A8 Low-fi Wireframes (annotated) ─► [PDS complete] ─► Figma hi-fi [Gate C]
```

**A1 — Object map** (whiteboard/FigJam): the product's nouns and relationships, checked against FND-001 and the PRD. One hour of object modeling prevents weeks of screen churn — screens are views over objects, and wrong objects make every screen wrong.

**A2 — IA placement** : objects slotted into EXP-001's tree, URLs drafted, ⌘K behavior noted. Produces PDS §4.

**A3 — Journey overlay**: the product's participation drawn onto the canonical journey maps (in the 02-Journeys Figma files, as product-colored lanes) — making cross-product journey load visible in one place. Produces PDS §5's first half.

**A4 — Workflow instantiation**: the seven-standards table filled in. The moment novelty is detected, it is written into §3's pioneer declaration or reformulated — *before* any screen exists to fall in love with.

**A5 — Wireflows**: screens as named boxes, flows as arrows, in the standard notation (§4.2). This is where screen inventory (§6) is born and where most design debate should happen — arguing about boxes is cheap.

**A6 — State tables**: every box's states enumerated with intended content. Filling these tables is what flushes out the questions PRDs must answer (e.g., "what does a Postgres branch show while its storage is copying?") — the PDS's function as *spec pressure-tester*, formalized.

**A7 — Content-first drafts**: §9's microcopy inventory and error catalog written in plain documents, reviewed by DES-007's owner, *before* layout exists. Writing the words first is the cheapest known usability method and enforces that Steloit screens are built around sentences, not around rectangles awaiting sentences.

**A8 — Low-fi wireframes**: only now, and only at the fidelity §5.1 permits. Annotated with pattern citations and state references. These live in the product file's *Exploration → Proposed* pages and become the PDS's appendix.

**Prototypes** are required in exactly two cases: (a) a declared pioneer pattern (its validation study per DES-008 needs something to test), and (b) any interaction whose feel determines its design (the environment-switch filter, the diff presenter). Everything else prototypes *after* G-C in hi-fi, if at all. Prototyping instantiations of proven patterns is spend without information.

---

# 4. Interaction Documentation Standards

The notational law of the system — so every PDS reads the same:

**4.1 State machines over adjectives.** Any object or screen with more than two states is documented as a state table (state / entry condition / display / available actions / exit transitions), using FND-003's state names verbatim. "The page shows a spinner while loading" is prose; the table form is specification.

**4.2 Wireflow notation.** Boxes = screens/panels (named per GOV-003), solid arrows = user actions (labeled with the action verb), dashed arrows = system transitions (labeled with the triggering event), red-bordered boxes = G3-tiered actions, ⌘ badge = keyboard-reachable, CLI ribbon = the parallel command beneath the flow. One notation, taught in one page of DES-000's appendix, used by everyone.

**4.3 Wireframe fidelity rules.** Low-fi means: real information hierarchy, real content (from A7 — never lorem), grayscale, token-agnostic, pattern-cited. Explicitly forbidden at this stage: color semantics (G2's job), precise spacing, visual styling. The line exists so review argues about *substance* at G-A/G-B and *rendering* at G-C — never both at once.

**4.4 The screen declaration.** Every screen in §6 carries the same six-line header: *Purpose · Pattern · Journey step(s) served · Primary object · States (link to table) · CLI counterpart.* A screen that cannot fill all six lines is not yet designed.

---

# 5. Domain-by-Domain: How Each Concern Is Specified

A one-place summary of where each named concern lives and in what form — the answer key for "where do I write this?":

| Domain | Global home | Per-product home | Form |
|---|---|---|---|
| Wireframes | fidelity rules here (§4.3) | PDS §6 + appendix | Annotated low-fi, A8 |
| User journeys | DES-003 maps | PDS §5 participation | Journey overlays, A3 |
| Information architecture | EXP-001, GOV-002 §4 | PDS §4 | Object map + placement, A1–A2 |
| Workflows | GOV-005 §5 standards | PDS §5 instantiation table | A4 table |
| Interaction models | DES-002 (G-decisions), DES-005 patterns | PDS §6 | State tables + wireflows, A5–A6 |
| Accessibility | G11 floor, DES-004 evidence | PDS §10 risk register | Testable assertions |
| Content | DES-007 voice, GOV-003 terms | PDS §9 inventories | Content-first drafts, A7 |
| Motion | DES-004 motion chapter | PDS §11 (usually N/A) | Purpose + token + fallback |
| AI behavior | G9, PRD-012 laws & registry | PDS §12 | Capability instantiation table |
| Components | DES-005/006, registry | PDS §8 declaration | Consumption + request lists |
| Cost & danger | G3/G4 | PDS §13 maps | Two tables |
| Surface parity | GOV-005 Principle 4 | PDS §7 | The four-column table |

---

# 6. How PDSs Relate to PRD, FND, DES, and EXP Documents

**6.1 The traceability chain.** Every buildable screen must trace upward without gaps:

```
FND (platform contract) ─► PRD (product behavior) ─► PDS (product experience) ─► Figma Build-Ready (rendering)
                                    ▲                        ▲
              DES/EXP (global experience law) ───────────────┘
```

- **PRD → PDS:** one-to-one for every product with a UI. The PDS cites PRD requirements by number; a PRD requirement with no PDS coverage is a completeness finding at G-A; a PDS element with no PRD basis is either scope creep or a discovered PRD gap — and the PDS process is *expected* to discover PRD gaps (state tables are ruthless). Gaps route back as PRD change requests through GOV-004's process; the PDS never silently invents product behavior.
- **FND → PDS:** indirect but binding. FND-002's error codes populate §9's catalog; FND-003's states populate §6's tables; FND-010's estimate contract shapes §13. The PDS renders contracts; it cannot amend them.
- **DES/EXP → PDS:** the inheritance described in §1. Precedence on conflict: **FND > PRD > DES > PDS** — with the crucial procedural rule that a designer who believes a global standard is *wrong* escalates a standards change (Systems Review, GOV-005 §10.1); winning locally by deviating is the one unforgivable move in the system.
- **PDS → Figma:** every Build-Ready frame's pinned citation (GOV-005 §8) now points to PDS sections, giving the frame-level annotation a real referent. The PDS is what "matching the design" *means* at Gate G-D.

**6.2 Registry integration.** PDSs are registered documents: ID scheme **PDS-NNN matching their PRD number** (PDS-001 = PostgreSQL, PDS-005 = Observability…), same lifecycle states as all specs (Draft → Review → Approved → Living), listed in GOV-004's master register with owner and pinned-dependency versions.

---

# 7. Validation: How Reviews Consume the PDS

The PDS does not add gates; it gives GOV-005's existing gates their material. The mapping:

**Gate G-A (Journey Gate) reviews PDS §§1–5 in Draft.** Checklist: intent is one page and falsifiable; delta statement matches pioneer status; object map consistent with FND-001/PRD; journey participation complete (no journey the PRD implies is missing); workflow table has no unexplained "custom" rows. Approvers per GOV-005. Exit: §§1–5 frozen at v0.x.

**Gate G-B (Pattern Gate) reviews PDS §§6–8 + §§11–13.** Checklist: every screen passes the six-line declaration; wireflows cover the unhappy paths (reviewers spot-check by picking three FND-002 error codes and asking "show me where this lands"); state tables complete; parity table has zero unjustified single-surface rows; consumption list valid against the registry; pioneer work confined to the §3 license; AI capabilities all registered; cost/danger maps reconciled against the PRD's billable dimensions and destructive actions. Exit: the PDS is Approved; **hi-fi may begin** — this is the system's central control point.

**Gate G-C (Fidelity Gate) audits Figma against the PDS.** The G-C checklist (GOV-005) is now generated *from* the PDS: every §6 screen exists with every state; every §9 string appears verbatim or has a recorded content-review change; §10 assertions verified; §13 tiers implemented. G-C stops being a taste review with a checklist and becomes a conformance review with taste on top.

**Review mechanics:** PDS reviews are document reviews (async written comments + one live session), timeboxed to one week per gate, fail-fast with same-week re-review per GOV-005. Reviewer roles are inherited from the gate definitions; §7 (parity) adds the DX lead as mandatory at G-B, and §12 adds PRD-012's owner when present.

---

# 8. Evolution of the System and Its Documents

**8.1 PDS lifecycle.** Approved PDSs are living documents synchronized with reality: post-ship, the PDS is updated within the sprint alongside the Shipped Figma pages (same rule, same deadline — GOV-005 §10.3). A shipped product whose PDS says something its UI doesn't do has a defect in one of them; drift is never ambient.

**8.2 Revision triggers.** A PDS revises when: its PRD revises (version-scoped, per the Master Plan's revision model — e.g., PDS-001 grows a branching-GA revision alongside PRD-001's v1.5 rev); a pinned DES/Kit version bumps with breaking changes (the drift dashboard schedules reconciliation); research invalidates a §14 assumption; or an incident/support pattern reveals an experience failure (the design mirror of a postmortem action item).

**8.3 Effort scales with role — the thin-PDS doctrine.** A pioneer PDS (PostgreSQL, Compute) is the full fourteen sections, substantial. An instantiator PDS (Valkey) should be *strikingly* thin: §§1–3 in two pages, §4 mostly Kit citations, §5 a table, §6 covering only type tabs, §§8–13 short. **Target: an instantiator PDS under 20% the length of PDS-001.** This number is tracked in the Pattern Registry next to the design-effort curve (GOV-005 §6) — the two metrics measure the same health from the document side and the effort side. A fat Valkey PDS triggers the same alarm as a slow Valkey design: fix the system, not the instance.

**8.4 Template evolution.** DES-000 itself is living. Section additions/removals require Systems Review + VP Design approval, apply to *new* PDSs immediately and to living PDSs at their next revision, and are recorded in a template changelog. The bar for adding a mandatory section is high: every [M] costs every future product; the template must stay small enough to be beloved, or it will be routed around.

**8.5 The system's own success criteria** (reported with GOV-005's monthly metrics): PRD-gap discoveries per PDS (healthy: several early, trending down); instantiator-PDS thinness ratio; % of G-C findings that are conformance (PDS-covered) vs novel (PDS-missed) — the latter trending toward zero is the system doing its job; and gate cycle time (a PDS that slows the pipeline gets simplified, not tolerated).

---

# 9. Rollout

1. **Approve DES-000** (this document) alongside DES-001/002 in stage D1.
2. **Author PDS-000-EXEMPLAR:** a worked, fictional mini-product PDS (one page per section) shipped as the template's teaching appendix — the system is learned by example, not by rulebook.
3. **PDS-SHELL and PDS-001 (PostgreSQL) first,** authored in D1–D2 in lockstep with the pioneer designs; their authorship is the template's shakedown cruise, and template amendments from it are expected and welcome.
4. **PDS-005 (Observability), PDS-007 (IAM)** follow; **PDS-002 (Valkey)** is the thinness test (§8.3).
5. Thereafter: no product enters hi-fi without an Approved PDS — the rule takes force the day PDS-001 is approved, and it has no exceptions clause, because the first exception is the end of the system.

---

# 10. Closing

The architecture gave Steloit nine primitives so a thousand features could stay coherent. The design strategy gave it eleven global decisions and one Kit so a thousand screens could stay coherent. The PDS system is the final joint: **fourteen sections, eight artifacts, one notation, and a delta discipline** that make each product's experience fully decided — objects, flows, states, words, costs, dangers, and its AI's manners — before a single high-fidelity pixel exists.

When it works, the sensation inside the design org will mirror the sensation the platform promises its developers: *open the fifth PDS and you already know how to read it; start the fifth product and you already know how to design it.* The bridge between architecture and Figma is not a handoff. It is the same coherence, written one layer down.

*— End of specification system —*
