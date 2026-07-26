# INF-001 — Steloit Infrastructure Constitution

| | |
|---|---|
| **Status** | Locked |
| **Date** | 2026-07-09 |
| **Applies to** | All infrastructure, substrate, and spend decisions until superseded |
| **Supersedes** | Nothing (first infrastructure decision record) |
| **Related** | GOV-002 (product architecture), the Constitution, API spec, `steloit-breakeven-model.xlsx` |

## 0. Governing principle

> **Cheap on capacity, never on shape.**

Capacity (nodes, replicas, retention, regions, cells) is reversible and may always be minimized to save money. Shape (data model, API surface, isolation boundaries, provisioning pattern) is expensive to change once customers exist and must be production-grade from day one. Every proposal touching infrastructure is evaluated against this sentence first.

External promise this serves: *know before you deploy*. Internal principle: *show your work*. This document is both applied to Steloit's own infrastructure: no spend before the estimate, no decision without recorded rationale.

---

## 1. Decisions (locked)

Each decision has an ID. Humans and AI agents MUST cite the relevant ID when proposing work that depends on, extends, or conflicts with it. Conflicts require an explicit amendment to this document, not a silent workaround.

### D1 — Build on hyperscaler primitives, never resell managed services
We run our own software on raw primitives (Kubernetes, VMs, object storage, networking). We do NOT wrap RDS/Cloud SQL, ElastiCache/Memorystore, SQS/Pub/Sub as our products.
**Rationale:** managed services have no branching (kills the flagship feature), stack margins on margins (kills cost transparency), and cap the product at the vendor's ceiling.
**Consequence:** we operate more software; that is where features and margins live.

### D2 — One cloud: GCP first
All environments run on GCP (GKE Standard, GCS, Compute Engine) until a paying enterprise customer demands otherwise. AWS is a possible future "dedicated cell" SKU, not a founding constraint.
**Rationale:** continuity from the 90-day sprint; GKE Sandbox gives managed gVisor; free zonal-cluster credit; sustained-use discounts are automatic; Google for Startups credits up to $200k. The grammar (D8) keeps this reversible.
**Consequence:** no multi-cloud work of any kind pre-revenue. Manifests stay plain Kubernetes to keep the exit cheap.

### D3 — Postgres substrate: operate Neon OSS ourselves *(amended: A4 — CNPG-operated vanilla PostgreSQL + CoW volume snapshots; the branching REQUIREMENT stands, the engine changes)*
The Postgres product is built on Neon's Apache-2.0 storage engine (Pageservers + Safekeepers + object storage), run in our infrastructure. Not white-labeled, not snapshot-based CoW, not a custom engine. *(amended: A4 — it is now precisely snapshot-based CoW)*
**Rationale:** branch-per-PR is the category-defining feature and lives in the storage engine; Neon's is the only open, production-proven implementation; operating it ourselves preserves unit-economics visibility that the cost-estimate promise requires. Native multi-tenancy (tenant = database, timeline = branch) makes free tiers and PR previews economically survivable. *(amended: A4 — the "only open, production-proven implementation" premise no longer holds for outside operators; see A4 evidence)*
**Known risks:** operational complexity (budget 2–3 senior engineers when hiring begins); Databricks controls the upstream roadmap (mitigation: Apache 2.0, forkable); Neon quirks (no true superuser, extension constraints) become our product quirks and must be documented in our grammar, never exposed as "Neon." *(amended: A4 — the Databricks-upstream risk fired pre-build; recorded in §7)*
**Open item (verify in sprint week 1):** Pageserver remote-storage backend against GCS. *(amended: A1.10 — spike checklist: multipart semantics, conditional writes/generation fencing, list pagination)* Docs list S3 and Azure natively; test GCS S3-interoperability mode (HMAC keys) with write → restart → restore. Fallbacks: native GCS backend in current repo, or thin S3-compatible layer in front of GCS.

### D4 — Object storage: proxy the hyperscaler, never self-host *(amended: A5.3 — DORMANT: no managed object-storage product is built in the planning horizon; storage is a Binding to the customer's own provider. D4 applies only if managed storage is ever revived by a future ADR, on a zero-egress backend.)*
Customer buckets are GCS (later optionally S3/R2) behind the Steloit API and grammar. We do not run MinIO/Ceph/Garage for customer data at any current scale.
**Rationale:** eleven-nines durability is fleet statistics + scrubbing discipline we cannot match; failure mode is permanent data loss (the one unforgivable sin); no signature feature depends on bucket internals; object storage is the bottom turtle (everything else's backups live there).
**Consequence:** margin here is a markup on public prices — priced transparently, which is on-message.

### D5 — Everything else: foundation-governed open source, assembled under the grammar
Compute: GKE + gVisor (GKE Sandbox). Cache: Valkey (per-project pods, never shared instances) *(amended: A5.1 — cache is OPTIONAL, provisioned only on explicit add, idle-suspended, hard-quota'd; never a pod per project by default; shared-multi-tenant-with-isolation permitted if idle economics require)*. Queues: Postgres-backed (pgmq-style) on the D3 substrate so queue state branches with the database; dedicated broker (NATS JetStream) only when throughput demands. *(amended: A1.2, A2.2 — idle-economics design constraint applies)* *(amended: A5.2 — the customer queue is NOT a separate service or broker; it is pgmq inside the customer's Postgres, consumed by a worker; the NATS fallback and the A1.2/A3.1 queue constraint are struck for the customer product, retained only for internal control-plane jobs via River)* Observability: VictoriaMetrics/Mimir *(amended: A1.10 — metrics backend is a deferred choice)* + Loki + OpenTelemetry, multi-tenant modes, tenant labels stamped at ingest by our collector. Secrets: OpenBao or KMS-backed envelope encryption (never invent crypto). AuthZ model: built (inseparable from org→project→environment); authN plumbing: adopted (OIDC libraries, Dex/Zitadel for SSO later). AI layer: inference bought via API (swappable); citation/evidence machinery built (it IS the differentiator).
**Standing rule:** prefer foundation-governed projects over single-vendor open source (Redis→Valkey, Elastic, MinIO, Vault→OpenBao all rhyme). D3 is the one deliberate exception.

### D6 — Two planes, cell-based data plane
**Control plane:** Steloit-the-application (console, API, provisioner, estimate engine, billing, IAM). One multi-tenant deployment, its own project/account, never runs customer code.
**Data plane:** cells. One cell = one zonal GKE Standard cluster + one Neon storage fleet *(amended: A4 — CNPG fleet + ZFS storage node pool)* + GCS bucket set + observability pipeline, in its own GCP project.
**Cell caps (policy, not physics):** ~500–1,000 paying customers or ~5,000–10,000 total signups per cell, ~50–60 workload nodes, 1–2 Pageservers. Chosen for blast radius, staged rollouts, quota headroom, and clean unit economics (one cell ≈ $2–3k/mo ≈ $25–50k MRR). *(amended: A1.1 — this figure is the cell baseline, not at-capacity cost)* A dedicated cell is the future enterprise SKU.
**Capacity rules of thumb:** active project ≈ 0.6 vCPU / 1.5 GB across pods; ~12–18 active projects per 8-vCPU node; idle projects ≈ zero pods (scale-to-zero) and are nearly free; one Pageserver serves thousands of idle tenants but only a few hundred concurrently busy databases.

### D7 — Tenancy mapping and isolation (non-negotiable)
Org → control-plane row (billing + IAM boundary, no infrastructure). Project → exactly one cell (`cell_id` column). Environment → Kubernetes namespace (`env-<environment_id>`) with default-deny NetworkPolicies, ResourceQuota, LimitRange. *(amended 2026-07-26, ADR-0012 — was `proj--env`; project names are unique only per ORG, so name-derived namespaces collided ACROSS orgs and shared the very isolation boundary this clause creates. The namespace derives from the globally-unique, immutable environment id; the 1:1 environment→namespace contract and its policies are unchanged.)* Service → pods in that namespace. Customer code ALWAYS runs under gVisor (GKE Sandbox); plain containers are not a tenant boundary. Databases: Neon tenant per database, timeline per branch, compute endpoint pod in the customer's namespace. *(amended: A4 — one CNPG cluster per project-environment; branch = CoW volume snapshot → recovered cluster, hibernated by default)* Buckets: one GCS bucket per project (cost attribution, lifecycle, hard delete); access via our API or STS-scoped presigned URLs only. *(amended: A1.4, A2.4 — customer-visible URLs are Steloit-controlled domains, never raw provider URLs)* Caches: dedicated small Valkey pods per project-environment. Observability: shared per-cell pipeline; `project_id`/`environment_id` labels stamped by our collector (never trusted from the customer), enforced at the query layer.

### D8 — The grammar is the only surface
No substrate concept (Neon, GKE, GCS, gVisor) *(amended: A4 — read CNPG/ZFS for the database layer)* ever appears in a customer-visible URL, response body, error message, metric name, or docs page. Every substrate concept has a Steloit-grammar name. This is the swap insurance that keeps D2, D3, D4 reversible.

### D9 — Reconciler pattern, never imperative provisioning
The control plane writes desired state; a per-cell agent converges actual state toward it. The control plane's database is the single source of truth for "what is running" (which the observability promise also needs). A control-plane outage degrades to "cannot make changes," never "customer apps down." *(amended: A2.5 — desired-state vs actual-state clarification)*

### D10 — Metering from day one
Every resource emits usage events (compute-seconds, CU-hours, GB-months, egress bytes) tagged org/project/environment, from the first deploy — even before billing exists. Actuals calibrate the estimate engine; per-tenant attribution makes the cost page trustworthy; tagged cloud resources reconcile against the provider bill (the delta is margin, per product). Backfilling usage history is impossible; therefore this cannot be deferred.

### D11 — No hires, no scope creep until revenue
Founders only until the first payment clears. Alpha scope is ONE path end-to-end: push repo → see cost estimate → approve → deployed service + Postgres → open PR → branched preview environment. Cache, queues, object-storage API, the AI layer, and the 152-screen console all wait behind evidence. CLI or minimal UI is acceptable for the alpha.

---

## 2. Day-one invariants (shape — never cheap out)

1. `cell_id` on every resource row; provisioner routes through a cell-selection function even while the answer is always `cell-1`.
2. Reconciler pattern (D9) from the first provisioned resource.
3. Everything via Terraform/Pulumi + git. Monthly fire drill: rebuild the entire environment from scratch in one afternoon. If the drill is scary, the architecture is wrong.
4. Grammar-only API surface (D8).
5. Namespace-per-project-environment with default-deny + quotas, even with three users.
6. gVisor for customer workloads from the first customer pod.
7. Metering events flowing from day one (D10).
8. Pod CIDR / IP ranges sized at cluster creation for the full-grown cell (fixed at birth; painful to change).

*(amended: A1.5 adds invariants 9–11: tenant restore drill, control-plane DB backup/DR, security floor.)*

## 3. Deferred until revenue (capacity — cheap now, dial later)

| Item | Starter state | Trigger | Upgraded state |
|---|---|---|---|
| Safekeepers | 1 | First paying customer | 3 (quorum), config + 2 pods |
| Cluster topology | Zonal, scale-to-zero workload pool; core pool floor of 1 *(amended: A1.6)* | First paying customer | Node floor > 0; regional when SLA sold |
| Pageservers | 1 pod | Hundreds of concurrently active DBs | Shard tenants across 2+ |
| Observability retention | Days, single replica | Paying customers | Weeks/months, replicated |
| Region | us-central1 for founder-only destroyable dev *(amended: A1.7)* | Design partner can touch it | Born in Mumbai (asia-south1); no migrations |
| Cell #2 | Schema column only | ~$25–40k MRR or cell-1 caps | Terraform apply |
| Products beyond the alpha path | Absent | Design-partner evidence | Slot into existing multi-tenant frame |
| Billing | Metering only | First price published | Attach billing to existing event stream |
| Dedicated cells / AWS SKU | Absent | Enterprise deal demands it | New cell, same Terraform, different params |

**The upgrade moment is knob-turning, not migration.** If any upgrade requires a rewrite, an invariant in §2 was violated; treat as an incident.

## 4. Phase plan and budgets

| Phase | Gate to enter | Infra budget | Content |
|---|---|---|---|
| **Rung 0 — demand validation** | Now | ₹0 | Clickable prototype from the 152 screens; landing page + waitlist on Cloud Run free tier; 30–50 developer conversations; ask for costly commitments (pre-order, design-partner LOI); register the customer-content domain *(extended: A2.4)* |
| **Rung 0.5 — estimate tool** | Parallel with Rung 0 | ₹0 | The estimate engine as a free standalone web tool; tests the core promise, builds audience |
| **90-day sprint** | GCP trial activated | $300 trial credit (≈$0 cash) | One zonal GKE cluster (free mgmt fee), scale-to-zero node pool, Neon at N=1 *(amended: A4 — CNPG + ZFS-LocalPV)*, full alpha path working by day 90 |
| **Post-90 holding pattern** | Trial expired, no revenue yet | ~$34–40/mo (~₹3,000) duty-cycled | Same setup on own card; sustainable indefinitely |
| **Cell #1 production** | Design partners active AND credits approved | Credits (Google for Startups / AWS Activate — apply NOW, approval takes weeks) | Turn §3 knobs; Mumbai; invite-only |
| **Scale** | First payments clearing | Funded by revenue + credits | Hire against D3 ops load; cell #2 per trigger |

Advancing a phase without its gate is a violation of the governing principle. Building because "the handoff package makes building feel ready" is the named trap: the package makes building FAST once demand is proven; it does not prove demand.

## 5. Verified cost reference (checked 2026-07-09, us-central1 unless noted)

- GKE management fee: $0.10/cluster-hour; free tier $74.40/month per billing account = one zonal Standard or Autopilot cluster free, PERMANENT (Always Free program, independent of the 90-day $300 trial). Credit does not cover regional clusters and does not roll over.
- e2-standard-2 (2 vCPU/8GB): ~$0.067/hr ≈ $48.92/mo on-demand; spot ≈ $24.5/mo (~50% off). Duty-cycled 8h×26d (208h): ~$14 on-demand, ~$7 spot.
- pd-balanced disk: ~$0.10/GB-month (bills 24/7 regardless of node state — this is the idle floor).
- GCS Standard: $0.020/GB-month.
- Internet egress: ~$0.12/GB. Cloud Logging free to 50GiB/month, then per-GiB — route logs to in-cluster Loki (also dogfooding).
- Mumbai (asia-south1) premium: ~10–15% over us-central1.
- Production cell (multi-AZ, from the AWS-basis estimate, GCP comparable): ~$1.8–2.8k/month on-demand; −30–55% with committed-use discounts once steady.

## 6. Unit economics summary (full model: `steloit-breakeven-model.xlsx`)

Assumptions: Pro $19, Team $99, 25% Team mix, +30% usage overage → blended ARPU ≈ $50.70. Marginal infra ≈ $14.75/paying; free-tier drag at 4% conversion ≈ $9.60; fees ≈ $1.52. **Contribution ≈ $25/paying customer/month.**

| Scenario | Fixed/mo | Break-even paying | Signups | MRR |
|---|---|---|---|---|
| A — founders, no salary | $3,000 | ~121 | ~3,000 | ~$6.1k |
| B — ramen team (BLR, $7k payroll) | $10,000 | ~403 | ~10,100 | ~$20.4k |
| C — 8-person team ($30k payroll) | $33,000 + 1 cell | ~1,410 | ~35,300 | ~$71.5k |

Most fragile assumption: conversion (at 2%, Scenario B needs ~34k signups). Profit levers in order: Team-tier mix (50% Team nearly halves break-evens), usage margin (transparent ≠ at-cost), Bengaluru cost base (protect it — payroll, not cloud, decides runway). Revenue is power-law: a few hundred Team accounts will matter more than thousands of hobbyists; ship the things that make growing startups stay (RBAC, audit, SLAs, dedicated cells) in roadmap order.

## 7. Known risks (standing register) *(extended: A1.8)*

| Risk | Exposure | Mitigation |
|---|---|---|
| Neon OSS stagnation under Databricks | D3 substrate | Apache 2.0 fork option; ecosystem co-users; grammar isolation (D8) | *(amended: A4 — risk REALIZED 2025-07→2026: public releases stopped, upstream dark; resolved by substrate change, not fork)* |
| GCS backend incompatibility with Pageserver | D3 on D2 | Week-1 spike (D3 open item); S3-interop or shim fallback |
| Egress/inter-zone bleed | Margins | AZ/zone-locality by design; GCS via private access; egress priced transparently to customers |
| Supplier-as-competitor (Google/AWS) | Strategy | Value lives in our layer (estimate, grammar, branching, evidence-citing AI); margin discipline |
| Conversion below 4% | Break-even | Estimate tool + content build top-of-funnel early; watch cohort data from day one |
| Solo-founder ops load on Neon stack *(amended: A4 — CNPG stack; materially lower)* | D3 | N=1 alpha honesty; first hires are substrate ops; alpha label buys forgiveness |

## 8. Instructions for AI agents working in this repo

1. Treat §1 decisions and §2 invariants as constraints, not suggestions. Cite decision IDs (e.g., "per D7") in every proposal that touches infrastructure.
2. Propose changes only with cited evidence (cost data, incident data, benchmark, customer signal) — the same rule the Steloit constitution imposes on the product's own AI layer. Never act on infrastructure unilaterally.
3. If a task appears to require violating an invariant, STOP and surface the conflict with a proposed amendment to this document instead of working around it.
4. Default all designs to: grammar-only naming (D8), desired-state/reconciler flows (D9), metering hooks (D10), and per-tenant isolation (D7). Absence of these in generated code/designs is a defect.
5. When estimating costs, state assumptions and give ranges; recompute from §5 prices and flag if prices may be stale (>90 days old — re-verify before financial decisions).
6. Scope guard: anything outside the D11 alpha path requires explicit human approval referencing the phase gate in §4.

---

---

## Amendment A1 — 2026-07-13

**Trigger:** external review conducted per §8 (findings 1–10, filed in repo alongside this document). All findings accepted; #2 and #7 accepted with modified resolutions. Rationale per finding below.

### A1.1 — D6 cell economics corrected (finding 1)
D6's "one cell ≈ $2–3k/mo" is the **cell baseline** (Neon fleet *(amended: A4 — CNPG fleet + storage pool; baseline shape unchanged)*, observability, node floor, LBs) — not the at-capacity cost. Corrected model: **baseline ~$2–3k/mo + ~$15/paying-customer marginal (per §6); fully loaded at the 500–1,000 paying cap ≈ $10–18k/mo**, against $25–50k MRR at the same cap. Budget cells from this formula, never from the baseline alone.

### A1.2 — Queues design constraint (finding 2)
D5's pgmq-on-substrate choice conflicts with D6's idle economics: polled queues keep compute endpoints awake, defeating scale-to-zero for exactly the projects most likely to hold queues. Queues are outside the alpha path (D11), so no mechanism is locked now. **Blocking constraint recorded:** the queues product MUST NOT require always-awake computes for idle projects. If no design satisfies this on the D3 substrate (note: LISTEN/NOTIFY and naive shared pollers both still require a held connection), queues move to a dedicated broker (NATS JetStream per D5), accepting loss of branch-coherence. This constraint gates the queues product's design review. *(extended: A2.2; candidates consolidated in A3.1)*

### A1.3 — Alpha durability floor (finding 3)
§3's single-Safekeeper starter state is bounded, not open-ended: **alpha RPO ≤ 5 minutes** *(recommended default — pending founder ratification)*. Implementation: Safekeeper WAL offload to GCS at a short interval; Safekeeper WAL on a persistent disk so node loss ≠ data loss (only disk loss exposes the window). *(amended: A2.3 — zonal PD means disk OR zone loss exposes the window)* The RPO number is printed in alpha terms of service. D4's "permanent data loss is unforgivable" is interpreted as: loss must never be **unbounded or undisclosed**; a small, chosen, published window during a labeled alpha is compliant. Quorum (3 Safekeepers, RPO ≈ 0) remains triggered at first payment per §3.

### A1.4 — Object URLs are grammar surface (finding 4)
D7's "STS-scoped presigned URLs" as written violates D8: a GCS presigned URL exposes `storage.googleapis.com` in a customer-visible surface. **Locked (shape):** all customer-visible object URLs are served from Steloit domains (e.g., `objects.<region>.steloit.dev` *(amended: A2.4 — example superseded; a separate registrable domain is required)*), fronted by our proxy/CDN/load-balancer layer; raw provider URLs never leave the platform boundary. Implementation choice (LB + Cloud CDN vs. application proxy) is capacity, decided with A1.9.

### A1.5 — Three invariants added to §2 (finding 5)
9. **Tenant restore drill:** monthly, restore one project's database from object storage to a fresh branch and diff — §2.3 rebuilds infrastructure; this drill proves *customer data* restores. Automate it; it later ships as a customer-facing trust feature.
10. **Control-plane DB backup/DR:** the D9 source-of-truth database gets PITR-grade backups to a separate GCS location from day one, restore-tested in the §2.3 drill. Its loss must degrade to "re-adopt state from cells," never to unknown customer state. *(amended: A2.5 — authoritative reading: restore desired state from backup, reconcile against actual state from cells)*
11. **Security floor:** workload identity everywhere; zero static service-account keys (CI included); signed images with provenance from the first build. Retrofit cost is brutal; day-one cost is near zero.

### A1.6 — Starter topology precision (finding 6)
§3 corrected: scale-to-zero applies to the **workload node pool only**. The core pool (control plane app + Neon fleet) has a floor of one node — the Neon fleet cannot scale to zero. *(amended: A4 — read "CNPG operator + active customer clusters"; hibernated customer clusters DO scale to zero, strengthening this economics)* Pre-revenue duty-cycling of the core pool = **scheduled platform downtime**, a deliberate, stated choice that ends at first design-partner onboarding (from then: core pool 24/7, workload pool remains scale-to-zero).

### A1.7 — Region rule (finding 7)
Founder-only, destroyable dev environments may run in us-central1 for price. **Anything a design partner can touch is born in asia-south1 (Mumbai).** No environment is ever scheduled for region migration; per §3's closing rule, a required migration is an incident.

### A1.8 — Risk register additions (finding 8)
| Risk | Exposure | Mitigation |
|---|---|---|
| GCP quota walls (fresh account won't get 50-node CPU quota; increases take days) | Cell-1 growth, demo timing | File staged quota increases at sprint start and at each §4 gate; track quota headroom as a §4 gate item |
| Free-compute abuse (scale-to-zero + free tier attracts miners; gVisor does not prevent this) | Margin, IP reputation | Alpha is invite-only (D11); free tier never offers anonymous compute; per-tenant egress caps + CPU-pattern detection before any open signup |

### A1.9 — Egress posture (finding 9)
Beyond transparent pricing: evaluate **Standard-tier networking** for non-SLA paths and **CDN in front of object/static paths** (jointly with A1.4's proxy) at cell-1 buildout. Decision recorded then; until then egress is priced to customers with margin per D4.

### A1.10 — Housekeeping (finding 10)
- D3 spike checklist named: multipart-upload semantics, **conditional writes / generation fencing** (Neon's split-brain protection depends on these), list pagination — the three known divergence areas of GCS S3-interop. *(amended: A4 — spike redefined: ZFS-LocalPV snapshot→clone→CNPG recovery end-to-end + hibernation wake time; the GCS S3-interop question is moot)*
- Metrics backend "VictoriaMetrics/Mimir" restated honestly as **deferred choice**; decision due at cell-1 buildout; Loki + OpenTelemetry remain locked.
- gVisor compatibility stance: a documented unsupported-syscall list (io_uring et al.), surfaced in grammar-named errors per D8 — never "gVisor doesn't support this."

---

## Amendment A2 — 2026-07-13

**Trigger:** re-review of the A1-amended document (notes R1–R5, filed alongside). All five adopted; R4 adopted in strengthened form.

### A2.1 — Amendment stamp convention (R1)
Append-only governs **semantic content**. Inline stamps of the form *(amended: A1.x)* placed next to superseded or extended sentences in earlier text are declared **permitted mechanical edits** — they carry no meaning of their own and exist so a cold reader is never left holding a superseded sentence unwarned. Stamps for A1 and A2 have been applied retroactively throughout this document. Every future amendment MUST stamp the text it supersedes in the same commit.

### A2.2 — Queue candidate designs recorded (R2)
Before the A1.2 fallback (dedicated broker, losing branch-coherence) is invoked, two D3-substrate designs must be evaluated, in order: **(a) producer-side signaling** — the producer's compute is by definition awake at enqueue; a trigger-fired outbound signal to the cell dispatcher costs no additional wakefulness; **(b) safekeeper/CDC-derived depth signals** — queue-table inserts already traverse the WAL stream through Safekeepers we operate, so enqueue events can be detected with zero held connections and full branch-coherence. Design (b) is the preferred candidate. Consumer wake is dispatcher-driven in both; customer-side polling loops are not the supported pattern. *(amended: A3.1 — (a) consolidated into (b) as its trigger-outbox variant)*

### A2.3 — RPO exposure statement corrected (R3)
A1.3's "only disk loss exposes the window" is understated: on a zonal cluster, persistent disks are zonal, so **disk failure or zone failure** exposes the ≤ 5-minute window. Alpha ToS wording: *"in a disk or zone failure, up to 5 minutes of the most recent database writes may be lost."* RPO design unchanged.

### A2.4 — Customer-content domain isolation (R4, strengthened)
The A1.4 example domain is replaced by a requirement: customer-controlled content (objects, preview environments, user-served assets) is served from a **separate registrable domain** from the console/API domains — the `githubusercontent.com` pattern. Rationale: a subdomain of the primary domain (e.g., `objects.steloit.dev`) still shares cookie scope and origin-adjacency with `console.steloit.dev`; only a distinct eTLD+1 provides the isolation. **Action: choose and register the content domain during Rung 0** (cost: one domain registration; retrofit cost: an origin migration across every customer URL). Locked as shape.

### A2.5 — D9 desired/actual state clarification (R5)
D9 restated precisely: the control-plane database is the single source of truth for **desired state**; cells hold and report **actual state**; "what is running" as shown to customers is desired state annotated with reconciliation status. DR for the control-plane DB (invariant §2.10) = restore desired state from backup, then reconcile against actual state reported by cells. This removes the apparent reversal in A1.5.10 and is the authoritative reading of D9.

---

## Amendment A3 — 2026-07-13

**Trigger:** third-pass review (findings F1–F4, filed alongside). All adopted. Mechanical stamps for F1 and F3 applied in this commit per A2.1.

### A3.1 — Queue candidates consolidated (F2)
A2.2's design (a) has no clean substrate implementation as an independent mechanism: Postgres triggers cannot safely make network calls, and NOTIFY-based signaling requires a dispatcher holding a LISTEN connection to the compute — reintroducing exactly the A1.2 problem. The honest form of (a) is a trigger writing to an outbox table whose insert traverses the WAL — which is design (b) by another ingestion path. A2.2 is restated as **one candidate family — WAL-derived signals at the safekeeper/CDC layer we already operate** — with two ingestion variants: (i) direct CDC on queue tables; (ii) trigger-outbox where queue writes need decoupling from customer schema. The A1.2 fallback ordering (dedicated broker, losing branch-coherence) is unchanged and remains last resort.

### A3.2 — Stamp vocabulary defined (F4)
A2.1's permitted mechanical stamps come in exactly two forms: ***(amended: X)*** — the stamped text is superseded or corrected by X — and ***(extended: X)*** — X adds to the stamped text without contradicting it. The pre-existing *(extended: A1.8)* stamp on §7 is hereby legitimate. No other verbs are permitted without a convention amendment.

---

## Amendment A4 — 2026-07-18

*Founder-ratified 2026-07-18. Full evidence, candidate matrix, and consequences: `steloit/cloud` `docs/adr/0003-database-substrate.md` (this constitution records the decision; the ADR records the analysis). Product ADR log entry: ADR-033.*

### A4.1 — D3 substrate re-decided: CNPG + copy-on-write volume snapshots replace Neon OSS

D3 is amended. The Postgres substrate is **CloudNativePG-operated vanilla PostgreSQL — one cluster per project-environment — with copy-on-write branching via CSI/ZFS volume snapshots (OpenEBS ZFS-LocalPV on a storage node pool), orchestrated by the Steloit control plane** (Xata OSS, Apache-2.0, as reference implementation; DBLab noted for internal CI only).

**Evidence (measured, 2026-07-18):** Neon OSS self-hosting was never a supported product (staff-stated ~2-week binary/config compatibility window; shipped compose "not intended for deploying a usable system"); the control plane was never open source; the public repo's release tags stopped 2025-07-29 with commits at ~1/month through 2026 post-Databricks; the only serious third-party operator was dev/test-grade. The §7 risk "Neon OSS stagnation under Databricks" is recorded as having **fired before build start** — the fork mitigation is rejected as economically absurd at founder scale. Meanwhile CNPG reached CNCF process, PG18-current, declarative fleet operation with a documented 450-cluster production precedent, and Xata open-sourced a CNPG-based CoW branching platform on this exact stack.

**What is unchanged:** D3's *requirement* — branch-per-PR as the category-defining feature with delta-priced previews — stands in full; it is satisfied at the storage-snapshot layer instead of the storage-engine layer. Unit-economics visibility stands (snapshot deltas + hibernated computes are directly meterable). D7 tenancy mapping now reads: one CNPG cluster per project-environment (a *stronger* isolation boundary than shared pageservers); branch = snapshot-recovered cluster, hibernated by default. D8 grammar isolation is what made this swap free — no customer surface changes.

**Consequences:** the week-1 spike is redefined (ZFS snapshot → clone → CNPG recovery e2e; hibernation wake latency; branch cost measurement). RPO ≤5 min (A1.3) is met by `archive_timeout`-bounded WAL archiving to GCS. A3.1's queue family simplifies to standard logical-decoding CDC on vanilla Postgres (no safekeeper layer exists; the constraint and fallback ordering are unchanged). The 2–3-substrate-engineer budget (D3 risk note) is expected to relax; re-verify at Cell-1. Ops-load risk (§7) drops accordingly.

---

## Amendment A5 — 2026-07-18

*Founder-ratified 2026-07-18. Full evidence, candidate analysis, and ripple: `steloit/cloud` `docs/plan/product-family-review.md` + `docs/adr/0004-product-families.md`. Product ADR log entry: ADR-034. Outcome of the adversarial product-family review — the boundary sharpens to "build only the differentiated products; everything commodity becomes a Postgres capability or a Binding."*

### A5.1 — D5 cache clause: optional, never default
Valkey remains the cache substrate. The "per-project pods, never shared" implication is amended: a cache is an **optional** service, **provisioned only on explicit add**, **idle-suspended**, and hard-quota'd — never a pod per project by default. Shared-multi-tenant-with-strong-isolation is a permitted implementation if idle economics require it. Rationale: cache cannot scale to zero, so a default dedicated pod per project is a permanent idle-cost floor hostile to §3 scale-to-zero economics.

### A5.2 — D5 queue clause: a Postgres capability, not a service
The customer queue is **not a separate service or broker**. Queue capability is **pgmq inside the customer's Postgres** (branches with the database for free; consumed by a worker that scales to zero like any compute). The A1.2/A3.1 scale-to-zero-queue constraint and the NATS-JetStream fallback are **struck for the customer product** and retained only as guidance for internal control-plane jobs (River, Architecture v1 §12). Risk R3 (queue scale-to-zero unsolvable) is retired with the apparatus it described.

### A5.3 — Object storage → integration (Storage Binding)
No managed object-storage product is built in the planning horizon. Storage is delivered as a **Storage Binding** to the customer's own provider (S3/GCS/R2): credentials, config injection, policy, lifecycle, audit — Steloit never proxies the bytes and never bears egress. D4 is marked dormant (above). A managed storage product may return only via a future ADR on a zero-egress backend.

### A5.4 — AI → integration (AI Binding, not a gateway)
AI is governed as an **AI Binding** to an external provider: allow-policy, credentials-in-Secrets, config injection, estimate-at-bind, cost visibility (provider usage API), lifecycle audit, soft spend control. Steloit **does not** proxy LLM traffic, route/failover across providers, or enforce hard in-line spend caps — those are the AI-gateway commodity, joining §3.7's never-build set in spirit. The four-laws assistant (ADR-005) is unaffected — that is Steloit's own AI; the AI Binding governs the customer app's AI dependency.

### A5.5 — Bindings extend to external providers
The Binding primitive (GOV-002 #6) may target an external provider (type + provider + region + secret-ref), not only an internal service — the shared mechanism for A5.3 and A5.4. Grammar isolation (D8) holds: no substrate/provider concept leaks into a customer surface beyond the provider name the customer chose. The managed `Product` surface is now exactly `[postgres, valkey, web, worker]`; `gpu-worker` is removed, `storage`/`ai-gateway`/`queue` are not managed products.

### A6 — dev/alpha branching substrate on GKE PD-CSI; ZFS-LocalPV re-scoped to Cell-1 (2026-07-19, ADR-0007)
A4 is amended on measured evidence (T1.0 spike, ADR-0007): at dev/alpha scale the CNPG branching substrate runs on **GKE PD-CSI (`pd-balanced`) volumes with PD incremental VolumeSnapshots**, not OpenEBS ZFS-LocalPV. The designed ZFS node failed to provision (`ZONE_RESOURCE_POOL_EXHAUSTED`, local-SSD+n2 coupling; transcript committed) and carries a permanent operational tax (Ubuntu-only nodes, privileged zpool bootstrap, kernel/image coupling, local-SSD ephemerality). Every A4 promise was met on PD-CSI with measured numbers: branch e2e 52.4 s, hibernation wake 8.0 s, restore-to-new 55.2 s, RPO ≤ 300 s by construction, delta-priced incremental snapshots (~62 MB per 10%-divergence Dev branch). **ZFS-LocalPV is re-scoped, not rejected**: it is the Cell-1 branch-density optimization, with an explicit measured trigger (branch count × delta economics vs a local-SSD storage node) and the spike kit retained as the runnable re-evaluation harness. D3's branching requirement, D8 grammar isolation, and A1.3's RPO bound are unchanged.

---

*Amendments: append-only, dated, with rationale (mechanical stamps per A2.1/A3.2 excepted). The governing principle in §0 may not be amended, only superseded by a new constitution.*
