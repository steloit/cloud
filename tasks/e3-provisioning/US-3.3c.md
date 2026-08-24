---
id: US-3.3c
title: "D7 isolation does not exist: NetworkPolicy is not enforced on the cell, and the policy set would fence CNPG"
epic: E3
status: done
phase: MVP
priority: critical
sprint: 4
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - infra/modules/gke-cell/**
  - infra/envs/**
  - services/cell-agent/internal/**
  - tasks/e3-provisioning/US-3.3c.md
verify:
  - "the gke-cell module enables NetworkPolicy enforcement (network_policy or ADVANCED_DATAPATH), asserted by a test, not by reading the plan"
  - "a pod in env A cannot open a socket to a pod in env B on a cell with enforcement on"
  - "a managed postgres reaches ready on a cell with enforcement on"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: agent
---

## Goal

Make the environment namespace an actual tenant boundary. Today it is a name.

## Why — US-3.3a's security review

US-3.3a set out to ship D7 (INF-001 §1: "default-deny NetworkPolicies,
ResourceQuota, LimitRange"). It shipped the namespace only, because the review
found each of the other three objects either inert or harmful. **The four
findings are one problem and must land together** — any one alone is a no-op or
an outage.

### 1. Nothing enforces a NetworkPolicy on the cells we build

`infra/modules/gke-cell/main.tf` creates a **GKE Standard** cluster with no
`network_policy { enabled = true }` and no `datapath_provider =
"ADVANCED_DATAPATH"`. Neither Calico nor Dataplane V2 is installed anywhere —
`grep -rn "network_policy\|datapath\|calico\|cilium" infra/` returns one comment
in `infra/spike/us33-e2e.sh` and nothing else.

On GKE Standard that means the API server **accepts and stores** every
NetworkPolicy and **nothing drops a packet**. Every apply returns 200, every
test is green, and the boundary does not exist. This is the failure mode where
the evidence all points the right way and the property is absent.

### 2. The policy set as written fences CNPG off everything it needs

Turning enforcement on with US-3.3a's allow-set would stop the first Postgres
pod reaching ready. Under default-deny egress, a `to:` peer carrying only
pod/namespace selectors never matches a non-pod IP, so all of these are denied:

- **169.254.169.254** — the Cluster sets `googleCredentials: gkeEnvironment: true`
  and the node pools set `workload_metadata_config { mode = "GKE_METADATA" }`.
  No metadata server, no Workload Identity token.
- **storage.googleapis.com:443** — `destinationPath` is `gs://<bucket>`; WAL
  archiving is external egress.
- **the kube-apiserver** — CNPG's in-pod instance manager watches its own Cluster
  CR and does leader election; `kubernetes.default.svc`'s backing IP is not a pod.
- **ingress from cnpg-system** — the operator reaches the instance manager on 8000.

Ordering makes it worse: the policies apply *before* the Cluster in the same
converge, so the pod comes up already fenced.

### 3. (moved to US-3.3d) The LimitRange default silently caps every managed Postgres

`internal/driver/cnpg/templates/cluster.yaml.tmpl` declares **no `resources:`**.
A LimitRange `default: {cpu: 500m, memory: 512Mi}` therefore becomes the hard
limit at pod admission for **every** cluster, existing ones included (SSA
reapplies each converge; it bites on the next restart or failover). A "standard"
Postgres OOMKilled at 512Mi is a product regression arriving through an isolation
task. Unlike the NetworkPolicies this one **is** enforced, which is why US-3.3a
could not ship it either.

The real fix is that the Cluster should declare resources derived from the sold
shape — `estimates` already resolves `memory_mb` — and only then can a LimitRange
default sit underneath it harmlessly.

### 4. (moved to US-3.3e) The quota envelope is a product decision with no owner

`requests.cpu: 8 / limits.cpu: 16 / 16Gi / 32Gi / 16 PVCs / 32 services` are
unsourced constants. `docs/founder-config.md` §5 owns quota knobs and has no
per-environment CPU/memory ceiling. They are also **plan-independent** — a Free
org and a Business org get the identical envelope, which contradicts the tiering
in `plans.json`. A 3-instance CNPG cluster consumes 3 PVCs and 3 Services, so
`persistentvolumeclaims: 16` caps an environment at ~5 HA clusters with no error
surface when it hits.

Both of these were originally bundled here. The architecture review was right
that the bundling is a rationalisation: they were withdrawn because they are
**wrong**, not because they are coupled to Dataplane V2. A LimitRange needs no
CNI, and the quota needs one founder number — gating either behind a cluster
migration is worse than splitting them out. They are **US-3.3d** and **US-3.3e**.

## Acceptance criteria

1. ~~The gke-cell module enables NetworkPolicy enforcement, asserted by a test
   against the module rather than by reading a plan by hand.~~ **TAKEN BY
   US-3.3f** (PR #323, Dataplane V2 + `terraform test` at the module AND both env
   layers). Left here struck through rather than deleted, because the rest of
   this task is written on the assumption that it lands first — this is now a
   dependency, not a deliverable. **There are zero GKE
   clusters in `steloit-dev` today** (`gcloud container clusters list
   --format=json` → `[]`, exit 0, with `projects describe` proving the credential
   works), so `infra/modules/gke-cell` has never been applied — this is the
   cheapest possible moment to fix it, and the only one that avoids recreating a
   cluster's node pools.
2. The default-deny + allow set is restored to `internal/driver/tenancy` **with**
   the four CNPG egress/ingress allowances above, each a separate assertion.
3. The allow policies are asserted **structurally**, not by substring: every
   ingress `from` and egress `to` peer has a non-nil podSelector, no bare `{}`
   peer, no unintended namespaceSelector, no ipBlock beyond the ones named here;
   the DNS rule is one peer with namespaceSelector kube-system AND podSelector
   k8s-app: kube-dns in the SAME peer, ports {UDP53, TCP53}. US-3.3a's tests
   proved the policies EXISTED and that default-deny denied; three mutations that
   widen the allows into a hole (`podSelector:{}`→`namespaceSelector:{}`,
   `egress: [- {}]`, DNS widened to all of kube-system) all survived green.
4. **The agent has RBAC for the cluster-scoped namespace write.** This branch
   made the agent PATCH `/api/v1/namespaces/<name>` on every converge — a
   privilege it never had. There is no ServiceAccount, ClusterRole or Deployment
   artifact anywhere in the repo (`grep -rn "kind: ClusterRole\|kind: ServiceAccount\|serviceAccountName"`
   over `*.yaml|*.tf|*.sh` returns nothing), and `NewInCluster` is the documented
   in-cluster path. `Apply` returns on the first error and the Namespace is first
   in the batch, so **a 403 halts convergence for every service on the cell** —
   including services in namespaces that already exist and were converging fine.
   Grant cluster-scoped `create`/`patch` on `namespaces`, and consider tolerating
   a 403 on the tenancy object specifically so a permissions gap degrades to
   "this environment is not created" rather than "this cell provisions nothing".
5. Two GKE configurations that break the DNS rule silently are pinned or ruled
   out: **Cloud DNS for GKE** (no kube-dns pods; resolution goes to
   169.254.169.254) and **NodeLocal DNSCache** (`k8s-app: node-local-dns`,
   hostNetwork, link-local VIP — a podSelector cannot match it). Neither is
   pinned by terraform today, so the current cell works by accident.
7. `infra/spike/us33-e2e.sh` asserts the boundary rather than describing it: the
   namespace exists with its labels, the policies exist, a pod in env A cannot
   reach a pod in env B, and a pod can still resolve DNS and reach its own
   database.

## Four more obligations, delegated here by ADR-0015

Numbered from 8: **AC 7 already exists** (the `infra/spike/us33-e2e.sh` runbook
assertion), and `US-3.3a.md` cites "US-3.3c AC 7" twice for it — so numbering
these 7-10 made a section written to stop obligations being cited without an
owner break the citation of a different one.

Named because they were being *cited* as owned by this task before they were in
it — ADR-0015 states, in the present tense, that "US-3.3c carries an AC that
`tenancy.Render` must refuse a policy carrying `endPort`". It did not. Now it
does:

8. **`endPort` must be REFUSED, not written.** Dataplane V2 silently does not
   enforce port RANGES on affected versions — the same class of defect ADR-0015
   exists to close: a policy the API server accepts and does not apply. The
   refusal belongs in `tenancy.Render`, where policies are produced, so no caller
   can forget it. A test must show a policy carrying `endPort` is rejected.
9. **Customer code blocked from `169.254.169.254`; managed CNPG allowed.** GKE
   Sandbox's own documentation recommends NetworkPolicy as the control for
   blocking the metadata server from sandboxed pods, while CNPG *requires* it for
   Workload Identity. Different pools, different policies — a design point, not
   one rule, and the asymmetry is the whole content of it.
10. **`infra/k8s/policy/network-logging.yaml` is applied by something**, and a
   denied connection actually appears in logs. It is authored and wired to
   nothing today (recorded in `spec-change-proposals.md`), while denied-connection
   logging is ADR-0015's FIRST stated reason for choosing Dataplane V2 over
   Calico — so that decision's primary rationale currently rests on a capability
   nothing installs.
11. **NetworkLogging's coverage of gVisor pods is confirmed either way.** Google
   documents it neither as supported nor unsupported, and this cell runs customer
   code under gVisor by design.

## Read first

- `services/cell-agent/internal/driver/tenancy/tenancy.go` — the package doc
  records why each object was withheld.
- `infra/modules/gke-cell/main.tf`
- `services/cell-agent/internal/driver/cnpg/templates/cluster.yaml.tmpl`
- **commit `7e94f26`** — the withdrawn NetworkPolicy manifests and their tests
  live there and nowhere else. Start from them; they are wrong in the ways above,
  and rewriting from scratch re-earns the same review findings.

## Outcome

The environment is a network boundary now, and it was **proven against a real
GKE cell** rather than argued from manifests. Full evidence:
`docs/dev/us33c-live-evidence.md`.

### Evidence classes, kept apart

- **Live GKE / runtime enforcement**: cross-environment isolation, Postgres
  reaching ready under the policy set, the metadata-server asymmetry, and
  denied-connection logging. All measured on a cluster provisioned for this and
  destroyed after.
- **Unit / rendered-manifest**: the structural peer assertions, the `endPort`
  refusal, the DNS-peer and CNPG-selector pins.
- **Terraform**: `datapath-policy` wiring validated at both env layers.

None is substituted for another. Where something was NOT verified live it says so.

### What the live run found — three defects, one class

Each is a policy that **looked** correct and did not do what it claimed, which is
the exact failure US-3.3a shipped.

1. **The DNS rule resolved nothing.** It named `k8s-app: kube-dns`, and
   `/etc/resolv.conf` names the kube-dns ClusterIP, so it read as right. NodeLocal
   DNSCache — on by default, **not pinned by our terraform** — answers the query,
   so policy is evaluated against the `node-local-dns` pod and the rule matched
   nothing. **AC 5's predicted failure, confirmed**, with one correction: AC 5
   assumed hostNetwork made it unmatchable; on this version node-local-dns has
   ordinary pod IPs, so a podSelector *can* match it. Both peers now ship.
2. **The CNPG allowances selected a label that does not exist at bootstrap.**
   CNPG bootstraps through a Job whose pod carries `cnpg.io/jobRole` and
   `cnpg.io/cluster` but **not** `cnpg.io/podRole: instance`. The initdb pod
   matched no allowance and the Cluster sat in `Setting up primary` indefinitely.
   Selecting `cnpg.io/cluster` (Exists) covers every stage and is still narrow.
   **This is a FIFTH allowance; the review named four.**
3. **The API server allowance must name the PRIVATE ENDPOINT, not the `kubernetes`
   Service ClusterIP.** An ipBlock for the ClusterIP matches nothing — Dataplane
   V2 evaluates egress post-translation. With `privateEndpoint/32` alone the
   cluster reached healthy in 45s. Same mechanism as (1): one rule, two symptoms.

### ACs

| AC | state |
|---|---|
| 1 (struck) | already US-3.3f — Dataplane V2 confirmed live (`anetd` on every node) |
| 2 | done — policy set restored WITH five CNPG allowances |
| 3 | done — peers asserted structurally (parsed), one named exception → US-3.3j |
| 4 | **NOT DONE** — see below |
| 5 | done — NodeLocal DNSCache confirmed live; Cloud DNS variant still unpinned |
| 7 | done — `us33-e2e.sh` now FAILS on a broken boundary instead of describing it |
| 8 | done — `endPort` refused on the rendered bytes |
| 9 | done — proven BOTH ways on one cluster: gVisor pod blocked, CNPG pod connected |
| 10 | done — `infra/modules/datapath-policy` applies it; denials appear in logs |
| 11 | **answered: YES** — the logged denials have a gVisor pod as source |

### Not done, and why

**AC 4 (agent RBAC) is not in this PR.** There is still no ServiceAccount,
ClusterRole or Deployment artifact for the agent anywhere in the repo, so there
was nothing to grant a permission to and nothing to run in-cluster. The live
policies were applied with `kubectl` from the renderer's own output, which proves
the POLICY and says nothing about the agent's permission to apply it. Shipping a
ClusterRole with no Deployment would be an artifact nothing uses. Filed
separately rather than half-done here.

Also outstanding: **`allow-cnpg-operator-ingress` admits the whole `cnpg-system`
namespace** (US-3.3j). My own structural test caught it; narrowing could not be
*verified* because the cell was already destroyed, and an unverified tightening
of a security control is worse than a named exception — it looks stronger and can
fence the operator off every managed Postgres.

### Side findings

- **`NetworkPolicy` was absent from kube's `plurals`/`apiVersions` maps**, so the
  agent could not have applied or deleted one. Both maps updated together.
- **`cloudresourcemanager.googleapis.com` is not in `project_base`'s service
  list**, yet `gke-cell`'s `google_project_iam_member` needs it — `envs/dev`
  would fail on a genuinely clean project.
- `us-central1-a` was out of `n2-standard-4` during the run (zone stockout, not
  quota). Only the throwaway root's capacity was changed; `dev`/`cell0` untouched.

```
services/cell-agent  go build && go vet && go test -count=1 -race ./...   ok
gofmt -l services/ · terraform fmt -check -recursive infra/               clean
terraform validate (envs/dev, envs/cell0)                                 Success
node scripts/spec-sync/validate.mjs                                       OK: 246 tasks
```
