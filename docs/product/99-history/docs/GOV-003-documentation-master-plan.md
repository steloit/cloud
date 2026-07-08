# Steloit Documentation Master Plan

**Document class:** Internal — Pre-Implementation Documentation Blueprint
**Status:** v1.0 — For approval
**Inputs (approved, immutable):**
- *Steloit — The Developer Cloud Platform* (Vision, doc **GOV-001**)
- *Steloit Product Architecture Specification* (Architecture, doc **GOV-002**)

**Audience:** Product, Engineering, Design, Security, and Operations leadership
**Purpose:** Define every document that must exist — and the order it must exist in — before and during implementation. This document does not write the specifications; it is the map of them.

---

## 0. Method and Principles

Before listing documents, the rules that generated the list. A world-class engineering organization does not measure documentation by volume; it measures it by *decision coverage*. Every document below exists because it either (a) records a decision engineers would otherwise make implicitly and divergently, (b) defines a contract two or more teams must build against, or (c) satisfies an external obligation (customers, auditors, regulators).

**Principle 1 — Contracts before products.** The Architecture (GOV-002) established that Steloit's coherence comes from shared mechanisms: nine primitives, one service grammar, one Binding fabric, one Event pipeline, one API shape. Therefore the documents that define those *shared contracts* must be written before any product specification, because every product spec will cite them. Writing the PostgreSQL spec before the Service Grammar spec would force the Postgres team to invent the grammar — exactly the divergence the architecture forbids.

**Principle 2 — Specify to the version horizon, not the vision horizon.** Documents are scoped to what their consuming version needs. The Networking spec written for v0 defines the environment-private-network model completely and *names* — but does not design — BYOC private networking. Speculative depth in early specs is a liability: it will be wrong, and it will be cited anyway.

**Principle 3 — One owner, one status, one ID.** Every document has exactly one accountable owner (a person, not a team), a lifecycle status (`Draft → Review → Approved → Living / Superseded`), and a stable ID used in cross-references. IDs follow the scheme `<CATEGORY>-<NNN>` defined in the map below.

**Principle 4 — Two document species, kept separate.**
- **Specifications** define *what must be true* (contracts, behaviors, invariants). They are approved, versioned, and changed only by revision.
- **Design documents / RFCs** define *how a team intends to satisfy a spec* (engine choices, schemas, algorithms). They are proposed by the implementing team, reviewed against the spec, and may be superseded freely.

This plan inventories the **specifications** exhaustively and identifies only those **design documents** significant enough to schedule (e.g., the Postgres HA design). Routine design docs are the teams' business.

**Principle 5 — The template is part of the system.** All specifications share one template — *Summary · Motivation (with citations to GOV-001/002) · Scope & Non-Goals · Definitions · Requirements (normatively numbered, RFC-2119 keywords) · Interfaces (API/CLI/Console surface) · Failure & Limits · Security Considerations · Billing & Metering Considerations · Open Questions · Version Applicability.* The mandatory Security and Billing sections in *every* spec are how "structural, not bolted on" (GOV-002 §2.4) and "cost transparency" (GOV-001) become organizational habits rather than aspirations.

---

# Deliverable 1 — The Documentation Map

The full hierarchy. Seven categories, ordered by dependency: each category may cite categories above it, never below. Counts: **58 scheduled documents** (44 required before or during v0–v1; 14 deliberately deferred).

```
GOV   Governance & North Star            (2 existing + 2 new)
FND   Platform Foundation Specifications (12)   ← the contracts everything cites
PRD   Product Specifications             (14)   ← one per product, per §Deliverable 2
EXP   Experience Specifications          (8)    ← Console, CLI, SDK, docs-as-product
INF   Infrastructure & Operations        (9)    ← Cells, SRE, deployment of Steloit itself
SEC   Security, Privacy & Compliance     (7)
BIZ   Business Systems                   (4)    ← pricing, billing ops, plans, support
PRC   Process & Working Agreements       (2)
```

## GOV — Governance & North Star

| ID | Document | Status |
|---|---|---|
| GOV-001 | Vision: The Developer Cloud Platform | ✅ Approved |
| GOV-002 | Product Architecture Specification | ✅ Approved |
| GOV-003 | **Terminology & Naming Standard** | To write |
| GOV-004 | **Documentation Governance & Template** | To write |

**GOV-003** exists because the platform's coherence is partly linguistic. It fixes, permanently: the canonical names of the nine primitives and every product; capitalization and CLI noun forms (`steloit db` vs `steloit postgres` — this must be decided once); reserved words; the resource **ID and slug scheme** (format, uniqueness scope, immutability, rename semantics); and naming rules for user-created resources. Every API path, CLI command, Console label, and error message will embed these choices; changing them post-v0 is a breaking change to everything. This is the cheapest document in the plan and among the highest-leverage.

**GOV-004** is this plan operationalized: the template of Principle 5, the ID registry, the review/approval workflow (spec owners, required reviewers per category, decision log), and the RFC process for design documents.

## FND — Platform Foundation Specifications

These twelve documents *are* the platform, in the same sense that GOV-002 argued the API is the platform. Every product spec is a client of these.

| ID | Document | Defines |
|---|---|---|
| FND-001 | **Resource Model & Hierarchy Spec** | The nine primitives as normative data model: attributes, states, state machines, parent/child rules, ID/slug behavior (per GOV-003), labels/folders, soft-delete and grace-period semantics (GOV-002 §5 "irreversible things are slow"). |
| FND-002 | **API Conventions Spec** | URL structure mirroring the hierarchy, versioning policy, auth headers, pagination, filtering, error envelope and error-code taxonomy, idempotency keys, rate limits, async-operation pattern (long-running provisioning), webhook delivery contract, OpenAPI as source of truth. |
| FND-003 | **Service Grammar Spec** | *The platform's constitution*, named but not yet written in GOV-002 §6.2: what every Service must implement — lifecycle verbs, standard states (`provisioning, ready, degraded, suspended, deleting`), config/scaling interface, standard metrics & log conventions, backup interface (stateful), health model, page anatomy contract, CLI shape, Binding participation. Includes the **conformance checklist** new service types must pass. |
| FND-004 | **Binding & Configuration Injection Spec** | Binding object model; credential minting and scoping per service type; rotation protocol and zero-downtime rotation contract; injection mechanism (env var naming standard, file mounts, SDK accessors); cross-environment/cross-project binding permissions; failure semantics when a binding target is unavailable. |
| FND-005 | **Identity & Access Model Spec** | Human identities, machine identities (API keys, service tokens: scoping, expiry, rotation), *service identity* for the Binding fabric; the fixed v1 role set and permission matrix (every permission enumerated); RBAC evaluation order and Policy interaction; session model. |
| FND-006 | **Secrets Spec** | Secret object model, versioning, scoping levels, encryption envelope (interface — implementation in a SEC design doc), access logging, injection via FND-004, org→project grant model. |
| FND-007 | **Policy Framework Spec** | Policy object model, attachment points, inheritance and floor/ceiling evaluation (GOV-002 §1.1), the v1 policy catalog (access, spend, retention, region, network exposure, AI-enablement), conflict resolution, audit of policy changes. |
| FND-008 | **Event Pipeline & Schema Spec** | The one pipeline (GOV-002 §6.3): canonical event envelope, taxonomy (audit / metric / log / alert / lifecycle / notification), schema evolution rules, retention classes, query semantics, ordering and delivery guarantees, tenancy isolation of event data. |
| FND-009 | **Environment & Networking Spec** | Environment as isolation unit: implicit private network mechanics, default-deny posture, Binding-opened paths, public exposure model for compute and the policy-gated public toggle for data services, region-as-environment-property, `*.steloit.app` addressing, custom domain & TLS contract (interface now; full compute ingress detail at v2). |
| FND-010 | **Metering & Billing Model Spec** | The metering event contract every service must emit; billable dimensions per service category; aggregation to service→environment→project→org rollups; the **provisioning-time cost-estimate contract** (GOV-001's flow — estimation must be a platform capability, not per-product improvisation); budgets/spend alerts as Policies; proration, suspension, and free-tier mechanics. |
| FND-011 | **Quotas & Limits Spec** | The limit taxonomy (hard platform limits, plan limits, adjustable quotas), default values per plan per resource, limit-exceeded behavior and error contract, quota-increase workflow. Absent this, every team invents limits ad hoc and support inherits the chaos. |
| FND-012 | **Deployment Model Spec** *(v2 horizon; interface stub at v0)* | Deployment primitive: artifact model, immutability, release phases (build→migrate→rollout), rollback semantics, promotion-with-diff contract, preview-environment lifecycle including automatic retirement. Stubbed early because FND-001's state machines must reserve its states; written fully in the v2 wave. |

## PRD — Product Specifications

Enumerated and justified in Deliverable 2. Fourteen documents: PRD-001 … PRD-014.

## EXP — Experience Specifications

| ID | Document | Defines |
|---|---|---|
| EXP-001 | **Console Information Architecture & Interaction Spec** | GOV-002 §4 made normative: context bar behavior, environment-switch-as-filter, navigation tree, page anatomy bindings to FND-003, topology map requirements, ⌘K search scope and ranking, notification inbox rules, settings placement test. |
| EXP-002 | **Design System Spec** | Tokens, components, the service-page anatomy as reusable composition, status/health visual language (one meaning of "green" platform-wide), data-viz standards for metrics, accessibility bar (WCAG 2.1 AA), content style guide citing GOV-003. |
| EXP-003 | **CLI Spec** | The full grammar: `steloit <noun> <verb>` mapped 1:1 to FND-001/002; context resolution (`--project/--env`, config files, env vars — exact precedence); output contract (human vs `--json`); exit codes; auth flows; `init` and `dev` command specs; plugin/extension policy (v1: none — say so). |
| EXP-004 | **SDK Architecture Spec** | Generation-from-OpenAPI pipeline, per-language ergonomic layer rules, the runtime helper contract (typed binding/secret accessors per FND-004), versioning and deprecation policy, support tiers per language per version. |
| EXP-005 | **IaC Provider Spec** | Terraform (then Pulumi) resource mapping to FND-001, import semantics, drift behavior, secrets handling in state, provider release cadence vs API versioning. |
| EXP-006 | **Onboarding & First-Run Spec** | The five-minute test (GOV-002 §10) as a specified flow: signup → org → guided project creation (including the AI-recommendation variant) → first connection; empty states; sample data policy; time-to-value instrumentation. |
| EXP-007 | **Public Documentation & Content Architecture Spec** | Docs-as-product: information architecture of docs.steloit.com, doc types (concept/task/reference/template), reference generation from OpenAPI and CLI, versioning of docs against platform versions, changelog policy. |
| EXP-008 | **Templates Spec** | Template package format (services + bindings + config + example code), validation, provisioning flow integration, first-party catalog for v1, the curation bar for later community submissions. |

## INF — Infrastructure & Operations Specifications

Steloit itself must be built, deployed, and operated; these specs are for the teams building the platform, invisible to customers but prerequisite to trust.

| ID | Document | Defines |
|---|---|---|
| INF-001 | **Cell Architecture Spec** | The Cell (GOV-002 §2.2) made concrete: control-plane/data-plane separation and interface (the seam BYOC and Self-Hosted depend on — must be right at v0), cell provisioning and fleet management, capacity model, cell-level blast-radius containment. |
| INF-002 | **Tenancy & Isolation Spec** | The single most safety-critical unwritten decision (see Deliverable 5): isolation technology per service class (dedicated instances vs containerized multi-tenant vs shared-with-row-isolation), noise-neighbor controls, tenant data separation guarantees, isolation testing requirements. |
| INF-003 | **Control Plane Spec** | Provisioning orchestration (desired-state reconciliation), the async-operation backbone behind FND-002, state store, control-plane availability targets and degradation behavior (data planes must survive control-plane outage). |
| INF-004 | **Data Plane Operations Spec — Stateful Services** | Common operational contract for Postgres/Valkey/Queue/Storage runtimes: provisioning automation, patching/upgrade windows and customer communication, failure detection and automated recovery, restore-drill requirements ("restore drills as CI," GOV-002 v0 risks). |
| INF-005 | **SRE & Reliability Spec** | SLO/SLA definitions per service per plan (the numbers behind v1 GA), error budgets and the budget-gates-features rule, incident severity taxonomy, on-call model, status page policy, postmortem standard. |
| INF-006 | **Release Engineering Spec** | How Steloit ships Steloit: environments-for-the-platform, progressive rollout across cells, feature flags, API-change review gate (enforcing FND-002 compatibility), emergency change process. |
| INF-007 | **Internal Observability Spec** | Steloit observing itself (distinct from PRD Observability, the customer product): golden signals per component, capacity dashboards, cost-of-goods telemetry feeding BIZ-001 pricing. |
| INF-008 | **Business Continuity & DR Spec** | Platform-level DR: control-plane recovery, cell-loss scenarios, backup of Steloit's own state, RTO/RPO commitments per component, customer-data recovery interplay with PRD backup products. |
| INF-009 | **BYOC Architecture Spec** *(v3 horizon)* | Remote cells in customer accounts: bootstrap, trust boundary, upgrade path, support access model, prerequisite matrix. Deferred; INF-001's seam is its down-payment. |

## SEC — Security, Privacy & Compliance

| ID | Document | Defines |
|---|---|---|
| SEC-001 | **Security Architecture Spec** | Threat model for the whole platform (STRIDE across control plane, data plane, Binding fabric, supply chain); trust boundaries diagram; security invariants every spec must uphold; secure-defaults doctrine (GOV-002 §2.4) as testable requirements. |
| SEC-002 | **Encryption & Key Management Spec** | At-rest and in-transit standards, per-org key hierarchy behind FND-006, rotation, future BYOK/HSM interface named. |
| SEC-003 | **Audit & Forensics Spec** | Audit event content standard (actor, including `via assistant` attribution per GOV-002 §7), immutability mechanism, retention, export, investigation tooling. |
| SEC-004 | **Data Lifecycle, Privacy & Residency Spec** | Data classification; deletion semantics end-to-end (customer deletes → backups age out → forensic copies?); region residency guarantees; DPA-relevant processing inventory; the AI data-use rules of GOV-002 §7.3 as binding policy. |
| SEC-005 | **Application & Supply-Chain Security Spec** | SDLC requirements, dependency and artifact signing policy, vulnerability management SLAs, pen-test cadence, bug bounty (v1+). |
| SEC-006 | **Compliance Roadmap & Controls Matrix** | SOC 2 control mapping started at v1 per GOV-002 v3 dependencies; evidence-collection automation; future frameworks named (ISO 27001, HIPAA?) with go/no-go criteria. |
| SEC-007 | **Abuse & Acceptable Use Spec** | Free-tier abuse (crypto-mining, spam infra), detection and enforcement ladder, suspension mechanics (ties to FND-010 suspension states), appeals. Chronically forgotten until the free tier launches and burns money; scheduled here at v0.5. |
| SEC-008 | **Customer-Facing Auth Surface Spec** | Signup, login, MFA, session and account-recovery flows for Steloit accounts themselves; SSO/SCIM deferred to v3 but recovery and MFA cannot wait. |

*(SEC-008 brings SEC to eight items; seven were promised in the summary table — the map is the source of truth, and this is exactly the kind of drift GOV-004's registry exists to catch. Count corrected: **59 documents**.)*

## BIZ — Business Systems

| ID | Document | Defines |
|---|---|---|
| BIZ-001 | **Pricing & Packaging Spec** | The actual prices and plan matrix (Free/Pro/Business/Enterprise) over FND-010's mechanics; unit economics guardrails from INF-007 cost telemetry; the transparency tests (estimate accuracy tolerance, "no spreadsheet needed" bill design). |
| BIZ-002 | **Billing Operations Spec** | Invoicing, payment processing integration, dunning, refunds/credits, tax handling, currency scope at launch. |
| BIZ-003 | **Plan Enforcement Spec** | How plan limits (FND-011) are enforced in product: upgrade prompts without dark patterns, grace behaviors, downgrade semantics (what happens to resources exceeding the lower plan). |
| BIZ-004 | **Support Model Spec** | Tiers per plan, channels, SLAs, escalation to engineering, support tooling access boundaries (what support can see — cites SEC-004), community support scope for Free. |

## PRC — Process & Working Agreements

| ID | Document | Defines |
|---|---|---|
| PRC-001 | **Engineering Working Agreements** | Repo/monorepo strategy, code review bar, testing pyramid expectations, definition of done including spec-conformance (FND-003 checklist), internal RFC lifecycle. |
| PRC-002 | **Launch Readiness Standard** | The gate checklist every product passes before each version milestone: conformance, security review, SLO instrumentation, docs complete, pricing wired, support runbooks, rollback plan. |

---
# Deliverable 2 — Product Specifications (PRD Series)

## 2.1 How product boundaries were drawn

A product gets a standalone specification when it has **its own team-sized surface, its own failure modes, and its own conformance obligations** under FND-003. Two anti-patterns were rejected:

- **Over-splitting:** separate specs for Monitoring, Logging, and Alerts would recreate on paper the fragmentation GOV-002 §3.3 rejected in product. One Observability spec.
- **Under-splitting:** a single "Data Services" spec would bury the deep, divergent engineering of Postgres branching vs. object storage lifecycle policies. Each data service stands alone, *thin*, atop the fat foundation specs.

The consequence of Principle 1 is visible in every scope below: product specs are **deltas over the foundation**. The Postgres spec does not define what "create" means, what a metric looks like, how credentials inject, or how backups are policy-governed — FND-003/004/007/008/010 do. It defines only what is *distinctively Postgres*. This keeps every product spec short, consistent, and honest.

## 2.2 The fourteen product specifications

Priority scale: **P0** = blocks v0 · **P1** = blocks v0.5/v1 · **P2** = blocks v2/v2.5 · **P3** = v3+.

---

**PRD-001 — Managed PostgreSQL** · **P0** · *Order: 1st*
- **Why standalone:** the anchor product and the hardest engineering on the roadmap (GOV-002 v0 risks). Data-loss risk alone justifies the most scrutinized spec in the company.
- **Scope:** instance classes and sizing; Postgres version policy and extension allowlist (incl. managed pgvector per GOV-002 v1.5); HA topology and failover behavior *as customer-visible contract* (RTO/RPO per plan); PITR windows; read replicas; **branching and cloning semantics** (the signature feature: branch creation time targets, data freshness, cost model, credential scoping per branch, retirement); connection pooling; query insights surface; migration-hook contract reserved for FND-012.
- **Dependencies:** FND-001–011, INF-002 (isolation choice shapes instance classes), INF-004.
- **Companion design docs to schedule:** *Postgres HA & Replication Design*, *Branching Storage Design* (copy-on-write approach) — the two riskiest technical designs in the v0–v1 program.

**PRD-002 — Managed Valkey** · **P1** · *Order: 4th*
- **Why standalone:** distinct engine, distinct failure modes (memory pressure, eviction), distinct modes (cache vs durable vs streams) that must be specified as explicit configurations per GOV-002 §3.1.
- **Scope:** modes and their durability contracts; eviction policies and defaults; sizing/clustering (clustering deferred to v1.5 — say so); pub/sub and rate-limiting usage patterns as documented contracts; what metrics distinctively matter (hit ratio, evictions).
- **Dependencies:** foundation set + INF-002/004.

**PRD-003 — Object Storage** · **P1** · *Order: 5th*
- **Why standalone:** S3-compatibility is a *compatibility contract* requiring its own conformance matrix (which S3 API subset, exactly — ambiguity here generates endless support pain).
- **Scope:** S3 API compatibility matrix; bucket model within the hierarchy (buckets are Services? or sub-resources of one Storage service? — resolve against FND-001; recommendation: one Storage service per environment, buckets as sub-resources, to preserve the topology map's legibility); signed URLs; lifecycle policies (as FND-007 Policies); versioning; public-bucket + CDN exposure per FND-009; consistency model.
- **Dependencies:** foundation set; FND-009 public-exposure model is a hard prerequisite.

**PRD-004 — Queue** · **P1** · *Order: 6th*
- **Why standalone:** delivery semantics are contract-critical: "at-least-once, not Kafka" (GOV-002 §3.1) must be pinned to exact guarantees.
- **Scope:** delivery semantics, ordering (or explicit absence), visibility timeouts, retries and DLQ behavior, delays and schedules, message size/retention limits (FND-011), consumer binding contract with future Workers (interface named, fulfilled at v2).
- **Dependencies:** foundation set; FND-004 consumer-injection pattern.

**PRD-005 — Observability** · **P1** · *Order: 3rd (interleaved with data products)*
- **Why standalone:** it is the customer-facing product over FND-008 — dashboards, correlation, alerting UX — and the load-bearing wall of the "no additional setup" promise.
- **Scope:** per-service default dashboards (the catalog, per service type); the environment-correlated timeline and pivot interactions (GOV-002 §4.3); log query grammar (one grammar everywhere — specified here, implemented by FND-008); default alert catalog per service type and customization model; notification routing to channels; retention per plan.
- **Dependencies:** FND-008 is the substrate; EXP-001 for interaction patterns; every PRD feeds it (their specs must declare metrics/logs per FND-003 — Observability consumes those declarations).

**PRD-006 — Backup & Restore** · **P1** · *Order: 7th*
- **Why standalone:** rehearsable restore is a *workflow product* (GOV-002 §5.8) spanning all stateful services; leaving it inside each data spec would fragment the one workflow that must feel identical everywhere.
- **Scope:** backup types per service class; retention as Policy; restore-to-new-branch/instance flows; cross-environment restore permissions; backup verification and the customer-visible "last verified restore" signal; interplay with INF-008 platform DR (boundary stated explicitly).
- **Dependencies:** FND-007, PRD-001/002/004 backup interfaces per FND-003.

**PRD-007 — IAM (customer-facing)** · **P0 (core) / P3 (enterprise)** · *Order: 2nd*
- **Why standalone:** FND-005 defines the *model*; PRD-007 defines the *product* — invitation flows, member management, API-key UX, team management. Model and product change on different cadences.
- **Scope:** v1 role set surfaces, invitations, key lifecycle UX, team CRUD; SSO/SCIM/custom roles explicitly out-of-scope pointers to a v3 revision.
- **Dependencies:** FND-005, SEC-008.

**PRD-008 — Secrets Manager** · **P1** · *Order: 8th* — thin product spec over FND-006: Console/CLI surfaces, version rollback UX, access-log views. Standalone because it has its own UX obligations and because merging it into IAM would blur two different mental models (who-can-act vs what-code-receives).

**PRD-009 — Cost & Usage Analytics** · **P1** · *Order: 9th*
- **Why standalone:** transparency is a named pillar; the bill is a product surface (GOV-002 §6.5).
- **Scope:** rollup views, estimate-vs-actual displays, budget/spend-alert UX over FND-007/010, bill explainability ("what changed"), export.
- **Dependencies:** FND-010/011, BIZ-001.

**PRD-010 — Compute (Web Services, Workers, Cron)** · **P2** · *Order: 10th (spec authored in v1.5 window for v2 build)*
- **Why standalone (and why one spec, not three):** GOV-002 §3.2 defines one runtime with three exposure shapes; one spec preserves that unity. It is the largest single spec in the plan.
- **Scope:** build inputs (Dockerfile/buildpacks — exotic runtimes explicitly out); Deployment lifecycle per FND-012; health checks and rollout gates; autoscaling ranges (sophistication deferred per GOV-002); HTTP ingress, custom domains, TLS (completing FND-009); Worker↔Queue binding fulfillment; cron semantics (timezones, overlap policy, missed-run behavior); `steloit dev` runtime parity contract with EXP-003.
- **Dependencies:** FND-012 full version, FND-009 ingress detail, PRD-004, INF-001 capacity model.

**PRD-011 — Git Integration & Preview Environments** · **P2** · *Order: 11th*
- **Why standalone:** the flagship v2 workflow (previews with branched databases) crosses Compute, Postgres, and Environments; it needs one owner and one spec or it will be three teams' edge case.
- **Scope:** repo connection model, push-to-deploy triggers, PR preview lifecycle (creation, database branching orchestration, retirement on close), promotion flows, GitHub first / GitLab named.
- **Dependencies:** PRD-001 branching, PRD-010, FND-012.

**PRD-012 — AI Assistant & Platform Intelligence** · **P1 (recommendation flow) / ongoing** · *Order: interleaved; first version alongside PRD-005*
- **Why standalone:** GOV-002 §7's four laws need one enforcing document, or every product team will implement "AI features" with divergent safety properties.
- **Scope:** the proposal-artifact contract (recommendation/plan/diff object model — reviewable, auditable, applied through normal permission checks); provisioning-recommendation flow (v0.5); per-product capability registry and its review gate; the never-interfere list as testable prohibitions; data-use rules binding to SEC-004; model-provider abstraction (provider choice is a design doc, not spec); AI kill-switch Policy per FND-007.
- **Dependencies:** FND-005/007, SEC-003 (`via assistant` attribution), SEC-004.

**PRD-013 — Vector & Search Services** · **P2 (v2.5)** · *Order: 13th* — deferred by design; pgvector needs only a PRD-001 section. This spec is written when graduation criteria (defined *now*, one paragraph, in PRD-001) are met.

**PRD-014 — Private Networking & Enterprise Connectivity** · **P3 (v3)** · *Order: 14th, with INF-009* — peering, private endpoints, allowlists as Policies. Named now so FND-009 reserves the extension points; written in the v3 wave.

## 2.3 Products that do NOT get specifications — and why

- **"Networking" as a standalone v1 product spec:** networking is a *foundation* (FND-009), not a product, until enterprise connectivity makes it one (PRD-014). Writing a product spec now would manufacture surface area.
- **Monitoring / Logging / Alerts separately:** rejected above; one Observability product, one spec.
- **Functions, Edge, Event Bus, Workflow, customer-app Identity, Marketplace, Analytics DB:** v4 portfolio options per GOV-002 §8; each will get a v0-style wedge spec *when chosen*. Speculative specs for optional futures are how documentation rots.
- **CI/CD, VMs, email, DNS, low-code:** never-build list (GOV-002 §3.7). No documents, by design — the absence is the decision, recorded here.

---

# Deliverable 3 — Execution Order

## 3.1 The dependency spine

```
GOV-003 Terminology ──┐
GOV-004 Governance ───┤
                      ▼
        FND-001 Resource Model ──► FND-002 API Conventions ──► FND-003 Service Grammar
                      │                                              │
                      ▼                                              ▼
        FND-005 IAM ─► FND-006 Secrets ─► FND-004 Bindings    (every PRD cites 001–011)
                      │
        FND-007 Policy ─► FND-008 Events ─► FND-010 Metering ─► FND-011 Quotas
                      │
        FND-009 Environments & Networking
                      ▼
     INF-001 Cells ─► INF-002 Tenancy ─► INF-003 Control Plane ─► INF-004 Stateful Ops
                      ▼
                 PRD wave 1 (PostgreSQL, IAM product, Observability)
                      ▼
                 PRD wave 2 (Valkey, Storage, Queue, Backup, Secrets, Cost)
                      ▼
                 EXP full set · SEC full set · BIZ set (parallel tracks, see below)
                      ▼
                 PRD wave 3 (Compute, Git/Previews — v2 window)
```

## 3.2 Waves and parallelism

**Wave 0 — the pens-down week (sequential, ~1–2 weeks).** GOV-003 and GOV-004 first, alone. Every later document embeds their choices; this is the only genuinely serial step and it is short. FND-001 and FND-002 follow immediately, by a single small author group (2–3 people), because their internal consistency matters more than speed.

**Wave 1 — foundation fan-out (parallel, ~4–6 weeks).** With FND-001/002 in review, the remaining foundation specs parallelize across four author tracks that match natural team boundaries:
- *Track A (Platform core):* FND-003 → FND-004
- *Track B (Identity & governance):* FND-005 → FND-006, FND-007
- *Track C (Data & money):* FND-008 → FND-010 → FND-011
- *Track D (Infra):* INF-001 → INF-002 → INF-003, with SEC-001 authored alongside as its reviewer-twin.
FND-009 sits with Track D. FND-012 gets its interface stub from Track A. **SEC-001 belongs in this wave, not later:** the threat model must review the foundations while they are still wet.

**Wave 2 — first products (parallel with late Wave 1, ~4–8 weeks).** PRD-001 (PostgreSQL) starts the moment FND-003 hits review — its authors are FND-003's toughest reviewers, which is intentional: the first product spec is the conformance test of the grammar. PRD-007 (IAM product), PRD-005 (Observability), and PRD-012 (AI, recommendation-flow scope) run alongside. EXP-001/002/003 start here too: Console IA, design system, and CLI must co-evolve with the first product, not follow it. INF-004/005 and SEC-008, SEC-004 complete the v0 safety net.

**Wave 3 — the data layer and business layer (parallel, ~6–8 weeks).** PRD-002/003/004/006/008/009; EXP-004/006/008; SEC-007 (before the free tier exists, not after); BIZ-001→002→003 (pricing needs INF-007 cost telemetry design underway); INF-006/007/008; PRC-001/002. This wave is wide because the grammar now exists: each data-service spec is a thin, fast delta.

**Wave 4 — the v2 window (starts during v1 execution).** FND-012 full, PRD-010, PRD-011, FND-009 ingress completion, EXP-007 (public docs must be mature before GA, so it actually starts in Wave 3 and completes here). Spec authorship deliberately leads implementation by one version: **while engineers build vN, authors write vN+1** — the pipeline discipline that prevents the "specs written by archaeologists" failure mode.

**Deferred (do not write yet):** INF-009 (BYOC), PRD-013/014, SEC-006 beyond its skeleton, all v4 candidates. Each has its extension points reserved in a Wave-1 document; that reservation is the correct amount of ink today.

**Unnecessary at this stage (explicitly):** marketing/positioning docs (GOV-001 already carries positioning), competitive analyses (decision inputs, not specifications), speculative multi-region designs, any document for the never-build list.

## 3.3 The critical path, stated plainly

`GOV-003 → FND-001 → FND-002 → FND-003 → PRD-001 → (INF-002 joins) → v0 build`.
Six documents stand between approval and the first line of production Postgres code. Everything else parallelizes around them. If leadership accelerates anything, accelerate these six — and note that **INF-002 (Tenancy & Isolation)** is the one with an unmade architectural decision inside it (Deliverable 5), making it the likeliest schedule risk.

---
# Deliverable 4 — Specifications Mapped to Product Evolution

The mapping rule follows GOV-002 §8's rhythm: a specification must be **Approved before the build of the version it governs begins**, and is *authored* during the preceding version's execution. "Version" below means *the version whose build the document unblocks*.

## v0 — The Wedge

**Must be Approved:** GOV-003, GOV-004 · FND-001, 002, 003, 004, 005, 006, 008 (core envelope + audit/log classes), 009 (environment isolation core), 010 (metering emission contract only — no prices yet), FND-012 *stub* · PRD-001, PRD-007 (core) · EXP-003 (CLI core grammar), EXP-001 (skeleton: context bar + service page anatomy) · INF-001, 002, 003, 004, INF-005 (SLO framework, not SLA numbers) · SEC-001, 002, 003, 008 · PRC-001.

**Justification:** v0's promise is Postgres inside the Project model with backups, monitoring, and secure defaults (GOV-002 §8 v0). Every listed document is load-bearing for that sentence: the primitives and grammar because PRD-001 is written as a delta over them; tenancy/isolation and stateful-ops because "zero data-loss incidents" is v0's survival criterion; metering emission because retrofitting billing telemetry is notoriously miserable — emit from day one, price later; audit (SEC-003) because GOV-002 §2.4 commits to audit "from day one, not retrofitted." Deliberately *absent*: pricing (BIZ-001 — alpha is unpaid), quotas beyond safety limits, Console beyond skeleton (v0 is CLI-first, per the wedge's focus).

## v0.5 — The Data Layer

**Authored during v0, Approved for v0.5 build:** FND-007 (Policies), FND-011 (Quotas) · PRD-002, 003, 004, 005 (full), 008, PRD-012 (provisioning-recommendation scope) · EXP-001 (full), 002, 004 (TS/Python), 006, 008 (format + first templates) · SEC-004, 007 · INF-006, 007 · BIZ-003 (free-tier enforcement skeleton).

**Justification:** v0.5 is the four-service data layer plus the guided/AI provisioning flow plus real Console (GOV-002 §8 v0.5). The three new data-service specs are the wave that tests whether FND-003 actually made product specs thin — a stated success signal for the documentation program itself. SEC-007 lands here because the public beta creates the abuse surface. PRD-012's first scope is exactly the vision's recommendation flow: describe the app, see reasoned suggestions and costs — nothing more.

## v1 — The Platform Promise (GA)

**Authored during v0.5, Approved for v1 build:** PRD-006 (Backup & Restore as workflow product), PRD-009 (Cost & Usage) · FND-010 complete (estimates contract GA), BIZ-001, 002, 004 · EXP-005 (Terraform), EXP-007 (public docs GA) · INF-005 complete (SLA numbers per plan), INF-008 · SEC-005, SEC-006 (skeleton + SOC 2 kickoff) · PRC-002.

**Justification:** GA is when money, SLAs, and support become contractual (GOV-002 §8 v1). Pricing, billing ops, support model, DR, and launch-readiness gates are precisely the documents whose absence turns GA into improvisation. PRD-006 graduates restore from per-service feature to rehearsable cross-service workflow — matching v1's "operational baseline" promise.

## v1.5 *(depth release — included for completeness)*

Revisions, not new documents: PRD-001 rev (query insights GA, pgvector section + graduation criteria), PRD-002 rev (clustering), PRD-003 rev (CDN), PRD-004 rev (schedules/DLQ tooling), EXP-004 rev (Java/Rust/C#), EXP-005 rev (Pulumi), SEC-003 rev (audit GA for Business tier). The deepen beat produces *revisions with changelogs*, demonstrating the living-document model working as designed.

## v2 — The Developer Cloud

**Authored during v1/v1.5, Approved for v2 build:** FND-012 (full), FND-009 (ingress/domains/TLS completion) · PRD-010, PRD-011 · EXP-003 rev (`deploy`, `dev` GA), EXP-006 rev (deploy-first onboarding path).

**Justification:** the compute promise (GOV-002 §8 v2). Note how short this list is: the foundation absorbed the complexity in advance — FND-012's stub reserved states in v0's data model precisely so v2 would be an extension, not a migration. That payoff is the architecture's coherence thesis, visible in the documentation plan.

## v3 — The Enterprise Platform

**Authored during v2/v2.5:** INF-009 (BYOC) · PRD-014 (Private Networking) · FND-005 rev (SSO/SCIM/custom roles), FND-007 rev (org policy GA), FND-009 rev (multi-region property) · SEC-006 complete (SOC 2 evidence), SEC-002 rev (BYOK) · BIZ-001 rev (enterprise packaging) · PRD-001 rev (cross-region replicas/DR).

**Justification:** GOV-002 §9.4's doctrine — enterprise as *intensification of existing primitives* — is enforced documentarily: v3 is dominated by **revisions** to foundation specs rather than new documents. If v3 planning ever produces a stack of brand-new specs, that is the early-warning sign of the "enterprise fork" disease, and this plan is designed to make it visible.

## Future (v2.5 / v4 — deliberately unwritten)

PRD-013 (Vector/Search — written when PRD-001's graduation criteria trigger), the v4 portfolio wedge specs (Functions, Event Bus, Workflow, Analytics DB, customer-app Identity, Edge, Marketplace, Self-Hosted). Each has today: a reserved doc ID, a named extension point in a foundation spec, and nothing else. **Justification:** GOV-002 §8 defines v4 as options exercised on customer pull; pre-writing them would convert options into commitments and documentation into fiction.

---

# Deliverable 5 — Readiness Assessment

**Verdict: the platform is ready for *specification*, not yet for *implementation* — and that is the correct state.** The Vision and Architecture are unusually complete at their altitude: primitives justified, boundaries argued, sequencing principled. But between GOV-002 and the first pull request stand decisions it deliberately (and rightly) did not make. Implementation may begin the moment the critical-path six (§3.3) are approved — realistically 6–10 weeks of focused authoring — *provided* the following gaps are resolved inside them.

## 5.1 Missing architectural decisions (blocking — must be decided in named specs)

1. **Tenancy & isolation model (→ INF-002).** GOV-002 defines Cells but not whether a Free-tier Postgres is a container on shared hardware, a shared instance with logical isolation, or a small dedicated VM. This single decision drives unit economics (BIZ-001), the Free tier's viability, noisy-neighbor behavior, security posture, and instance-class design in PRD-001. *The highest-consequence unmade decision in the program.*
2. **Initial cloud substrate and region (→ INF-001).** Which provider hosts the first Cell, and where. Affects egress economics, service latitude, and the BYOC seam's first proof. (GOV-001's Bengaluru-adjacent talent pool and target markets may argue for specific launch regions — a business input the spec must record, not assume.)
3. **Postgres HA and branching technology (→ PRD-001 design docs).** The architecture promises branching as the signature feature without choosing a storage approach (copy-on-write filesystem, storage-layer replication, logical). This choice bounds branch-creation latency and cost — both customer-visible contract numbers PRD-001 must publish.
4. **Resource identity scheme (→ GOV-003/FND-001).** ID format, slug mutability, uniqueness scopes, rename semantics. Trivial-seeming; permanent; embedded in every URL, API path, and Terraform state file ever created.
5. **API versioning policy (→ FND-002).** Header vs path versioning, deprecation windows, beta-surface rules. "API-first" (GOV-001) is only as credible as its compatibility policy.
6. **Metering pipeline authority (→ FND-010).** What is the billing source of truth, its granularity, and its reconciliation guarantees. Revenue correctness is not retrofittable.
7. **AI proposal-artifact model (→ PRD-012).** GOV-002 §7 defines the four laws; the *reviewable artifact* (plan/diff object, its lifecycle, its audit binding) needs a concrete design before any product ships an AI feature — otherwise each team invents its own approval UX and the laws become slogans.

## 5.2 Ambiguities to resolve (non-blocking for start, blocking for their wave)

- **Bucket/Service granularity** in Object Storage (flagged in PRD-003): does the topology map show "Storage" or every bucket? Resolve against FND-001 before v0.5.
- **Environment cost boundaries:** preview environments with database branches — who pays, what limits, what the estimate shows (FND-010 × PRD-011). The flagship feature must not become the surprise-bill feature.
- **"External compute" mode ergonomics:** GOV-002 blesses Steloit-as-data-layer as a permanent mode; FND-004 must define how Bindings inject into *non-Steloit* runtimes (Vercel env-var sync? copy-paste with rotation warnings?) — currently undefined and central to v0/v1 adoption.
- **Org-to-project secret grants** (FND-006): grant semantics named in GOV-002 §2.1 but not specified.
- **Suspension semantics** across non-payment, abuse, and quota breach (FND-010/011 × SEC-007 × BIZ-003): three documents touch it; one must own the state machine — recommend FND-001 owns the states, others own the triggers.
- **Terminology drift risk:** "Observe" (nav), "Observability" (product), "Operations" (vision family) — GOV-003 must pick the customer-facing word once.

## 5.3 Risks in the documentation program itself

- **Foundation-spec perfectionism.** The critical path runs through six documents; gold-plating FND-001–003 delays everything. Mitigation: timebox Wave 0–1, ship specs at "Approved with open-questions register," let PRD-001's authorship pressure-test them (it will).
- **Spec–implementation divergence.** Living documents rot without enforcement. Mitigation: PRC-002 launch gates include spec-conformance sign-off; FND-003's checklist is executable review material, not prose.
- **The Postgres double-risk concentration.** The hardest spec, the hardest engineering, and the signature feature share one team. Mitigation is organizational (staffing, external Postgres expertise) and belongs on the leadership risk register now.
- **Enterprise pull-forward.** An early large customer will request v3 documents (SSO, BYOC) out of order. The plan's answer — extension points reserved, specs deferred — must be defended by leadership, or the sequencing collapses.
- **AI feature pressure without PRD-012.** Market pressure to demo AI features before the proposal-artifact contract exists would violate the four laws in the platform's first impression. PRD-012's minimal scope (provisioning recommendations) is deliberately small so there is no excuse to bypass it.

## 5.4 Assumptions to document before engineering begins (the Assumptions Register — an appendix of GOV-004)

1. Single region, single underlying provider at v0; multi-region is v3 (per GOV-002 — restated because engineers *will* ask).
2. Kubernetes as internal Cell substrate is an implementation choice, revisable, never customer-visible (GOV-002 §3.2).
3. Alpha (v0) is unpaid; metering emits regardless (FND-010).
4. CLI-first for v0; Console reaches parity at v0.5 — a sequencing choice, not a philosophy change.
5. English-only surfaces at launch; localization unscheduled.
6. GitHub before GitLab for every Git-adjacent feature.
7. The Free tier exists from public beta onward and its abuse economics are a solved-on-paper problem (SEC-007 + INF-002) *before* beta opens.
8. AI features use external model providers behind PRD-012's abstraction at launch; no commitment to self-hosted models.
9. No Workspace primitive at launch; folders + teams + policies compose the equivalent (GOV-002 §1.2) — recorded so enterprise conversations don't reopen it casually.
10. Data-plane availability must survive control-plane outage (INF-003) — an invariant, assumed by every SLA conversation to come.

## 5.5 Readiness scorecard

| Area | State | Gate to green |
|---|---|---|
| Vision & positioning | 🟢 Approved | — |
| Product architecture | 🟢 Approved | — |
| Naming & identity scheme | 🔴 Undefined | GOV-003, FND-001 |
| Platform contracts (grammar, bindings, events, API) | 🔴 Unwritten, fully shaped by GOV-002 | Wave 0–1 |
| First product definition (Postgres) | 🟡 Architecturally scoped | PRD-001 + two design docs |
| Isolation & unit economics | 🔴 Undecided | INF-002 → BIZ-001 |
| Security foundations | 🟡 Doctrine set, controls unwritten | SEC-001–004, 008 |
| Experience system | 🟡 IA designed in GOV-002 §4 | EXP-001–003 |
| Business systems | 🔴 Philosophy only | FND-010, BIZ-001 (v1 gate, not v0) |
| Operational readiness | 🔴 Unwritten | INF-004/005 (v0 gate) |

**Bottom line:** approve this plan, staff Wave 0 this week, and hold implementation kickoff to a single condition — the critical-path six Approved with INF-002's isolation decision made. Everything else proceeds in parallel, in waves, exactly one version ahead of the builders.

---

# Appendix A — Master Document Register (summary)

| Category | Count | v0 | v0.5 | v1 (+1.5 revs) | v2 | v3 | Future |
|---|---|---|---|---|---|---|---|
| GOV | 4 | 4 | — | — | — | — | — |
| FND | 12 | 10 (+1 stub) | 2 | rev | 2 completions | 3 revs | — |
| PRD | 14 | 2 | 5 | 2 (+revs) | 2 | 1 (+revs) | 2+ |
| EXP | 8 | 2 | 5 | 2 | revs | — | — |
| INF | 9 | 5 | 2 | 2 | — | 1 | — |
| SEC | 8 | 4 | 2 | 2 | — | revs | — |
| BIZ | 4 | — | 1 | 3 | — | rev | — |
| PRC | 2 | 1 | — | 1 | — | — | — |
| **Total** | **61** | **28** | **17** | **12** | **4** | **5+revs** | **reserved** |

*(Register counts are the source of truth and supersede in-text tallies; GOV-004's registry maintains this table henceforth.)*

**The one-sentence summary of this entire plan:** write the contracts first, write each product as a thin delta over them, keep authorship one version ahead of construction, and let the deferred documents stay deferred — the architecture earned its simplicity by sequencing, and the documentation must be built the same way.

*— End of master plan —*
