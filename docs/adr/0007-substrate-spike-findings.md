# ADR-0007 — T1.0 substrate spike findings (measured evidence)

**Status:** Proposed · 2026-07-19 · Closes T1.0 · **proposes Architecture v1.3 (supersedes §3's storage sentences) + INF-001 Amendment A6 (founder ratification required — 00-sources is human-only)**
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
- **F2 — Capacity stockouts can span a whole machine family.** The n2 family was
  exhausted in us-central1-a during the spike window (a **single-zone,
  single-day observation** — committed transcript:
  `infra/spike/results/provisioning-failures.log`; both node names show repeated
  `ZONE_RESOURCE_POOL_EXHAUSTED` inserts). Any substrate plan that pins one
  family + local SSD has no degradation path when this happens. (INF-001 "cheap
  on capacity, never on shape" cuts both ways: shape must not *depend* on scarce
  capacity.) **Honesty note:** the working fallback (attempt 3) used
  `e2-standard-4`, and the e2 family cannot attach local SSDs at all — so the
  fallback that unblocked the spike also *foreclosed* measuring the ZFS path in
  this window. The ZFS numbers remain unmeasured, not disproven; what §1 proves
  is the provisioning-fragility coupling, and what §2 proves is that PD-CSI
  meets every promised capability without them.

## 2 · Measured numbers (PD-CSI path, GKE `spike-e2`, e2-standard-4, pd-balanced)

Phase 1 (`infra/spike/run-pdcsi.sh`, 2026-07-19, results/pdcsi.log):

| # | Measurement | Value | Notes |
|---|---|---|---|
| 1 | Baseline dataset | `baseline_rows=1200000`, `baseline_db_bytes=347240127` (~331 MB) | synthetic load |
| 2 | VolumeSnapshot create | **`snapshot_create_s=34.6`** | PD-CSI snapshot of a 10 Gi volume, ready-to-use |
| 3 | Branch e2e (snapshot → CNPG `bootstrap: recovery` → accepting connections) | **`branch_create_s=52.4`** | the D2 branch primitive — **under a minute** |
| 4 | Branch data identity | `branch_data_identical=1` | 1.2 M rows equal on both sides |
| 5 | Primary restart → accepting connections | **`restart_to_ready_s=17.7`** | pod-kill recovery (cold-start floor) |
| 6 | Snapshot storage | `snapshot_restore_size=10Gi` (LOGICAL volume size — the log's `unit=bytes` is a mislabel); billed bytes are the *incremental* `storageBytes`, measured in phase 2 (`cow_delta_bytes`) | the per-branch cost basis |

Phase 2 (`infra/spike/run-pdcsi-phase2.sh` — WAL→GCS via workload identity,
hibernation wake, divergence snapshot delta, PITR-to-new; results/pdcsi-phase2.log):

| # | Measurement | Value | Notes |
|---|---|---|---|
| 7 | WAL archiving to GCS | **working** (`ContinuousArchiving=True`; base+WALs listed in the bucket) | barman → `gs://…-wal-customer`, WI auth. The run's "no WAL in 300s" WARN was a **false negative** (glob missed barman's nested `serverName/` dir); verified post-hoc in-session, and the fixed glob + condition/bucket listing now land in the log on every run (see results/PROVENANCE.md) |
| 8 | Divergence snapshot delta | **`cow_delta_bytes=64855872`** (~62 MB) | 2nd snapshot's *incremental* storageBytes after ~10% new rows (~33 MB data + WAL/checkpoint overhead) — PD snapshots bill deltas, confirmed |
| 9 | Hibernation wake | **`wake_latency_s=8.0`** | CNPG declarative hibernation off → accepting connections |
| 10 | RPO | **bound ≤ 300 s by construction** (`archive_timeout`); the run's `rpo_measured_s=2` was the *unarchived window* at a hypothetical failure instant | **a bound, not a kill test** — no destructive mid-write kill was executed (runbook step 7 deviation); the destructive loss measurement is deferred to T1.2's harness. A1.3 consumes the 300 s bound |
| 11 | Restore to a NEW cluster (base backup + full WAL replay) | **`pitr_to_new_s=55.2`** | post-backup marker + all 1,320,000 rows verified restored; restore-never-in-place ✓. First produced by in-session commands after the scripted `targetTime` attempt hung 903.8 s (F4); the producing path (Backup manifest + restore-to-latest template + rewritten step E) is **now committed**, and provenance is recorded (results/PROVENANCE.md) |

**Branch-cost economics (from rows 2/6/8):** pd-balanced is $0.10/GB-mo; PD
snapshots bill ~$0.026/GB-mo on *incremental* bytes — row 8 measured a Dev-shape
10%-divergence delta at ~62 MB ⇒ **~$0.0016/mo of snapshot storage per such
branch**. A branch's dominant cost is its awake volume (10 Gi ⇒ ~$0.033/day) and
compute — governed by hibernation (row 9: 8 s wake), not storage. The canon
`$0.07/day` preview line holds with wide headroom on the storage component.

### 2b · Operational findings that must shape T1.2 (each earned the hard way)

- **F3 — WAL archiving alone is NOT restorable.** A recovery bootstrap needs a
  **base backup** in the object store; with WALs only, restore fails
  (`no target backup found`). A scheduled base backup (`Backup`/ScheduledBackup)
  is **load-bearing from day one**, not an optimization. (An on-demand base
  backup of the 331 MB cluster took seconds.)
- **F4 — `targetTime` PITR has a sharp edge.** A target beyond the end of
  archived WAL leaves recovery looping/waiting for future segments (observed
  live). The reconciler must compute recovery targets from the **last archived
  WAL position**, never wall-clock "now". Restore-to-latest is deterministic
  (55.2 s); targeted PITR needs target validation first.
- **F5 — Workload identity is a hard prerequisite** for barman→GCS: default GKE
  node scopes are storage-read-only; the spike cluster needed WI retrofitted
  (~15 min of cluster+nodepool updates). T1.1's Terraform already provisions WI
  — never create a cluster without it.
- **F6 — CNPG in-tree Barman is deprecated, removed in 1.31.** The 1.30 pin
  (architecture §3) is now a **hard ceiling**: staying current past 1.30 requires
  migrating to the Barman Cloud Plugin. A version-knob note becomes a tracked
  migration item for T1.2.

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

## 4 · Decision (proposed — a FORMAL architecture delta, per the ADR-0003 precedent)

**The dev/alpha substrate runs CNPG on GKE PD-CSI (`pd-balanced`), using PD-CSI
VolumeSnapshots for the branch primitive.** ZFS-LocalPV is **not dead** — it is
re-scoped to the *cell-scale* optimization it really is (CoW density for
hundreds of branches/node), to be re-evaluated at Cell-1 when branch density,
not branch mechanics, is the binding constraint (an INF-001 phase-gate question,
with these numbers as the baseline to beat).

**This IS a frozen-architecture delta and is proposed as one** (review finding
C1 — an earlier draft wrongly claimed otherwise): `architecture.md` §3 (v1.1)
and INF-001 A4 both name **OpenEBS ZFS-LocalPV / ZFS copy-on-write** as the
branching storage. Ratifying this ADR therefore requires, exactly as ADR-0003
did:

1. **INF-001 Amendment A6** (founder ratification required; proposed text below).
2. **architecture.md §3 rewrite to v1.3**: "OpenEBS ZFS-LocalPV storage node
   pool" → "GKE PD-CSI (`pd-balanced`) volumes at dev/alpha; ZFS-LocalPV
   retained as the Cell-1 density option (A6)"; "ZFS copy-on-write" →
   "CSI VolumeSnapshot (PD incremental, delta-priced)". The CNPG ≤1.30 pin
   sentence gains: "in-tree Barman is REMOVED in 1.31 — the pin is a hard
   ceiling; the barman-cloud plugin migration is a tracked T1.2 item (F6)."
3. Product ADR-log entry via the S-process (decisions.md is human-only).

### Proposed INF-001 Amendment A6 (for founder to apply — 00-sources is human-only)

> **A6 (2026-07-19, ADR-0007).** A4 is amended on measured evidence (T1.0 spike,
> ADR-0007): at dev/alpha scale the CNPG branching substrate runs on **GKE
> PD-CSI (`pd-balanced`) volumes with PD incremental VolumeSnapshots**, not
> OpenEBS ZFS-LocalPV. The designed ZFS node failed to provision
> (`ZONE_RESOURCE_POOL_EXHAUSTED`, local-SSD+n2 coupling; transcript committed)
> and carries a permanent operational tax (Ubuntu-only nodes, privileged zpool
> bootstrap, kernel/image coupling, local-SSD ephemerality). Every A4 promise
> was met on PD-CSI with measured numbers: branch e2e 52.4 s, hibernation wake
> 8.0 s, restore-to-new 55.2 s, RPO ≤ 300 s by construction, delta-priced
> incremental snapshots (~62 MB per 10%-divergence Dev branch). **ZFS-LocalPV is
> re-scoped, not rejected**: it is the Cell-1 branch-density optimization, with
> an explicit measured trigger (branch count × delta economics vs a local-SSD
> storage node) and the spike kit retained as the runnable re-evaluation
> harness. D3's branching requirement, D8 grammar isolation, and A1.3's RPO
> bound are unchanged.

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

## Verdict

**CNPG branching mechanics: proven.** Snapshot 34.6 s → branch accepting
connections in 52.4 s, byte-identical data; hibernation wake 8.0 s; restore to a
new cluster in 55.2 s with full WAL replay verified; RPO bounded ≤ 300 s **by
construction** (A1.3 holds; a destructive kill test is a T1.2 item). **Substrate
for dev/alpha: PD-CSI, not ZFS-LocalPV** — the ZFS path failed at the
provisioning layer before a single database ran (local-SSD capacity coupling,
F1/F2; the ZFS numbers are unmeasured, not disproven) and carries a permanent
operational tax (§3); every promised capability was delivered by the managed
driver. ZFS-LocalPV is re-scoped to a Cell-1 density optimization with an
explicit measured trigger. **This is a formal Architecture v1.3 delta + INF-001
Amendment A6 — founder ratification required** (§4); until ratified,
architecture.md §3 stands as written and T1.2 must not productionize either
storage path. All measurement provenance is recorded
(results/PROVENANCE.md); all temporary resources destroyed and verified by the
committed idempotent teardown (results/teardown.log: 0 clusters, 0 VMs, 0
disks, 0 snapshots). Spike total cost: well under $1 of free credits.
