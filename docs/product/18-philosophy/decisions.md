# Decision log (ADRs)

Append-only register of load-bearing decisions: what was decided, what lost, why, and where it's visible — so nothing gets relitigated by someone (or some agent) who wasn't there. Entries are extracted from the design spec's on-the-record reversals, GOV-002, and the `99-history/` program docs; new entries are added going forward, and **constitution amendments land here too** (18-philosophy §How-this-document-is-used).

Format: **Decision · Context & alternatives · Why · Consequences · Refs**. Status is `accepted` unless marked. Dates before Jul 2026 are era-level.

---

## Era 1 — Architecture (GOV-002)

**ADR-001 · Nine primitives, and Workspace is not one of them.** Considered a Workspace level between Org and Project (as some platforms have); rejected — 95% of customers never need it and it muddies "create a project." Folders + labels + Teams + Policies compose the same outcome; the hierarchy deliberately leaves the slot open if enterprise scale ever demands it. *Consequence:* pressure to add a primitive is a signal to think harder, never a casual extension. → GOV-002 §1.2

**ADR-002 · The Project is the atom.** Infrastructure organized around applications, not service types — the platform's core bet. Everything (provisioning, cost, permissions, deletion) rolls up to a Project. *Consequence:* the first-deploy story is "deploy your app," not "buy a database." → GOV-002 §0, §1.1

**ADR-003 · Never build: IaaS, CI/CD, comms infra, low-code.** The boundary heuristic: build where fragmentation is the pain, integrate where ecosystems have gravity, never sell undifferentiated infrastructure. Kubernetes is an internal detail (Cells), never a product. → GOV-002 §3.7–3.8

**ADR-004 · Region is an environment property; provider is a facet of region.** Alternatives: per-service region dropdown (normalizes accidental cross-region topologies), provider as a second axis (forks the UX per cloud). Environment sets the home region; instances inherit; overrides are explicit, priced exceptions (C7); multi-region production = more environments. *Consequence:* BYOC later slotted in as "ownership is a facet too" with zero new structure. → GOV-002 §2.2 · spec §Region model · X2–X3

**ADR-005 · The four AI laws are permanent.** Suggest-never-act · explainable with evidence · read broadly act within permissions · platform whole without AI. No auto-apply path exists in the API by design, not by policy. *Consequence:* every AI feature everywhere is a proposal object; the `ai-assistant` org policy can remove the layer cleanly. → GOV-002 §7 · 12-ai/

**ADR-006 · Expand/deepen release rhythm; data before compute.** Integer versions change what Steloit *is*, halves make it excellent; never two expansions in a row. Compute ships fourth (v2), not first — data services earn trust while the harder problem is built. → GOV-002 §8

## Era 2 — Design program → LATTICE (the `99-history/` transition)

**ADR-007 · "Instrument" superseded by LATTICE.** The first visual language (periwinkle accent, IBM Plex Mono, gold=production closed hue law) was replaced wholesale: steel `#4D7CFE` accent, JetBrains Mono, no gold, violet reserved for AI. *Consequence:* nothing visual may be taken from 99-history. → 99-history/docs/DES-004-006 · 01-design-system/

**ADR-008 · Gallery-as-spec replaced the PDS pipeline.** The documentation program (GOV-003's 61 docs, DES-000's 14-section PDSs, gate reviews) gave way to the current model: 152 validated frames + one design spec + thin derived docs, cross-referenced by frame id. The PDS *principles* survive (delta documents, thinness as a health metric); the machinery does not. → 99-history/docs/GOV-003, DES-000, DES-009

**ADR-009 · EXP-002's seven UX laws carried forward, evolved.** URL-as-state, context worn not visited, ten-second truth, verbs over destinations, console teaches CLI, calm is a feature, never a dead end — all alive in the current spec under new names. Kept as an ADR so their origin is findable. → 99-history/docs/EXP-002

**ADR-010 · One pioneer at a time.** GOV-005's rule, retained in the playbook: exactly one product may establish new patterns at any moment; all others instantiate. Two pioneers is how design systems fork. → 99-history/docs/GOV-005 · 21-playbooks/new-product.md

## Era 3 — The console spec (LATTICE, 152 frames)

**ADR-011 · The shell is rail + product sidebar (Dub pattern), not GOV-002 §4.2's two-section sidebar.** Three variants explored. The rail renders only the project's actual products (the app's silhouette), the second sidebar is the selected product's workspace, service tab rows retired. *Consequence:* GOV-002's IA sketch is superseded where they differ — the design spec is the later, binding refinement. → spec §Chosen shell

**ADR-012 · No Organization rail item; org admin lives in settings mode behind the gear.** Org concerns are low-frequency admin; account settings deliberately hang off the avatar instead (a person spans orgs; MFA/sessions/tokens must not live inside any one of them). → spec §Chosen shell

**ADR-013 · Environment is a filter, not a container or dimension.** Three models weighed (filter / AWS-style container / dimension). Filter wins with the refinement *shape is per-project; presence and scale are per-environment*; the rail never reshuffles on env switch; the Environments page is a parity matrix. → spec §M3

**ADR-014 · Create is one surface, not a dialog + per-product pages.** The dialog was defended as "a router, not a workspace" — held until `+` became a lit rail item; a rail destination owns a surface. C2/C3/C9–C12 are *states* of C1. Three doors, one room. → spec §C1 (revision on record)

**ADR-015 · Tier-2 creation chrome keeps the rail.** Began as fully-stripped "creation-as-checkout" chrome; once the canvas shape existed, stripping the rail lost its justification. The checkout signal survives in what's absent: no sidebar, no bell. → spec §Consistency contract

**ADR-016 · Dashboards live under Home — not under Observe, not a rail item.** Observe nesting broke on cross-plane widgets (a parent that doesn't own its child's data); a rail slot taxes the jump-to-the-thing persona and creates a "Home or Dashboards?" decision tax. Single door: reached from Home only. Scope (org|project) is orthogonal to visibility. → spec §Dashboards (reversal on record)

**ADR-017 · Observe and Deploy are the only rail domains besides Home.** Rail admission = weight × frequency; the two are the console's verbs (*watch it*, *ship it*). Environments evaluated and kept off — its everyday presence is the crumb; the manager is reached through the filter (env menu → ⚙). Insights can never be a rail item (Law 4: it must vanish cleanly). → spec §Observe suite, §AI

**ADR-018 · The project sidebar is dissolved; Tier-1 has two shapes.** After Settings went behind the gear and Deploy to the rail, Home held one destination — a one-item sidebar is pure chrome. Domain shape (rail item + sidebar) vs canvas shape (full-width page). → spec §Project surface

**ADR-019 · Interaction tiers are a decided rule; inline forms are prohibited.** Modal (≤4 fields / one decision, typed-confirm escalation) · drawer (previews, live checks) · page (multi-step/provisioning). A full census of all 152 frames found and fixed six inline-form violations. → 06-interactions/ · spec §Interaction tiers

**ADR-020 · Assistant in the chrome, not a FAB; ⌘K imperative, assistant interrogative.** A floating bubble is the archetypal bolted-on chatbot and context-blind. Top-nav violet button (⌘J) → context-scoped drawer ⇄ full workspace, one conversation at two sizes. → spec §AI access

**ADR-021 · Templates copy, never link; secrets never captured.** Editing/deleting a template never touches instantiations; credentials re-mint per consumer; excluded-service bindings become required inputs. Estimate-before-provision extends to a template's birth. → spec §Templates · F8

**ADR-022 · Hybrid pricing; BYOC at Business+ (deliberate deviation, flagged).** Subscription = platform capabilities, pay-as-you-go = infrastructure, overage = beyond quotas. The brief placed BYOC at Enterprise; the console places cells at Business+ (a metered capability, not a contract feature) — Enterprise keeps dedicated cells, audit export, SLA. Gateway *tokens* are infra; assistant *requests* are a platform quota. → spec §Subscription experience · F9

## Era 4 — The handoff package (Jul 2026)

**ADR-023 · Product philosophy adopted as constitution.** North Star (eliminate uncertainty) → show your work → know before you deploy → don't break the grammar → question ownership → grammar-as-code. Amendments land in this log. → 18-philosophy/product-philosophy.md

**ADR-024 · Service status vocabulary is `ready`/`deleting`, not `running`/`deleted`.** models.md briefly diverged from the frames; frames + billing contract ("metering starts at ready") win. Decided while authoring the full OpenAPI spec (2026-07-08). → 08-api/ · 09-data-models/

**ADR-025 · Money crosses the API as integer cents (`*_cents`).** From architecture.md's server-side rule, extended to the wire format so "one arithmetic" is testable end-to-end; clients render via `fmtMoney`. (2026-07-08) → 08-api/ x-conventions

**ADR-026 · One canon world; no demo data outside it.** Fixtures shaped as API responses conforming to the OpenAPI schemas; arithmetic invariants verified by script; frame-fixed facts vs `$representative` fill-ins explicitly marked. (2026-07-08) → 19-canon/

**ADR-027 · Flat numbered directories, kept; history quarantined at 99.** Semantic reorganization considered and rejected — the numbers encode reading order, carry every cross-reference, and are fixed in git history. Superseded material lives in `99-history/` and is never citable as authority. (2026-07-08) → README

**ADR-028 · Console light steel `#3D63DD` ≠ brand daylight steel `#3B63E8`.** Status: **accepted-pending-confirm.** The gallery's `.light` block (product ground truth) and brand.md's daylight ramp differ by one step; treated as deliberate plane separation (console vs marketing) under the spec-wins rule. If unintended, it's a one-token amendment. (2026-07-08) → 15-assets/tokens.css · 17-brand/brand.md
