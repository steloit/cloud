# ADR-0015 — The cell's datapath is GKE Dataplane V2

**Status:** Accepted (agent, 2026-08-23) — implementation decision under ADR-0001's
frozen substrate; **not** a change to the substrate itself. Founder ratification
not required, but the create-time irreversibility below is worth knowing about.

**Deciders:** agent · **Relates to:** ADR-0001 (architecture v1, GKE), ADR-0007
(substrate spike), ADR-0012 (namespace shape), INF-001 D7 (tenant isolation),
US-3.3a (which found the defect), US-3.3c (which needs the enforcement)

## Context

`infra/modules/gke-cell` created a GKE **Standard** cluster with no network policy
provider. On such a cluster the API server **accepts and stores** every
NetworkPolicy object and **nothing drops a packet**.

US-3.3a discovered this the expensive way: it rendered D7's default-deny boundary,
proved every manifest correct against the API shape, and had to withdraw the whole
set on finding there was nothing to enforce it. *Rendered, stored and enforced are
three different things*, and a green test suite covered the first two.

`docs/architecture.md` has no networking or CNI section, so the choice had never
been made explicitly — the cluster simply inherited GKE's default.

## Options

| | enforcement | migration cost | observability |
|---|---|---|---|
| **nothing** (the state we were in) | **none** — policies stored, ignored | — | — |
| **Calico** (`network_policy`) | yes | enable later; **recreates nodes** | none beyond audit logs |
| **Dataplane V2** (`ADVANCED_DATAPATH`) | **built in** | **no documented path for an existing Standard cluster** | `NetworkLogging` CRD |

Google: *"GKE Dataplane V2 comes with Kubernetes network policy enforcement
built-in. This means that you don't need to enable network policy."* The two are
mutually exclusive — setting both is rejected: *"Enabling NetworkPolicy for
clusters with DatapathProvider=ADVANCED_DATAPATH is not allowed."*

## Decision

**Dataplane V2**, set at cluster creation.

Two reasons, in ADR-040's review order.

**Operability first.** The `NetworkLogging` CRD is a Dataplane V2 object. Choosing
Calico means giving up denied-connection logging, and with it the one signal that
would have caught US-3.3a's defect from the outside: a default-deny namespace that
produces *no* denials at all is a namespace whose policies are being ignored.

**Then the timing.** Google documents no migration path for an existing Standard
cluster, so this is create-time-only — free before the first cell exists, a full
cluster-and-node-pool rebuild afterwards. Calico is the option whose cost does
*not* depend on that premise, which is a real argument for it; the counter is that
the premise is **verified** — as of 2026-08-23, `gcloud container clusters list
--project=steloit-dev` returns `[]` and `gs://steloit-dev-tfstate/dev/default.tfstate`
(serial 47) holds exactly one resource, the state bucket itself, so `gke-cell` has
never been in state — and the observability difference is permanent while the
timing difference is one-off.

## Consequences

**Accepted known issues**, from Google's own list:

- **NetworkPolicy `endPort` (port RANGES) is silently not enforced.** This is the
  same class of defect this ADR exists to close — a policy the API server accepts
  and does not apply. The current D7 set uses single ports only; US-3.3c carries
  an AC that `tenancy.Render` must *refuse* a policy carrying `endPort`, so the
  constraint has an owner where policies are written rather than living in prose.
- Hairpin connections can drop; `hostPort` conflicts with the NodePort range.
  Neither affects the D7 set (no `hostPort`).

**GKE Sandbox is unaffected.** INF-001 D7 requires customer code to run under
gVisor, and the workload pool sets `sandbox_type = "gvisor"`. Google's GKE Sandbox
limitations list names Cloud Service Mesh, hostPath, privileged containers,
VolumeDevices, portforward and kernel security modules — it does **not** mention
Dataplane V2, Cilium or NetworkPolicy. Positively: the sandbox documentation
*recommends NetworkPolicy* as the control for blocking `169.254.169.254` from
sandboxed pods, and Autopilot runs Dataplane V2 and GKE Sandbox together by
default. Documented compatibility, not a guarantee — **NetworkLogging's coverage
of gVisor pods specifically is not documented either way**, and US-3.3c should
confirm it on the first real cell.

That recommendation also sharpens a design point for US-3.3c: customer code must
be *blocked* from the metadata server, while managed CNPG *requires* it for
Workload Identity. Those are different pools and need different policies.

**What this does not do.** It does not prove a pod in one environment cannot reach
another. `terraform test` asserts configuration; only a live cell proves
behaviour, and there is no cell. US-3.3c owns that assertion.
