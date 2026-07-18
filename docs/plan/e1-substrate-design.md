# E1 Substrate Design (SP0-1) — Terraform layout · reconciler protocol · T1.0 spike runbook

**Status:** Sprint-0 design doc (SP0-1) · 2026-07-18 · Consumed by Sprint-1 tasks (T1.1–T1.6, US-1.x); T1.x enrichment references these sections. Authority: INF-001 (D1–D11, A1–A5), architecture.md §3/§12/§14/§15, ADR-0003. Nothing here changes frozen shape — this fills in the layout those documents left open.

## 1 · Terraform module layout (two GCP projects — T1.1)

**Projects** (architecture §14): `steloit-dev` (founder dev, us-central1, destroyable, duty-cycled per A1.6) and `steloit-cell0` (partner-facing, born asia-south1 per A1.7). One Terraform state per project, GCS backend in that project, locked via native state locking. Workload identity everywhere; zero static keys (D5).

```
infra/
  modules/
    project-base/     # APIs, GCS state+artifact buckets, KMS keyring, workload-identity pool
    network/          # VPC (one per project), subnets, Cloud NAT, LB + certs; content eTLD+1 zone (A2.4)
    gke-cell/         # zonal GKE Standard: core pool (floor 1, A1.6) · zfs-storage pool (local SSD,
                      #   OpenEBS ZFS-LocalPV) · workload pool (gVisor, scale-to-zero)
    cnpg/             # CNPG operator install + control-plane cluster (single instance, PITR → own bucket, inv. 10)
    observability/    # OTel collector (tenancy stamping, D7) + Prometheus/Loki/Tempo/Grafana single-replica (§8)
    duty-cycle/       # dev-only: scheduled scale-to-zero/restore of the dev cell (T1.6)
  envs/
    dev/              # composes all modules, small shapes, duty-cycled
    cell0/            # asia-south1; identical shape ("cheap on capacity, never on shape"), no duty-cycle
```

Rules: envs compose modules and set only size/region/count variables — **shape lives in modules, capacity in envs** (INF-001's principle expressed in code). Every resource carries `cell_id` labels from birth (US-1.1, invariant 1). CI plan/apply via GitHub Actions with workload-identity federation; `terraform plan` on PR, apply on main by the founder.

## 2 · Reconciler protocol (D9/A2.5 — T1.2/US-1.3, cell-agent contract)

**Truth model:** control-plane Postgres holds **desired** state; cells are **actual**; DR = restore desired, reconcile actual (A2.5). No imperative provisioning ever — a request handler writes rows; the cell-agent converges (a job that provisions imperatively is a defect).

**Tables (models.md additions ride T1.2's migration):** `services` gains `desired jsonb, generation bigint, observed_generation bigint, last_reconciled_at, cell_id`; `cells(id, region, status, agent_last_seen_at)`. Desired = product + shape + intent + lifecycle flags; actual is never stored authoritatively — it is observed.

**Agent loop (cell-agent, Go, controller-runtime patterns; speaks `/v1` like every client — §9/§15):**
1. `GET /v1/reconcile/{cell}/desired?since_generation=` (reconciler-scoped token; long-poll/SSE later — poll at alpha).
2. Level-triggered converge per service: render CNPG/K8s manifests from desired, server-side-apply, never diff-by-memory.
3. Status writeback: `POST /v1/reconcile/{cell}/status` with `{service_id, observed_generation, status, conditions[], event}` — the API writes `services.status` (the six-mark vocabulary, metering starts at `ready`, D10 events emitted on every edge).
4. Heartbeat rides the status call; `agent_last_seen_at` staleness alerts (O4).
Idempotency: converge is safe to repeat; deletes converge to absence then report `deleting→gone`. The reconciler endpoints enter openapi.yaml via S-process before T1.2 implements (internal-plane tag, same contract discipline — "no internal-only protocols").

## 3 · T1.0 spike runbook (executes the moment P1 lands — GCP account is the gate)

**Goal:** branch-cost numbers for the estimate engine + the $0.07/day check; de-risk CNPG+ZFS mechanics (ADR-0003).

| Step | Command sketch | Measurement |
|---|---|---|
| 1 | `envs/dev` apply: 1 zonal GKE + 1 zfs-pool node (local SSD) + CNPG operator | — |
| 2 | Create CNPG cluster `spike-a` (Dev shape, 4 GB); `pgbench -i -s 50` (~750 MB) | baseline size |
| 3 | `VolumeSnapshot` of spike-a's PVC (ZFS CoW) | snapshot create time (target: seconds) |
| 4 | New CNPG cluster `spike-b` via `bootstrap: recovery` from snapshot | branch e2e time (snapshot→accepting connections) |
| 5 | Write 10%-scale divergent data to both; `zfs list -o used,refer` | CoW delta bytes (→ per-branch $/GB-mo) |
| 6 | CNPG declarative hibernation on both; wake spike-b via annotation | wake latency (cold-connect target for the gateway) |
| 7 | WAL archiving → GCS, `archive_timeout: 300s`; kill a pod mid-write | measured RPO ≤ 5 min (A1.3) |
| 8 | PITR to a NEW cluster at T-10min | PITR-to-new-branch time; verifies restore-never-in-place |
| 9 | Teardown; record all numbers | findings ADR (docs/adr/) closes T1.0 |

**Evidence contract:** every measurement lands in the findings ADR with the exact commands; the branch-delta and wake numbers feed T3.1's pricing table and validate canon's `$0.07/day` preview line. If any number breaks a promise (RPO > 5 min, wake > acceptable cold-start), that is *implementation evidence* — the ADR-0001 trigger discipline applies, never silent adjustment.

**Review note (SP0-1 AC):** solo-founder adaptation — "both engineers" = founder review of this doc + the agent's cross-check against INF-001/architecture (done in authoring). Sprint-1 enrichment must cite §1 for T1.1, §2 for T1.2/US-1.3, §3 for T1.0.
