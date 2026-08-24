# US-3.3c — LIVE GKE evidence (cell-verify, steloit-dev, us-central1-a)

Captured 2026-08-24T19:55:38Z against a real GKE cluster that was
provisioned for this verification and destroyed immediately after.

## Evidence classes — kept distinct, per the task
- **Live GKE / runtime enforcement**: everything in this file.
- Unit, rendered-manifest and terraform-plan evidence live in the PR; none of it
  is substituted for anything below.

## The cell
    cell-verify  RUNNING  1.35.6-gke.1641000  ADVANCED_DATAPATH
    (datapathProvider ADVANCED_DATAPATH = Dataplane V2 — the thing that ENFORCES)

### Enforcement is installed, not just configured
    anetd   4     4
    (anetd = Dataplane V2's Cilium agent, running on every node)

## Policies under test — rendered by tenancy.Render, NOT hand-written
    allow-cnpg-egress
    allow-cnpg-operator-ingress
    allow-dns-egress
    allow-same-namespace
    default-deny-all

## Results
    All three test pods are on the SAME NODE, so a block is policy dropping
    packets, not network topology. Allowed path answers in ~5ms; denied paths
    time out at 8s with curl exit 28 (no route/drop), not exit 7 (refused).

| # | test | expected | measured |
|---|------|----------|----------|
| 1 | env-a pod → env-b pod | blocked | `HTTP 000 in 8.00s, curl exit 28` |
| 2 | env-a pod → env-a pod (control) | allowed | `HTTP 200 in 0.0058s` |
| 3 | env-b pod → env-a pod (reverse) | blocked | `HTTP 000 in 8.00s, curl exit 28` |
| 4 | DNS from env-a | resolves | `kubernetes.default → 34.118.224.1` |
| 5 | gVisor customer pod → 169.254.169.254 | blocked (AC 9) | `blocked` |
| 6 | CNPG pod → 169.254.169.254 | allowed (AC 9) | `TCP CONNECTED` |
| 7 | managed Postgres with enforcement ON | reaches ready | `Cluster in healthy state, readyInstances=1` |
| 8 | denied connection appears in logs (AC 10) | logged | `disposition=deny src=env-a/client dest=env-b/server` ×3 |
| 9 | NetworkLogging covers gVisor pods (AC 11) | answered | **YES** — the logged denials in row 8 have a gVisor pod as source |

Rows 5 and 6 are the same probe, same namespace, same policy set, differing only
by pod selector — which is the whole content of AC 9.

## Three defects only a live run could find

**1. The DNS rule did not work.** `allow-dns-egress` named
`k8s-app: kube-dns`, and `/etc/resolv.conf` names the kube-dns ClusterIP — so it
looked correct. It resolved nothing: NodeLocal DNSCache (on by default, **not**
pinned by our terraform) answers the query, so policy is evaluated against the
`node-local-dns` pod and a rule naming kube-dns matches nothing. Measured:
`;; connection timed out; no servers could be reached`. Fixed by adding a second
peer for `k8s-app: node-local-dns`; both peers stay, because which one serves
depends on a cluster setting we do not pin. **This is AC 5's predicted failure,
confirmed** — with the correction that on this version node-local-dns has
ordinary pod IPs, so a podSelector *can* match it (AC 5 assumed hostNetwork).

**2. The CNPG allowances selected a label that does not exist at bootstrap.**
They selected `cnpg.io/podRole: instance`; CNPG bootstraps through a **Job**
whose pod carries `cnpg.io/jobRole: initdb` and `cnpg.io/cluster` but *not*
`podRole`. So the initdb pod matched no allowance, was fenced by default-deny,
and logged `dial tcp 34.118.224.1:443: i/o timeout` while the Cluster sat in
`Setting up primary` indefinitely. Fixed by selecting `cnpg.io/cluster` (Exists),
which is present at every stage and carried only by operator-managed pods — so
the AC 9 asymmetry is preserved. **This allowance is a fifth one; US-3.3a's
review named four.**

**3. The API server allowance must name the PRIVATE ENDPOINT, not the Service
ClusterIP.** Pods dial `34.118.224.1:443`, so an ipBlock for that ClusterIP looks
obviously right — and matches nothing, because Dataplane V2 evaluates egress
against the post-translation destination. Measured: with the ClusterIP, the same
`i/o timeout`; with `privateEndpoint/32` (10.30.0.2/32) alone, the cluster
reached `Cluster in healthy state` in 45s. The public endpoint is not needed.
Same mechanism as defect 1 — one rule, two symptoms.

## What was NOT verified live
- The agent's own RBAC (AC 4): no agent Deployment/ServiceAccount artifact exists
  in the repo yet, so there was nothing to run in-cluster. The policies here were
  applied with `kubectl` from the rendered output, which proves the POLICY, not
  the agent's permission to apply it.
- `endPort` refusal (AC 8) is a render-time guard; it is unit-tested, not live.
- Cloud DNS for GKE (the other half of AC 5) — this cluster runs kube-dns +
  NodeLocal DNSCache, so the Cloud DNS variant remains unpinned and untested.

## Provisioning and teardown, exactly

Provisioned from a TEMPORARY root consuming the repository's real modules
(`modules/network`, `modules/gke-cell`, `modules/cnpg`) — not a hand-rolled
cluster, and not `infra/envs/dev` wholesale.

`infra/envs/dev` was deliberately NOT used: `module.project_base` creates a KMS
key ring, and **GCP key rings can never be deleted**, plus a Workload Identity
pool with a 30-day soft delete. Neither is needed for these ACs and neither can
be cleaned up. It also pulls in `cost_guardrails`, whose `monthly_budget_units`
is the still-blocked founder decision (O2).

Two deviations from the repo's dev capacity numbers, both in the temporary root
only (`infra/envs/dev` and `cell0` are untouched):
- `storage_machine_type` e2-standard-2 rather than n2-standard-4 — **us-central1-a
  was out of n2-standard-4** (`does not have enough resources available`, a zone
  stockout, not quota). Machine type is an env-layer capacity knob and does not
  affect Dataplane V2 enforcement.
- `module.project_base` omitted, as above.

`cloudresourcemanager.googleapis.com` had to be enabled by hand: it is **not** in
`project_base`'s service list, yet `gke-cell`'s `google_project_iam_member`
requires it — so `envs/dev` would fail on a genuinely clean project. Recorded as
a finding; the API was left enabled (free, and needed by the intended layers).

### Teardown — verified independently, not on Terraform's word

    Destroy complete! Resources: 16 destroyed.

Then checked directly: clusters, instances, disks, routers, addresses and
subnets all empty; only the pre-existing `default` network and the default
compute service account remain. One resource Terraform did NOT own survived the
destroy and was removed by hand: the CNPG PVC's PD (`pvc-7a30fcee…`, 10 GB) —
a dynamically-provisioned disk outlives the cluster that requested it.

**The remote state bucket was not touched.** `gs://steloit-dev-tfstate/` holds
one object, `dev/default.tfstate`, dated 2026-07-26 — a month before this work.
The temporary root used LOCAL state, which was discarded. (An earlier note in
this session called the bucket empty; that reading came from a shell glob error,
not from the bucket.)

## An availability note the live run did not cover

`Render` returns Namespace → quota → limits → `default-deny-all` → the allows,
`Client.Apply` iterates in order and aborts on the first error, and the CNPG
manifests ride the same single `Apply` — so on a NEW environment no pod can exist
in the window where the namespace denies nothing. Fail-closed, correctly.

The **upgrade** path is different and was not exercised: on an environment that
already has a RUNNING Postgres, an apply that lands `default-deny-all` and then
fails before `allow-cnpg-egress` fences the live instance manager off the
apiserver until the next tick succeeds. Fail-closed is still the right default —
the alternative is a window with no boundary — but it is an availability event on
upgrade, not only a create-time property. Stated here rather than fixed, because
the fix (ordering the allows before the deny) trades a security window for an
availability one and is a decision, not a bug fix.
