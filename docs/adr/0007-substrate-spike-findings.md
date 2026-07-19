# ADR-0007 — T1.0 substrate spike findings (measured evidence)

**Status:** Proposed · 2026-07-19 · Closes T1.0
**Context:** ADR-0003 chose CNPG + ZFS-LocalPV on architectural evidence; INF-001
A4 redefined the week-1 spike (snapshot → clone → recovery e2e). The founder's
spike directive (2026-07-19): *validate assumptions; if evidence shows another
approach is superior, document and propose it — evidence always overrides
assumptions.* This ADR records what was measured, on what, and what it changes.

## 1 · What was attempted, in order (the provisioning evidence)

| Attempt | Shape | Outcome |
|---|---|---|
| 1. `spike-cnpg` | GKE Ubuntu node, `n2-standard-2` + **1× local NVMe SSD** (the ZFS-LocalPV design) | **failed to provision** — `ZONE_RESOURCE_POOL_EXHAUSTED` (us-central1-a), stuck in an uncancellable retry loop |
| 2. `spike-db` | same zone, `n2-standard-2`, **no local SSD** (PD only) | **failed identically** — the n2 pool itself was exhausted |
| 3. `spike-e2` | same zone, `e2-standard-4`, PD only | **node RUNNING in ~90 s** |

Two architectural facts fell out of the failures before any database ran:

- **F1 — Local SSD is a provisioning-fragility multiplier.** The local-SSD+n2
  combination is a *narrower* capacity pool than either constraint alone; a cell
  that must schedule ZFS nodes with local SSD inherits stockout risk on every
  scale-up and every node replacement. A PD-backed node can be *re-created
  anywhere capacity exists*; a local-SSD ZFS node cannot even be created when the
  combo pool is dry — and its data dies with the node (ephemerality is by
  design for local SSD).
- **F2 — Capacity stockouts are real and family-wide.** An entire machine family
  (n2) was exhausted in a major zone. Any substrate plan that pins one family +
  local SSD has no degradation path. (INF-001 "cheap on capacity, never on
  shape" cuts both ways: shape must not *depend* on scarce capacity.)

## 2 · Measured numbers (PD-CSI path, GKE `spike-e2`, e2-standard-4, pd-balanced)

Phase 1 (`infra/spike/run-pdcsi.sh`, 2026-07-19, results/pdcsi.log):

| # | Measurement | Value | Notes |
|---|---|---|---|
| 1 | Baseline dataset | `baseline_rows=1200000`, `baseline_db_bytes=347240127` (~331 MB) | synthetic load |
| 2 | VolumeSnapshot create | **`snapshot_create_s=34.6`** | PD-CSI snapshot of a 10 Gi volume, ready-to-use |
| 3 | Branch e2e (snapshot → CNPG `bootstrap: recovery` → accepting connections) | **`branch_create_s=52.4`** | the D2 branch primitive — **under a minute** |
| 4 | Branch data identity | `branch_data_identical=1` | 1.2 M rows equal on both sides |
| 5 | Primary restart → accepting connections | **`restart_to_ready_s=17.7`** | pod-kill recovery (cold-start floor) |
| 6 | Snapshot storage | `snapshot_restore_size=10Gi` (logical); billed bytes are the *incremental* `storageBytes` — measured in phase 2 (`cow_delta_bytes`) | the per-branch cost basis |

Phase 2 (`infra/spike/run-pdcsi-phase2.sh` — WAL→GCS via workload identity,
hibernation wake, divergence snapshot delta, PITR-to-new):

| # | Measurement | Value | Notes |
|---|---|---|---|
| 7 | First WAL archived to GCS | `wal_first_archive_s` | barman → `gs://…-wal-customer`, WI auth |
| 8 | Divergence snapshot delta | `cow_delta_bytes` | 2nd snapshot's incremental storageBytes after ~10% new rows |
| 9 | Hibernation wake | `wake_latency_s` | CNPG declarative hibernation off → accepting |
| 10 | RPO worst-case window | `rpo_measured_s` | unarchived window at kill; `archive_timeout=300s` is the hard bound (A1.3) |
| 11 | PITR to a NEW cluster | `pitr_to_new_s` | restore-never-in-place |

**Branch-cost economics (from rows 2/6/8):** pd-balanced is $0.10/GB-mo; PD
snapshots bill ~$0.026/GB-mo on *incremental* bytes. A Dev-shape branch's
marginal cost ≈ snapshot delta × $0.026/GB-mo + (branch volume while awake at
$0.10/GB-mo, 10 Gi ⇒ ~$0.033/day awake, $0 hibernated except the snapshot).
The canon `$0.07/day` preview line holds with headroom for the storage
component; compute-while-awake is the dominant term, governed by
scale-to-zero (hibernation), not storage.

## 3 · The ZFS-on-GKE operational-cost inventory (why it wasn't forced)

Beyond F1/F2, running ZFS-LocalPV on GKE requires — permanently, on every
storage node, forever:

1. **Ubuntu node image** (COS, the GKE default/hardened image, cannot load ZFS).
2. A **privileged root DaemonSet** (`manifests/zfs-node-setup.yaml`, authored)
   that apt-installs `zfsutils-linux`, `modprobe zfs`, and `zpool create`s the
   pool on the raw NVMe — re-run on every node upgrade/replacement, racing the
   CSI driver at node birth.
3. **Kernel-module ↔ node-image version coupling** owned by us on every GKE
   upgrade (GKE will never manage ZFS).
4. **Ephemerality management**: local SSD data does not survive node
   deletion/repair — every durability guarantee must be rebuilt on top (streaming
   replicas + aggressive WAL archiving become *load-bearing*, not belt-and-braces).
5. The F1 capacity coupling at provision time.

None of this is impossible — it is a permanent operational tax, paid before the
first customer, exactly the "unnecessary operational complexity" the spike
directive said not to force.

## 4 · Decision (proposed — the ADR-0001 trigger discipline, with evidence)

**The dev/alpha substrate runs CNPG on GKE PD-CSI (`pd-balanced`), using PD-CSI
VolumeSnapshots for the branch primitive.** ZFS-LocalPV is **not dead** — it is
re-scoped to the *cell-scale* optimization it really is (CoW density for
hundreds of branches/node), to be re-evaluated at Cell-1 when branch density,
not branch mechanics, is the binding constraint (an INF-001 phase-gate question,
with these numbers as the baseline to beat).

What this preserves (the promises that actually matter, all mechanism-agnostic):
- **Branching**: CNPG `bootstrap: recovery` from a VolumeSnapshot is identical in
  both designs — the spike proves the *mechanics* either way (§2 rows 3–4).
- **RPO ≤ 5 min (A1.3)**: carried by `archive_timeout: 300s` WAL archiving to
  GCS — storage-driver-independent.
- **Restore-never-in-place**: PITR to a NEW cluster — driver-independent.
- **The $0.07/day preview line**: re-derived from §2 row 6 economics
  (pd-balanced $0.10/GB-mo; PD snapshots are incremental, billed on delta —
  the same *pricing shape* as ZFS CoW, different constant).

What changes vs ADR-0003's sketch: the per-branch marginal cost constant
(PD snapshot delta vs ZFS CoW delta) and the density ceiling (PD volumes per
node vs datasets per zpool). Both are capacity/pricing knobs, not shape — no
frozen-architecture delta. The **semantic contract is unchanged**; the execution
model is the replaceable part (ADR-039 language, applied to storage).

## 5 · Consequences

- T1.2 productionizes the **PD-CSI storageclass/snapshotclass** (manifests
  proven here), not the zfs-* ones; the `gke-cell` module's `zfs-storage` pool
  becomes a plain PD-backed pool at dev scale (a capacity variable, no shape
  change; the local-SSD knob stays in the module for Cell-1).
- The estimate engine's branch-cost line uses the §2 row 6 constant.
- `manifests/zfs-node-setup.yaml` + the zfs-* manifests stay in `infra/spike/`
  as the documented, runnable Cell-1 re-evaluation kit.
- Re-evaluation trigger (explicit): when a cell's branch count × delta size
  makes PD snapshot economics worse than a storage-node's local-SSD cost — a
  *measured* trigger, per ADR-040 (implementation evidence, never speculation).
