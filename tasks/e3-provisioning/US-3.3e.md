---
id: US-3.3e
title: "The per-environment resource envelope is a product decision with no owner"
epic: E3
status: done
phase: MVP
priority: medium
sprint: 4
issue: 0
labels: [Platform]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/tenancy/**
  - services/cell-agent/internal/render/**
  - services/cell-agent/internal/kube/**
  - services/api/internal/billing/**
  - services/api/internal/provisioning/**
  - docs/founder-config.md
  - tasks/e3-provisioning/US-3.3e.md
  - tasks/e3-provisioning/US-3.3g.md
verify:
  - "the rendered ResourceQuota reads its numbers from the founder-owned source, per plan"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -race ./..."
owner: agent
---

## Goal

Give an environment a resource ceiling that someone owns.

## RULED 2026-08-23 (founder)

| plan | CPU | memory | PVC |
|---|---|---|---|
| `free` | 1 vCPU | 2 GiB | 10 GiB |
| `pro` | 8 vCPU | 16 GiB | 100 GiB |
| `business` | 12 vCPU | 24 GiB | 200 GiB |
| `enterprise` | 16 vCPU | 32 GiB | 250 GiB |

Per ENVIRONMENT, against the four authoritative plan identifiers (`orgs.plan`'s
CHECK constraint lists exactly these; there is no `standard` tier).

## Why — US-3.3a's security review

US-3.3a shipped `requests.cpu: 8 / limits.cpu: 16 / 16Gi / 32Gi /
persistentvolumeclaims: 16 / count/services: 32` as literals in a driver. Three
problems:

- **Unsourced.** No owner, no derivation, no `NEEDS FOUNDER INPUT` row. A
  mutation inflating every number 1000× (8000 CPU / 16000Gi) survived the whole
  suite green — correct only once an owner exists to pin them against.
- **Plan-independent.** A Free org and a Business org get the identical envelope,
  which contradicts the tiering in `plans.json`. Retyped constants in a driver are
  what the canon rule exists to prevent.
- **Quietly too small in one dimension.** A 3-instance CNPG cluster consumes 3
  PVCs and 3 Services, so `persistentvolumeclaims: 16` caps an environment at ~5
  HA Postgres clusters *before branches* — and there is no error surface when it
  hits. The customer sees provisioning stop.

## Acceptance criteria

- [x] The envelope is read from the founder-owned source, per plan — not retyped.
- [x] All four plans render their own envelope; no two plans share one, so a
  wrong-plan render is detectable.
- [x] CPU, memory and PVC/storage are each rendered and admission-enforced, in
  the environment's own namespace. **But only storage BINDS today**:
  `cluster.yaml.tmpl` declares no `resources:`, so every CNPG container is
  admitted at the LimitRange's `defaultRequest` and the cpu/memory ceilings act
  as a pod-count proxy rather than as compute the customer paid for. US-3.3d
  closes it; disclosed rather than left implied by the word "enforced".
- [x] Malformed, missing, zero, negative, unitless, fractional and
  YAML-injecting envelopes all fail closed, on both sides of the wire.
- [x] An unknown plan is denied by default.
- [x] Changing `plans.json` cannot leave a stale limit elsewhere.
- [ ] Hitting the quota produces a problem+json error rather than a stalled
  converge — **US-3.11** owns it: the agent has no terminal-failure writeback at
  all, so an over-quota pod is invisible for the same reason any permanent render
  error is.

## Outcome

**One definition.** The envelope lives in `plans.json` beside the plan it belongs
to, validated at boot (`billing.parse` fails the process on a missing or
malformed one — an unquota'd plan is an environment with no ceiling). The control
plane resolves it (`billing.Table.Envelope`, deny-by-default) and ships the
VALUES in the desired doc; the cell-agent never sees a plan name and holds no
copy of the plan table, the same boundary as pricing.

**No k8s dependency in the control plane.** Validating quantities with
`k8s.io/apimachinery` was the obvious move and would have been wrong:
`services/api/go.mod` contains no `k8s.io/*` at all, and that absence is the
evidence for the two-plane split (D6, ADR-0001) — it is the argument US-3.3a used
to establish that the control plane must not hold cluster credentials. A closed
grammar (whole cores; whole Mi/Gi/Ti) is stricter than Kubernetes' parser and
keeps the boundary. Re-checked independently on the cell side, because the two
ends of the wire are separate modules.

**Requests, not limits.** A `limits.*` quota forces every pod to declare a limit,
and the only way to supply one for the CNPG Cluster — which declares no resources
until US-3.3d — is a LimitRange `default`, which then becomes its hard cap. That
is how an earlier revision would have OOMKilled every managed Postgres at 512Mi.
Requests are also what allocate capacity: a pod requesting 1 CPU occupies 1 CPU
of schedulable capacity whether or not it uses it.

**Enforced, not merely rendered.** The ResourceQuota admission controller is in
Kubernetes' default-enabled plugin list and needs no add-on — the precise
opposite of NetworkPolicy, which needs a provider this cell did not have, which
is why D7's policies were stored and ignored (US-3.3f fixed that). The quota and
LimitRange ship as a PAIR because enforcement is real: a quota on
requests.cpu/memory makes the API server REJECT any pod that declares neither, so
the quota alone would refuse ordinary pods.

**Eight mutations RED**, each on a harness whose no-mutation baseline was asserted
GREEN first — and three of them were GREEN until this round:

| mutation | |
|---|---|
| quota constrains `limits.*` instead of `requests.*` | RED |
| ResourceQuota rendered into the wrong namespace | RED |
| the cpu grammar check removed | RED |
| a LimitRange `default` limit reintroduced (the OOMKill) | RED |
| the storage dimension dropped entirely | RED |
| a missing envelope accepted | RED *(was GREEN — an equivalent mutant until the guard owned its own message)* |
| the renderer hardcodes `pro`'s envelope, ignoring the doc | RED *(was GREEN — every fixture shipped `pro`)* |
| `plans.json` `business` CPU 12 → 99 | RED *(was GREEN — the "authoritative" test read plans.json AND rendered from it, so it proved the renderer echoes its input and nothing about the numbers)* |

That last one is the sharpest: an authority test that derives both sides from the
same file is a tautology. The numbers are now stated as literals **once**, in
`billing.TestThePlanEnvelopesAreTheFounderApprovedValues`, because they are a
ruling rather than a derivation — `plans.json` is not self-authorising.

Two API-baseline probes reported NOT-GREEN mid-sweep and were investigated rather
than reported: the first copy lacked the repo root that `canon` reads.

**Deferred citations.** US-3.3d and US-3.3g are on this branch. **US-3.11**
(terminal-failure writeback, which the unchecked AC points at) is filed on
`task/T3.4c`, and **US-3.3f** (enforcement) on its own branch — neither is an
ancestor here, so those two references resolve only once those branches merge.
Stated rather than left dangling.

**Scope note:** the original AC said "CPU, memory, PVC count and Service count".
The founder's table rules bytes, not counts, so `count/services` is deliberately
absent — recorded rather than silently dropped.

**The backfill was absorbed, not filed.** `tenancy.Render` refuses a doc with no
envelope, so a service created before this branch does not run unbounded — it
stops converging entirely, in a retry loop with no customer-visible status. That
is a deploy-stop, so `20260823140000_service_quota_backfill` ships here. It
repeats the four envelopes in SQL (SQL cannot read `plans.json`) and
`TestTheBackfillMigrationMatchesThePlanTable` fails if the two ever differ, so
the duplication is detectable rather than silent. A migration is immutable once
applied: a later envelope change is US-3.3g's rewrite path, never an edit to
that file.

**Filed, not absorbed: US-3.3g.** The desired doc is rebuilt when a SERVICE
changes, not when an org's PLAN changes, so an upgrade does not reach the cell
until each service is next touched. The downgrade direction is the worse one — we
would bill the smaller plan while enforcing the larger envelope.

## Read first

- `docs/founder-config.md` §5
- commit `7e94f26` — the withdrawn ResourceQuota
