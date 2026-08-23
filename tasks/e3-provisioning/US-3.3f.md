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
  - infra/envs/dev/tests/**
  - infra/envs/cell0/tests/**
  - infra/k8s/policy/**
  - docs/adr/0015-cell-datapath-dataplane-v2.md
  - services/api/internal/platform/testenv/**
  - docs/product/claudedocs/spec-change-proposals.md
  - .github/workflows/ci.yml
  - tasks/e3-provisioning/US-3.3f.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/infra/modules/gke-cell\" && terraform init -backend=false && terraform test"
  - "cd \"$(git rev-parse --show-toplevel)/infra/envs/dev\" && terraform init -backend=false && terraform test"
  - "cd \"$(git rev-parse --show-toplevel)/infra/envs/cell0\" && terraform init -backend=false && terraform test"
  - "terraform fmt -check -recursive infra"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 ./internal/platform/testenv/"
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
- [x] **Each ENV's planned value** is asserted, not just the module's literal.
  The two are different representations: with the module reading a variable and
  `envs/dev` passing `LEGACY_DATAPATH`, the module test, `fmt -check` and both
  `validate`s are all green while dev deploys with no enforcement. Measured, and
  the env tests turn it red.
- [ ] Denied connections are logged. **AUTHORED, NOT APPLIED.** Nothing in the
  repo applies `infra/k8s/policy/network-logging.yaml` — the only apply paths are
  `module.cnpg`'s `kubernetes_manifest` resources for `k8s/storage/` and
  `k8s/control-plane/`, and `k8s/policy/` is wired to nothing. The sibling
  ClusterImagePolicy has the identical gap, already recorded in
  `spec-change-proposals.md`. Ticking this was the PR's own thesis — *rendered,
  stored and enforced are three different things* — reproduced one layer up:
  **authored is not applied.** The YAML gate is content-blind, so five mutations
  of this file (deny.log false, a bogus apiVersion, a misspelled kind, a
  different name, an empty spec) all pass; content assertions are added below,
  but they pin the FILE, not that anything reads it.
- [ ] A pod in env A cannot reach a pod in env B — **US-3.3c**, needs a cell.

## Decision record

The datapath choice is an **ADR** (`docs/adr/0015-cell-datapath-dataplane-v2.md`),
not a Terraform comment. It is create-time irreversible, it accepts three
known-issue classes, and `docs/architecture.md` has no networking section at all —
ADR-0012 set the bar for something far more reversible.

## Outcome

`terraform test` with `mock_provider` runs credential-free in the existing `infra`
CI job, and asserts against the planned resource rather than the file's text.

Making the module testable required one production change: `output
"cluster_ca_certificate"` indexed `master_auth[0]`, a computed block list that is
empty until the API answers, so the module could not be evaluated by **any**
harness short of a real apply. It is a splat + `one()` now, with **no** `try()` — the
`try()` was itself removed as a finding (it swallowed every error in the
expression, including `one()`'s own), and an earlier version of this Outcome
described the superseded revision. Which is also the
better failure (a null that both envs' `base64decode()` rejects loudly, rather than a plan-time
index error inside the module — measured; an earlier revision claimed "an empty
kubeconfig field", which is simply false).

Four mutations RED on a verified-GREEN baseline: the setting removed, set to
`LEGACY_DATAPATH`, `network_policy` added alongside it, and Workload Identity
removed. The first baseline probe reported NOT-GREEN and was itself the bug —
`grep -cE '^Success!'` anchors at line start and terraform colours its output —
so the four results were discarded and re-measured.

**Rounds 3-5, which are most of the coverage.** The Outcome above describes the
first two and would understate what merged by a wide margin.

Round 3 added the CI gate (`terraform test` over every discovered directory) and
the env-layer tests, because a module literal and an env's planned value are two
different representations. Round 4 closed the env-layer half of the enforcement
invariant (`network_policy` alongside ADVANCED_DATAPATH passed everything while
GKE rejects it at apply), four measured node-config survivors, and the
`core_min_nodes >= 1` validation.

**Round 5 replaced the instrument.** Round 4 had pinned the gate's `exit $fail`
with a contiguous substring needle; review defeated it by moving `fail=0` ONE
LINE UP, inside the loop — all three suites failing, step exiting 0. A substring
wall can always be stepped around when the property is control flow. So
`infra_step_test.go` now EXTRACTS the step from `ci.yml` and EXECUTES it under
`bash --noprofile --norc -eo pipefail` against a stub `terraform`. Five insertion
variants died at once.

That test passed for the wrong reason first: an exit-for-everything stub made
`terraform init` fail, `-e` aborted before the loop, and "the step exits nonzero"
was satisfied by an unrelated statement. Split the stub, and the mutation went
red. Discovery also moved off the `module "gke_cell"` label onto the module
SOURCE, after three semantics-preserving refactors were measured dropping an env
out of coverage; and each discovered suite must now carry a `run` block that
mentions `datapath_provider`, because a suite with none — and a directory with no
test files — both exit 0 with "Success! 0 passed, 0 failed".

Two branches were REMOVED rather than pinned (an unreachable arm, and a
set-difference the per-directory loop subsumed): dropping either changed no
outcome, which is what made them unpinnable. `xargs -r` is load-bearing — GNU
xargs runs the command once on empty input, so without it the emptiness guard is
unreachable on `ubuntu-latest` (rc=123, no annotation) while being reachable on
macOS, meaning the harness would exercise a different control flow than CI.

Also closed in round 5: the Calico ADDON path (a second way to trip the
combination GKE rejects), `remove_default_node_pool = false` (which keeps a
fourth pool on the default compute SA that every by-name pool assertion is blind
to), module-layer `ip_allocation_policy`, three sibling capacity validations, and
a text pin on `cluster_ca_certificate` — which no `terraform test` can cover,
because under `mock_provider` `master_auth` is empty and the expression is never
evaluated. That pin was itself broken on arrival: a bare `strings.Contains` over
the file passed when the old expression was parked in a `# was:` comment and the
output gutted, which is the exact hole `stripShellComments` exists to close in
the sibling gate. It strips comments now.
