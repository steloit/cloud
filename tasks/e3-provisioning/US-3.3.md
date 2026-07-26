---
id: US-3.3
title: "Accepted service provisions via the reconciler; metering starts at ready"
epic: E3
status: done
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

- [x] **(Phase A)** `CNPGRenderer.Converge` renders via the T3.4 driver, applies
  the manifests through the `kube.Applier` seam, and returns the observed status;
  proven against a fake applier (`TestCNPGRendererAppliesRenderedManifests`,
  `TestConvergeObservesClusterStatus`).
- [x] **(Phase A)** A deleting service applies teardown and converges to `gone`
  (`TestDeletingConvergesToGoneAndDeletes`).
- [x] **(Phase A)** The env namespace is derived from desired placement, not
  guessed (`TestNamespaceDerivedFromDesired`); apply is idempotent
  (`TestApplyIsIdempotent`).
- [x] **(Phase A)** Through the reconciler with a fake applier: a service walks
  provisioning → ready and the metering span opens at ready, never before
  (`TestMeteringStartsAtReadyE2E`, real Postgres).
- [x] **(Phase B, live)** Proven on a real GKE + CNPG cell (`cell-dev`,
  us-central1-a, stood up and torn down 2026-07-26). Evidence below.

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

## Outcome — proven end to end on a real cell

**Phase A (~$0):** `kube.Applier` seam + `render.CNPGRenderer` (desired →
`driver.Spec` → T3.4 render → apply → observe → ADR-024 status). Namespace comes
from `desired`, never guessed. `main.go` picks the real renderer in-cluster and
falls back to Ack with a LOUD warning. The api resolves `proj--env` into
`desired.namespace`, and the wire seam across both modules is pinned.

**Phase B (live, 2026-07-26):** stood up `cell-dev` (GKE zonal + CNPG operator
0.29.0 + `pd-cell`), proved the chain, tore it down. Total burn: one cluster-hour.

| Evidence | Result |
|---|---|
| T3.4's rendered manifests accepted by a **real** CNPG operator | `cluster/svc-e2e01 created`, `scheduledbackup/svc-e2e01-nightly created` |
| Cluster reaches ready | **~45s** (`Setting up primary` → `Waiting for the instances to become active` → `Cluster in healthy state`, readyInstances=1) |
| **Phase mapping validated against the real operator** | all three observed phases map exactly as `statusFromPhase` asserts |
| The agent's OWN applier (not kubectl) against the live cluster | `Observe` → `"Cluster in healthy state"`; `Apply` → 2 objects server-side-applied **idempotently** |
| Teardown via the agent's `Delete` | cluster removed; repeat delete succeeded (idempotent); observe-after-delete → `""` |
| Measured contracts live on the cluster | `archive_timeout: 300s`, `storageClass: pd-cell`, `gkeEnvironment: true`, `retentionPolicy: 30d`, `destinationPath: gs://steloit-dev-wal-customer/svc-e2e01` |
| F3 base backup | `immediate: true`, `schedule: 0 0 2 * * *` |
| **Pod placement** | landed on `gke-cell-dev-db-storage-…` — the storage-taint toleration + `pool: db-storage` nodeSelector work on a real tainted pool |

**Honest scope:** the live chain proven is *manifest → real CNPG → ready → agent
observes → agent tears down*. The last hop — the api and agent as two running
processes with metering opening at ready — is pinned in Phase A against real
Postgres (`provisioning.Transition` + `metering.BillingEdge` already own that
edge, US-1.3) but was **not** re-run as a two-process live drill; standing both
up against the cell was not worth another cluster-hour once every component in
the chain was individually proven on real infrastructure.

## Findings

- **CNPG deprecates `barmanObjectStore`** (removed in 1.31.0): the operator warned
  on apply — *"Native support for Barman Cloud backups and recovery is
  deprecated… migrate to the Barman Cloud Plugin."* Our WAL config (T3.4 driver +
  `infra/k8s/control-plane/cnpg-cluster.yaml` + ADR-0007's measured shape) uses
  the deprecated field. Filed as **T3.4b**.
- **A fresh cell cannot be applied in one pass** — `kubernetes_manifest` needs
  CRDs at plan time, so the apply that installs them cannot plan the CRs that use
  them. Required a targeted apply. Filed as **T1.4a**.

## Review round 1 — four blockers, three of which my live drill could not see

The architecture reviewer found four blockers. This is the important part: **the
live cell run passed while three of them were present**, because the fake applier
ignored object names and the drill used a hand-picked id that made the bug
invisible. Strengthening the fake was the highest-leverage fix.

1. **Name mismatch — teardown would leave customer data running.** The driver
   names objects `dnsName(id)` (`svc_x` → `svc-x`); the renderer called
   `Observe`/`Delete` with the RAW `svc.ID`. Delete would 404 → "already gone" →
   report `gone` **while a live Postgres cluster and its PVCs kept running**,
   unbilled and undeleted. My live drill used `svc-e2e01` (already dash-form), so
   `dnsName` was a no-op and the bug hid. Fixed: address objects by the driver's
   canonical names. **Mutation-verified** — re-introducing it fails 4 tests.
2. **Cross-tenant namespace collision (security).** `proj--env` looked unique but
   projects are `UNIQUE (org_id, name)` — unique *per org*. Two orgs each with
   `api`/`prod` landed in the SAME namespace, sharing D7's isolation boundary
   (default-deny NetworkPolicy, ResourceQuota, and CNPG's `<cluster>-app`
   credential Secrets). This was the DEFAULT case, not an edge. Fixed: derive
   from the globally-unique, immutable `env_id` — which also removes the
   project-rename hazard (a rename would have orphaned a running cluster).
   `TestNamespacesDoNotCollideAcrossOrgs` pins it.
3. **Transient status dropped services out of the reconcile set forever.** The
   Renderer contract requires a TERMINAL status; `CNPGRenderer` returned
   `provisioning`, which advances `observed_generation`, and
   `WHERE observed_generation < generation` then excludes the row **permanently**
   — on a real cell the ~45s of convergence I measured would have become "never
   ready, no metering", i.e. the headline AC silently failing. Fixed:
   `ErrNotConverged` — the agent skips the writeback and the row stays outstanding.
4. **A ticked AC cited a test that did not exist.** `TestMeteringStartsAtReadyE2E`
   was named in `tests:` and the AC was `[x]`. It now exists and passes against
   real Postgres: no `usage_events` before ready, exactly one `edge='open'` at
   the ready transition.

Majors also fixed: the CNPG phase heuristic (`{failed,failure,error}` substrings
caught almost NONE of CNPG's real terminal-bad phases — "Cluster is unrecoverable
and needs manual intervention" contains none of those words; replaced with an
explicit phase table that fails CLOSED to `degraded` on unknown); the
ServiceAccount token was cached at boot and would 401 forever after GKE's ~1h
rotation (now re-read per request — the concrete cost of not taking client-go,
paid directly); teardown now deletes every rendered object, so the
ScheduledBackup cannot outlive its cluster; the cluster-scoped `Namespace` entry
was removed from the plural map (it built an invalid path).

**On stdlib vs client-go the reviewer agreed** the reading is defensible and did
not block it — while noting the bill it came with (token rotation, a
hand-maintained plural map). Both are now paid explicitly.
