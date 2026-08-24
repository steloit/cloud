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
- Letting a substrate name (cnpg/zfs/gke/gcs — or the retired neon) into an id, error, URL, or metric label (D8).
- Metering only "when billing ships" — metering is day-one (D10). Scale-to-zero designs that require an always-awake poller (A1.2 — internal jobs; the customer queue is pgmq-in-DB drained by a scale-to-zero worker, A5.2).
- Building storage/AI/queue as a *managed service* — they are Bindings or a Postgres capability (A5); a new managed product needs an ADR, and classification is by state semantics, not implementation (ADR-038's State Test).
- Surfacing a capability's dependency as homework instead of composing it — "Jobs without Postgres" is a proposal, never homework (the composer proposes the shaped service + estimate).
- Backend-swapping a capability into different semantics ("Jobs on Kafka") — a semantic divergence is a NEW named service via the gate, never a swap (ADR-038 scope clause); and an execution model must never change silently under a Product, nor migrate without a visible, priced, consented estimate (ADR-040).
- An external Binding that proxies bytes/traffic, routes across providers, or enforces a hard in-line cap — that is the commodity we don't build (A5.3/A5.4).
- Zombie state on failed provisioning: failures must converge to a clean desired/actual pair, never bill.
- **Moving a guard without asking what else it was doing.** US-3.8's five blockers over three rounds were one
  cause: a guard that worked until a boundary moved under it. Making `desired` track the override column left
  the price restore in the shape branch, so un-pinning released capacity and kept charging — permanently, the
  row now unsweepable. Splitting the sweep's UPDATE into SELECT+UPDATE moved the expiry predicate off the
  write path, silently reverting and mis-billing a concurrent edit. Unifying the pricing path deleted an
  `estimates.Price` call that was also the merged shape's first validator and owned `ShapeError → 422`, so a
  client typo answered 500 on one endpoint while the same input answered 422 on another. **The question after
  any restructure is not "does the new code work" — it is "what did the moved or deleted code guarantee, and
  who guarantees it now?"** Each had a second job nobody wrote down; a green suite found none.
- **A test that does not test the thing it is named for.** Two shapes, both green. *Re-implementing the
  guard:* US-3.8's `TestTheDesiredDocNeverCarriesADeadPin` called `overrideInstances` itself — the production
  line copied into the test body — so deleting the real one from `services.go` left it GREEN and only a
  mutation sweep found it. Same in the sweep predicate: an inner `AND pg_input_is_valid(x) AND x::timestamptz
  <= now()` could never fire because an *earlier* OR arm was already `OR pg_input_is_valid(x) = false` and OR
  short-circuits left to right — delete the earlier arm too and the statement aborts. *Answered by a different
  object than it names:* US-3.3a's D8 check widened from `objs[0]` (the Cluster) to the concatenated applied
  set, which the tenancy manifests satisfy alone, so rendering the Cluster into another tenant's namespace
  stayed GREEN; its repair was still `strings.Contains`, so `…-shadow` survived — widening one object to a set
  IS a weakening. Ask: **if I delete the line this test is named after, does it fail?** Fix: drive the real
  entry point and read the real column; one flat `CASE` (nesting it in the `OR` absorbed the `IS NULL` arm via
  `ELSE`). *Corrected 2026-07-27:* this first blamed query-plan reordering; it was deterministic duplication.
  Postgres does not promise WHERE evaluation order (so `CASE` is still right), but that was not the mechanism
  — **naming the wrong cause teaches the wrong reflex** (O11).
- **"The delete was ACCEPTED" is not "the workload is GONE".** k8s answers 2xx the moment it accepts
  one, finalizers pending, and `kube.Delete` maps any 2xx to nil — so US-3.3b's namespace-teardown
  gate meant acceptance while calling itself absence. Observe absence before reporting it.
- **Verify the no-mutation baseline is GREEN before AND after any mutation sweep.** A module-only
  `cp -R` is red on arrival wherever tests reach outside the module, and the list keeps growing —
  find it by RUNNING the copy, never by recall. So far: `services/api` needs
  `docs/dev/money-range-audit.md` + `docs/product/08-api/openapi.yaml`; `services/cell-agent` needs
  `AGENTS.md` + `infra/{k8s,spike}` + `pricing.json` + `billing/plans.json`. US-3.3a shipped a
  25-row table on a RED baseline — every row unfalsifiable. Assert the mutation APPLIED (two false
  SURVIVEDs came from a match string that silently missed) and that the clean copy passes BOTH sides.
- **If the same invariant must hold at more than one site, give it one owner — prefer one the compiler
  enforces (ADR-0014).** US-3.8 spent six review rounds on ONE repeated error: a guard applied per-site
  instead of made unrepresentable. An overflow bound went into one arm of a three-arm pricing switch
  (siblings kept wrapping, and a wrapped price disabled the org's spend cap permanently); a
  404-for-no-standing conversion went into one transport of two. Both correct, both partial; more care
  at the next site would not have changed the rate. `int64` cents compiles `a * b`; `money.Cents` is a
  struct and does not. **Ask not "is this guard right" but "how many places must be right, and what
  stops the next being missed?"** If the answer is "a reviewer", the design is wrong. *Deferral (founder,
  2026-07-27: US-3.8 keeps only the one-arm bound) closed by O16* — `money.Cents` is live across every
  priced dimension, so those sibling arms can no longer wrap.
- **A row read, priced, and written back needs a generation fence.** `UpdateServiceShape`'s
  pre-existing stale-read race was a mere desired-doc divergence until US-3.8 wrote the price column on
  every PATCH — then it could disagree three ways at once: the column holding one shape, the cell
  rendering another, the invoice charging a third rate no reprice could detect (both sides of the
  comparison come from the same stale read). Fence on the generation read, 409 "re-read and retry" —
  a silent overwrite of money is not recoverable.
- **Never hand-append to a committed raw evidence log.** Commit the producing command as a
  script/manifest with line-by-line provenance (results/PROVENANCE.md); reviewers WILL catch the
  timestamp/format seams of an unattributed append (T1.0).
- **A findings ADR that changes frozen text is a formal delta, not a reinterpretation.** If
  architecture.md/00-sources literally name what you're replacing, propose the amendment text
  (the ADR-0003→A4 precedent) — never "no delta because the semantic contract is unchanged"
  (T1.0 review C1).
- **Spike kits need a committed idempotent teardown with evidence.** "I deleted it in the console" is
  not teardown: commit a re-runnable script (waits for PVC reclaim, clears bucket-IAM tombstones of
  deleted SAs, ends with an orphan sweep) and its output as the $0-state proof (T1.0 QA).
- **Shipping a constraint without finding what enforces it.** US-3.3a rendered D7's default-deny
  NetworkPolicies and proved every manifest correct, but `infra/modules/gke-cell` is GKE Standard with
  no `network_policy`/`ADVANCED_DATAPATH`: the API server stores them, nothing drops a packet.
  Rendered, stored and enforced are three representations; the suite covered one. Inert-but-wrong is
  no no-op either — the allow-set denied what CNPG needs, so enabling enforcement WOULD have fenced
  the first Postgres pod, from another directory in another task.
- **Widening a lookup table without widening its consumer turns a loud error into a silent success.**
  Four kinds added to `kube`'s `plurals` were not inert: `Delete` hardcoded the CNPG apiVersion, so they
  built plausible paths under the wrong group, 404'd, and 404 maps to "already gone" — US-3.3b's exact
  call would report success while the namespace lived. A kind absent from the consumer must be REFUSED,
  and the two key sets asserted EQUAL: the first fix's own test could not fail, because every kind it
  tried was missing from BOTH maps, so the refusal came from the path builder, not the guard it named.
- **Guard every document, element and path — not the first of each.** `yaml.Unmarshal` returns only
  document 1 and a nil error, which has now bitten three times: a kind-based absence guard and a
  cross-namespace check both passed while a SECOND document carried an arbitrary object, and
  US-3.3b's teardown classified an object's SCOPE from doc 1 (a cluster-scoped doc 2 would never be
  deleted). Refuse multi-doc, as `kube.applyOne` does. Pinning for one element, then 0-1, then 0-3 is
  a constant a mutation can match; a skip keyed on byte length or `spec:` stayed green because every
  fixture was a hand-written stub — build fixtures from the real renderers. And `Converge`'s deleting
  branch returns before the renderer, so a guard in `Render` covered create only:
  `"../../../api/v1/namespaces/kube-system"` was refused on create and ACCEPTED on teardown.
- **A driver reading a key the API's closed schema forbids is silent contract drift.** T3.4c:
  `cnpg.storageForShape` sized the PVC from `shape["storage"]`, which `estimates.shapeSchema`
  rejects, so the priced `storage_gb` was never read — a customer billed for 78 GB got the size
  default, with invoice and audit trail both agreeing. Bind the driver to the catalog with a test
  that READS `pricing.json`; check every path the refusal sits on (the same call served TEARDOWN,
  making the service undeletable); and **prove the wire** — deleting the key en route left the
  agent suite green — US-3.3b hit it twice more, pinning the poll's `environments` key and the
  teardown's whole HTTP path on ONE side of a two-module wire.
- **A floor HIDES a wrong sign.** `max(declared, included)` is the coherent way to default
  `storage_gb`, but it made the negative-value check unreachable. Reject out-of-range BEFORE
  defaulting.
