---
id: US-3.3a
title: "Nothing creates the env namespace (nor its D7 default-deny policies)"
epic: E3
status: done
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/internal/provisioning/**
  - services/cell-agent/internal/**
  - services/cell-agent/cmd/**
  - infra/**
  - contexts/provisioning.md            # the living file (AGENTS.md §8)
  - tasks/e3-provisioning/US-3.3*.md
verify:
  - "a service in a brand-new environment converges without a hand-created namespace (LIVE — needs a cell; NOT YET RUN, see AC 3)"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go build ./... && go vet ./... && go test -race ./..."
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && test -z \"$(gofmt -l .)\""
owner: agent
---

## Goal

An environment's Kubernetes namespace — and the isolation D7 requires — must be
created by the system, not by a human running a runbook.

## Why — found in US-3.3's review

US-3.3 renders into `desired.namespace`, but **nothing creates that namespace**:
not the control plane, not the agent, not terraform (`module.cnpg` creates only
`control_plane`). The live e2e worked because the runbook did
`kubectl create ns` in preflight. On a real cell a genuinely new project/env
would 404 on first apply — and, until US-3.3's ErrNotConverged fix, would have
retried forever silently.

D7 also requires the namespace to carry **default-deny NetworkPolicies, a
ResourceQuota and a LimitRange** — the tenant isolation boundary. None exist.
US-3.3 fixed the namespace *collision* (env-id derived) but not its *creation*
or *policing*, so isolation is nominal until this lands.

## Acceptance criteria

- [x] Creating an environment creates its namespace (control-plane-side at env
  creation, or agent-side as part of converge — decide and record why).
- [ ] The namespace carries default-deny NetworkPolicy, ResourceQuota, LimitRange
  (D7). **Withdrawn to US-3.3c** — implemented, then removed before merge because
  the security review showed each object is inert or harmful. See below.
- [ ] A service in a brand-new environment converges with no hand-created
  namespace; **proven without the runbook's preflight**. NOT PROVEN — see
  "AC 3 is not checked, and why" below.
- [ ] Deleting an environment removes the namespace (and nothing else's).
  **Filed as US-3.3b** — no env-teardown path exists to hook.

## Related

US-3.3 (found it) · INF-001 D7 · US-3.3b · US-3.3c · `infra/modules/cnpg`

## Decision — agent-side, and it is not a preference

**The control plane has NO Kubernetes dependency.** `services/api/go.mod` contains
no `k8s.io/*` at all, because the two-plane split (D6, frozen by ADR-0001) is that
the control plane writes desired state and the cell-agent converges it. Creating
the namespace control-plane-side would mean giving the control plane cluster
credentials and a kube client — an architecture delta, and ADR-0040 says a delta
needs evidence from implementation or a customer. There is none; there is just a
missing namespace.

Agent-side is also **level-triggered for free**: a namespace deleted out from
under us is recreated on the next converge. A create-once call at environment
creation would never notice.

## What was actually missing — checked in all three places

| where | what it does with the namespace |
|---|---|
| control plane | **derives the name only** (`namespaceForEnv`), never creates |
| cell-agent | **consumes** it as `driver.Spec.Namespace`; drivers render *into* it |
| terraform | creates `cnpg-system` and `control-plane` — nothing per-env |

So the task's claim held exactly. The live e2e worked because a runbook ran
`kubectl create ns` in preflight.

## AC 3 is not checked, and why

The AC says **"proven without the runbook's preflight"**. It was ticked on the
first version of this branch and should not have been. `infra/spike/us33-e2e.sh`
still runs the preflight — its NOTE is corrected, but removing the line would make
the runbook unrunnable until a cell exists to prove the replacement. And **there is
no cell**: `gcloud container clusters list --project=steloit-dev --format=json`
returns `[]` with exit 0 (a control call, `projects describe` → `steloit-dev
ACTIVE`, rules out a silent auth failure), so `infra/modules/gke-cell` has never
been applied and the US-3.3 live drill ran against a cell that no longer exists.
The evidence for "the namespace is created before anything is applied into it" is
a Go test against a fake applier: real evidence for the ordering invariant, none
at all for the AC as worded. The live statement is back in `verify:`, marked NOT
YET RUN; US-3.3c AC 7 owns turning the runbook into something that asserts the
boundary rather than creating it.

## D7 was implemented and then WITHDRAWN — this is the substance of the task

The first version of this branch shipped all six D7 objects. The security review
found the isolation boundary it created does not exist, and that two of the four
objects are actively harmful. All four findings are in **US-3.3c**; in short:

| object | enforced today? | why it did not ship |
|---|---|---|
| Namespace + labels | **yes** | shipped — this is the real gap |
| NetworkPolicies ×3 | **no** | `infra/modules/gke-cell` is GKE Standard with no `network_policy` and no `ADVANCED_DATAPATH`. The API server stores them; nothing drops a packet. |
| NetworkPolicies ×3 | — | and when enforcement *is* on, the allow-set fences CNPG off the metadata server (Workload Identity), GCS (WAL archiving) and the apiserver (instance manager) — the first Postgres pod never reaches ready |
| LimitRange | **yes** | `cluster.yaml.tmpl` declares no `resources:`, so `default: 512Mi` becomes the hard cap on every managed Postgres, existing ones included |
| ResourceQuota | **yes** | unsourced, plan-independent envelope; `persistentvolumeclaims: 16` caps an env at ~5 HA clusters with no error surface. Founder-owned. |

Shipping the policies unenforced would have put a green suite and a "D7 done"
behind a boundary that is not there. `tenancy.Render`'s package doc records why
each is absent, and `TestTheD7PolicyObjectsAreDeliberatelyNotRenderedYet` makes
re-adding one impossible without reading it.

## Two things this uncovered

**1. The applier could not express a cluster-scoped resource.** `resourcePath`
hardcoded `/namespaces/<ns>/<plural>/<name>` and rejected an empty namespace — but
a Namespace lives at `/api/v1/namespaces/<name>` and *has* no namespace. Nesting it
is a 404 at apply time on a live cluster: the exact class the explicit `plurals`
map exists to prevent, one level up. `clusterScoped` is now explicit for the same
reason `plurals` is.

**2. Test fixtures still used the pre-ADR-0012 namespace shape.** Eleven fixtures
in `internal/render` used `acme--prod` — the `proj--env` form ADR-0012 superseded
*because it collided across orgs and shared the very isolation boundary it was
meant to create*. `tenancy.Render` rejects it, which is how they surfaced.
Corrected to `env-9f3c1a2b` (`sanitize(env_id)`, ADR-0012 §36). The review
independently diffed all eleven and confirmed none asserted anything *about* the
`proj--env` shape, so no coverage was destroyed.

## What six review rounds found — all blocking, all mine

Recorded because the pattern is the finding: **each round I verified the
representation I had just edited and not the one that decides.**

**I WEAKENED A TEST AND CLAIMED THE OPPOSITE.**
`TestCNPGRendererAppliesRenderedManifests`'s D8 assertion — "the namespace came
from placement, not a guess" — went from `objs[0]`, the CNPG Cluster
specifically, to a substring search over the **concatenated** applied set. Every
tenancy manifest also contains `namespace: env-9f3c1a2b`, so it was answered by a
different object than the one it names: rendering one tenant's database into
another tenant's namespace (`driver.Spec.Namespace` → `env-victim`) left it
**GREEN**, and the branch described the change as making it stronger. Widening an
assertion from one object to a set reads as generalisation and is a weakening. It
now parses the `kind: Cluster` object and compares `metadata.namespace` as a
value — the first repair was still `strings.Contains`, so `…-shadow` survived.

**One test genuinely was made stronger.**
`TestDeletingAFailedServiceLeavesNothingBehind` began failing because env objects
survive a *service* teardown — correct: the namespace belongs to the environment.
It now pins **both** directions, with the env set DERIVED from `tenancy.Render`
rather than retyped.

**Six more of mine, each measured:** a `steloit.dev/environment-id` label that
named nothing (`9f3c1a2b` for the id `env_9f3c1a2b`, and US-3.3b planned to
delete by it); `Apply` routing by the caller's namespace without ever reading
`metadata.namespace`, so a cross-namespace write was fail-closed only because the
API server answers 400; widening `plurals` while `Delete` hardcoded the CNPG
apiVersion, turning a loud refusal into "already gone" — the exact call US-3.3b
will make, and a **pre-existing** trap for `Secret` and `StatefulSet`; two
multi-document bypasses, since `yaml.Unmarshal` returns only document 1; a
teardown path that never validated the namespace, so
`"../../../api/v1/namespaces/kube-system"` was refused on create and **accepted
on teardown**; and boot validation split across three representations where only
two were covered.

**Then the same class relocated eight times** — `manifests[0]` → `_mi<2` →
`_mi<4` → kind → body shape → the request contract → the path → the response.
Every one had the shape "correct for index 0 only", expressible only because the
rules lived inline in a loop. `Apply` is now a loop that orders and aborts over
`applyOne`, which takes one manifest and has no index, so the state is
unrepresentable rather than tested-against. That also closed real defects: a
swallowed non-2xx (ready with no ScheduledBackup — no PITR), `Delete` treating
any 4xx/5xx as gone (service marked deleted, metering stopped, cluster running),
and a transport where dropping `RootCAs`, adding `InsecureSkipVerify` or
downgrading to `http://` were all green.

**THE FIXTURES WERE THREE TIMES SHORTER THAN ANYTHING PRODUCTION CAN MINT.**
`ids.New` produces a 32-hex suffix, so a real namespace is `env-<32 hex>` (36
chars) and a real service id `svc_<32 hex>`. Every fixture drove `env-9f3c1a2b`
(12) and `svc_db01` (8), so any rule keyed on identifier LENGTH was unpinned —
four mutations survived, and two of them switch off this task's headline
behaviours for **every real environment**: a `Delete` that no-ops above 12
characters, and a teardown that deletes nothing and still reports `gone`.

The provenance is the lesson. ADR-0012 writes the shape as
`env_9f3c… → env-9f3c…` with a typographic **ellipsis**, and the fixture read
that elision as a literal, then described itself as "the ADR-0012 shape". An
elided example became the test data — which is what AGENTS.md means by *examples
are normative*. Fixtures are production-shaped now (32-hex, 22 places).
**"With one short case kept" was wrong** — a count asserted without running one:
four lengths are kept on purpose (1, 4, 8 and the real 32), because the survivors
were keyed on LENGTH and a suite that knows one length sweeps neither class.

**An illegal status edge, retried forever.** `"Waiting for user action"` mapped
to `degraded`, but the control plane allows `provisioning → {ready, failed,
deleting}` only. The writeback is rejected every tick, `observed_generation`
never advances, and the service is retried forever with nothing visible — the
exact failure `statusFromPhase` argues against thirty lines below, arrived at
from the other side. Now `failed` — and that is ALL that shipped of it. Round 12 tried to go further,
with a FROM-aware `statusFor` in the AGENT and a repo-root artifact pinning two
copies of the transition table; **round 13 reverted all of it** as a regression.
The three measured reasons, and where the real fix lives, are in the Outcome.

**What round 13 keeps:** every CNPG phase pinned by CLASSIFICATION, not just by
count — reclassifying `"Cluster has incomplete or invalid image catalog"` from
`failed` to `provisioning` was green in both modules, and a permanently broken
cluster read as still-converging is the defect `statusFromPhase`'s own comment
argues thirty lines about. Pre-existing, closed here.

**And `-count=1` in CI, which is not hygiene.** cmd/go only tracks fixtures a
test opens under its OWN package directory, so an edit to a repo-root or
cross-module fixture comes back `ok (cached)`. Measured: the artifact edit this
round was built to catch was green in both modules on a warm cache and RED in
both with `-count=1`, and `actions/setup-go` restores GOCACHE across commits. It
failed OPEN. The flag is pinned through the executable text of three `ciGates`
needles — `runContains` matches against `stripShellComments`, so a gate can
never be armed by a comment, which is the harness working as designed.

Six review rounds, every one blocking, and the pattern is the finding: **each
round I verified the representation I had just edited and not the one that
decides.** Manifests vs. what enforces them; the struct field vs. the bytes; the
create path vs. teardown; the validator vs. its call site vs. `main` honouring
it; `manifests[0]` vs. `_mi<2` vs. `_mi<4` vs. the batch's *composition*; and,
worst, 25 mutation results vs. the harness that produced them — a module-only
`cp -R` whose baseline was already RED, published as evidence.

**61 DISTINCT rows RED** on one harness with a green baseline asserted before and
after; 34 of them once GREEN; two accepted survivors recorded below. Derivation,
because two earlier published counts (59, and 61-before-dedup) were restated
rather than re-derived: 34 once-GREEN rows + 29 in the PR = 63 entries, minus the 2
that appear verbatim in both (the `ValidateCell` call; `main` ignoring `run`'s
error) = **61**. Rows, not mutations — PR row 29 bundles two refuse-everything
controls, so 61 rows cover **62 mutations**. Five lessons in
`contexts/provisioning.md`.

**As merged, an environment is a namespace and nothing else, and deleting one
leaks it (US-3.3b).**

## Accepted survivors — measured green, deliberately not closed

Named here because an unrecorded survivor is indistinguishable from one nobody
looked for.

| survivor | why it stands |
|---|---|
| `Apply` guards skipped for index ≥ **12** | Any hand-written sweep has a ceiling; this one is 12, against a largest-ever batch of 8 (`7e94f26`). **This was published as "≥ 16" for one round while the sweep already ran to 12** — the ceiling was lowered in the same commit that widened the fixtures, and four claims were not updated. Corrected here and measured: ≥ 12 GREEN, ≥ 13 GREEN, ≥ 16 vacuous. The real close is a property test over random `n` and random composition, filed with US-3.3c. |
| `defer stop()` dropped in `run` | `stop()` only releases the signal handler and the process exits immediately after `run` returns, so leaking it until exit is a no-op. True only because `run` has exactly ONE production caller — a second one reopens this. |
| ~~the in-cluster `CELL_GSA_EMAIL`/`CELL_WAL_BUCKET` guard~~ | **CLOSED.** The stated reason was false: `NewInCluster` reads two env vars and two files, so a temp SA dir reaches it in milliseconds — `saDir` is now a `var` (the `NewClientForTest` precedent) and `TestRunTakesTheInClusterBranchAndRequiresGSAandWAL` drives it. Substituting `panic()` for the CNPG renderer had been green: the only arm that runs on a cell was dead code to the suite. |
| the `signal.NotifyContext` hoist into `main()` | Reverted, and the landmark ordering is verified byte-identical to `origin/main`, but only a comment stops the next refactor re-landing it. Pinning it needs a subprocess that can be signalled *during* boot, which is a race against a sub-millisecond window. |

## Outcome

Shipped: the environment's Kubernetes namespace, created **agent-side** on every
converge (level-triggered, so a namespace deleted out from under us comes back),
with `resourcePath` taught cluster-scoped routing, `Apply` restructured so the
"correct for index 0 only" class is unrepresentable, and the transport pinned.
D7's policy objects are deliberately NOT here — see the withdrawal section above.

**The status work is a single line: `"Waiting for user action"` maps to `failed`,
not `degraded`.** Round 12 tried to go further — a FROM-aware `statusFor` and a
transition table in the agent, pinned by a repo-root JSON artifact — and **round
13 reverted all of it**, because both reviewers measured it as a regression:

- It **collapsed the transient guard.** `statusFor("ready","provisioning")` finds
  no legal edge and returns `from`; `ready` IS terminal, so Converge's
  `!terminal(status)` BLOCKER never fired. A READY service observed mid-upgrade
  reported `"ready", nil` instead of `ErrNotConverged` — the agent declaring a
  generation converged during an apply, `observed_generation` advancing, the row
  leaving the outstanding set. Strictly worse than the 409 loop it replaced.
- **Its premise was false.** "Separate go.mod files, so neither module can import
  the other" — `apps/cli/go.mod` already `require`s and `replace`s
  `packages/contracts/go` across exactly that boundary, and
  `docs/architecture.md` names the cell-agent as an importer too. An
  architectural constraint I asserted without checking the repo.
- **It deleted the test that caught its own headline incident.** The replacement
  cross-product skips any answer where `got == from`, so the
  `"Waiting for user action": "degraded"` mutation — cited three times as the
  motivating live consequence — became a survivor.

The real fix is **US-3.3h**, in `reconcile.Writeback`: the only place holding
both `from` and the machine, and where this is not a plane leak (ADR-0001
D9/A2.5). **O31** owns the five-representation status vocabulary.

Round 13 also keeps two pre-existing closures: every CNPG phase pinned by
CLASSIFICATION (reclassifying a terminal-bad phase as transient was green in both
modules), and `-count=1` on all three Go modules in CI. That one is stated as an
observation, not a mechanism: an edit to ONLY a repo-root fixture two modules
assert against came back `ok (cached)` in both and RED in both with the flag, and
setup-go restores GOCACHE across commits — so it failed OPEN. A synthetic
reproduction did **not** confirm the obvious explanation (in a scratch module such
a test is simply uncacheable), so no claim about package-vs-module roots is made.
Measured: the CI `go` job is 514s with the flag against 586s without, so it is
free either way.

**61 DISTINCT rows RED** across the branch (34 once-GREEN, enumerated in the PR
with the other 27), two accepted survivors below. Five lessons in
`contexts/provisioning.md`.

**As merged, an environment is a namespace and nothing else, and deleting one
leaks it (US-3.3b).**

## NOT done — AC 2, AC 3, AC 4

**AC 2 (D7 policies)** → **three** tasks, not one. "The four are one change" was
right for the policies and wrong for the rest: the LimitRange and the quota were
withdrawn because they are *wrong*, not because they are coupled to Dataplane V2,
and bundling them would gate a one-number founder decision behind a cluster
migration. **US-3.3c** (`ready`, critical) — enforcement, the CNPG allow-set, and
the agent RBAC this namespace write now needs; they land together because you
cannot write a correct egress set without a cell that enforces it. **US-3.3d**
(`ready`, high) — the Cluster declares resources from the sold shape, then a
LimitRange; CNPG-driver scope, needs no CNI. **US-3.3e** (`blocked`) — the
per-plan envelope, blocked on a founder number rather than sitting
`critical`/`ready` where nobody can take it.

**AC 3 (live proof)** → needs a cell; in `verify:` marked NOT YET RUN, runbook
owned by US-3.3c AC 7. **AC 4 (env deletion)** → **US-3.3b**: teardown today is
per-service, and env deletion needs a control-plane contract that does not exist.
