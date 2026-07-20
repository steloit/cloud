# START-HERE — Engineering Operating Manual

**The single source of truth for autonomous implementation.** Optimized for a
session with **zero chat history**. Read it completely before doing anything.

**Maintenance contract:** update this file at every major milestone, and keep it
**net-flat — to add something, remove something.** It is ~520 lines and must not
drift past **550**; if an addition would, delete the content it supersedes first.
Per-PR narrative belongs in `git log`, not here. If a fact can be derived by
running a command, print the command instead of the answer — a stale number is
worse than none.

---

## 1. Session Contract — how every session starts

Run these, in order, before any implementation:

```sh
# from the repository root
git checkout main && git pull --ff-only
git status --short                    # must be clean
node scripts/spec-sync/validate.mjs   # graph valid + ready-unblocked count

# the real roadmap position — MUST exclude _epic.md/_template.md, which carry
# their own `status:` and are not work items (a plain grep over tasks/ overcounts)
for f in $(find tasks -name "*.md" ! -name "_epic.md" ! -name "_template.md" ! -path "*_archive*"); do
  grep -m1 "^status:" "$f"; done | sort | uniq -c | sort -rn   # sums to 182

git ls-remote --heads origin 'task/*' # LOCKS LIVE ON ORIGIN — see §2
```

`git branch --list` is **local-only**: on a fresh clone it prints nothing and you
would restart locked work. A lock is a **pushed** branch.

Then:

1. **Read this document fully.** Adopt §5 (rules), §6 (review), §7 (traps).
2. **Confirm position** against those commands. If they disagree with §2/§3,
   **the repository wins** — this file is stale: fix it as part of your work.
3. **Pick the first item in §3 order** that is not runtime-blocked (§2).
4. **Check its `status:` — this determines what you do next:**
   - `ready` → claim and implement.
   - **`stub` → you must enrich it first (§1a). Never implement a stub.**
   - `in-progress` → a lock exists; resolve it per §2 before touching it.
5. **Claim**: branch `task/<id>`, flip `status: in-progress`, commit, **and
   push**. The *pushed* branch is the lock — an unpushed branch locks nothing.
6. Load the task file → its `contexts:` packs → its Read-first list. Stop there.
   Do not read the whole spec package.
7. Implement, then satisfy §4's Definition of Done before opening the PR.

**Never**: work on `main`; implement a stub; skip review on a significant PR.

### 1a. The enrichment gate (applies to nearly all remaining work)

**88 of 182 tasks are `stub`. Only 1 is `ready`.** A stub is a placeholder — it
has no acceptance criteria, `files:` globs, or `verify:` block, so there is
nothing to implement against and no way to prove done. `AGENTS.md` therefore
forbids starting one.

**Enrichment is its own reviewable PR**, done with the `spec-author` skill
(`.claude/skills/spec-author`): mine the owning specs → fill `tasks/_template.md`
→ `verify:` commands that fail before and pass after → `validate.mjs` passes →
flip `status: ready`. Target a **~250-line body**; the enforced gate is
`validate.mjs`, which errors above **320** lines. Enrich **just-in-time** (only
the next wave); distant specs rot.

So the real cadence for each §3 item is: **enrich → review → merge → implement →
review → merge.** Budget for both.

**Founder approval on critical-path specs happens *at the PR*, never before it**
(`spec-author` step 5). Draft the enrichment, run both §6 reviews, open the PR,
and request sign-off there — every §3 item is critical-path, so this applies from
your first PR. It is **not** an interrupt: you keep working while it is pending.
Interrupting is reserved for the triggers in §14 and for a `❓ NEEDS FOUNDER
INPUT` row (§8).

**The three skills in `.claude/skills/`:** `spec-author` (stub → `ready`),
`task-pickup` (claim + load order), `verify` (run a task's `verify:` block).
Use them, with one override: `task-pickup` treats *any* `task/<id>` branch on
origin as blocking, but §2's rule is narrower and correct — **a branch is a lock
only if its task file also says `in-progress`**; branches whose task is `done` are
squash residue.

---

## 2. Current State

**Roadmap: 86 `done` / 183 work items** — plus 87 `stub`, 6 `blocked`,
2 `in-progress`, **2 `ready`** (`O6`, `US-10.3`). Milestone: **E11 Billing
flagship complete**, E10 Observe in progress. Re-derive with the §1 commands rather than
trusting this line; `validate.mjs` checks the *graph* (deps, caps, frontmatter)
and prints ready-unblocked, **not** a done total.

**Locks — re-derive them, don't trust this paragraph's count.** A `task/*` branch
is a lock **only if it is on `origin` AND its task file says `in-progress`**.
Everything else you will see is noise:

- **`done` task** → squash-merge residue (still shows "ahead" of `main`); ignore.
- **No matching file under `tasks/`** → docs or tooling work, not a task lock;
  ignore. (`task/start-here-manual` is one.)
- **Local-only branch** → machine-specific leftover; never a lock.

At the last update the live locks were `task/O2` (cost guardrails) and
`task/T1.3` (image pipeline), each holding one unmerged *authored* commit. That
is real work: inspect the branch, then either finish and PR it or deliberately
park it (reset `status:`, note why). **Never silently restart a locked task.**

**6 `blocked` tasks.** `US-3.3`–`US-3.6` + `CK-M3` (E3 provisioning) await the
runtime plane. `US-10.6` is different: it waits on a **founder frame amendment**
(§8), not on engineering. Leave all six blocked.

**Built — the control plane is broadly complete:** identity/auth/orgs/RBAC ·
projects/envs/services/bindings/deployments · estimates + the gate · metering +
billing (plans, quotas, subscriptions, invoices, budgets, spend cap) · policies ·
templates · dashboards · assistant threads · notifications · console SPA · GCP
foundation + CNPG substrate.

**Not built:** the runtime/execution plane (cell agent, reconciler, CNPG
provisioner driver) · observability data plane (Loki/OTel — `T1.5`) · data plane
(SQL exec, backups/branches API) · payment provider (`T11.4`) · AI inference
(`T13.6`). **This is why almost all remaining work is control-plane.**

**Runtime-blocked despite `deps: []`** — do not start these until E1/T1.5 land:
`T14.1`, `T14.6`, `US-10.5` (traces need trace data), `O3` (needs a live workload),
`US-1.1`–`US-1.3`, `T3.4`. Prefer control-plane work.

---

## 3. Next Work (execution order)

### 1. `US-10.3` — Bell projection · `tasks/e10-observability/US-10.3.md` · **`ready` → IMPLEMENT**
- Enrichment merged (#284, 2026-07-20). The task file carries the design, edge
  cases, ACs and deferrals — read it, not this entry.
- **Design in one line:** bell = spine projection over a `bell_scanned`
  **terminal ledger**. Never an `at`-based cursor: `events.at` is tx-*start*
  time, so a long tx committing late is skipped forever. Both rejected designs
  are in `contexts/events-spine.md`'s mistake bank — read them before "improving"
  the scan.
- Server half only; five concerns were split out across four review rounds.

### 2. `US-10.4` — 12 transactional emails · `tasks/e10-observability/US-10.4.md` · **`stub` → enrich first**
- **Objective:** one skeleton; subject grammar `[org/project] <fact>`; one event →
  one email → one action; **security emails unmutable**; a banner/email number
  mismatch is a defect.
- **Deps:** `T10.4` ✅.
- **Deliverables:** the templates + an invariant that email numbers equal the
  console/banner numbers, **sourced from canon, never retyped** (Trap T4).

### 3. `T14.8` — Spec-closure endpoints · `tasks/e14-data-plane/T14.8.md` · **`stub` → enrich first**
- **Objective:** bindings `:rotate`; domain recheck/delete; schedule + lifecycle
  per-row ops and `:run`; deployment `:pause`/`:abort`; org/project `:transfer`;
  git integration; cell `:drain`/`:detach`.
- **Deps:** none. **Large — every item is an owner-level contract addition.**
- **How:** sequence by blocked-surface count (proposals §9). Enrich and ship
  **one coherent slice per PR**, never the whole rollup. Most `:action` paths hit
  Trap T1.

### 4. Then, in dependency order
Control-plane: `US-11.3` (proration remainder) · `US-13.4` (AI disable across
every surface) · `T8.1`/`T8.3` (console regen pipeline, four-state harness) ·
`Q5`. Everything else waits on the runtime plane.

**`O6` is the one `ready` task** (`tasks/eops/O6.md`) — agent registration (§6) —
but it is **not** an easy first PR: its `verify:` entries are English sentences,
not commands, so it cannot satisfy §4 as written, and two of its ACs describe
**harness** behavior (plugin precedence, hard-fail on unknown agent) not reachable
from its `files: .claude/agents/**`. Closing it means first rewriting `verify:` as
executable probes and narrowing the ACs to what this repo controls.

---

## 4. Definition of Done (every task, before `status: done`)

A task is CLOSED only when **all** of these hold. If any fails, it is **PARTIAL** —
record it in the ledger (§8) with the remaining work and keep the task open.

**An enrichment PR (stub → `ready`, §1a) uses a different list — these criteria
*replace* the checklist below, they do not supplement it:** every template
section filled · nothing with an owner pasted in verbatim · `files:` globs are the
honest touch-set · `verify:` commands are **executable and demonstrably fail
before the work** (prose is not a `verify:` entry — `validate.mjs` only checks the
block is non-empty, so this is on you) · `validate.mjs` passes · **founder
sign-off requested in the PR** if the task is critical-path (§1a — at the PR, not
before it). It takes the full review pipeline.

- [ ] **Behavior exists.** The user-visible behavior or enforcement path is
      implemented and reachable — not merely an available primitive with no caller.
- [ ] **Every AC is satisfied and testable.** No AC is marked `[x]` on the strength
      of an assertion in prose.
- [ ] **The task's `verify:` block passes**, and its output is pasted in the PR.
      Assertions don't count; evidence does.
- [ ] **A structural guarantee exists** where practical — an invariant,
      tripwire, deterministic test, or measured number that fails on regression.
- [ ] **The test has a proven red state** where it is defense infrastructure (a
      tripwire that cannot fail is worse than none).
- [ ] **Contract changes are spec-first and regenerated** (§7 T2) — `openapi.yaml`
      edited, all clients regenerated and committed, `include-operation-ids`
      covers exactly the implemented ops.
- [ ] **No new flake.** No wall-clock, ordering, or network dependence
      (inject clocks via the `WithClock` seams).
- [ ] **Both reviews passed** (§6); blocking findings fixed **and re-verified**;
      non-blocking findings recorded in the ledger.
- [ ] **CI green** (`go`, `infra`, `validate`).
- [ ] **Task file updated in the same PR:** `status: done` + a 5–10 line
      `## Outcome` stating what shipped, what was deferred, and why.
- [ ] **Lessons landed in a living file** — a pack's mistake bank, the nearest
      `AGENTS.md`, an ADR, or this document. A lesson that changed no living file
      didn't happen.

---

## 5. Standing Engineering Rules (permanent)

1. **CLOSED means implemented behavior or an enforcement path that exists** —
   never merely an available primitive.
2. **PARTIAL** = primitive exists, wiring incomplete → ledger entry with the
   reason and the remaining work; the follow-up stays open.
3. **Deterministic CI only.** A flake, lint, or validate failure is a **defect to
   root-cause**, never a reason to retry until green.
4. **Evidence over assumptions** — measured numbers, committed transcripts, tests.
5. **Never overclaim completion.** Findings are recorded, never silently resolved.
6. **Requirements become structural guarantees** wherever practical.
7. **Reviews are adversarial, not ceremonial.**
8. **Repository review definitions are authoritative** (`.claude/agents/`) — and
   **no Kernel or external reviewer plugin**, evaluated and rejected
   (`docs/plan/kernel-workflow-review.md`; ADR-0008).
9. **Never start work on a stub** (§1a) — enrich it in its own reviewed PR first.
10. **Never invent product behavior or a founder decision.** A capability, term,
    policy, or number with no owner → propose it; do not improvise (agent-guide §4).
11. **One concern per PR.** Never mix restructuring with feature work. *(Carve-out:
    correcting **this file** or the §8 ledger may ride along with the work that
    revealed the error — that is bookkeeping, not restructuring.)*
12. **Scope discipline.** Build what the task asks; defer-and-record the rest.

**Repo hard rules that bind every change** (`AGENTS.md`, agent-guide §3):
never add an endpoint/component/term the spec lacks (spec-change first) · API
types are generated, never hand-written · money is **integer cents** (ADR-025) ·
every error is problem+json carrying `remediation` · microcopy verbatim from
frames · demo data from `19-canon` only · status vocabulary is `ready`/`deleting`
(ADR-024) · `docs/product/00-sources/**` and `18-philosophy/decisions.md` change
by **human decision only** (hook-enforced).

---

## 6. Review Process (ADR-0008 — mandatory before merge)

```
Implementation Agent → Architecture Reviewer → Security/QA Reviewer → CI → Merge
```

- **Architecture Reviewer** = `.claude/agents/reviewer.md` — all six dimensions:
  architecture, security, performance, API consistency, contract drift, ADR
  compliance. Output: `VERDICT` + severity-tagged findings.
- **Security/QA Reviewer** = `.claude/agents/qa.md` — test gaps, edge cases,
  regression risk, fuzz targets. Output: `GAPS` / `FUZZ` / `REGRESSION`.
- Both are **read-only** and **independent**. They review the **branch diff before
  merge**. Blocking findings are fixed **and re-verified by the same reviewer**.
  Non-blocking findings are **recorded in the ledger**, never dropped.
- CI is the **final** gate, never the first line of defense.
- **Significant** = changes behavior, adds a feature, touches
  security/authz/money/contracts, or modifies CI. Pure typo/comment/doc edits are
  exempt. **An enrichment PR is always significant** — the doc-edit exemption
  covers prose only, never a task spec. Skipping a reviewer on a significant PR
  is a **process violation**.

**Invoking the reviewers.** Try `subagent_type: "reviewer"` and
`subagent_type: "qa"` by name first — they now resolve from `.claude/agents/`.
**Fallback if they fail to resolve:** run `subagent_type: "general-purpose"` and
paste the **verbatim contents** of the agent file into the prompt, instructing
read-only behavior. **Never** substitute `kernel:*` or any plugin reviewer.

**Give the reviewer real context** — PR number, branch, intent, and the specific
risks you want scrutinized. A vague prompt produces a ceremonial review.

`O6` would make repo agents first-class (precedence over global plugins,
fail-fast on an unknown agent). Named resolution already works; precedence and
fail-fast remain unverified — see §3 before picking it up.

---

## 7. Known Traps (each cost real time; do not rediscover)

- **T1 — stdlib-mux rejects `{param}:action`.** `net/http.ServeMux` panics on
  `/x/{id}:action` ("bad wildcard segment") — 1.22+, **re-verified on the repo's
  Go 1.26**, so don't spend time re-testing. Colons are fine on a *static* segment
  (`/me/notifications:read`). For a new action endpoint use the **sub-path** form
  (`/deployments/{dep}/rollback` is the precedent). Affects `testWebhook` and most
  of T14.8.
- **T2 — contract changes: which route?** **Reshaping an existing operation** to
  satisfy a platform constraint (e.g. a `:action` path that trips T1) is done
  **inline** in your PR and noted in the ledger. **Adding or removing an
  operation** changes the product surface — that is a **ledger proposal first**,
  never an inline decision. Then the mechanics:
  Edit `docs/product/08-api/openapi.yaml` →
  `make gen-go` (server + Go client — **also syncs the RBAC matrix CSV** into
  `internal/identity/rbac/`) → `make gen-ts` (SDK + the console's `openapi.yaml`
  copy + client; it depends on `gen-canon`) → `make gen-sql` after any
  `db/queries/*.sql` edit → add the op to `include-operation-ids` in
  `services/api/oapi-server.cfg.yaml` → implement → commit **all** generated files.
  Reproduce CI's drift gate locally before pushing:
  ```sh
  make gen-go gen-ts gen-canon && git add -A && git diff --cached --exit-code
  ```
  `gen-ts` drift fails the **`validate`** job; `gen-go`/`gen-canon` drift fails the
  **`go`** job. **`gen-sql` drift is checked by neither** — regenerate it by hand
  or stale SQL ships green.
- **T3 — the error catalog is closed.** `internal/platform/problem` mirrors
  `openapi.yaml`'s `x-error-catalog` exactly; **a new error class is an S-process
  contract change, never a local addition.** Reuse an existing type (the hard
  spend cap correctly reuses `quota_exceeded` 402).
- **T4 — never retype canon.** Any number from `docs/product/19-canon` must be
  imported (`packages/canon`, `billing.Table`, the estimate engine), never
  hard-typed in a test. A second copy of a pricing constant is a bug.
- **T5 — no wall-clock tests.** A test that asserts behavior gated by time must
  inject a clock (`WithClock`). A quiet-hours test once failed *every* run
  between 22:00–07:00 UTC. Pin the clock inside/outside the window deliberately.
- **T6 — biome forbids exports from test files.** `export` in a `*.test.ts` fails
  `pnpm --filter console lint` (`noExportsInTest`) and therefore CI's `validate`
  job. It comes from biome's **recommended** ruleset — you will not find it named
  in `apps/console/biome.json`, which only sets `"linter": {"enabled": true}`.
  Keep test helpers local, and run `lint` before pushing console changes: `test`
  alone will not catch it.
- **T7 — permission-literal tripwire.** `TestEnforcedListMatchesSource` scans
  handler source for permission **string literals** at the `requireOrg`/`Require`
  call site. Passing a `perm` variable makes the permission invisible → put the
  literal at the call site and register it in `enforcedPermissions`
  (`services/api/internal/identity/endpoint_permission_coverage_test.go`).
- **T9 — `go test -run` exits 0 when nothing matches**, printing
  `ok … [no tests to run]`, so a `verify:` entry naming a not-yet-written test
  passes green *before* the work. **24 `done` tasks carry the unguarded form**
  (every test they name does exist — audited 2026-07-20, latent not live). Guard
  new entries by asserting an explicit PASS line **and** the exit code:
  `go test -v >"$o" 2>&1; rc=$?; grep -q '^--- PASS: TestX (' "$o" && [ $rc -eq 0 ]`.
  Three sub-traps, each of which cost a review round here: a bare `… | grep -q`
  pipeline returns *grep's* status and masks a post-PASS panic; the pattern must
  end at `(` because `-v` prints `--- PASS: TestX (0.00s)`, so a `$` anchor is
  red on a **passing** test; and **prove the green state as well as the red** — a
  guard that never matches passes a red-state probe perfectly.
- **T8 — non-members get 404, not 403, on id-addressed reads.** Never leak
  resource existence. (The billing/subscription surface is the deliberate
  exception: `requireOrg` denies uniformly with 403 there.)

---

## 8. Decision Ledger — unresolved, founder-gated (do **not** invent)

Ledger file: `docs/product/claudedocs/spec-change-proposals.md`.

**This table covers spec-change proposals only.** `docs/founder-config.md` holds
a *separate* set of founder-gated values, and a **`❓ NEEDS FOUNDER INPUT` row
there is the one thing that justifies interrupting** for a value in its space
(providers, identifiers, pricing, secrets). Two are open today: **KEK rotation
procedure** (L85) and **founder-org default budget** (L101). Consume that file
directly and check it before assuming any provider, id, price, or secret; a
`⏳ PENDING` row means do the unblocked work and switch when the value lands.

| Item | Status | Why blocked | Implementation impact |
|---|---|---|---|
| **P6/N2 frame amendment for the ruled taxonomy** | Open — **founder action**, not engineering | The founder ruled seven notification kinds on 2026-07-20 (Option B: `billing`/`security` are first-class, not folded into `alert`), but `00-sources` P6's routing matrix and N2's type chips still list six. `docs/product/00-sources/**` changes by human decision only (hook-enforced), so no agent can land it. | `US-10.6` (pin `NotificationList.kind` to the seven-value enum + audit legacy rows) stays `blocked`. `notify.Classify` (US-10.3) emits only kinds that have a frame row to source a verbatim title from, so billing/security bells stay silent until the frames land — correct behavior, not a gap. **No eighth kind may be invented** (§5 rule 10). |
| **Dashboard share-grant model** | Open | §2c names "share grants" with **no shape**; no restricted-share member model exists (`restricted` = owner-only). Designing it is a product decision. | Dashboards ship personal/org/restricted only; `restricted` behaves as owner-only; no share-with-specific-people surface. |
| **Prebuilt-dashboard topology generation** | Open | "Generated views, always current" needs service topology + metrics (data plane). | Prebuilt dashboards are read-only + forkable rows; generation unimplemented; rows must be seeded. |
| **`architecture.md` version drift** | Open finding (not founder-gated) | `docs/architecture.md:3` and `CLAUDE.md` both say **v1.2**, but ADR-042 (`decisions.md:81`) records the substrate delta as **v1.3**. The version bump was decided but never applied to the file's status line. | Cite the file as-is (v1.2) and treat ADR-042 as the authority for the substrate delta. Fixing the status line is a doc-only PR; per the hook, `decisions.md` itself is human-only. Recorded rather than silently corrected (§5 rule 5). |
| **B3 invoice-fixture numbers (S5 ruling)** | Open | `inv_2026_06` lines sum to **41,492¢** vs a printed total of **47,700¢**; GST is 31.8%, not 18%. Which is authoritative is an S5 canon ruling; the mock deliberately carries it "as a finding, not silently fixed". | §74 lines-sum is enforced everywhere **except** this fixture; the console invariant excludes it via a `B3_PENDING_S5` marker. Remove the marker when S5 rules. |

---

## 9. Open PARTIAL Items (primitive exists, wiring incomplete)

| Item | Why PARTIAL | Remaining work | Owner |
|---|---|---|---|
| `billing.GateCapability` | Consults `IsNeverGated` before a plan gate (table-tested) but has **zero callers** — no capability→plan gate exists (the only `PlanGated` site is the project-**limit** gate, not a capability). | Bind at the first real capability gate (SSO, audit-export…). | US-11.1 → US-11.2 |
| `quota.ClampOverageToCap` | Halt primitive (table-tested incl. boundary), **not wired** into rollup/`Evaluate` — a running service's soft overage is not clamped in production. | Needs a semantic ruling first: US-11.7 says "running services untouched", so **where** soft overage is billed-vs-halted at the cap must be decided, then wired. | US-11.7 → US-11.2 |
| `notify.NotifyInput.Urgent` | Bypasses quiet hours (tested) but **no security/paging source** sets it. It also still respects a member's **email-off** pref, while glossary/design-spec P6 ("escalation paging is org policy") arguably implies it should override that too. | The E7/auth security-notification source sets `Urgent: true` **and** rules the email-off-override explicitly. | US-10.2 → E7/auth |

---

## 10. Architectural Invariants (enforced, not aspirational)

**ADR citation convention** (two schemes, one character apart — read carefully):
`ADR-000X` = a file in `docs/adr/`; `ADR-0XX` = an entry in
`docs/product/18-philosophy/decisions.md`. So `ADR-0005` is `docs/adr/0005-byoc.md`
while `ADR-005` is the AI four laws. **`docs/adr/` has two files numbered 0007** —
cite the substrate one by filename (`0007-substrate-spike-findings.md`), never
bare, or a reader lands on `0007-org-key-authorization.md`.

- **Architecture is FROZEN** (`docs/architecture.md`; ADR-0001/0003/0004/0005):
  Go/stdlib-http/sqlc/River/REST+SSE; product surface exactly
  `[postgres, valkey, web, worker]`; queue = pgmq **capability**; storage/AI =
  external **Bindings**. Deltas need an ADR **and** implementation or customer
  evidence — a cleaner abstraction is never a trigger (ADR-040).
- **Substrate:** CNPG on **GKE PD-CSI (`pd-balanced`) + VolumeSnapshots** at
  dev/alpha (`docs/adr/0007-substrate-spike-findings.md`, ADR-042, INF-001 A6).
  ZFS-LocalPV re-scoped to a Cell-1 density option. **CNPG ≤ 1.30 is a hard
  ceiling** (in-tree Barman removed in 1.31). Branching guarantees are unchanged
  by that delta: branch · PITR · restore-**never**-in-place · hibernation ·
  delta-priced snapshots.
- **Money is integer cents end-to-end** (ADR-025) — estimate, invoice, quota, cap.
- **Invoice grammar == estimate grammar** (§74): every line is a **stated plan
  allowance** or a **metered quantity at a published unit price**; lines sum to the
  subtotal. Enforced at estimate, invoice, **and** console layers.
- **Hard spend cap is a real bound** (F9): an over-cap provision is refused at
  accept time with **402 `quota_exceeded`** + the arithmetic, on **create and
  scale**; every hit emits `billing.spend_cap_reached` on the spine.
- **Cancel ≠ Delete** (B12): cancelling changes the **plan**, never the
  **resources**. Metering keys off **resource** status, so cancel cannot stop a
  running service's billing span. One-click return: `/subscription/reactivate`.
- **Plans gate capabilities, never safety** (B6/F9). Never-gated set is
  drift-proof by set-equality: `tls, backups, mfa, policies, alerts,
  dunning_protections, self_deletion`.
- **AI four laws** (ADR-005): **no auto-apply path exists in the API**; every
  `/assistant/*` handler must call `AIAssistantEnabled` (structural tripwire) and
  return **404-invisible** when disabled — deleting nothing, restoring instantly.
- **Estimate-before-provision** is enforced at the **API layer**, never only in a
  client.
- **Events on every state change**; the spine is append-only; denials are audited.
- **Two-layer RBAC**: matrix ceiling, then tighten-only policy (ADR-0006/036).
  Policies can never widen what the matrix denies.
- **Org fencing:** a foreign/invisible id is **404**, never 403 (except the
  billing surface — Trap T8).
- **Secrets:** envelope-encrypted (KEK + AES-256-GCM + AAD); reveal-once; never
  plaintext at rest; templates strip all secret material, DB-checked at write.
- **Invoice close is idempotent** and freezes: re-closing a period returns the
  same invoice; later usage changes never re-price it.
- **Quiet hours affect routing only** — never recording, never firing.

---

## 11. Evidence Ledger (measured / reproducible)

**Substrate spike (ADR-0007, live GCP; transcripts `infra/spike/results/`,
provenance `PROVENANCE.md`, re-runnable via `infra/spike/run-*.sh`):**

| Measurement | Value |
|---|---|
| VolumeSnapshot create (10 Gi) | **34.6 s** |
| Branch e2e (snapshot → CNPG recovery → connections) | **52.4 s** |
| Hibernation wake | **8.0 s** |
| Restore to a NEW cluster (base backup + full WAL replay; 1,320,000 rows verified) | **55.2 s** |
| RPO | **≤ 300 s by construction** (`archive_timeout`) — a bound, not a kill test |
| Incremental snapshot delta (10%-divergence Dev branch) | **~62–65 MB** |

**Structural guarantees (tests that fail on regression):**

| Test | Guarantees |
|---|---|
| `TestEveryAssistantHandlerGatesOnPolicy` | every implemented `/assistant/*` handler calls `AIAssistantEnabled` (has a proven red state) |
| `TestEnforcedListMatchesSource` | the enforced-permission list equals what handler source actually enforces |
| `TestSafetyNeverGated` · `TestSafetyOperationsNeverPlanGated` | never-gated set-equality; MFA/self-deletion reference no plan gate (fails loudly if a file is renamed) |
| `TestQAScenario3Dunning` · `TestQAScenario4DowngradeBlock` | QA scenarios 3 & 4 in **~4 s** via the Q9 warped-clock harness |
| `TestBudgetCapAndExports` · `TestBudgetCapBoundaryAndUncapped` | cap blocked-not-warned, ±1¢ boundary, uncapped-free, cap-event payload |
| `TestCancelDoesNotStopServicesOrMetering` · `TestCancelledOrgStillBilledAtRollup` | Cancel ≠ Delete at the span **and** money layers |
| `TestInvoiceGenerator` · `TestEstimateLineGrammar` | §74 grammar enforced at the invoice and estimate layers |
| `tests/invoice-lines-sum.test.ts` | §74 at the console layer — a **staged** guard: it asserts `checked === 0` because B3 is the only lined mock and is excluded pending S5 (§8). Starts enforcing when B3 is fixed or real invoices land; separately pins the B3 defect so a silent "fix" fails. |
| `TestQuietHoursRouteOnlyNeverRecordOrFire` | routing-only, clock pinned inside the quiet window |
| `TestTemplateRefreshVersioning` | refresh mints a version; `used_by_count` preserved |
| `tests/realtime.test.ts` | SSE degrade → backoff → recovery on a virtual clock |

---

## 12. Operational Commands

```sh
# generate (see Trap T2 for the required order)
make gen-go            # Go server + client from openapi.yaml
make gen-ts            # SDK + console client + console openapi copy
make gen-sql           # sqlc, after editing services/api/db/queries/*.sql
make migrate-new name=<x>   # new golang-migrate pair

# task graph + GitHub projection
node scripts/spec-sync/validate.mjs   # MUST pass before every PR
node scripts/spec-sync/sync.mjs       # one-way repo → issues/board

# Go — CI builds THREE modules; the API alone is not enough
cd services/api && go build ./... && go vet ./... && go test ./...
cd packages/contracts/go && go build ./...
cd apps/cli && go build ./... && go vet ./... && go test ./...

# Go integration tests need a Docker socket (testcontainers + real Postgres).
# With colima (derive the socket from `docker context inspect` if this path misses):
export DOCKER_HOST="unix://$HOME/.colima/default/docker.sock"
export TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock
export TESTCONTAINERS_RYUK_DISABLED=true
go test ./internal/identity/ -run TestX -count=1
# First compile of a package's test tree can exceed 3 min — `go build ./...` first.

# console — all four, exactly as CI runs them (lint matters: Trap T6)
pnpm --filter console lint       # baseline is 14 pre-existing WARNINGS, exit 0 —
pnpm --filter console typecheck  # a new error hides in that noise; read the output
pnpm --filter console test
pnpm --filter console build
```

CI (`.github/workflows/ci.yml`) runs three jobs: **`validate`** (task graph,
regeneration drift, console lint/typecheck/test/build, SDK, canon), **`go`**
(build + vet + tests on real Postgres), **`infra`** (Terraform).

---

## 13. Milestone Log (newest first — keep 4, drop the oldest)

- **2026-07-20 — Notification taxonomy ruled + first enrichment merged.** Founder
  ruled **seven** kinds (`alert proposal approval deploy lifecycle billing
  security`) — `billing`/`security` are first-class, not folded into `alert`;
  no eighth kind without a new ruling. `US-10.3` enriched to `ready` (#284, four
  review rounds); `US-10.6` filed, blocked on the frame amendment.
- **2026-07-20 — E11 Billing flagship complete.** Cancel≠Delete,
  plans-gate-capabilities-not-safety, hard spend cap as an enforced 402 bound,
  invoice==estimate grammar, soft/hard quotas + 80% warning — all with
  integer-cent invariants. E10 Observe started (quiet-hours routing-only).
- **2026-07-19 — Substrate ratified.** CNPG on GKE PD-CSI replaces ZFS-LocalPV at
  dev/alpha on measured evidence (§11); INF-001 A6; productionized (T1.2).
  Architecture v1.3 decided but not applied to the file (§8 drift finding).
- **2026-07-19 — Review pipeline corrected.** ADR-0008 fixes reviewer identity to
  the repo-native `reviewer`/`qa` agents; external/plugin reviewers (incl.
  `kernel:*`) forbidden; `O6` filed for agent registration.

**Why the review pipeline exists (durable evidence):** on this codebase it has
caught, pre-merge — secret-smuggling template shapes; a scheduled downgrade that
*erased* a pending cancel; an SSE channel that died ~30 s after every mount; a
cross-tenant existence/audit leak; read-only tokens that could edit, delete, and
create resources; and a ledger entry overclaiming unwired primitives as "closed".
Treat it as load-bearing, not ceremony.

---

## 14. Startup Prompt (copy-paste)

```
Read START-HERE.md at the repository root completely, then execute its §1
Session Contract exactly before writing any code.

Sections §4 (Definition of Done), §5 (Rules), §6 (Review), §7 (Traps) are
binding. §8 (ledger) and §9 (PARTIAL items) are founder-gated: never invent
behavior for them, and never mark one CLOSED until its wiring genuinely exists.

Two things the doc will tell you but that are easy to skip: almost every
remaining task is a `stub` and must be enriched to `ready` in its own reviewed
PR first (§1a), and every significant PR — enrichment included — takes the two
independent read-only reviews in §6.

Keep CI deterministic: a flake, lint, or validate failure is a defect to
root-cause, never a reason to retry until green.

Work continuously. Interrupt only for: a founder decision, an external blocker,
a critical architectural discovery, or a major milestone. Update START-HERE.md
at each milestone by replacing stale content, never appending.
```
