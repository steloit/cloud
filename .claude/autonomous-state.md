# Autonomous session state

Handoff between autonomous Claude Code sessions. **Not authoritative** — the repo,
git, CI and `tasks/` are. Verify before trusting; correct this file when it is wrong.

**Last updated:** 2026-08-23 (session 2) · `main` @ `695b4c4` · CI **green**

---

## Current objective

Work the `tasks/` ready queue under the AGENTS.md task protocol: claim → implement →
`verify:` → **both reviewers (ADR-0008)** → CI → merge. No standing objective beyond
that; the queue is the plan.

## Current phase

**Session 2: gate-hardening + the two investigations.** Four PRs in flight, none
merged on my own assessment — each waits for its review.

---

## Next concrete action

**Land the four open PRs** (below), then start `US-3.3a`.

| PR | task | state |
|---|---|---|
| #317 | O29 — 23 CI gates pinned | CI running. **Two review rounds, both BLOCKING, all applied.** Every mutation class individually RED |
| #318 | O2 — comparison + `currency_code` deleted | CI running. **Two review rounds, both applied.** `plan` now refuses without the amount |
| — | O20 | ✅ merged (#319): tested detection queries, docs+test only |
| — | T3.4c | ✅ merged (#315): ambiguity recorded, task stays `ready`, no code |

**Start `US-3.3a`** (`tasks/e3-provisioning/US-3.3a.md`, `high`) — nothing creates an
environment's Kubernetes namespace, nor its D7 default-deny NetworkPolicy /
ResourceQuota / LimitRange. Tenant isolation is nominal until it lands.

Two things to settle first, both stated in the task:
1. **Namespace creation goes control-plane-side at env creation, or agent-side during
   converge — the task says decide and record why.** That is an engineering decision,
   not a founder one.
2. Its `verify:` block is prose, not commands. Replace with executable checks.

Do **not** pick `T3.4c` — see "Blocked on a human" below.

---

## Blocked on a human — do not guess these

| # | Decision | Where |
|---|---|---|
| 1 | **T3.4c**: what does a `standard` postgres with `storage_gb` unset receive? Price says 50 GB included; the AC implies 0. Cannot both hold, and it changes what customers get. Both readings written up. | PR **#315** (open, CI green) |
| 2 | **ADR-0014 ratification.** Code is on the trunk; `Status: Proposed`. Flipping it would forge a founder decision. The ADR states declining does **not** imply a revert. | `docs/adr/0014-*.md` |
| 3 | **O2 cost guardrail.** Terraform (50/80/100% monthly budget), the task (50/80/100% of the $300 trial credit) and live GCP (`steloit-dev tripwire`: 50/90/100% of ₹1000/month, empty `notificationsRule`) describe three different alarms. | `tasks/eops/O2.md` (kept `stub` with facts recorded) |

---

## Session 2 additions — do NOT redo

- **Audited all five CI gates against a real run log**, not `ci.yml`. All five
  EXECUTE. The container gate is proven by behaviour (identity 350.8s, reconcile
  146.5s, db 31.1s, 0 SKIP) because its env var is masked in logs.
- **O29** — four of five gates could be deleted from `ci.yml` with a green suite.
  Now parsed with `yaml.v3` and asserted on `steps[].run`/`.env` + no `if:` on the
  job + `pull_request` trigger. **Twelve mutation classes red.** Two earlier
  versions were theatre (whole-file text match; then whole-line-comment strip,
  defeated by an *inline* comment plus an emptied env value).
- **O20** — `docs/dev/money-range-audit.md` + a test that EXTRACTS the SQL from the
  doc and proves it finds seeded poison in all five money locations. My first
  query used `services.org_id`, which does not exist (chain is
  services→environments→projects).
- **O2** — the module is **INERT** (`billing_account` defaults `""`, no tfvars sets
  it), so there is no drift: a dormant module and a hand-made budget that never met.
  `currency_code` is **DELETED** (the server supplies the account currency — the
  discovery doc says "provided on output"; setting it is the only way to fail). My
  first fix made it a variable, which REMOVED a guard: `INR` + `units=300` = ₹300/mo
  ≈ $3.60. The amount is now a required env variable with no default, so `plan`
  refuses until it is chosen alongside `billing_account`.
- **O2 has SIX founder decisions now, not four** — the amount/currency pairing and
  `alert_emails` were missed. With currency server-supplied the direction of the
  amount divergence FLIPS: live's ₹1000 is ~3.3× larger than terraform's ₹300.

## Completed in session 1 — do NOT redo

12 PRs merged (#305–#314, #316). Tasks 101 → 116 done; stale `in-progress` 3 → 0.

**Live defects closed** (all verified against merged `main`, not inferred from diffs):
- **O16** — integer overflow let one authenticated request permanently disable an org's
  hard spend cap (`postgres {storage_gb:1e18}` → `-5340232221128652948`, persisted, and
  `enforceBudget` projects against `SumOrgMonthlyEstimate`).
- **Q10/O14** — data race in `reconcile`'s test doubles; killed the test **binary** in
  ~1% of plain runs via `fatal error: concurrent map read and map write`.
- **O22** — duplicated `cost_guardrails` terraform module had CI's `infra` job red, so
  cell0 validation and all `infra/k8s/**` parsing had never run.

**CI gates built — none existed before:**
| gate | note |
|---|---|
| `make gen-sql` in the drift step | O7; also sweeps stale output so a *deleted* query is caught |
| `gofmt -l` per module | O13; list **derived** from `git ls-files '*/go.mod'` |
| `-timeout 30m` | O13; default is 10 min **per package** and identity measures ~350s |
| `-race` | O13; **had never run in this repo**. Runs ~9 min on the hosted runner |
| containers actually started | O23; `STELOIT_REQUIRE_CONTAINERS` — pinned to `ci.yml` by a test |

**Also:** O15 (composition-root worker registry), O24/O27 (task-state drift; T3.4 was
`critical`/`ready` but had shipped in #298), O25/O26 (see "What went wrong").

---

## Architectural decisions recorded

- **`money.Cents` is a struct, not `type Cents int64`** — Go will not compile `a + b`, so
  arithmetic can only go through checked methods. `MaxMonthly` is **derived** from a full
  billing month because `metering.Rollup` multiplies an accepted rate by a month of
  seconds. (ADR-0014, Proposed.)
- **`problem.FromDenial`** is the single denial→response mapping. It lives in `problem`
  because `events` cannot import `identity` (`identity` imports `events`).
- **O23's gate is one decision in one place** (`testenv.SkipOrFail`) — three helpers each
  making it independently is how one quietly stops.
- `secondsInLongestMonth` is **unexported**; `Cents.SurvivesBillingMonth()` answers the
  question without handing out the constant.

## Discoveries worth keeping

- **Container runtime is required for real verification.** Without it the suite prints
  `ok` having skipped. Use:
  `DOCKER_HOST=unix://$HOME/.colima/default/docker.sock TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock TESTCONTAINERS_RYUK_DISABLED=true`
- **GCP access is available**: project `steloit-dev`, billing `016006-61AFB9-0DD7E7`.
  `gcloud` defaults to a *different* project — pass `--billing-project=steloit-dev`
  explicitly; do **not** change the user's global config.
- Full `services/api` suite: ~4 min plain, ~9–10 min under `-race`. `identity` alone is
  250–500s and is the binding package for the timeout.

---

## Known risks / defects (all filed, none silent)

| task | pri | what |
|---|---|---|
| **O19** | high | `MaxMonthly` bounds one service-month; `Rollup` accumulates across every service in the org. Two services at the ceiling wrap `quota_usage.rate_cents`. Pre-existing. |
| **O20** | high | O16 closed the ingress, shipped no detection for rows the old overflow already wrote. Failure mode changed from silent-permissive to silent-**blocking** (permanent 409). |
| **O28** | medium | O23's gate is negative-only: with a runtime present and the gate armed, `t.Skip("quarantined")` still yields `ok` with zero containers. Also `ckm3_checkpoint_test.go` panics in `t.Cleanup`, aborting the whole binary (~100 identity results lost). |
| **O21** | medium | `reconcile/http.go`'s `problem.Carrier → 409` arm is deletable with the whole suite green. |
| **Q11** | medium | `TestIdempotentCreateWebhookReplaysTheRevealOnceSecret` failed once under `-race`; output not preserved. Observation, not a diagnosis. Reveal-once is a security property. |
| **O18** | low | Nothing detects duplicate tasks (Q10/O14 were the same defect, both `ready` for a month) nor a `ready` task whose named tests all already exist (T3.4). |

---

## What went wrong — read this before working

Reviews caught defects in my own work on **six** branches. Three shared one cause:
**verifying one representation of something that has two.**
- pinned 1 of 5 fail-closed guards while writing "all five re-verified"
- wrote a test with only positive assertions — in the fix for an only-positive test
- drove one disjunct of a two-disjunct condition

Also, and worse:
- **Lost an entire review round.** Made fixes in a `git worktree`, never committed;
  `git push` sent only the pre-existing commit, the PR merged that, and
  `worktree remove --force` destroyed the rest. **The merged PR description claimed work
  it did not contain.** Corrected publicly on #310; O26 re-landed it.
- Ran fault injection against the **working tree** (a failed `cd` left a stale cwd).
- Nearly reverted all of O13 by branching from a stale `main` — caught by reading the
  diffstat against `origin/main` before pushing.

**Rules now in `AGENTS.md` (with *measured* tells — my first version stated two that were
false):** a worktree is not a commit; the tell is that plain `git worktree remove`
**refuses** and names `--force`. Needing `--force` *is* the warning. Fault injection goes
in a `cp -R` copy, absolute paths only.

**Operating consequence:** do not merge on your own assessment of a review you requested.
Wait for the re-verification. A green suite is not evidence a test *discriminates* — and a
mutation result is only evidence about the **invocation it was measured at** (18/100 →
3/100 → 100/100 at CI's plain `go test`, while `-count=50` looked fine throughout).

---

## Conventions that saved time

- Work in a `git worktree` per task; check `git status` **before** removing it.
- After pushing: `git rev-list --left-right --count origin/<branch>...HEAD` → `0 0`.
  (Necessary, not sufficient — it compares commits, so it cannot see uncommitted work.)
- Reviewers: `subagent_type: "reviewer"` and `"qa"`, **by name**. They are worth the wall
  clock; they found real defects every single time.
- `node scripts/spec-sync/validate.mjs` after any `tasks/` edit.
- Context packs have a hard budget (`contexts/*.md`, 150 documented / 160 enforced — O12
  owns that discrepancy). Compress an existing entry to fund a new one.
