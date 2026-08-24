# Autonomous session state

A **small current-state handoff**, not a diary. **Not authoritative** — the repo,
git, CI and `tasks/` are. Verify before trusting; correct this file when wrong.
Durable lessons live in `contexts/provisioning.md` (mistake bank) and `AGENTS.md`,
never here.

**Updated:** 2026-08-25 · `main` @ `9700569` · **US-3.3c (#333) in flight, round 2** · 5 PRs merged

---

## Current objective

Work the `tasks/` ready queue under the AGENTS.md protocol: claim → implement →
`verify:` → **both reviewers (ADR-0008)** → CI → merge. The queue is the plan.

## In flight

**US-3.3c — PR #333**, branch `task/US-3.3c`. Round 2 pushed. `validate` and
`infra` green (infra is the job that was RED — that is what confirms the provider
blocker is really fixed); `go` still running. Both reviewers ran once and both
found blockers, so a third pass is a judgement call under ADR-0016 — round 2 was
fixes to named findings.

### GCP: NOTHING IS RUNNING. Verified, not assumed.

A real GKE cell (`cell-verify`, us-central1-a) was provisioned for the live ACs
and **destroyed**: `Destroy complete! Resources: 16 destroyed`, then checked
directly — 0 clusters / 0 instances / 0 disks, only the pre-existing `default`
network and default compute SA. One resource Terraform did NOT own survived and
was deleted by hand: the CNPG PVC's PD. **The remote state bucket was never
written** (local state, discarded); `gs://steloit-dev-tfstate/dev/default.tfstate`
is dated 2026-07-26 and is not ours.

**Only `steloit-dev`, never `humane-dev-493708`** (the machine's active gcloud
config points at the latter). Scope every call with
`CLOUDSDK_CORE_PROJECT=steloit-dev`; do not mutate global config. ADC is EXPIRED —
Terraform runs on `GOOGLE_OAUTH_ACCESS_TOKEN=$(gcloud auth print-access-token)`.

### What the live run proved, and what it cost to learn

Three defects only a live cell could find, all "the policy looks right and does
not do what it claims":
1. **DNS resolved nothing** — NodeLocal DNSCache answers, so a rule naming
   `kube-dns` matches nothing. AC 5 confirmed; its hostNetwork premise was wrong
   (node-local-dns has ordinary pod IPs here).
2. **The CNPG allowances missed the bootstrap Job** (`cnpg.io/jobRole`, not
   `podRole`) — a FIFTH allowance the review did not name.
3. **The apiserver ipBlock must be the PRIVATE ENDPOINT**, not the `kubernetes`
   Service ClusterIP — Dataplane V2 evaluates egress post-translation.

Then the reviewers found my own first cut shipped **three guards that could not
fire** (endPort unreachable behind ValidateNamespace; ValidateCIDR never called;
`assert_boundary` defined and never invoked) and **a provider that bound a second
unconfigured client** (gavinbunney vs alekc — ADR-0017). And QA's M4: flipping
the CNPG selector `Exists → DoesNotExist` selects exactly the CUSTOMER pods and
inverts AC 9, green. Tests now PARSE policies instead of grepping them.

## Recently merged — the parts easiest to re-break

**US-3.3b (#332)** — an environment's namespace is torn down. Three rounds; the
first two premises were wrong.

1. **`gone` from a cell means the workload was observed ABSENT.** It used to mean
   "the delete was accepted" — k8s answers 2xx immediately, finalizers pending —
   and the namespace-teardown gate was built on that. The renderer now Observes
   the Cluster gone before reporting; anything else is `ErrNotConverged`. **If
   that check is removed the gate silently weakens to "accepted" and a database
   is deleted mid-termination.**
2. **The teardown poll AND the confirmation both carry the same `NOT EXISTS`
   fence.** `torn_down_at` is a one-way latch: confirming an environment that
   became ineligible strands it forever. `CreateService` also refuses a
   scheduled environment — the two guards fail independently.
3. **Scope is read from the rendered bytes, and multi-doc is refused.**
   `yaml.Unmarshal` returns doc 1 with a nil error — third time that has bitten.

**US-3.3h (#328)** — convergence is derived from the destination:
`settledStatuses[to] && (to == observed || observed == "")`. A cell may not
decide a lifecycle hold in EITHER position (as an observation, or as the row's
`from`). `gone` and `""` are different. `provisioning.StatusVocabulary` is the
one ADR-024 definition.

**US-3.12 (#331)** — the reconcile request enum ships in FOUR artifacts
(openapi, api gen, contracts/go, and the TS clients). `make gen-ts` is a
separate gate from `make gen-go`; running only one is how the first push failed.

## Next task

Ready queue after US-3.3c lands: **US-3.3g**, **US-3.10**, **US-3.11**,
**US-3.13**, **US-3.14**, plus the four this task filed —
**US-3.3j** (narrow the operator-ingress peer; needs a LIVE cell to verify),
**US-3.3k** (high: the agent has no deployment artifact, so no RBAC, and this PR
widened the gap), **US-3.3l** (`cnpg.io/cluster` is a capability label with no
owner), and **US-3.12/US-3.13** from earlier.

**Provisioning a cell is now a known, cheap procedure** (~$0.36/hr, ~25 min up,
~10 min down) — `docs/dev/us33c-live-evidence.md` records exactly how, including
why `infra/envs/dev` must NOT be applied wholesale (project_base creates a KMS
key ring, which can NEVER be deleted, and a WI pool with a 30-day soft delete).
US-3.3j and US-3.3k both need one.

## Blocked on a human## Blocked on a human

| # | decision | where |
|---|---|---|
| 1 | **O2 cost guardrail.** The founder gave ₹1,000/mo at 50/80/100 and "use the existing ops recipient", then said an **updated spec is coming**. Nothing has been written for O2 — `task/O2` worktree is clean. WAIT for it. | `tasks/eops/O2.md` |
| 2 | **A SIZE downgrade's price** while storage is retained (T3.4c). Not ruled; three options recorded with market evidence. | `docs/founder-config.md` §5 |
| 3 | **O30 — `quota_usage.rate_cents` is written as cent-seconds and read as cents.** Measured: a $24/mo service billed one hour invoices **$86,400**. Which side moves, and the proration divisor, are pricing decisions. | `tasks/eops/O30.md` |

**Answered 2026-08-23:** T3.4c `included_gb` semantics · ADR-0014 **ratified**
(merged, #321) · US-3.3e envelope (free 1/2Gi/10Gi · pro 8/16Gi/100Gi ·
business 12/24Gi/200Gi · enterprise 16/32Gi/250Gi, per environment).

## GCP access

`gcloud` is active as **hashir@humanetechnologies.in** (the founder's instruction:
never `admin@`). Re-verified 2026-08-23: `container clusters list --project
steloit-dev` → `[]`, exit 0, and `projects describe` proves the credential works.
ADC is saved; the quota project `humane-dev-493708` lacks `serviceusage.services.use`,
so pass `--billing-project=steloit-dev`.

## Do NOT redo

- **US-3.3a's namespace half.** `services/api/go.mod` has **no `k8s.io/*`**, so
  control-plane-side creation is an architecture delta (D6/ADR-0001). Agent-side,
  in `internal/driver/tenancy`, applied before the service manifests.
- **D7's policy objects were withdrawn on purpose.** Two of the three are BACK:
  US-3.3e reinstates the ResourceQuota and the LimitRange (`defaultRequest` only —
  a `default` would cap every managed Postgres, since `cluster.yaml.tmpl` declares
  no resources). NetworkPolicy is still withheld: not enforced on the cell today
  (#323 turns it on) and the allow-set would fence CNPG off Workload Identity /
  GCS / the apiserver. `tenancy.Render`'s package doc records the current split.
- **Only STORAGE actually binds** in the quota today; cpu/memory bind as a
  pod-count proxy until US-3.3d makes the Cluster declare its own resources.
- **There are ZERO GKE clusters in `steloit-dev`** — re-verified 2026-08-23 under
  `hashir@`. `infra/modules/gke-cell` has never been applied, which is what makes
  #323's create-time-only Dataplane V2 flip free. US-3.3a's AC 3 ("proven without
  the runbook's preflight") is **not** provable yet, and US-3.3c's pod-to-pod
  assertions need a cell first.
- **O2's module is INERT** (`billing_account` defaults `""`, no tfvars sets it),
  `currency_code` is deliberately DELETED (the server supplies it), and the
  amount is a required env variable with no default so `plan` refuses.

## Verification rules that keep catching me

These are in `contexts/provisioning.md` in full. The short form:

1. **A mutation harness must produce a GREEN no-mutation baseline first.** A
   module-only `cp -R` of `services/cell-agent` is RED on arrival (three
   `parity_test.go` cases need the repo root); scaffold one:
   `AGENTS.md` + `infra/k8s` + `infra/spike` + `services/cell-agent`. Assert 0
   FAILs before and after the sweep, and assert each mutation applied.
2. **Verify every representation**, not the one you edited: rendered vs stored vs
   *enforced*; struct field vs bytes; create path vs teardown; validator vs its
   call site vs `main` honouring it; the test's fixtures vs production's bytes.
3. **Never widen an assertion from one object to a collection** — it reads as
   generalisation and is a weakening.
4. **Never merge on your own read of a pending review.**
5. `git worktree remove` refusing and naming `--force` IS the warning.
6. **Verify the branch you are about to change, not the one you remember.** r12
   asserted "separate go.mod files, so neither module can import the other" —
   `apps/cli/go.mod` already imports `packages/contracts/go` across exactly that
   boundary with a `replace`. An architectural premise is checkable in ten seconds.
7. **A fix can be a regression.** r12's mapping made a transient phase terminal
   for any non-provisioning from-state, defeating the guard three lines below it.
   When a change makes a guard's INPUT different, re-check the guard.
8. **A count restated is not a count measured.** US-3.3a published "59 distinct"
   and "one short fixture kept"; counting mechanically gave **61 distinct** (34+29
   entries, 2 duplicates) and **four** fixture lengths. Re-derive every number you
   repeat — a figure carried forward from a previous round is an assertion.
9. **Pin duplicated knowledge against a shared artifact, not a copy of the other
   side** — and note a repo-root fixture is invisible to the Go test cache
   (`-count=1` is now in CI for all three modules, pinned through executable text).
10. **A legality sweep cannot see "no change".** Skipping answers equal to `from`
   hides the case where no change is the WRONG answer. Assert the DESTINATION.
11. **A substring needle cannot pin control flow.** Pinning a CI step's `exit $fail`
   by text was defeated twice by moving `fail=0` one line. EXECUTE the step in a
   test instead — and split the stub, or the script dies at `init` under `-e` and
   the assertion passes for an unrelated reason.
12. **A guard nothing can distinguish is not a guard.** Three separate branches on
   one CI step turned out to be subsumed by the loop below them; removing each
   changed no outcome. Remove them rather than inventing a test.
13. **Run the SHIPPED artifact, not a paraphrase.** Two migrations this session had
   defects invisible because a test hand-copied a simplified statement, or ran the
   real one only against an empty database where a no-op UPDATE looks correct.
14. **`strings.Contains` over a file cannot tell code from a comment.** A text pin
   passed with the expression parked in a `# was:` line and the value gutted.

## Conventions

- Worktree per task; `git status` in it **before** removing it.
- After pushing: `git rev-list --left-right --count origin/<branch>...HEAD` → `0 0`.
- Reviewers by name: `subagent_type: "reviewer"` and `"qa"` — **ONCE per PR, on
  the final diff, code only** (ADR-0016, founder 2026-08-24). Not after every
  round; task-file narrative/counts/citations are the implementer's own pass.
- `node scripts/spec-sync/validate.mjs` after any `tasks/` edit.
- Context packs: documented cap 150 lines, validator enforces >160 (O12 owns the
  discrepancy). Compress an existing entry to fund a new one.
- Container runtime for the API's integration tests:
  `DOCKER_HOST=unix://$HOME/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock TESTCONTAINERS_RYUK_DISABLED=true`
- GCP: project `steloit-dev`, billing `016006-61AFB9-0DD7E7`. `gcloud` defaults to
  a different project — pass `--billing-project=steloit-dev`; never change global config.

## Known risks (all filed)

`O19` high — `MaxMonthly` bounds one service-month while `Rollup` accumulates
org-wide · `O20` high — detection shipped, no remediation for rows the old
overflow wrote · `O28` medium — the container gate is negative-only · `O21`
medium · `Q11` medium — a reveal-once webhook test failed once under `-race` ·
`O18` low. Stray tracked empty file `300-line` at the repo root (from `c3f0876`,
O24) — harmless, unowned, delete when convenient.

**Container-suite flakiness is a PATTERN now, not one test.** Three separate
local failures this session, each in a package the diff did not touch, each
passing on a clean re-run: `TestConnectEnforcesThePostgres16Floor`
(platform/db — "connection refused" then "invalid body length" against a
just-started container) and `TestEmailOutboxDeliversInvite` (identity), plus
`Q11`'s earlier one. CI has not reproduced any of them. Before treating one as a
real failure: re-run the single package twice and check whether the diff touches
it at all. Not yet filed — no diagnosis, and a flake report without one is
noise; if a fourth appears, file it with the three timestamps.
