---
id: US-3.3
title: "Accepted service provisions via the reconciler; metering starts at ready"
epic: E3
status: ready
phase: MVP
priority: critical
sprint: 4
estimate: 1.5ew
deps: [US-1.3, T3.4]
issue: 57
labels: [Platform, Backend]
module: M4 Provisioning
contexts: [provisioning, api-conventions, canon-testing]
files:
  - services/cell-agent/internal/agent/**
  - services/cell-agent/internal/render/**
  - services/cell-agent/internal/kube/**
  - services/cell-agent/cmd/cell-agent/main.go
  - services/api/internal/provisioning/services.go
  - services/api/db/queries/services.sql
  - services/api/internal/reconcile/**
  - infra/**
  - tasks/e3-provisioning/US-3.3.md
apis: []
tables: [services]
events: [service.status_changed]
tests: [TestCNPGRendererAppliesRenderedManifests, TestConvergeObservesClusterStatus, TestDeletingConvergesToGoneAndDeletes, TestNamespaceDerivedFromDesired, TestApplyIsIdempotent, TestConvergeReturnsProvisioningUntilReady, TestMeteringStartsAtReadyE2E]
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go build ./... && go vet ./... && go test ./..."
  - "PHASE B (live cell): an accepted postgres service reaches ready in the env namespace and a metering span opens at ready (manual/live — GCP creds, not CI)"
owner: agent
---

## Goal

An accepted service provisions via the reconciler: the cell-agent renders the
service's desired state to CNPG/K8s manifests, server-side-applies them, observes
the cluster to `ready` (ADR-024), and reports back — and **metering starts at
`ready`, never before** (D10). Control-plane outage never touches a running app.

## Authority

`docs/plan/e1-substrate-design.md` §2 step 2 ("render manifests from desired,
server-side-apply, never diff-by-memory"). This wires the **real Renderer** that
US-1.3 and T3.4 both deferred as "the apply seam / a later live-integration
task" — T3.4 renders, US-1.3 runs the loop, this connects them to a real cluster.

## Two phases — Phase A is ~$0, Phase B needs a live cell

**Phase A — the apply-seam Renderer (no cluster, deterministic).**
- `render.CNPGRenderer` (a `Renderer`, replacing `AckRenderer`): maps the agent's
  `DesiredService` → `driver.Spec`, renders via the T3.4 CNPG driver,
  server-side-applies via a `kube.Applier` seam, observes the CNPG cluster's
  status → the reconciler vocabulary (provisioning until healthy, then ready),
  and returns it. A deleting service applies teardown and converges to `gone`.
- `kube.Applier` interface: `Apply(ctx, namespace, manifests)` +
  `Observe(ctx, namespace, name) → status`. A **fake** applier (records applied
  objects, returns a scripted status) makes convergence provable without a cluster.
- **Namespace + placement in the desired doc:** the control plane resolves the
  env namespace (`proj--env`) + GSA + WAL bucket and carries them in `desired`
  (extend US-1.3a's `desiredDoc` + the reconcile poll's `DesiredService`), so the
  agent renders with real placement. The agent's `DesiredService` decodes them.
- Metering-at-ready is already control-plane-side (`provisioning.Transition` +
  `metering.BillingEdge`, US-1.3): the writeback drives the edge, so a real
  cluster reaching ready opens the span exactly once — Phase A pins this through
  the reconciler with a fake applier returning ready.

**Phase B — the live cell (GCP free tier, gated on gcloud re-auth).**
- The real `kube.Applier` (k8s server-side apply via the API; observe CNPG
  `.status`). Stand up the zonal GKE cluster + CNPG operator (T1.1/T1.2 terraform)
  and run the agent against it with a reconciler token + workload identity.
- E2e: accept a postgres service → desired row → agent polls → applies the CNPG
  manifest in `proj--env` → cluster reaches ready → agent reports ready →
  metering span opens. This is US-3.3's headline AC; it is a live/manual
  verification (needs GCP creds; not CI while Actions is quota-blocked).

## Acceptance criteria

- [ ] **(Phase A)** `CNPGRenderer.Converge` renders via the T3.4 driver, applies
  the manifests through the `kube.Applier` seam, and returns the observed status;
  proven against a fake applier (`TestCNPGRendererAppliesRenderedManifests`,
  `TestConvergeObservesClusterStatus`).
- [ ] **(Phase A)** A deleting service applies teardown and converges to `gone`
  (`TestDeletingConvergesToGoneAndDeletes`).
- [ ] **(Phase A)** The env namespace is derived from desired placement, not
  guessed (`TestNamespaceDerivedFromDesired`); apply is idempotent
  (`TestApplyIsIdempotent`).
- [ ] **(Phase A)** Through the reconciler with a fake applier: a service walks
  provisioning → ready and the metering span opens at ready, never before
  (`TestMeteringStartsAtReadyE2E`, real Postgres).
- [ ] **(Phase B, live)** An accepted postgres service reaches `ready` in its env
  namespace on a real GKE+CNPG cell, and usage events flow. Live/manual evidence
  pasted (not CI).

## Out of scope

Branch/PITR orchestration (control-plane metadata/routing — later); the reconciler
protocol itself (US-1.3); the CNPG manifest shapes (T3.4). Duty-cycle scheduling
(T1.6).

## Read first

`docs/plan/e1-substrate-design.md` §2 · `services/cell-agent/internal/agent/renderer.go`
(the AckRenderer seam) · `services/cell-agent/internal/driver/` (T3.4) ·
`services/api/internal/provisioning/services.go` (Transition + metering edge) ·
`contexts/provisioning.md`

## Progress — Phase A landed (apply-seam Renderer), Phase B blocked on gcloud re-auth

**Done (this branch, ~$0, no cluster):**
- `internal/kube/Applier` — the server-side-apply seam (Apply/Observe/Delete).
- `internal/render/CNPGRenderer` — the real Renderer: `DesiredService` →
  `driver.Spec` → T3.4 render → `Applier.Apply` → `Applier.Observe` →
  status-from-CNPG-phase (ADR-024: provisioning until "Cluster in healthy state",
  then ready; failure phases → failed). Deleting → teardown + gone. Namespace is
  read from `desired.placement`, never guessed. 8 tests under `-race` (apply,
  observe→status, deleting, no-namespace error, idempotent, apply-error surfaces).

**Remaining (Phase B — needs the founder's `gcloud auth login`):**
1. **api-side placement:** extend `desiredDoc` (US-1.3a) so the control plane
   resolves the env namespace (`proj--env`) + GSA + WAL bucket into
   `desired.placement`, and the reconcile poll + agent `DesiredService` carry it.
   (~$0, api-side; needed so a real agent gets placement.)
2. **real `kube.Applier`:** k8s server-side apply via the API + observe CNPG
   `.status.phase`. Wire `main.go` to select `CNPGRenderer` when a kube config is
   present, else `AckRenderer`.
3. **live cell:** stand up the zonal GKE + CNPG operator (T1.1/T1.2 terraform),
   run the agent in-cluster with a reconciler token + workload identity.
4. **e2e (headline AC):** accepted postgres service → desired row → agent applies
   the CNPG manifest in `proj--env` → cluster reaches ready → agent reports ready
   → metering span opens. Live/manual evidence (GCP creds; not CI while Actions is
   quota-blocked).

**Blocker:** `gcloud` token for `hashir@humanetechnologies.in` expired; re-auth is
interactive. Phase A needs none of this; Phase B (steps 3-4) needs the cluster.
Step 1-2 are ~$0 and can proceed before the cluster.
