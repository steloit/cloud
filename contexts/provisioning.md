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
  project → exactly one cell; environment → namespace `env-<environment_id>` (ADR-0012: name-derived
  `proj--env` collided across orgs) with default-deny NetworkPolicies +
  quotas; **customer code always under gVisor**; **one CNPG cluster per project-environment**;
  branch = ZFS CoW volume snapshot → CNPG-recovered cluster, hibernated by default. Branching is a
  **product** capability orchestrated by our control plane — never exposed as a database feature.
- **Grammar-only surface (D8):** CNPG/ZFS/GKE/GCS/gVisor never appear in any customer-visible URL,
  response, error, metric, or doc.
- **Metering (D10):** every lifecycle edge emits usage events (compute-seconds, CU-hours, GB-months,
  egress) tagged org/project/env from the FIRST deploy. Backfill is impossible — never defer.
- **BYOC is demand-gated to v3 (ADR-0005):** the cell/reconciler/`cell_id` design is justified by
  isolation/regions/resilience/scale — **BYOC is a free rider on it, not its justification.** Carry the
  portable shape; build **zero** BYOC-specific machinery (no multi-cloud abstraction, no cross-account
  IAM, no AWS/Azure drivers) until all five exit criteria hold. Run the residency ladder on any "BYOC"
  ask (residency → regional cell; network → PrivateLink; keys → BYOK; only contractually-required
  sovereignty → BYOC). Not every enterprise request is a BYOC request.
- **Estimate gate:** `createService` requires an accepted `estimate_id`; nothing provisions or
  bills before acceptance; the estimate's line grammar is the invoice's line grammar, verbatim.

## Drivers

**The managed `Product` surface is exactly `[postgres, valkey, web, worker]` (ADR-0004/A5).** It names the
*architecture plane* only — the catalog is outcome-first (ADR-039/040): intents resolved by the Composer
to named resolutions with stated semantics; **execution models are replaceable, semantic contracts are not.**
Per-product
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
- Classifying by implementation instead of state semantics (ADR-038's State Test: state location/ownership decides Service vs Capability vs Binding — lifecycle, backup, branching, billing all follow state).
- Surfacing a capability's dependency as a prerequisite or error instead of composing it (the catalog sells intents; the composer proposes the shaped service + estimate — "Jobs without Postgres" is a proposal, never homework).
- Backend-swapping a capability into different semantics ("Jobs on Kafka") — a semantic divergence is a NEW named service via the gate, never a swap (ADR-038 scope clause).
- An external Binding that proxies bytes/traffic, routes across providers, or enforces a hard in-line cap — that is the commodity we don't build (A5.3/A5.4).
- Letting an execution model change silently under a Product, or migrating one without a visible, priced,
  consented estimate (ADR-040: the Composer proposes; the accepted estimate is the contract).
- Zombie state on failed provisioning: failures must converge to a clean desired/actual pair and never bill.
- **Moving a guard without asking what else it was doing.** US-3.8 shipped five blockers across three
  review rounds and every one was the same: a guard that existed and worked, until a boundary moved
  under it. Making `desired` track the override column turned "any PATCH un-pins" into a real path but
  left the price restore only in the shape branch, so un-pinning released the capacity and kept charging
  for it — permanently, since the row was then unsweepable. Splitting the sweep's one UPDATE into
  SELECT+UPDATE moved the expiry predicate off the write path, so a concurrent customer edit was
  silently reverted and billed at the pre-edit rate. Unifying the pricing path deleted the `estimates.Price`
  call that was also the merged shape's first validator and owned the `ShapeError → 422` conversion, so a
  client typo started answering 500 "contact support" on one endpoint while the same input still
  answered 422 on another. **The diagnostic question after any restructure is not "does the new code
  work" — it is "what did the moved or deleted code guarantee, and who guarantees it now?"** Each of the
  three had a second job nobody had written down, and a green suite found none of them.
- **A test that re-implements the guard it is named for asserts nothing about that guard.** US-3.8's
  `TestTheDesiredDocNeverCarriesADeadPin` called `overrideInstances` on its own input, nilled the pin,
  and then asserted `desiredDoc` produced no pin — which is the production line, copied into the test
  body. Deleting the real one from `services.go` left the test GREEN; a mutation sweep found it. The
  same shape hid in the sweep predicate: `… AND pg_input_is_valid(x) AND x::timestamptz <= now()`
  looked like it guarded the cast, and deleting BOTH guards changed nothing — because an *earlier* OR
  arm was already `OR pg_input_is_valid(x) = false`. The same guard, written twice. OR short-circuits
  left to right, so the duplicate shielded the cast and the inner copy could never fire; delete the
  earlier arm too and the statement aborts. Both were caught by the same question: **if I delete the
  line this test is named after, does the test fail?** If the answer needs a copy of that line
  somewhere else to be yes, the class is open. The fix for the first is to drive the real entry point
  and read the real column; for the second, one flat `CASE` — a `CASE` nested inside the `OR` merely
  moved the problem, silently absorbing the `IS NULL` arm through its `ELSE`.
  *Corrected 2026-07-27 after review:* this entry first blamed query-plan reordering. It was not that —
  the shield was deterministic duplication. Postgres genuinely does not promise WHERE-clause evaluation
  order (which is why `CASE` is still right, forward-looking), but that was not the mechanism here, and
  "a plan might reorder this" and "I wrote this guard twice" have different fixes. **A mistake bank
  entry that names the wrong cause teaches the wrong reflex** — the same failure O11 was corrected for.
- **A module-only `cp -R` gives a red baseline for tests that reach outside the module.** The
  fault-injection rule says copy the module, mutate the copy — but `TestCKM3EstimateGatedProvisioningEndToEnd`
  and `TestEveryAssistantHandlerGatesOnPolicy` read `../../../../docs/…` and `apps/cli`, so under
  `cp -R services/api` they fail before any mutation is applied. Establish the baseline FIRST and
  name the pre-failing tests, or copy from the repo root when the sweep touches those packages.
  A reviewer who skips the baseline spends the session chasing a red they introduced.
- **If the same invariant must hold at more than one site, give it one owner — and prefer an owner
  the compiler enforces (ADR-0014).** US-3.8 spent six review rounds on ONE error repeated: a guard
  applied per-site instead of made unrepresentable. An overflow bound went into one arm of a
  three-arm pricing switch (the siblings kept wrapping, and a wrapped price disabled the org's spend
  cap permanently); a 404-for-no-standing conversion went into one transport of a two-transport
  endpoint (one request header reopened the oracle). Both fixes were correct and both were partial,
  and no amount of care at the next site would have changed the rate. `int64` cents compiles `a * b`,
  so every priced dimension is an opportunity to forget; `money.Cents` is a struct, so it does not
  compile at all. **The question after writing any guard is not "is this one right" — it is "how many
  places must be right, and what stops the next one being missed?"** If the answer is "a reviewer",
  the design is wrong.
- **A row read, priced, and written back needs a generation fence.** `UpdateServiceShape` had a
  pre-existing stale-read race that was merely a desired-doc divergence — until US-3.8 wrote the price
  column on every PATCH, at which point it could put three facts in disagreement at once: the column
  holding one shape, the cell rendering another, and the invoice charging a third rate that no reprice
  could detect (both sides of the comparison come from the same stale read). Fence on the generation
  read, return 409 "re-read and retry"; a silent overwrite of money is not recoverable.
- **Never hand-append to a committed raw evidence log.** If an ad-hoc command produced the number
  (a diagnosis mid-spike), the fix is: commit the producing command as a script/manifest, and record
  line-by-line provenance (results/PROVENANCE.md) — an unattributed append to a "raw" log defeats
  the entire point of committing evidence, and reviewers WILL catch the timestamp/format seams (T1.0).
- **A findings ADR that changes frozen text is a formal delta, not a reinterpretation.** If
  architecture.md/00-sources literally name the thing you're replacing, propose the amendment text
  (the ADR-0003→A4 precedent) — never claim "no delta because the semantic contract is unchanged";
  the catalog-plane "execution models are replaceable" language does not apply to founder-ratified
  infrastructure decisions (T1.0 review C1).
- **Spike kits need a committed idempotent teardown with evidence.** In-cluster cleanup + "I deleted
  it in the console/session" is not teardown: commit a script that is safe to re-run (waits for PVC
  reclaim before cluster delete, removes bucket-IAM tombstones of deleted SAs, ends with an orphan
  sweep) and commit its output as the $0-state proof (T1.0 QA review).
