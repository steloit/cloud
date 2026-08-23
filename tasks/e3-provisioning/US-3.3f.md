---
id: US-3.3f
title: "Nothing on the cell enforces a NetworkPolicy — D7's boundary is stored and ignored"
epic: E3
status: done
phase: MVP
priority: critical
sprint: 4
issue: 0
labels: [Platform, Security, Infrastructure]
module: M4 Provisioning
contexts: [provisioning]
files:
  - infra/modules/gke-cell/**
  - infra/k8s/policy/**
  - .github/workflows/ci.yml
  - tasks/e3-provisioning/US-3.3f.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/infra/modules/gke-cell\" && terraform init -backend=false && terraform test"
  - "terraform fmt -check -recursive infra"
owner: agent
---

## The defect

`infra/modules/gke-cell` creates a **GKE Standard** cluster with no network
policy provider at all. On such a cluster the API server **accepts and stores**
every NetworkPolicy object and **nothing drops a packet**.

US-3.3a discovered this the expensive way: it rendered D7's default-deny
boundary, proved every manifest correct, and had to withdraw the whole set on
finding there was nothing to enforce it. Rendered, stored and enforced are three
different things, and a green suite covered the first two.

## The decision — Dataplane V2, and it must happen before any cell exists

`datapath_provider = "ADVANCED_DATAPATH"` (GKE Dataplane V2, eBPF/Cilium).

| option | enforcement | migration | chosen |
|---|---|---|---|
| nothing (today) | **none** — policies stored, ignored | — | |
| `network_policy` (Calico) | yes | can be enabled later; recreates nodes | no |
| `ADVANCED_DATAPATH` | **built in** | **no documented path for an existing Standard cluster** | **yes** |

Google's docs: *"GKE Dataplane V2 comes with Kubernetes network policy
enforcement built-in. This means that you don't need to enable network policy."*
The two are mutually exclusive — setting both fails with *"Enabling NetworkPolicy
for clusters with DatapathProvider=ADVANCED_DATAPATH is not allowed."*

**Timing is the whole argument.** Google documents no migration path for an
existing Standard cluster, so this is a create-time decision: free today, a full
cluster-and-node-pool rebuild at any later point. There are currently **zero
clusters** in the project (`gcloud container clusters list --format=json` → `[]`),
which is why this lands ahead of the policies it enables rather than with them.

Dataplane V2 caveats accepted, from Google's own known-issues list: NetworkPolicy
`endPort` (port RANGES) is silently not enforced on affected versions — read a
policy back and check the field survived before relying on a range; hairpin
connections can drop; hostPort conflicts with the NodePort range. None affect the
D7 set, which uses single ports and no hostPort.

## Enforcement has to be observable, or it is not operable

`infra/k8s/policy/network-logging.yaml` turns on logging for **denied**
connections and leaves **allowed** off. A denied connection is otherwise
indistinguishable from an application bug — the customer says "my service can't
reach X" and nothing in our logs says a policy dropped it. Logging every allowed
packet between every pod is a bill and a haystack, not a signal.

It doubles as a continuous liveness check on enforcement itself: a default-deny
namespace that produces **no** denials at all is a namespace whose policies are
being stored and ignored.

## What this does NOT prove

That a pod in one environment cannot reach a pod in another. `terraform test`
asserts the cluster's *configuration*; only a live cell proves the *behaviour*,
and there is no cell. **US-3.3c owns that assertion** and the allow-set it
protects. Saying so explicitly is the point: "the config is right" is exactly the
evidence US-3.3a mistook for isolation.

## Acceptance criteria

- [x] The cell enables NetworkPolicy enforcement.
- [x] Asserted by a test against the planned resource, not by reading main.tf or
  a plan by hand — `terraform test` with a mocked provider, credential-free, in CI.
- [x] The mutually-exclusive `network_policy` block is asserted absent.
- [x] Denied connections are logged.
- [ ] A pod in env A cannot reach a pod in env B — **US-3.3c**, needs a cell.

## Outcome

`terraform test` with `mock_provider` runs credential-free in the existing `infra`
CI job, and asserts against the planned resource rather than the file's text.

Making the module testable required one production change: `output
"cluster_ca_certificate"` indexed `master_auth[0]`, a computed block list that is
empty until the API answers, so the module could not be evaluated by **any**
harness short of a real apply. It is `one()` + `try()` now — which is also the
better failure (an empty kubeconfig field rather than a plan-time index crash).

Four mutations RED on a verified-GREEN baseline: the setting removed, set to
`LEGACY_DATAPATH`, `network_policy` added alongside it, and Workload Identity
removed. The first baseline probe reported NOT-GREEN and was itself the bug —
`grep -cE '^Success!'` anchors at line start and terraform colours its output —
so the four results were discarded and re-measured.
