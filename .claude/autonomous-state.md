# Autonomous session state

A **small current-state handoff**, not a diary. **Not authoritative** — the repo,
git, CI and `tasks/` are. Verify before trusting; correct this file when wrong.
Durable lessons live in `contexts/provisioning.md` (mistake bank) and `AGENTS.md`,
never here.

**Updated:** 2026-08-24 · `main` @ `779f9ea` · **US-3.3h MERGED (#328)** · T3.4d (#329) in flight

---

## Current objective

Work the `tasks/` ready queue under the AGENTS.md protocol: claim → implement →
`verify:` → **both reviewers (ADR-0008)** → CI → merge. The queue is the plan.

## In flight

**T3.4d — PR #329**, branch `task/T3.4d`, worktree `.../442530ba-.../scratchpad/wt-t34d`.
Round 2 pushed, main merged in and the merged tree verified. Both reviewers have
run once (reviewer: APPROVE-WITH-NOTES; QA: one blocking, now fixed and
mutation-verified). **Waiting on CI, then merge.** No third pass — round 2 was
fixes to named findings (ADR-0016).

## Recently merged

**US-3.3h (#328)** — four rounds. The control plane maps a cell's report onto a
status legal from the current one, and convergence is **derived from the
destination** rather than written per hop:
`converged = settledStatuses[to] && (to == observed || observed == "")`, where
`settledStatuses` = {ready, failed}. That one rule subsumed four hand-written
flags and closed a class both reviewers kept finding one instance of at a time.

Three things from it that are easy to re-break:

1. **A cell may not decide a lifecycle hold, in EITHER position.** `deleting`/
   `suspended` as an *observation* are refused (`ReportableByCell`, plus a 422 at
   the route). And a *held row* converges only on `gone` — because
   `DeleteService` bumps the generation and only then transitions, so a deleting
   row is outstanding by construction, and one `ready` report used to finish the
   generation and abandon the teardown forever.
2. **`gone` and `""` are different** and the handler no longer collapses them.
   `gone` from a live row means the workload vanished while desired still wants
   it — the row stays outstanding so the agent re-creates it.
3. **`provisioning.StatusVocabulary` is the one ADR-024 definition.** It was
   retyped in four places; reconcile's wire gate and the sweeps derive from it.

## Next task after T3.4d

Ready queue (`grep -l "^status: ready" tasks/**/*.md`): **US-3.3b** (env teardown
— nothing removes a namespace), **US-3.3g**, **US-3.10**, **US-3.11**, plus the
two US-3.3h filed:

- **US-3.12** (high) — the reconcile status contract advertises `suspended`/
  `deleting`, which the API now **422s**, and an "omit for a heartbeat" clause
  that no longer holds. Real contract drift; needs `openapi.yaml` + regenerated
  types, which is why it was not done inline.
- **US-3.13** (medium) — a row whose `observed_generation` never advances has no
  owner. A steadily `degraded` service 409s forever (measured: 25 ticks, no
  convergence). **US-3.11 does NOT cover this** — its ACs are about the *agent's*
  render errors. Not reachable today (no CNPG phase maps to `degraded`) but
  `render.terminal()` already accepts it, so one phase-mapping change makes it
  live.

**US-3.3c is priority `critical` but two of its ACs need a live GKE cell**, and
there are still zero clusters in `steloit-dev`. Do not claim it expecting to
finish it.

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
