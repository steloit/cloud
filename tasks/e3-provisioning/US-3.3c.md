---
id: US-3.3c
title: "D7 isolation does not exist: NetworkPolicy is not enforced on the cell, and the policy set would fence CNPG"
epic: E3
status: ready
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
  - docs/founder-config.md
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

### 3. The LimitRange default silently caps every managed Postgres

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

### 4. The quota envelope is a product decision with no owner

`requests.cpu: 8 / limits.cpu: 16 / 16Gi / 32Gi / 16 PVCs / 32 services` are
unsourced constants. `docs/founder-config.md` §5 owns quota knobs and has no
per-environment CPU/memory ceiling. They are also **plan-independent** — a Free
org and a Business org get the identical envelope, which contradicts the tiering
in `plans.json`. A 3-instance CNPG cluster consumes 3 PVCs and 3 Services, so
`persistentvolumeclaims: 16` caps an environment at ~5 HA clusters with no error
surface when it hits.

**NEEDS FOUNDER INPUT** — the per-plan environment envelope. Read it from the
same source `plans.json` owns; do not retype constants into a driver.

## Acceptance criteria

1. The gke-cell module enables NetworkPolicy enforcement, asserted by a test
   against the module rather than by reading a plan by hand.
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
4. The Cluster declares resources from the sold shape, and only then does a
   LimitRange default go back — with a test that a managed postgres of each
   catalog shape is not capped below its shape.
5. The quota envelope comes from the founder-owned source, per plan.
6. Two GKE configurations that break the DNS rule silently are pinned or ruled
   out: **Cloud DNS for GKE** (no kube-dns pods; resolution goes to
   169.254.169.254) and **NodeLocal DNSCache** (`k8s-app: node-local-dns`,
   hostNetwork, link-local VIP — a podSelector cannot match it). Neither is
   pinned by terraform today, so the current cell works by accident.
7. `infra/spike/us33-e2e.sh` asserts the boundary rather than describing it: the
   namespace exists with its labels, the policies exist, a pod in env A cannot
   reach a pod in env B, and a pod can still resolve DNS and reach its own
   database.

## Read first

- `services/cell-agent/internal/driver/tenancy/tenancy.go` — the package doc
  records why each object was withheld.
- `infra/modules/gke-cell/main.tf`
- `services/cell-agent/internal/driver/cnpg/templates/cluster.yaml.tmpl`
- `docs/founder-config.md` §5
