---
id: provisioning
owns: [services/cell-agent/**, services/api/src/provisioning/**, infra/**]
see: [api-conventions, billing]
---

# Provisioning, cells & the reconciler

Governing law: INF-001 (`docs/product/00-sources/INF-001-infrastructure-constitution.md`) —
**"cheap on capacity, never on shape."** Cite decision IDs (D1–D11) in any infra-touching change;
if a task requires violating an invariant, STOP and surface it (§8).

## Shape (day-one, non-negotiable)

- **Two planes (D6):** control plane = the api service (desired state, estimates, billing, IAM);
  data plane = cells (zonal GKE + CNPG fleet on a ZFS storage pool + GCS per cell, per ADR-0003).
  Control-plane outage must degrade to
  "cannot make changes", never "customer apps down".
- **Reconciler (D9/A2.5):** control plane writes desired state to its DB; the cell-agent converges
  actual state and reports back. No imperative provisioning, ever. DR = restore desired, reconcile actual.
- **Tenancy (D7, as amended A4):** every resource row carries `cell_id` (even while it's always cell-0);
  project → exactly one cell; environment → namespace `proj--env` with default-deny NetworkPolicies +
  quotas; **customer code always under gVisor**; **one CNPG cluster per project-environment**;
  branch = ZFS CoW volume snapshot → CNPG-recovered cluster, hibernated by default. Branching is a
  **product** capability orchestrated by our control plane — never exposed as a database feature.
- **Grammar-only surface (D8):** CNPG/ZFS/GKE/GCS/gVisor never appear in any customer-visible URL,
  response, error, metric, or doc.
- **Metering (D10):** every lifecycle edge emits usage events (compute-seconds, CU-hours, GB-months,
  egress) tagged org/project/env from the FIRST deploy. Backfill is impossible — never defer.
- **Estimate gate:** `createService` requires an accepted `estimate_id`; nothing provisions or
  bills before acceptance; the estimate's line grammar is the invoice's line grammar, verbatim.

## Drivers

**The managed `Product` surface is exactly `[postgres, valkey, web, worker]` (ADR-0004/A5).** Per-product
drivers behind one provisioner interface (Postgres/CNPG is the pioneer — cluster create, snapshot-branch,
hibernate/wake, PITR-to-new-branch; **Valkey** instantiates the anatomy — one pioneer at a time, ADR-010).
Reference for branching mechanics: Xata OSS (Apache-2.0); substrate record: `docs/adr/0003-database-substrate.md` + INF-001 §A4.

**Not products — do not build these as managed services (A5):**
- **Cache (Valkey)** is *optional* — provision-on-add, idle-suspend, hard quotas; never a pod per project by default (A5.1).
- **Queue** is a *Postgres capability* — pgmq inside the customer's DB, consumed by a worker. No queue service, no broker, no NATS; A3.1/R3 retired (A5.2).
- **Storage & AI** are *external-provider Bindings* (A5.3/A5.4/A5.5): the Binding primitive extended to external targets (provider + config + secret-ref). Steloit never proxies bytes/traffic and never bears egress; credentials live in Secrets; estimate-at-bind shows the provider's price. AI Binding is control-plane governance only — **no proxy, no routing, no hard in-line caps** (that's the gateway commodity). Distinct from the four-laws assistant.
- **GPU** is removed; a future GPU need is a Binding to Modal/Replicate, v4+.

Preview/content served on the content eTLD+1 (A2.4) applies to *preview environments* (E4), not to a storage product.

## Mistake bank

- Provisioning imperatively from a request handler instead of writing desired state (violates D9).
- A resource table without `cell_id`, or provisioning that skips the cell-selection function (inv. 1).
- Letting a substrate name (cnpg/zfs/gke/gcs — or the retired neon) into an id, error, URL, or metric label (violates D8).
- Metering only "when billing ships" — metering is day-one (D10).
- Scale-to-zero designs that require an always-awake poller (A1.2 — internal jobs; the customer queue is now pgmq-in-DB drained by a scale-to-zero worker, A5.2).
- Building storage/AI/queue as a *managed service* — they are Bindings or a Postgres capability (A5); a new managed product needs an ADR.
- An external Binding that proxies bytes/traffic, routes across providers, or enforces a hard in-line cap — that is the commodity we don't build (A5.3/A5.4).
- Zombie state on failed provisioning: failures must converge to a clean desired/actual pair and never bill.
