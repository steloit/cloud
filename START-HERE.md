# START-HERE — Autonomous Implementation Handoff

**Canonical session handoff. Read this completely before doing anything.**
Updated: 2026-07-20 · Reflects repository state at `main` (post-PR #282).

This document — not prior chat history — is the source of truth for continuing
implementation. Update it at every major milestone.

---

## 1. Current Project State

| Fact | Value |
|---|---|
| Roadmap progress | **86 of 182 tasks `done`** (`node scripts/spec-sync/validate.mjs`) |
| Current milestone | **E11 Billing flagship stories complete**; E10 Observe in progress |
| Active branch | **`main`** (clean, no work-in-progress branch) |
| Last merged PR | **#282** (US-10.2) |
| CI | Green on `main` (`go`, `infra`, `validate`) |

**What is built (control plane):** identity/auth/orgs/members/invites/API-keys ·
RBAC two-layer evaluator · projects/environments/services/bindings/deployments ·
estimates + estimate-before-provision gate · metering spans + rollup · billing
(plans, quotas, subscriptions, invoices, budgets, spend cap) · policies +
governance · templates · dashboards · assistant thread store · notifications
(bell/prefs/webhooks) · console SPA (canon mode + real-API slices) · GCP
foundation (Terraform) + CNPG substrate productionized.

**What is NOT built:** the runtime/execution plane (cell agent, reconciler,
CNPG driver behind the provisioner), the observability data plane (Loki/OTel —
T1.5), the data plane (SQL exec, backups/branches API), payment provider
(Stripe, T11.4), AI inference (T13.6).

**Consequence:** tasks needing a live runtime or observability pipeline
(T14.1, T14.6, US-10.5 traces, O3 abuse controls) are effectively blocked on
E1/T1.5 even where `deps: []`. Prefer control-plane work.

---

## 2. Next Work (exact execution order)

### 1. US-10.3 — Notifications family
- **Objective:** close the remaining notification-family surface.
- **Deps:** S4 ✅, T10.3 ✅ (both done).
- **Already shipped (T10.3):** `listNotifications`, `markNotificationsRead`,
  `get/updateNotificationPrefs`, `listWebhooks`, `createWebhook`.
- **Known gaps:**
  1. `testWebhook` is in `openapi.yaml` but **not** in `include-operation-ids`
     (unimplemented). The spec path is `/webhooks/{wbh}:test` — **the Go 1.22
     stdlib mux rejects a wildcard-then-colon** (same failure as T12.7's
     `:fork`). Spec-change-first to a sub-path `/webhooks/{wbh}/test`,
     regenerate, then implement. `notify.Router` already has the synthetic
     `webhook.test` event path (`notify.go:202`).
  2. `deleteWebhook` — verify presence; implement if missing.
  3. AC invariant: **unread notification count consistent with the events spine.**
- **Expected deliverables:** the endpoint(s) + a self-verifying invariant test
  for unread-count consistency + webhook delivery verification.

### 2. US-10.4 — 12 transactional emails on one skeleton
- **Objective:** one email skeleton; subject grammar `[org/project] <fact>`;
  one event → one email → one action; **security emails unmutable**; a
  banner/email number mismatch is a defect.
- **Deps:** T10.4 ✅ (mailer done).
- **Known gaps:** the 12 templates + the numbers-match-the-console invariant.
- **Expected deliverables:** templates on one skeleton + an invariant test that
  email numbers equal the console/banner numbers (canon-sourced, never retyped).

### 3. T14.8 — Remaining spec-closure endpoints (§2b/2c/2d rollup)
- **Objective:** bindings `:rotate`, domain recheck/delete, schedule/lifecycle
  per-row ops + `:run`, deployment `:pause`/`:abort`, org/project `:transfer`,
  git integration, cell `:drain`/`:detach`.
- **Deps:** none.
- **Known gaps:** **large — each item is an owner-level contract addition.**
  Sequence by blocked-surface count (proposals §9). Several `:action` paths will
  hit the stdlib-mux wildcard-colon limit → use sub-paths.
- **Expected deliverables:** spec-change-first per endpoint, regenerate, implement,
  test. **Do one coherent slice per PR**, not the whole rollup at once.

### 4. Then, remaining dependency-ordered items
Control-plane candidates: `US-11.3` (proration remainder), `US-13.4` (AI disable
across every surface), `T8.1`/`T8.3` (console pipeline + four-state harness),
`Q5` (UI consistency), `O6` (see §4). Runtime-blocked: `T14.1`, `T14.6`,
`US-10.5`, `O3`, `US-1.x`, `T3.4`.

---

## 3. Standing Engineering Rules (permanent)

1. **CLOSED means implemented behavior or an enforcement path that actually
   exists** — never merely an available primitive.
2. **PARTIAL** = primitive exists, wiring incomplete → keep the follow-up open in
   the ledger with the reason recorded.
3. **Deterministic CI only.** A flake, lint, or validate failure is a **defect to
   root-cause**, never a signal to retry until green.
4. **Eliminate flaky tests** at the root (pin clocks, remove wall-clock/ordering
   dependence).
5. **Evidence over assumptions.** Measured numbers, committed transcripts, tests.
6. **Never overclaim completion.** Record findings; never resolve silently.
7. **Requirements become structural guarantees** wherever practical — tripwires,
   invariants, deterministic tests, measurable benchmarks.
8. **Reviews are adversarial, not ceremonial.**
9. **Repository review definitions are authoritative** (`.claude/agents/`).
10. **No Kernel or external reviewer plugins** — evaluated and rejected
    (`docs/plan/kernel-workflow-review.md`; ADR-0008 fixes reviewer identity).

**Repo hard rules that bind every change** (see `AGENTS.md`, agent-guide §3/§4):
never invent an endpoint/component/term (spec-change-first in `openapi.yaml`);
API types generated, never hand-written; money is integer cents (ADR-025);
problem+json with `remediation` always; microcopy verbatim from frames; demo data
from `19-canon` only; `docs/product/00-sources/**` and `18-philosophy/decisions.md`
change by **human decision only** (hook-enforced).

---

## 4. Review Process (ADR-0008 — mandatory before merge)

```
Implementation Agent → Architecture Reviewer → Security/QA Reviewer → CI → Merge
```

- **Architecture Reviewer** = `.claude/agents/reviewer.md` (all six dimensions:
  architecture, security, performance, API consistency, contract drift, ADR).
- **Security/QA Reviewer** = `.claude/agents/qa.md` (test gaps, edge cases,
  regression risk, fuzz targets).
- Both are **read-only** and **independent**. Blocking findings are fixed and
  **re-verified**; non-blocking findings are **recorded in the ledger**.
- CI is the **final** gate, never the first line of defense.
- "Significant" = changes behavior, adds a feature, touches
  security/authz/money/contracts, or modifies CI. Pure typo/doc edits are exempt.

**Agent invocation — check this first:**
The repo-native agents now appear registered by name in the agent registry
(`reviewer`, `qa`, plus `docs`, `research`). **Try `subagent_type: "reviewer"`
and `subagent_type: "qa"` directly.**

**Fallback workaround (only if the named agents fail to resolve):** run
`subagent_type: "general-purpose"` and paste the **verbatim contents** of
`.claude/agents/reviewer.md` / `qa.md` into the prompt, instructing read-only
behavior. This was the documented workaround while `O6` was open. **Never**
substitute `kernel:*` or any plugin reviewer.

**O6 (agent registration)** is `ready` in `tasks/eops/O6.md`: repo agents
first-class, precedence over global plugins, **fail-fast on unknown agents (no
silent fallback)**. Named resolution now appears to work; the precedence and
fail-fast clauses remain unverified — close O6 only when verified.

---

## 5. Decision Ledger (unresolved — founder-gated; do NOT invent)

| Item | Status | Why blocked | Implementation impact |
|---|---|---|---|
| **Dashboard share-grant model** | Open | §2c names "share grants" with **no shape**; no restricted-share member model exists (`restricted` = owner-only, T12.6). Designing the grant model is a product decision (agent-guide §4). | Dashboards ship with personal/org/restricted visibility only. `restricted` behaves as owner-only. No sharing-with-specific-people surface. |
| **Prebuilt-dashboard topology generation** | Open | "Generated views, always current" requires the service topology + metrics (data plane). | T12.7 ships prebuilt dashboards as **read-only, forkable** rows; generation from topology is unimplemented. Prebuilt rows must be seeded. |
| **B3 invoice-fixture numbers (S5 ruling)** | Open | `inv_2026_06` lines sum to **41,492¢** but the printed total is **47,700¢**; its GST is 31.8%, not 18%. Which value is authoritative (itemization vs printed total) is an **S5 canon ruling**; the mock deliberately carries it "as a finding, not silently fixed". | US-11.6 enforces the §74 lines-sum rule everywhere **except** this fixture; the console invariant test excludes `inv_2026_06` via a `B3_PENDING_S5` marker. Remove the marker when S5 rules. |

Ledger location: `docs/product/claudedocs/spec-change-proposals.md`.

---

## 6. Open PARTIAL Items (primitive exists, wiring incomplete)

| Item | Why PARTIAL | Remaining work | Owning task |
|---|---|---|---|
| `billing.GateCapability` | Consults `IsNeverGated` before a plan gate (table-tested), but has **zero callers** — no capability→plan gate exists anywhere (the only `PlanGated` site is the project-**limit** gate, not a capability). | Bind it at the first real capability gate (SSO, audit-export…) when one lands. | US-11.1 → US-11.2 |
| `quota.ClampOverageToCap` | Halt primitive (table-tested incl. boundary) but **not wired** into the rollup/`Evaluate` path — a running service's soft overage is not clamped in production. | Requires a semantic decision first: US-11.7 says "running services untouched", so **where** soft overage is billed-vs-halted at the cap must be ruled, then wired. | US-11.7 → US-11.2 |
| `notify.NotifyInput.Urgent` | Bypasses quiet hours (tested), but **no security/paging notification source** sets it. Also: it currently still respects a member's **email-off** pref — glossary/design-spec P6 ("escalation paging is org policy") arguably implies a security page should override email-off too. | The E7/auth security-notification source sets `Urgent: true` **and** rules the email-off-override question explicitly. | US-10.2 → E7/auth |

---

## 7. Architectural Invariants (established, enforced by tests)

- **Architecture is FROZEN** (`docs/architecture.md` v1.3, ADR-0001/3/4/5):
  Go/stdlib-http/sqlc/River/REST+SSE; product surface exactly
  `[postgres, valkey, web, worker]`; queue = pgmq capability; storage/AI =
  external Bindings. Deltas require an ADR + implementation/customer evidence.
- **Storage substrate:** CNPG on **GKE PD-CSI (`pd-balanced`) + VolumeSnapshots**
  at dev/alpha (ADR-0007/ADR-042, INF-001 A6). ZFS-LocalPV re-scoped to a Cell-1
  density option, not removed. CNPG **≤1.30 is a hard ceiling** (in-tree Barman
  removed in 1.31).
- **Branching guarantees:** branch / PITR / restore-**never**-in-place /
  hibernation / delta-priced snapshots — unchanged by the substrate delta.
- **Integer-cent money end-to-end** (ADR-025) — estimate, invoice, quota, cap.
- **Invoice grammar == estimate grammar** (§74): every line is a **stated plan
  allowance** or a **metered quantity at a published unit price**; lines sum to
  the subtotal. Enforced at estimate, invoice, and console layers.
- **Hard spend cap is a real bound** (F9): an over-cap provision is refused at
  accept time with **402 `quota_exceeded`** + the arithmetic, on **create and
  scale**; every cap hit emits `billing.spend_cap_reached` on the spine.
- **Cancel ≠ Delete** (B12): cancelling changes the **plan**, never the
  **resources**. Metering keys off **resource** status, so cancel cannot stop a
  running service's billing span. One-click way back (`/subscription/reactivate`).
- **Plans gate capabilities, never safety** (B6/F9): the never-gated set is
  drift-proof (set-equality) — `tls, backups, mfa, policies, alerts,
  dunning_protections, self_deletion`.
- **AI four laws** (ADR-005): no auto-apply path exists in the API; every
  `/assistant/*` handler must call `AIAssistantEnabled` (structural tripwire) and
  return **404-invisible** when disabled — deleting nothing, restoring instantly.
- **Quiet hours affect routing only** — never recording, never firing.
- **Two-layer RBAC**: matrix ceiling then tighten-only policy (ADR-0006/036).
- **Errors** are problem+json from the closed `x-error-catalog`; a new error
  class is an S-process contract change, never a local addition.
- **Deterministic CI**: no wall-clock-dependent tests (clocks are injected via
  `WithClock` seams).

---

## 8. Evidence Ledger (measured, reproducible)

**Substrate spike (ADR-0007, live GCP; transcripts in `infra/spike/results/`, provenance in `PROVENANCE.md`):**

| Measurement | Value |
|---|---|
| VolumeSnapshot create (10 Gi) | **34.6 s** |
| Branch e2e (snapshot → CNPG recovery → accepting connections) | **52.4 s** |
| Hibernation wake | **8.0 s** |
| Restore to a NEW cluster (base backup + full WAL replay, 1,320,000 rows verified) | **55.2 s** |
| RPO | **≤ 300 s by construction** (`archive_timeout`) — a bound, not a kill test |
| Incremental snapshot delta (10%-divergence Dev branch) | **~62–65 MB** |

**Structural guarantees / tripwires (tests that fail on regression):**
- `TestEveryAssistantHandlerGatesOnPolicy` — every implemented `/assistant/*`
  handler calls `AIAssistantEnabled` (+ a proven **red state**).
- `TestEnforcedListMatchesSource` — the enforced-permission list equals the set
  actually enforced in handler source.
- `TestSafetyNeverGated` — never-gated set-equality (both directions).
- `TestSafetyOperationsNeverPlanGated` — MFA/self-deletion handlers reference no
  plan gate; fails loudly if a required file is renamed (never inert).
- `TestQAScenario3Dunning` / `TestQAScenario4DowngradeBlock` — QA scenarios 3 & 4
  in **~4 s** via the Q9 warped-clock harness (vs real days).
- `TestBudgetCapAndExports` / `TestBudgetCapBoundaryAndUncapped` — cap blocked-not-
  warned, ±1¢ boundary, uncapped-free, cap event payload.
- `TestCancelDoesNotStopServicesOrMetering` / `TestCancelledOrgStillBilledAtRollup`
  — Cancel ≠ Delete at the span **and money** layers.
- `invoice-lines-sum.test.ts` + `TestInvoice…` + `TestEstimateLineGrammar` — §74
  grammar at console, invoice, and estimate layers (console has a proven red state).
- `TestQuietHoursRouteOnlyNeverRecordOrFire` — clock pinned **inside** the quiet
  window (deterministic).
- `TestTemplateRefreshVersioning` — version mints, `used_by_count` preserved.
- `tests/realtime.test.ts` — SSE degrade/backoff/recovery with a virtual clock.

---

## 9. Session Summary (factual)

**Roadmap tasks merged (14):** T12.4 (#266), Q9 (#267), T8.2 (#268), T13.3 (#269),
T12.6 (#270), T12.3 (#271), T12.7 (#274), US-11.5 (#275), US-11.1 (#276),
US-11.7 (#278), US-11.6 (#279), US-11.2 (#280), T12.5 (#281), US-10.2 (#282).

**Workflow/quality PRs merged (3):**
- **#272** ADR-0008 reviewer identity fixed to repo-native `reviewer`/`qa`;
  external/plugin reviewers (incl. `kernel:*`) forbidden.
- **#273** O6 filed — repo agents first-class, precedence over plugins, fail-fast.
- **#277** `TestNotifyRouting` wall-clock flake eliminated (it failed **every** run
  between 22:00–07:00 UTC because it set quiet-hours then asserted email delivery
  against the real clock; fixed by pinning the router clock).

**Major defects caught pre-merge by review:** open-bag template shapes could smuggle
secrets (T12.4); a scheduled downgrade **erased a pending cancel** (Q9); the SSE
channel went **permanently dead ~30 s after every mount** in canon mode (T8.2); a
non-member could learn org-existence + AI-state and pollute another tenant's audit
spine (T13.3); an owner's **read-only token could edit/delete dashboards** (T12.6);
a read-only token could **write via `/fork`** (T12.7); a ledger overclaim marking
unwired primitives "CLOSED" (US-11.2); an estimate-layer parity claim that was
untested (US-11.6).

**Architectural changes:** ADR-0007 ratified (substrate → PD-CSI; architecture.md
v1.3; INF-001 A6; ADR-042 in decisions.md); ADR-0008 amended (reviewer identity);
contract additions — subscription `/reactivate`, dashboards `/fork` `/duplicate`
`DELETE /widgets/{wdg}`, assistant `createThread`/`listThreads`, dashboards CRUD,
templates CRUD, `updateService` 402.

---

## 10. Startup Prompt (copy-paste for the next session)

```
Read START-HERE.md at the repository root completely before doing anything.

Adopt every standing engineering rule in §3, the review process in §4, and treat
§5 (decision ledger) and §6 (PARTIAL items) as binding — do not invent behavior
for any founder-gated item, and do not mark a PARTIAL item CLOSED until its
wiring genuinely exists.

Verify the current roadmap position:
  git checkout main && git pull --ff-only
  node scripts/spec-sync/validate.mjs

Then continue autonomous implementation from the first unblocked roadmap item in
§2 (currently US-10.3), in the listed dependency order.

For every significant PR run the mandatory ADR-0008 pipeline: two independent
READ-ONLY reviews — Architecture (.claude/agents/reviewer.md) and Security/QA
(.claude/agents/qa.md). Try subagent_type "reviewer" and "qa" by name first; if
they fail to resolve, use "general-purpose" carrying those files' verbatim
instructions. NEVER use kernel:* or any external reviewer plugin. Fix blocking
findings, re-verify, record non-blocking findings in
docs/product/claudedocs/spec-change-proposals.md, and merge only on green CI.

Keep CI deterministic: any flake, lint, or validate failure is a defect to
root-cause, never a reason to retry until green. Turn requirements into
structural guarantees (tripwires, invariants, deterministic tests, measured
numbers) wherever practical, and leave that evidence behind at each milestone.

Do not pause for routine progress. Interrupt only for:
  - a founder decision
  - an external blocker
  - a critical architectural discovery
  - a major roadmap milestone

Update START-HERE.md at every major milestone so future sessions never depend on
chat history.
```
