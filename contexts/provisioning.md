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
  portable shape; build **zero** BYOC-specific machinery — no multi-cloud abstraction, no cross-account
  IAM, no AWS/Azure drivers (ADR-0005 §Ripple assigns this here) — until all five exit
  criteria hold. **Not every enterprise request is a BYOC request:** run the residency ladder (residency →
  regional cell; network → PrivateLink; keys → BYOK; only contractual sovereignty → BYOC).
- **Estimate gate:** `createService` requires an accepted `estimate_id`; nothing provisions or
  bills before acceptance; the estimate's line grammar is the invoice's line grammar, verbatim.

## Drivers

Per-product drivers sit behind ONE provisioner interface (Postgres/CNPG is the pioneer — cluster
create, snapshot-branch, hibernate/wake, PITR-to-new-branch; **Valkey** instantiates the anatomy, one
pioneer at a time, ADR-010). The product surface and the outcome-first catalog are CLAUDE.md hard
rules; branching mechanics reference Xata OSS (Apache-2.0), substrate `docs/adr/0003-*` + INF-001 §A4.

**Not products (A5)** — CLAUDE.md carries the rule; what it does not carry: Cache (Valkey) is
*provision-on-add*, idle-suspend, hard quotas — never a pod per project by default (A5.1). Queue is
pgmq **inside the customer's DB**, drained by a scale-to-zero worker (A5.2; A3.1/R3 retired). A
Binding is provider + config + secret-ref and estimate-at-bind shows the PROVIDER's price; **NO Binding
proxies bytes/traffic, routes across providers, bears egress, or enforces a hard in-line cap** — that is
the commodity we don't build, and the AI Binding is control-plane governance only (A5.3-A5.5). GPU is
out; a future GPU need is a Binding to Modal/Replicate, v4+.

Preview/content served on the content eTLD+1 (A2.4) applies to *preview environments* (E4), not to a storage product.

## Mistake bank

- Provisioning imperatively from a request handler instead of writing desired state (violates D9).
- A resource table without `cell_id`, or provisioning that skips the cell-selection function (inv. 1).
- Letting a substrate name (cnpg/zfs/gke/gcs — or the retired neon) into an id, error, URL, or metric label (D8).
- Metering only "when billing ships" — it is day-one (D10). Scale-to-zero designs needing an always-awake poller (A1.2 — *internal jobs*; the customer queue is pgmq-in-DB drained by a scale-to-zero worker).
- Backend-swapping a capability into different semantics ("Jobs on Kafka"): a NEW named service via the gate, never a swap, and never a silent or unpriced migration under a live Product (ADR-038/040).
- Zombie state on failed provisioning: failures must converge to a clean desired/actual pair, never bill.
- **Moving a guard without asking what else it was doing.** US-3.8's five blockers over three rounds
  were one cause. Making `desired` track the override column left the price restore in the shape
  branch, so un-pinning released capacity and kept charging, the row now unsweepable. Splitting the
  sweep's UPDATE into SELECT+UPDATE moved the expiry predicate off the write path, silently
  mis-billing a concurrent edit. Unifying the pricing path deleted an `estimates.Price` call that was
  also the merged shape's first validator and owned `ShapeError -> 422`, so a typo answered 500 on one
  endpoint and 422 on another. **The question after a restructure is not "does the new code work" —
  it is "what did the moved code guarantee, and who guarantees it now?"**
- **A test that does not test the thing it is named for.** Several shapes, all green.
  *Re-implementing the guard:* US-3.8's test called `overrideInstances` itself — the production line
  copied into the test body — so deleting the real one left it GREEN; and an inner `AND` arm of the
  sweep predicate could never fire because an EARLIER `OR` arm already matched (one flat `CASE`
  fixed it). *Answered by a different
  object:* US-3.3a's D8 check widened from the Cluster to the concatenated applied set, which the
  tenancy manifests satisfy alone — widening one object to a set IS a weakening. *Text presence is
  not semantics:* US-3.3c's policy tests grepped for `cnpg.io/cluster`, so flipping the selector's
  operator `Exists`->`DoesNotExist` — which selects exactly the CUSTOMER pods and INVERTS the rule —
  stayed green — as would a label left only in a comment. Parse it; assert on the parsed selector.
  Ask: **if I delete the line this test is named after, does it fail?** *Corrected 2026-07-27:* this
  first blamed query-plan reordering; it was deterministic duplication — **naming the wrong cause
  teaches the wrong reflex** (O11).
- **"The delete was ACCEPTED" is not "the workload is GONE".** k8s answers 2xx the moment it accepts
  one, finalizers pending, and `kube.Delete` maps any 2xx to nil — so US-3.3b's namespace-teardown
  gate meant acceptance while calling itself absence. Observe absence before reporting it.
- **Enumerate every workload in the flow, INCLUDING the ones that exist only at bootstrap — and
  verify a security rule while the environment still exists.** US-3.3c's CNPG allowances selected
  `cnpg.io/podRole: instance`; CNPG bootstraps through a JOB whose pod carries `cnpg.io/jobRole`
  and `cnpg.io/cluster` but NOT podRole, so the initdb pod matched no allowance and the cluster
  never started: a fifth WORKLOAD none of the four allowances US-3.3a's review enumerated covered.
  An enumeration of rules is not a proof of coverage, and a selector correct in steady state can
  match nothing at t=0. The headline's second half is costlier still — a tightening discovered AFTER
  the cell is destroyed cannot be tested, and shipping it unverified is worse than a named exception:
  it reads as stronger and can fence the very thing it protects (US-3.3j, wide on purpose).
- **Verify the no-mutation baseline is GREEN before AND after any mutation sweep.** A module-only
  `cp -R` is red on arrival wherever tests reach outside the module, and the list keeps growing —
  find it by RUNNING the copy, never by recall. So far: `services/api` needs
  `docs/dev/money-range-audit.md` + `docs/product/08-api/openapi.yaml`; `services/cell-agent` needs
  `AGENTS.md` + `infra/{k8s,spike}` + `pricing.json` + `plans.json` + `08-api/openapi.yaml`. US-3.3a shipped a
  25-row table on a RED baseline — every row unfalsifiable. Assert the mutation APPLIED (two false
  SURVIVEDs came from a match string that silently missed) and that the clean copy passes BOTH sides.
- **If the same invariant must hold at more than one site, give it one owner — prefer one the compiler
  enforces (ADR-0014).** US-3.8 spent six review rounds on ONE repeated error: a guard applied per-site
  instead of made unrepresentable. An overflow bound went into one arm of a three-arm pricing switch
  (siblings kept wrapping, and a wrapped price disabled the org's spend cap permanently); a
  404-for-no-standing conversion went into one transport of two. Both correct, both partial; more care
  at the next site would not have changed the rate. `int64` cents compiles `a * b`; `money.Cents` is a
  struct and does not. **Ask not "is this guard right" but "how many places must be
  right, and what stops the next being missed?"** If the answer is "a reviewer", the design is wrong.
  (Closed by O16: `money.Cents` is live across every priced dimension, so the siblings cannot wrap.)
- **A row read, priced, and written back needs a generation fence.** `UpdateServiceShape`'s stale-read
  race was mere desired-doc divergence until US-3.8 wrote the price column on every PATCH — then column,
  cell and invoice could disagree three ways at once, undetectably by a reprice (both sides of that
  comparison come from the same stale read). Fence on the generation read, 409 "re-read and retry".
- **Never hand-append to a committed raw evidence log.** Commit the producing command with
  line-by-line provenance; reviewers WILL catch an unattributed append's seams (T1.0).
- **A findings ADR that changes frozen text is a formal delta, not a reinterpretation.** If
  architecture.md/00-sources literally name what you're replacing, propose the amendment text
  (the ADR-0003→A4 precedent) — never "no delta because the semantic contract is unchanged"
  (T1.0 review C1).
- **Spike kits need a committed idempotent teardown with evidence.** "I deleted it in the console" is
  not teardown: commit a re-runnable script (waits for PVC reclaim, clears bucket-IAM tombstones of
  deleted SAs, ends with an orphan sweep) and its output as the $0-state proof (T1.0 QA).
- **Shipping a constraint without finding what enforces it — and NAMING THE COMPONENT THAT ACTUALLY
  SERVES THE TRAFFIC.** US-3.3a rendered D7's default-deny policies and proved every manifest
  correct while `gke-cell` was GKE Standard with no enforcement: stored, and nothing drops a packet.
  Rendered / stored / enforced are three representations and the suite covered one. US-3.3c proved
  the same class LIVE, twice: the DNS rule named `k8s-app: kube-dns` and resolved NOTHING, because
  NodeLocal DNSCache (default-on, unpinned by our terraform) answers the query; and the apiserver
  peer named the `kubernetes` ClusterIP, which Dataplane V2 never matches — it evaluates egress
  POST-translation, so the private endpoint is the real destination. Both LOOKED right and
  `/etc/resolv.conf` corroborated the wrong one. Observe the datapath: a platform-managed component
  you did not install is still the thing enforcing.
- **Widening a lookup table without widening its consumer turns a loud error into a silent success.**
  Kinds added to `kube`'s `plurals` were not inert: `Delete` hardcoded the CNPG apiVersion, so they
  built paths under the wrong group, 404'd, and 404 maps to "already gone" — reporting success while
  the object lived. A kind absent from the consumer must be REFUSED, the key sets asserted EQUAL,
  **and the VALUES pinned**: `networkpolicies`->`networkpolicys` survived equal keys (US-3.3c).
- **Guard every document, element and path — not the first of each.** `yaml.Unmarshal` returns only
  document 1 with a nil error, which has now bitten three times, most recently US-3.3b classifying an
  object's SCOPE from doc 1. Refuse multi-doc, as `kube.applyOne` does. Pinning for one element, then
  0-1, then 0-3 is a constant a mutation can match, and a skip keyed on byte length or `spec:` stayed
  green because every fixture was hand-written — **build fixtures from the real renderers.** And
  `Converge`'s deleting branch returns before the renderer, so a guard in `Render` covered create only:
  `"../../../api/v1/namespaces/kube-system"` was refused on create and ACCEPTED on teardown.
- **A driver reading a key the API's closed schema forbids is silent contract drift.** T3.4c:
  `cnpg.storageForShape` sized the PVC from `shape["storage"]`, which `estimates.shapeSchema`
  rejects, so the priced `storage_gb` was never read — a customer billed for 78 GB got the size
  default, with invoice and audit trail both agreeing. Bind the driver to the catalog with a test
  that READS `pricing.json`; check every path the refusal sits on (the same call served TEARDOWN,
  making the service undeletable); and **prove the wire** — deleting the key en route left the
  agent suite green — US-3.3b hit it twice more, pinning a key and a whole HTTP path on ONE side of a
  two-module wire.
- **A floor HIDES a wrong sign.** `max(declared, included)` is the coherent way to default
  `storage_gb`, but it made the negative-value check unreachable. Reject out-of-range BEFORE
  defaulting.
