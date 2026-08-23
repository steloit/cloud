# Autonomous session state

A **small current-state handoff**, not a diary. **Not authoritative** — the repo,
git, CI and `tasks/` are. Verify before trusting; correct this file when wrong.
Durable lessons live in `contexts/provisioning.md` (mistake bank) and `AGENTS.md`,
never here.

**Updated:** 2026-08-23 · `main` @ `db3cfdf` · CI green · **4 open PRs**

---

## Current objective

Work the `tasks/` ready queue under the AGENTS.md protocol: claim → implement →
`verify:` → **both reviewers (ADR-0008)** → CI → merge. The queue is the plan.

## In flight

| PR | task | state |
|---|---|---|
| **#320** | US-3.3a | round 11 pushed, CI green. Eleven rounds, all blocking, all correct. |
| **#322** | T3.4c | round 3 pushed, CI green. |
| **#323** | US-3.3f | NetworkPolicy ENFORCEMENT (Dataplane V2). CI running. |
| **—** | US-3.3e | `task/US-3.3e`, stacked on US-3.3a. Reviewer returned **5 blockers**; QA still running. Do not edit that worktree until it reports. |

**Stacking:** US-3.3e branches from US-3.3a. Merge order must be
US-3.3a → US-3.3e; US-3.3f and T3.4c are independent.

## Next action

Land #322 (T3.4c) and #320 (US-3.3a) once their reviews report. Then take the
next `high` from the ready queue: **O19**, **O9**, **T1.4a**, **US-10.7**.

## Blocked on a human

| # | decision | where |
|---|---|---|
| 1 | **O2 cost guardrail.** The founder gave ₹1,000/mo at 50/80/100 and "use the existing ops recipient", then said an **updated spec is coming**. Nothing has been written for O2 — `task/O2` worktree is clean. WAIT for it. | `tasks/eops/O2.md` |
| 2 | **A SIZE downgrade's price** while storage is retained (T3.4c). Not ruled; three options recorded with market evidence. | `docs/founder-config.md` §5 |

**Answered 2026-08-23:** T3.4c `included_gb` semantics · ADR-0014 **ratified**
(merged, #321) · US-3.3e envelope (free 1/2Gi/10Gi · pro 8/16Gi/100Gi ·
business 12/24Gi/200Gi · enterprise 16/32Gi/250Gi, per environment).

## GCP access is DEGRADED

`gcloud` is now active as `admin@humanetechnologies.in`, which lacks
`container.clusters.list` on `steloit-dev`; `hashir@humanetechnologies.in` needs
an interactive reauth (`! gcloud auth login`). So the earlier "zero GKE clusters"
evidence **cannot currently be re-verified** — do not build reasoning on it.
US-3.3e's backfill is written to be correct either way rather than assuming an
empty fleet.

## Do NOT redo

- **US-3.3a's namespace half.** `services/api/go.mod` has **no `k8s.io/*`**, so
  control-plane-side creation is an architecture delta (D6/ADR-0001). Agent-side,
  in `internal/driver/tenancy`, applied before the service manifests.
- **D7's policy objects were withdrawn on purpose** to US-3.3c/d/e. NetworkPolicy
  is **not enforced** on the cell (`infra/modules/gke-cell` is GKE Standard, no
  `network_policy`, no `ADVANCED_DATAPATH`), the allow-set would fence CNPG off
  Workload Identity / GCS / the apiserver, and the LimitRange would cap every
  managed Postgres at 512Mi. `tenancy.Render`'s package doc records why.
- **There are ZERO GKE clusters in `steloit-dev`** (`gcloud container clusters
  list --format=json` → `[]`, exit 0; `projects describe` proves the credential
  works). `infra/modules/gke-cell` has never been applied. So US-3.3a's AC 3
  ("proven without the runbook's preflight") is **not** provable yet, and
  US-3.3c's pod-to-pod assertions need a cell first.
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

## Conventions

- Worktree per task; `git status` in it **before** removing it.
- After pushing: `git rev-list --left-right --count origin/<branch>...HEAD` → `0 0`.
- Reviewers by name: `subagent_type: "reviewer"` and `"qa"`. Worth the wall clock.
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
