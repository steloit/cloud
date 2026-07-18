# Steloit — Product Implementation Plan

**Status:** Execution roadmap · 2026-07-18
**Scope:** From "handoff package complete + console UI built" to a running product with paying customers. Five clients have committed to using the platform; the plan optimizes for getting them onboarded and converting them to revenue.
**Amended 2026-07-18:** database substrate re-decided per ADR-0003 / INF-001 A4 — CNPG + CoW volume snapshots replace Neon OSS; substrate references below are updated (branching remains a product capability; no customer contract changes).
**Provenance:** Derived from `00-sources/` (GOV-002, INF-001 incl. A1–A3, 152-frame gallery, design spec), the derived docs 01–23, the console build's findings ledger (`claudedocs/spec-change-proposals.md`), and the current state of `steloit/console`. Decision IDs (D1–D11, ADR-###) are cited per INF-001 §8.

---

## 0 · How to read this plan

Three facts shape everything below — ignore any one of them and the plan is wrong:

1. **The frontend is already built.** `steloit/console` renders all 152 frames against an MSW mock layer serving canon fixtures, with six audit passes complete (commit `85e96ca`). The frontend workstream is *integration* (progressively replacing mocks with the real API), not screen construction.
2. **The architecture is already decided.** INF-001's shape decisions (D1–D11) govern *how* everything is built: production-grade shape, minimal capacity ("cheap on capacity, never on shape"). Data model, API surface, isolation boundaries, and provisioning pattern are day-one correct; nodes, replicas, retention, and cells scale up on triggers, never via migrations.
3. **Five committed clients are waiting.** The MVP is the wedge they signed up for — *push repo → cost estimate → approve → deployed service + Postgres → open PR → branched preview* (the D11 alpha path). Everything is sequenced to put that in their hands fastest, then expand to the full platform behind them.

**Team assumption:** 2 founding engineers + AI coding agents (the leverage model that built the 152-frame console in ~4 phases), founders-only until first payment clears (D11). Sprint = 2 weeks. Nominal capacity 4 engineer-weeks (EW)/sprint; well-specced generation work (this package's whole point) has demonstrated ~1.5–2× effective leverage, so sprints are loaded at 5–6 EW of plan-value. Estimates are planning ranges, untested against this backend's unknowns — treat ±40% as normal until Sprint 2 calibrates them.

---

## 1 · Current state

| Layer | State | Evidence |
|---|---|---|
| Product spec | Complete — 152 validated frames, design spec, 22 derived docs, canon world | `00-sources/`, `19-canon/` |
| Console frontend | **Built.** All 152 frames, four-state grammar, overlay census, canon mode (MSW), Playwright verification scripts, Vitest arithmetic invariants | `steloit/console` @ `85e96ca` |
| API contract | `08-api/openapi.yaml` v1 — 51 operations, conventions locked (problem+json, cursor pagination, `*_cents`, `?env=`) | agent-verified inventory (§7) |
| Backend | **Does not exist.** No auth service, no control plane, no data plane, no billing, no AI plane | console runs on a local session seam + MSW |
| CLI | **Does not exist** (grammar fully specced in `20-clients/cli.md`) | — |
| Infra | **Does not exist**; substrate spike (ZFS→CNPG branch e2e) not yet run (ADR-0003) | INF-001 §3 |
| Customers | **Five clients committed** and ready to use the platform | founder-confirmed, 2026-07-18 |
| Spec debt | ~40 missing endpoints, 13 schema amendments, 9 frame↔fixture conflicts, 4 ADR drafts awaiting human sign-off | `claudedocs/spec-change-proposals.md` |

---

## 2 · Sprint 0 — decisions & setup before Sprint 1

Not engineering tasks; they are the founder-owned decisions and account/registration work that Sprint 1 consumes. All of it fits in one week alongside E1 design docs.

### 2.1 Setup & standing items

| # | Item | Why it's needed | Status |
|---|---|---|---|
| P1 | GCP account + billing set up; trial credit activated ($300 covers the entire MVP phase — GKE zonal management is free-tier, permanent) | Sprint 1 infrastructure | Open |
| P2 | Register the customer-content domain (separate eTLD+1, A2.4 — shape-locked) | Preview URLs and object URLs are served from it (E4, E9) | Open |
| P3 | File Google for Startups / AWS Activate credit applications + staged GCP quota increases (both have weeks of lead time; fresh accounts won't get 50-node CPU quota) | Cell-1 scale-out and larger node pools later | Open |
| P4 | Ratify the alpha RPO value (≤ 5 min recommended, A1.3 — open since 2026-07-13) | ToS wording, WAL-archiving configuration, and partner expectations all quote it | **Open** |
| P5 | Onboard the five committed clients as design partners: named contacts, expectations doc (alpha = the wedge path, CLI-first, invite-only, support commitment), onboarding schedule targeting M4 | They are the alpha cohort and the first revenue | Open |

### 2.2 Spec rulings (from the console build's findings ledger)

Human sign-off needed; each unblocks a block of work below. Recommended dispositions in parentheses are the proposals already filed in `claudedocs/spec-change-proposals.md` §7 — they need a yes/no, not new analysis.

| # | Decision | Unblocks | Proposal on file |
|---|---|---|---|
| S1 | **ADR draft 7.4 — auth surface.** openapi.yaml has no sign-in/session/MFA section; the console fakes a local seam | Epic E2 (identity); the whole A/P plane integration | Add an auth section to the API |
| S2 | **ADR draft 7.1 — data-plane API scope.** "No console-only capabilities" vs the D-tab data plane (~20 endpoints) | Epic E14; D-tab integration | v1 grows a data-plane section, destructive verbs policy-gated |
| S3 | **Mechanical spec defects:** `/invites/{invite}/renew` missing path param; `DELETE /invites/{invite}` missing | Clean generated client (frontend + CLI) | Fix both — 30-minute edit |
| S4 | **Notifications family** (8 blocked surfaces — the highest-frequency gap) | Epic E10 | Add `GET /me/notifications` + `:read` + webhook routes |
| S5 | The 9 frame↔fixture conflicts (typed-confirm slug-vs-name, alert-rule names, B-series numerics, …) | QA baselines; canon seed data | Rule each: frame wins / fixture wins / intended divergence |
| S6 | ADR drafts 7.2 (canon statefulness) + 7.3 (X1 gateway canonization) | Demo world, seed data, QA invariants | As filed |
| S7 | **Idempotency strategy** for mutating POSTs (createService, createDeployment, changePlan) — unspecified in the yaml; "failed provisioning never bills" needs it | E3 onward | Propose `Idempotency-Key` header, 24h dedupe window (needs an ADR) |

**Recommendation:** batch P1–P5 and S1–S7 into one founder session (half a day); S3 is mechanical and can be committed immediately after.

---

## 3 · Product breakdown into modules

Modules follow the backend structure in `14-development/architecture.md` and GOV-002's two-plane split (D6). Each module lists its primitives (GOV-002 §1) and owning epics.

| # | Module | Contents | Primitives | Epics |
|---|---|---|---|---|
| M1 | **Platform substrate** (data plane) | Cell-0: zonal GKE, gVisor, namespaces, CNPG fleet + ZFS storage pool (ADR-0003), GCS buckets, per-cell agent | — (beneath the grammar, D8) | E1 |
| M2 | **Identity & access** | Users, sessions, MFA, personal tokens, org API keys, RBAC two-layer evaluation, policy engine hook | Organization, Policy | E2, E7 |
| M3 | **Control plane core** | Orgs, members, invites, projects, environments, cells registry, desired-state store, reconciler protocol | Organization, Project, Environment | E2, E3 |
| M4 | **Provisioning & estimates** | Estimate engine, service CRUD (estimate-gated), shape/scaling, secrets, bindings, per-product drivers | Service, Binding, Secret | E3, E9 |
| M5 | **Deploy** | Build pipeline, deployments, promotion, rollback, PR preview envs with branched DBs, domains & TLS | Deployment, Environment | E4 |
| M6 | **Observe** | Events spine (SSE), metrics, logs, traces, alert rules + backtest, notifications & routing, quiet hours | Event | E6, E10 |
| M7 | **Billing** | Metering (day one, D10), usage, quotas soft/hard, plans & subscription lifecycle, dunning, invoices, payment provider | Organization | E6 (metering), E11 |
| M8 | **Governance** | Policies CRUD + dry-run, templates, audit views, dashboards | Policy, Event | E12 |
| M9 | **AI plane** | 8 read-only tools, context resolver, threads, insights, proposals, `ai-assistant` policy | (cross-cutting layer, GOV-002 §F) | E13 |
| M10 | **Clients** | CLI (`steloit` grammar), generated SDK core, console API integration | — | E5, E8 |
| M11 | **Comms** | 12 transactional email templates + event→email routing | Event | E10 |
| M12 | **Data-plane depth** | SQL exec/tables/insights, key browse, messages/DLQ, objects, backups/restore, branches, shell | Service | E14 |

---

## 4 · Prioritization: MVP → V1 → Future

### MVP — Private Alpha ("the wedge") — Sprints 1–6, ~90 days
> push repo → see cost estimate → approve → deployed service + Postgres → open PR → branched preview environment

This is the path the five clients committed to; it ships first, alone, so they get real value in ~90 days instead of a broader platform in twice that.

- **In:** Cell-0, identity core, orgs/projects/envs, estimate engine, Postgres provisioning (CNPG cluster + snapshot branches, ADR-0003), secrets + bindings (incl. bind-to-external-host — a first-class v0 mode per GOV-002 §1.4), one compute service type (web) via push-to-deploy, PR-triggered branched previews, metering events from first deploy (D10 — cannot defer; backfill is impossible), baseline DB metrics/logs, **CLI as the primary client** (D11: "CLI or minimal UI is acceptable"), audit/events ledger.
- **Deferred to V1:** Valkey, Queue, Object-storage API, AI layer, billing/charging (metering only), the full console (a thin login + read-only project/estimate view rides along as E8-lite because it's nearly free), dashboards, alerts UI, templates, BYOC.
- **Users:** the five design partners, invite-only (no anonymous compute — abuse control per A1.8).

### V1 — Design partners → first revenue — Sprints 7–16
Maps to GOV-002 v0.5→v1: data layer completes, console goes live against the real API, operations & billing ship, first payment clears (which also lifts D11's hiring freeze).

- Auth hardening (MFA, sessions, org API keys), invites full lifecycle
- Console integration in slices (auth → create/estimate → deploy → observe → settings/billing)
- Valkey + Object storage (proxied GCS, Steloit-domain URLs per A1.4, content eTLD+1) + Queue (after the A1.2/A3.1 WAL-signal design review — scale-to-zero must survive)
- Observability product (metrics/logs/traces/events queries, alert rules + backtest, notifications family, emails)
- Billing end-to-end (usage → quotas → plans → dunning → invoices) + payment provider + **the hard spend cap as flagship (F9, US-11.7)**
- **Masking-by-policy V1 depth (F14, T4.9)** — policy-driven rules + DDL-aware feeds; the defensible half of the preview demo
- Governance (policies + dry-run, templates, dashboards)
- AI plane (four laws, proposals, insights) — last, per GOV-002 Law 4: the platform must already be whole without it
- Capacity knob-turns at first payment: CNPG replicas for paid tiers, core-pool floor, Mumbai cell for partner-touchable envs (A1.7)

### Future — V2+ (not planned in sprints here; recorded so nobody re-litigates)
- v2: Workers, Cron, autoscaling depth, `steloit dev` GA, canary/blue-green (compute expansion — ADR-006: data before compute)
- v3: BYOC cells GA (X2/X3 — connect flow exists in console), SSO/SCIM, private networking, SOC 2, multi-region
- v4: Functions, event bus, analytics DB, customer auth, marketplace
- Data-plane depth (E14) straddles V1/Future: read-only surfaces (tables, insights, key browse, messages view) land in V1; destructive verbs (purge, FLUSHALL, shell) land post-V1 behind policy gates, per the S2 ruling

---

## 5 · Epics

Format: scope → representative user stories (US) → technical tasks (T). Stories carry acceptance criteria drawn from the spec's own business rules; the full field-level detail lives in `05-features/` and `07-forms/` and is not duplicated here. Effort = engineer-weeks (EW), including tests.

---

### E1 · Platform substrate — Cell-0 (M1) — **8 EW** · MVP · Sprints 1–2

The data plane's skeleton, shaped per D6/D7 from day one, sized per "cheap on capacity."

**Scope:** GCP org/project structure (control-plane project + cell-0 project), Terraform for everything (invariant 3), zonal GKE Standard cluster (free mgmt tier), scale-to-zero workload pool + core pool floor of 1 (A1.6), gVisor node class (invariant 6), namespace-per-project-env with default-deny NetworkPolicies + quotas (invariant 5), CNPG operator + ZFS-LocalPV storage pool (ADR-0003), GCS bucket layout, per-cell reconciler agent skeleton (D9), workload identity + zero static keys + signed images (invariant 11), pod CIDR sized for the full-grown cell (invariant 8).

**The week-1 spike (gates everything technical):**
- T1.0 **ZFS snapshot → CNPG branch end-to-end** (ADR-0003): cluster → write → VolumeSnapshot → recovered cluster → CoW divergence; hibernate/wake latency; WAL-to-GCS RPO ≤5 min; PITR-to-new-cluster. All documented paths — the spike measures cost/latency numbers for the estimate engine. **Findings ADR by end of week 1.**

**Stories & tasks:**
- US-1.1 *As the platform, every resource row carries `cell_id` and provisioning routes through a cell-selection function, even though the answer is always cell-0.* (invariant 1)
- US-1.2 *As a founder, I can rebuild the entire environment from scratch in one afternoon from git.* → T: monthly fire-drill runbook + first drill executed before M2. (invariant 3)
- US-1.3 *As the control plane, I write desired state; the cell agent converges actual state and reports back.* → T: reconciler protocol (desired-state tables, agent poll/apply loop, status writeback, drift report). (D9, A2.5)
- T1.1 Terraform: GCP projects, GKE, node pools, workload identity, GCS, IAM floors
- T1.2 CNPG operator (pinned ≤1.30) + ZFS-LocalPV storage class on a storage node pool + cluster-per-project-env template (hibernation defaults, WAL/base-backups to GCS) (ADR-0003)
- T1.3 Image pipeline: build → sign → provenance from the first build (invariant 11)
- T1.4 Control-plane Postgres (Cloud SQL or self-run on core pool) + PITR backups to a **separate** GCS location, restore-tested (invariant 10)
- T1.5 In-cluster Loki + OTel collector (logs routed away from Cloud Logging's paid tier; labels stamped by our collector, never trusted from customer — D7)
- T1.6 Duty-cycling schedule for the pre-partner window (declared downtime per A1.6; core pool goes 24/7 the day the first partner touches the platform)

**Exit:** spike ADR recorded; `terraform apply` from empty → working cell; first tenant DB created/branched/destroyed by hand; metering events emitted from a test pod.

---

### E2 · Identity, access & the cross-cutting spine (M2, M3) — **7 EW** · MVP · Sprints 2–3

Everything else depends on this epic; it is deliberately boring and maximally specced.

**Scope:** users, signup/signin (email+password for alpha; SSO deferred to Business tier work), sessions, personal tokens (`stp_`, reveal-once, hash-stored), orgs, members, roles, the RBAC two-layer evaluator, the policy-engine hook (evaluation only; authoring UI is E12), the events/audit append-only ledger, and the problem+json error framework with the full x-error-catalog.

**Stories:**
- US-2.1 *As a new user I sign up, create an org (name → permanent slug, home region) and land in an empty world with a way forward.* AC: slug `[a-z0-9-]{3,32}` immutable; A5 microcopy verbatim.
- US-2.2 *As any caller, every mutating request is evaluated as `matrix[role][perm]==Y AND policies.evaluate(actor, perm, {org,project,env})==permit`; matrix denials name the missing role, policy denials name the policy, and both are audited.* (11-permissions contract — this exact sentence is the acceptance test)
- US-2.3 *As a user I create a personal token, see the plaintext exactly once, and a later role demotion shrinks the live token immediately.* AC: QA scenario 6 passes end-to-end.
- US-2.4 *As an owner I cannot leave or be demoted while I am the last owner.* AC: DB trigger + 409 with `reasons[]`.
- US-2.5 *As the platform, every state change lands in the events ledger with `via ∈ {user, assistant, system}` and serves both `/events` and `/audit`.* (one pipeline, GOV-002 primitive 9)

**Tasks:** T2.1 users/sessions/password (argon2id, session store) · T2.2 token mint/verify middleware (personal + org API keys share the `tokens` table) · T2.3 RBAC evaluator + `rbac-matrix.csv` loaded as data, not code · T2.4 policy evaluation stub (closest-wins inheritance, tighten-only) · T2.5 events ledger (append-only, `idx(org_id, at desc)`) + SSE fanout skeleton · T2.6 problem+json middleware with required `remediation` and the 402/409/422/429 extension fields · T2.7 org/member/invite endpoints (12 ops) · T2.8 **S1 ruling implemented**: auth section added to openapi.yaml first, then generated (never hand-written types — 14-development rule).

**Exit:** QA scenarios 6 (token reveal) and 8 (invite lifecycle) automated and green; RBAC evaluation contract has a table-driven test over the full matrix.

---

### E3 · Projects, environments, estimates, Postgres provisioning (M3, M4) — **9 EW** · MVP · Sprints 3–4

The estimate-before-provision law made real. This epic is the product's soul; the estimate engine's numbers must equal the invoice grammar from birth (one arithmetic, ADR-025).

**Stories:**
- US-3.1 *As a developer I create a project and a production environment; the environment sets the region; services inherit it.* (ADR-004; alpha regions: us-central1 founders-only, asia-south1 once partner-touchable per A1.7)
- US-3.2 *As a developer I request a Postgres service shape and receive an estimate (`est_`) whose line grammar is byte-identical to the eventual invoice line; nothing provisions or bills before I accept it.* AC: `POST /estimates` → `createService` requires `estimate_id`; a service created without one is impossible at the API layer, not the UI layer.
- US-3.3 *As a developer my accepted service provisions through the reconciler: desired row → cell agent → CNPG cluster in the env namespace (ADR-0003) → status walks `provisioning → ready`; metering starts at `ready`, not before.* (ADR-024; D10)
- US-3.4 *As a developer I bind my external app to the database: credentials minted at bind, `<TARGET>_URL` injected/displayed, rotate on unbind.* AC: bindings cost $0; read-only scope enforced by the datastore itself, not middleware. (F3; GOV-002 v0's "bind to external apps" first-class mode)
- US-3.5 *As a developer, deleting anything takes a final backup first and typed-confirm names dependents.* AC: QA scenario 10.
- US-3.6 *As the platform, a failed provisioning never bills and never strands state.* → needs S7 (idempotency ruling).

**Tasks:** T3.1 estimate engine (pricing tables as data; shape → line items; the `$208`-family canon numbers as regression fixtures) · T3.2 project/env CRUD (7 ops) · T3.3 service CRUD + shape jsonb + status machine (`provisioning|ready|degraded|failed|suspended|deleting`) · T3.4 CNPG driver (cluster create, snapshot branch, hibernate/wake, PITR-to-new-branch) behind the provisioner interface (per-product drivers, 14-development) · T3.5 secrets (versioned, scoped, envelope-encrypted via KMS — D5; no CRUD API in v1 yaml, internal service only — flag as finding) · T3.6 bindings (mint/rotate/restrict; `UNIQUE(source,target)`, `ON DELETE RESTRICT`) · T3.7 metering emitters on every resource lifecycle edge (compute-seconds, CU-hours, GB-months, egress) tagged org/project/env.

**Exit:** CLI (E5) can run `steloit db create` → estimate → `--yes` → ready, end-to-end on cell-0; canon arithmetic invariants imported (never retyped) and green against the estimate engine.

---

### E4 · Deploy & branched previews (M5) — **9 EW** · MVP · Sprints 4–6

The certainty demo the five partners are waiting for: *a PR gets a working preview with masked production data and the price printed on the comment.* (Branching/snapshots are the mechanism — F14 masking is the defensible half.)

**Stories:**
- US-4.1 *As a developer I `git push` (or connect a GitHub repo) and get a built, deployed web service with a URL.* AC: buildpack or Dockerfile path; image signed; deploy states walk `queued → building → live`; time-to-first-connection contributes to the <5-min test (GOV-002 five-minute test).
- US-4.2 *As a developer, opening a PR creates a preview environment with a **branched** database (CoW snapshot, ADR-0003), and closing the PR tears it down.* AC: preview env `kind=preview`, `expires_at` enforced by a background job; the bot comment carries the canonical demo line: `db: production data (masked · policy) · $0.07/day · capped`.
- US-4.3 *As a developer I roll back a bad deploy in one click/command in <60s (redeploy previous image).*
- US-4.4 *As the platform, deploy markers land on the events spine so every chart of the affected env can show them.* (F4; QA scenario 1's #142/#143 replay)
- US-4.5 *As a developer my preview URL is on the customer-content domain, never the console origin.* (A2.4 — needs P2)

**Tasks:** T4.1 GitHub App (webhook: push, PR open/close) + repo link storage · T4.2 build pipeline on the workload pool (Kaniko/BuildKit, gVisor, signed output) · T4.3 deployment records (immutable history) + rollback · T4.4 preview-env orchestration (create env → branch timeline → deploy → comment; teardown on close) · T4.5 ingress + TLS for `*.previews.<content-domain>` · T4.6 masking-by-policy v0 for previews (F14: static ruleset at branch creation, fail-closed; policy-driven depth is T4.9 in V1) · T4.7 `DELETE /envs/{env}` — **missing from the yaml** (spec-change §2b); needed for teardown; add via S-process · T4.8 **custom domains & TLS (F5/U5, V1 · Sprint 9, +1 EW):** add-domain drawer contract (CNAME+TXT, 60s recheck, async-safe — closing the drawer never cancels verification), cert auto-issue, bell on success; TLS never optional, never plan-gated; DNS >48h → guidance, never expiry.

**Exit:** the wedge is real: push → estimate → approve → live service + Postgres → open PR → preview URL with branched DB → merge → teardown. This is milestone M4 and the partner-onboarding trigger.

---

### E5 · CLI v0 (M10) — **5 EW** · MVP · Sprints 3–5 (interleaved)

Alpha is CLI-first (D11). The CLI is a thin client of the same API (no CLI-only capabilities — mirror of the console promise).

**Stories:**
- US-5.1 *As a developer, `steloit init` links my repo, `steloit db create` shows an estimate I must see before `--yes` accepts it (no skip-seeing flag exists), and context (`org/project/env`) is echoed on every state-changing command.* (20-clients safety grammar)
- US-5.2 *As a developer, `--json` gives me raw API shapes verbatim; `--quiet` gives ids; human mode gives aligned tables with the six status marks; exit codes follow the 0–7 map.* AC: `NO_COLOR`/non-TTY degrade losslessly.
- US-5.3 *As a developer, destructive commands state blast radius and require `--confirm <exact-name>`; no `--force` exists.*
- US-5.4 *As a developer, errors render problem+json as three lines: what / why-where / what-next.*

**Tasks:** T5.1 CLI skeleton (Go or TS — decide by SDK-core reuse; sdk.md's generated-core pattern) · T5.2 auth (device flow against E2 sessions or token paste) · T5.3 nouns for alpha: `project env db bind deploy logs token init connect` (+`dev` stub) · T5.4 `-f` live tail over SSE · T5.5 output layer (tables/marks/json/quiet) as a shared package the SDK inherits · T5.6 **TS SDK v0 (V1 · Sprint 8, +1 EW):** generated core from openapi.yaml + the ergonomics layer per `20-clients/sdk.md`; published alongside the console slice-3 flip · T5.7 **docs (V1 · Sprint 7, +0.5 EW):** API reference generated from openapi.yaml, CLI reference from help output, partner onboarding guide versioned with releases.

**Exit:** every alpha-path step executable by CLI alone; asciinema of the full loop recorded for partner onboarding.

---

### E6 · Baseline observe + metering pipeline (M6, M7) — **4 EW** · MVP · Sprints 5–6

Only what the alpha path and D10 demand — the observability *product* is E10.

**Stories:** US-6.1 *DB and web-service metrics (CPU, connections, p95) and logs are queryable per env* (`GET /envs/{env}/metrics|logs`, SSE tail). · US-6.2 *Usage events flow from first deploy into an append-only metering store; a founder can produce a per-org usage report by month* (no invoicing yet — billing attaches in E11 when the first price is published).

**Tasks:** T6.1 metrics query API over the OTel/Prometheus pipeline · T6.2 logs query API over Loki (label enforcement at query layer — D7) · T6.3 metering store + rollup job (`quota_usage(org, meter, period)`) · T6.4 events SSE endpoint hardened (the spine drives CLI `-f` and later the console bell).

---

### E7 · Auth hardening & org surface completion (M2) — **4 EW** · V1 · Sprint 7

MFA (TOTP + WebAuthn), recovery codes, session list/revoke, password reset emails, org API keys with scopes, invite emails + renew/decline (S3 fixes), leave-org/account-delete grace windows. Closes ADR-draft 7.4 fully so the console's A/P planes integrate without the seam.

---

### E8 · Console integration (M10) — **10 EW** · V1 · Sprints 7–11 (sliced)

Swap MSW → real API, surface by surface, behind a per-family flag (`VITE_API_MODE=canon|real` already exists as canon mode). Canon mode is retained permanently as the demo world (ADR-026) and as the E2E fixture harness.

**Slices (each = flag flip + Playwright suite green):**
1. **E8-lite (MVP tail, Sprint 6, ~1 EW):** login + org/project read + estimate view against the real API — gives the design partners a read-only console alongside the CLI.
2. Auth + org/members/invites (A/G planes) — Sprint 7
3. Create canvas + estimates + services + bindings (C/W/U) — Sprint 8
4. Deploy + previews (DP) — Sprint 9
5. Observe (O1–O6) + events/bell (N) — Sprint 10
6. Settings/billing/policies/templates/dashboards (G/B/T/DB) — Sprint 11, as E11/E12 land
7. AI plane (AI1–12) — Sprint 14, as E13 lands

**Standing tasks:** T8.1 regenerate client per spec amendment (Hey API, `pnpm gen:api`) · T8.2 SSE wiring for toasts/bell/status pills (state.md contract: SSE-primary, poll-fallback 2s→10s) · T8.3 four-state verification per slice (the fault-injection technique from the audit) · T8.4 remove the session seam · T8.5 responsive/mobile polish + real-SSE items from the console's remaining-polish list.

---

### E9 · Data layer completion (M4) — **8 EW** · V1 · Sprints 8–10

Per GOV-002 v0.5, in this order:

1. **Valkey** (2 EW): per-project-env pods (never shared — D5), driver + estimate shapes + bindings. Frames S2/D3/D6/D14.
2. **Object storage** (3 EW): proxied GCS (D4), one bucket per project, STS-scoped presigned URLs rewritten to Steloit content-domain (A1.4/A2.4), lifecycle rules (expire/tier_cold). Frames S3/D7/D15/D16.
3. **Queue** (3 EW + design review): **the A3.1 design review comes first** — WAL-derived signals via logical-decoding CDC on vanilla Postgres (A3.1 as amended by INF-001 A4; direct-CDC vs trigger-outbox variants), dispatcher-driven consumer wake; queues must not defeat scale-to-zero (A1.2), and the NATS fallback is last resort (loses branch-coherence). The review is a Sprint 8 deliverable with its own ADR; implementation follows its outcome. Frames S4/D8/D17/D18.

Each product instantiates the same anatomy (21-playbooks: one pioneer at a time — Postgres is the pioneer; these instantiate).

---

### E10 · Observability product + notifications + emails (M6, M11) — **8 EW** · V1 · Sprints 9–11

**Stories:**
- US-10.1 *Alert rules via drawer: query, condition, window (1m/5m/15m), routes (bell/email/webhook), with a 7-day backtest that reproduces historical firings.* AC: the 431ms canon backtest is the regression fixture. (F6, U8)
- US-10.2 *Quiet hours affect routing only — recording and firing are untouched; security/paging classes are never gated.* AC: QA quiet-hours check.
- US-10.3 *The notifications family exists: `GET /me/notifications`, `:read`, webhook routes + test delivery.* (S4 — the 8-surface gap; bell N1–N3 goes live)
- US-10.4 *Every event kind that emails does so through one skeleton, subject grammar `[org/project] <fact>`, same numbers as the console — a banner/email mismatch is a defect.* → 12 templates from 23-emails; security emails unmutable.
- US-10.5 *Traces: `GET /envs/{env}/traces` list + span detail* (spec-change §2c).

**Tasks:** T10.1 alert evaluator (streaming eval over metrics pipeline; states `ok|watching|firing|fired|silenced`) · T10.2 backtest executor · T10.3 notification store + routing matrix (P6) + webhook delivery with test endpoint · T10.4 email service (provider-integrated; event→template map; 12 templates) · T10.5 alert-rule PATCH/DELETE + silences + firing history (spec-change §2c additions, per S-ruling).

---

### E11 · Billing & subscription (M7) — **9 EW** · V1 · Sprints 10–13

Nearly all backend rules (F9). Metering already flows (E6); this epic prices, gates, and charges it.

**Stories:**
- US-11.1 *Plans gate capabilities, never safety: TLS, backups, MFA, policies, alerts, dunning protections, self-deletion are free at every tier.* AC: the never-gated list is a table-driven test.
- US-11.2 *Soft quotas keep working and bill (egress $0.09/GB, seats $7, builds $0.01/min, events $1.20/M, AI requests $2/1k); hard quotas fail loudly with 429+Retry-After; 80% warns with banner+bell+email showing the math.* AC: QA scenario 2 verbatim (87/100GB → ~$1.62, no upsell when overage is cheaper).
- US-11.3 *Upgrades are immediate + prorated; downgrades take effect at anchor and are blocked with all reasons when over limits.* AC: QA scenario 4 (Business→Pro with 12 members & 2 cells → 409 listing both).
- US-11.4 *Dunning: day 0 fail → retries → day 7 provisioning paused → day 21 suspend (state kept) → day 90 deletion with final notice; payment clears everything instantly.* AC: QA scenario 3 with clock-warped tests.
- US-11.5 *Cancel ≠ delete: services keep running and metering after plan cancellation.*
- US-11.7 *As an org owner I set a hard monthly spend bound; at the bound provisioning pauses and overage stops accruing — nothing deleted, never alerts-only; an estimate that would cross the cap is refused at accept with the math.* AC: F9's "the cap is real" clause; crossing the cap is impossible by construction. (Flagship proof point #7.)
- US-11.6 *Invoices carry the same line grammar as estimates; lines sum to their subtotal; money is integer cents end-to-end.* (ADR-025; fixes the B3 canon defect per S5 ruling)

**Tasks:** T11.1 pricing/quota tables as data · T11.2 subscription state machine (incl. trial, `cancelled_at_anchor`, dunning states) · T11.3 invoice generator (accrue → open → paid/failed) · T11.4 payment provider integration (Stripe or Razorpay — decide by partner geography; INR-first suggests Razorpay + Stripe for intl; needs a founder call) · T11.5 quota evaluator middleware (soft → `confirm=true` overage path; hard → 402/429 catalog) · T11.6 budget endpoint + exports (spec-change §2d, per ruling).

**Milestone M7 = first real payment clears** → lifts D11's hiring freeze; triggers capacity knob-turns (CNPG replicas for paid tiers, core-pool floor, retention up).

---

### E12 · Governance: policies, templates, dashboards (M8) — **7 EW** · V1 · Sprints 11–13

- **Policies (3 EW):** CRUD + versioning + `?dry_run=true` impact preview; enforcement wired into the E2 evaluator (tighten-only, closest-wins); `ai-assistant` policy ready for E13; violation counts on the events spine. G7–G11, AI3.
- **Templates (2 EW):** capture (secret-stripping checked at write — `contents NEVER contain secrets` is a DB-level check), frozen-copy semantics (ADR-021), required-input synthesis for excluded bindings, estimate at save/list/consume, refresh-as-new-version. AC: QA scenario 5. T1–T3 frames.
- **Dashboards (2 EW):** CRUD, widgets, scope⊥visibility axes, pre-built generated views (fork-to-customize, never files), share grants + fork/duplicate/widget-delete (spec-change §2c per ruling). DB1–DB8.

---

### E13 · AI plane (M9) — **8 EW** · V1 · Sprints 13–15

Last by design (Law 4: the platform is already whole). The four laws are architecture, not policy:

**Stories:**
- US-13.1 *No auto-apply path exists in the API — applying a proposal is a normal human-session call that re-runs the full two-layer RBAC evaluation for the underlying change.* AC: attempt to construct an AI-applied mutation fails at the type level (no such endpoint), not at runtime.
- US-13.2 *Every claim cites evidence ids (metric/trace/log/plan/config/cost/schema/event); answers without citable evidence say so.* → context envelopes carry **ids only**; a resolver fetches data at prompt-fill time, RBAC-scoped to the viewer (Law 3).
- US-13.3 *Insights `{severity, evidence[], hypothesis}` from org-wide telemetry scans, evidence re-checked against viewer scope at read time; proposal drafting refuses change classes outside the allowlist (no IAM/secrets/network/destructive).*
- US-13.4 *Disabling the `ai-assistant` policy hides every AI surface instantly, deletes nothing, re-enables instantly; Create/Observe/Deploy/Settings are byte-identical with AI off.* AC: QA scenario 7.
- US-13.5 *Dismissing an insight requires a logged reason; applied proposals audit as "applied as ⟨user⟩, via assistant".*

**Tasks:** T13.1 the 8 read-only tools (`get_metrics get_logs get_trace get_config get_events get_billing_summary price_estimate draft_proposal`) as internal RBAC-scoped services · T13.2 resolver + prompt templates (server-side, from 12-ai) · T13.3 threads/messages store (retained-but-hidden under disable) · T13.4 insight generator job + store · T13.5 proposal store + apply flow (`403` when caller lacks underlying perm) · T13.6 inference via bought API behind a swappable interface (D5) · T13.7 describe-to-provision (AI1) mapping intent → services[] + estimate, unknowns → questions never guesses.

---

### E14 · Data-plane depth (M12) — **10 EW** · V1-tail/Future · Sprints 15–16+

Shaped by the S2 ruling. If accepted as proposed (v1 grows a data-plane section):

- **Wave 1 (read-only, V1-tail, ~5 EW):** SQL exec (read-committed, statement-timeout) + tables + pg_stat_statements insights (D1/D2/D4); Valkey key browse + TTL (D3); messages list + payload (D8/D18); object list (D7); backups list (D5); branches list/create (W5 — also unblocks CLI `db branch`).
- **Wave 2 (destructive, policy-gated + audited, Future, ~5 EW):** redrive/discard/purge, FLUSHALL, restore-to-branch (never in place), shell exec (audited, TTL'd unlock), branch promote/delete.

Also carries the remaining spec-change §2b/§2c/§2d endpoints not already landed by E7–E12 (bindings rotate, domain recheck/delete, schedule/lifecycle per-row ops, deployment pause/abort, org/project transfer, git integration, cell drain/detach) — sequenced by blocked-surface count (§9 of the proposals doc).

---

## 6 · Dependencies

```
Sprint 0 (P1–P5 setup, S1–S7 rulings)
               ▼
E1 substrate ──▶ E2 identity/RBAC/events ──▶ E3 projects/estimates/postgres ──▶ E4 deploy/previews ──▶ M4 ALPHA
     │                    │                            │                              │
     │                    │                            ├──▶ E5 CLI (needs E2 auth, E3 verbs, E4 deploy)
     │                    │                            └──▶ E6 baseline observe/metering (needs E3 services)
     │                    │
     │                    ├──▶ E7 auth hardening ──▶ E8 console slices 2+ (each slice needs its backend epic)
     │                    │
     │                    └──▶ E12 policies (evaluator hook exists in E2; authoring later)
     │
     ├──▶ E9 data layer (drivers plug into E3's provisioner; queue follows the A3.1 review)
     │
     └──▶ E10 observability product (pipeline from E1/E6; notifications need E2 events spine)

E6 metering ──▶ E11 billing (cannot invoice what wasn't metered — D10)
E10 notifications ──▶ E11 dunning/quota emails
E11 + E12 ──▶ E8 slice 6 (settings/billing UI)
E2+E3+E6+E10 read surfaces ──▶ E13 AI tools (AI reads the platform; ships last by Law 4)
S2 ruling ──▶ E14 data-plane depth
First payment (M7) ──▶ capacity knob-turns · hiring unfreeze (D11) · Mumbai cell (A1.7)
```

**Critical path to alpha:** P1 → E1 (spike!) → E2 → E3 → E4 → M4. E5/E6 run parallel off E2/E3. The week-1 substrate spike (ZFS→CNPG branch e2e, ADR-0003) runs first and produces the branch cost/latency numbers; post-A4 it is measurement, not an existential gate.

---

## 7 · Database & API implementation order

From the models.md/openapi.yaml dependency analysis. Migrations land in this tier order; each tier's API endpoints ship with it (contract-first: amend yaml → regenerate types → implement — never hand-write types).

| Tier | Tables | API surface | Epic |
|---|---|---|---|
| 0 | `users` (**spec gap: undefined in models.md — define it first, S1**), `orgs` | auth section (new), `/orgs` | E2 |
| 1 | `members` (≥1-owner trigger), `invites` (partial-unique pending), `tokens`, `events` (append-only), `subscriptions` (row exists, inert), `policies` (evaluation), `quota_usage` | members/invites (12 ops), `/me/tokens`, `/orgs/{org}/audit`, `/orgs/{org}/api-keys` | E2 |
| 2 | `projects`, `environments`, `cells` (registry; cell-0 row) | projects/envs (7 ops) | E3 |
| 2.5 | `estimates` (transient, accepted-before-provision) | `POST /estimates` | E3 |
| 3 | `services` (+`cell_id`, shape jsonb, status machine), `secrets` (**no API surface in v1 — internal only, flagged**) | services CRUD | E3 |
| 4 | `bindings`, `deployments`, `domains`, `lifecycle_rules`, `schedules` | bindings, deployments+rollback, domains, rules `:dryRun`, schedules `:preview` | E3/E4/E9 |
| 5 | `invoices`, payment methods (**absent from models.md — define, S-process**), alert_rules, notifications (**new family — S4**), `dashboards`, `widgets`, `templates` | billing family, observe family, governance family | E10–E12 |
| 6 | `insights`, `proposals`, threads/messages | assistant family | E13 |
| 7 | data-plane depth resources | spec-change §2a | E14 |

**Cross-cutting from tier 0:** `cell_id` columns (invariant 1), `via` actor on events, `*_cents` integers, prefixed ids, cursor pagination helpers, problem+json.

---

## 8 · Workstreams

Two engineers can't hold five swimlanes concurrently; these are *interleaved concerns*, each with a named owner-of-record per sprint:

| Workstream | Contents | Peak sprints |
|---|---|---|
| **W-BE Backend/control plane** | E2, E3, E7, E10–E12 API + domain logic | 2–13 |
| **W-PLAT Platform/data plane** | E1, CNPG/ZFS ops, drivers (E3/E9), deploy pipeline (E4), capacity knob-turns | 1–6, then on-call |
| **W-FE Console integration** | E8 slices, SSE, four-state verification | 6–11, 14 |
| **W-CLI Clients** | E5, SDK-core extraction, `steloit dev` stub | 3–5 |
| **W-QA Quality** | §10 below — suites are built *with* each epic, owned per-epic, audited per-release | continuous |
| **W-AI** | E13 | 13–15 |

Suggested split: Engineer A owns W-PLAT+W-BE-infra-adjacent; Engineer B owns W-BE-product+W-FE+W-CLI; AI agents fan out within each epic exactly as the console build did (spec-extract → parallel build → shared wiring → headless verify).

---

## 9 · Sprint plan & milestones

**Sprint 0 (one week, runs now):** the founder session (P1–P5 + S1–S7) · S3 mechanical yaml fixes committed · content-domain registered · credit/quota applications filed · partner onboarding schedule agreed with the five clients · E1 design docs (Terraform module layout, reconciler protocol) · CLI/SDK language decision. *Exit: all Sprint-0 decisions recorded; Sprint 1 starts unblocked.*

### MVP — Private Alpha (Sprints 1–6, ~90 days; GCP trial credit covers the infra spend)

| Sprint | Focus | Deliverables | Load (EW) |
|---|---|---|---|
| **1** | E1 | **Week-1 substrate spike (ZFS→CNPG branch e2e) + findings ADR** · GCP/Terraform skeleton · GKE up, gVisor pool, image signing | 5 |
| **2** | E1→E2 | Reconciler agent v0 · control-plane DB + PITR · first fire drill · users/orgs/sessions · problem+json framework | 5.5 |
| **3** | E2→E3, E5 starts | RBAC evaluator + matrix tests · events ledger + SSE · members/invites/tokens · projects/envs · CLI skeleton+auth | 6 |
| **4** | E3→E4 | Estimate engine + canon invariants green · Postgres provisioning e2e via reconciler · bindings · CLI create-path | 6 |
| **5** | E4, E6 | GitHub App + build pipeline · deploy + rollback · metrics/logs baseline · metering rollups · CLI deploy/logs/`-f` | 6 |
| **6** | E4 finish | **PR → branched preview → teardown** · masking v0 · content-domain ingress/TLS · E8-lite console · alpha hardening, docs, partner onboarding runbook | 5.5 |

**Milestones:**
- **M1** (end S1): cell-0 alive; spike findings ADR recorded with branch cost/latency numbers (ADR-0003).
- **M2** (end S3): identity + RBAC + audit real; QA scenarios 6 & 8 green.
- **M3** (end S4): `steloit db create` → estimate → approved → `ready`, metered.
- **M4** (end S6, ≈ day 90): **full alpha path live; the five design partners onboard.** Demo = the wedge, for real.

### V1 (Sprints 7–16; infra stays minimal until the Cell-1 scale-out on credits filed in Sprint 0)

| Sprint | Focus | Deliverables | Load |
|---|---|---|---|
| **7** | E7, E8-2 | MFA/WebAuthn/sessions/recovery · org API keys · invite emails · console A/G planes live | 5.5 |
| **8** | E9-1/2, E8-3 | Valkey · object storage (proxied, content-domain URLs) · **queue design review ADR (A3.1)** · TS SDK v0 · console create/services live | 6 |
| **9** | E9-3, E10, E8-4 | Queue impl (per review outcome) · alert evaluator + backtest · custom domains & TLS (F5) · console deploy/previews live | 6 |
| **10** | E10, E11 starts, E8-5 | Notifications family + bell · email service (12 templates) · pricing tables + subscription machine · console observe live | 6 |
| **11** | E11, E12, E8-6a | Quota evaluator + 80% warnings · invoices · policies CRUD+dry-run · console settings/policies live | 6 |
| **12** | E11, E12 | Payment provider · dunning timeline (clock-warped tests) · templates (capture/consume) · plan change/cancel | 6 |
| **13** | E11 finish, E12, E13 starts | **First charge to design partners → M7** · dashboards · AI tools + resolver | 6 |
| **14** | E13, E8-7 | Threads/insights/proposals · describe-to-provision · console AI plane live · **capacity knob-turns post-M7** | 6 |
| **15** | E13 finish, E14-W1 | AI polish + Law-4 disable QA · data-plane read surfaces (SQL/tables/keys/messages/backups/branches list) | 6 |
| **16** | E14, hardening | Remaining spec-closure endpoints · load/failure drills · **external security review / pen-test** · SOC2-lite hygiene checklist · **v1 GA review → M8** | 5 |

**Milestones:** **M5** (end S9): console fully usable for the alpha path against the real API. **M6** (end S10): data layer complete. **M7** (S13): first payment clears — hiring unfreeze, knob-turns, Mumbai. **M8** (end S16): v1 GA-ready (public signup timing is a separate founder call, contingent on abuse controls per A1.8).

---

## 10 · Testing & QA plan

QA is not a phase; each epic ships with its suite. The package already defines the targets:

1. **The 10 canonical scenarios** (`16-qa/qa.md`) become the E2E backbone, seeded from `19-canon/fixtures.json`, run headless (Playwright — the console repo's verify-script pattern graduates into CI). Mapping: sc-6/8→E2 · sc-9/10→E3 · sc-1→E4+E10 · sc-2/3/4→E11 · sc-5→E12 · sc-7→E13.
2. **Arithmetic invariants imported, never retyped** (canon.md §invariants): unit tests in the estimate engine, invoice generator, and console (already present) — the same numbers, three layers.
3. **Contract tests:** every service's handlers validated against openapi.yaml (schema round-trip); generated clients (console, CLI, SDK) rebuilt in CI on any yaml change so drift fails the build, not the user.
4. **RBAC table-driven suite:** full `rbac-matrix.csv` × endpoint sweep; policy tighten-only property tests; denial responses name role/policy (E3 grammar).
5. **UI consistency checks** (already automated in the console audits): one active nav item, banner class set, no inline forms, h1 grammar, disabled-with-reason, money formatting — keep green per slice flip.
6. **Regression checklist per release** (16-qa): the 15-line list including a11y (keyboard-only, focus trap, contrast).
7. **Platform drills as tests:** monthly environment rebuild (invariant 3), monthly tenant-restore-and-diff (invariant 9), control-plane DB restore (invariant 10) — calendared, with drill logs committed.
8. **Fault injection:** the audit's sessionStorage fetch-wrapper technique generalizes to a chaos flag in staging (problem+json 4xx/5xx, latency) — four-state truth verified per slice.
9. **Billing time-travel:** dunning/anchor/proration tests run against a warped clock, never real waits.

---

## 11 · DevOps & deployment

- **IaC:** everything Terraform + git (invariant 3); two GCP projects (control-plane, cell-0); no console-clicked resources, ever.
- **CI/CD:** GitHub Actions → build → sign (provenance, invariant 11) → deploy control plane via its own reconciler-applied manifests. Workload identity only; zero static SA keys including CI.
- **Environments:** founder dev = us-central1, destroyable, duty-cycled (declared downtime, A1.6/A1.7). Partner-touchable = born in asia-south1, core pool 24/7 from the first partner onward. No migrations between them — parallel builds.
- **Secrets:** OpenBao/KMS envelope (D5); never invent crypto.
- **Observability of Steloit itself:** the same Loki/OTel pipeline, separate tenancy labels; alerting to founders (paging class exempt from quiet hours, eating our own F6 rules).
- **Backups/DR:** control-plane PITR to separate GCS location; DR = restore desired state, reconcile actual from cells (A2.5); drills per §10.7.
- **Cost guardrails:** billing alerts at 50/80/100% of the trial credit; §5 prices re-verified before any financial decision after ~2026-10-07 (they expire).
- **Abuse controls before any open signup:** invite-only, no anonymous compute, per-tenant egress caps, CPU-pattern detection (A1.8).

---

## 12 · Release plan

| Release | When | Audience | Contents | Entry criteria |
|---|---|---|---|---|
| **Alpha** (M4) | ≈ day 90 | The five committed design partners | The wedge path, CLI-first + E8-lite console; honest scope notes (region, RPO per P4, support commitment) | Sprint-0 items done; 10-scenario subset green; fire drill done |
| **Beta** (M5–M6) | Sprints 9–10 | Partners + waitlist trickle | Console live, data layer complete, notifications/emails | E8 slices 2–5 green; abuse controls for trickle |
| **Paid** (M7) | Sprint 13 | Partners convert | Billing end-to-end, plans, dunning | QA sc-2/3/4 green; payment provider live; pricing published |
| **v1 GA** (M8) | Sprint 16+ | Public signup = separate founder decision | Full V1 scope incl. AI plane, data-plane reads | Regression checklist; abuse controls full; Cell-1 on credits |

Rollout mechanics: feature flags per console slice; canon mode ships forever as `demo.steloit.app` (the one demo world, ADR-026); versioned API stays `/v1` — additive changes only, the S-process governs the yaml.

---

## 13 · Risks, assumptions, blockers

### Blockers (hard, now — all founder-owned, all fit in Sprint 0)
| # | Blocker | Unblocks |
|---|---|---|
| B1 | GCP account/billing not set up (P1) | Sprint 1 |
| B2 | Content domain unregistered (P2) | E4 previews, E9 object URLs |
| B3 | RPO value unratified (P4) | ToS, partner expectations doc, WAL-archiving config |
| B4 | S1 auth-surface & S7 idempotency rulings | E2, E3 |
| B5 | Credit apps + quota filings not submitted (P3) | Cell-1; large node pools |

### Top risks
| # | Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|---|
| R1 | Substrate spike surprises (ZFS snapshot semantics, CNPG recovery/hibernation edge cases) | Low | Medium — all documented paths; the prior Critical Neon↔GCS risk was retired by ADR-0003 | Week-1 spike measures; CNPG pinned ≤1.30 until barman-cloud plugin stabilizes |
| R2 | **Partner expectations exceed alpha scope** — five clients ready to *use* the platform; the alpha ships one path | High | High — churn before revenue | P5 expectations doc up front; visible V1 roadmap; weekly partner check-ins; E8-lite gives early visual progress |
| R3 | Queue scale-to-zero constraint unsolvable at the WAL layer | Medium | High — queues slip or lose branch-coherence via NATS | A3.1 design review is a Sprint 8 deliverable with its own ADR; queues are not on the alpha path |
| R4 | Two-founder ops load (CNPG/ZFS fleet, support commitment to five partners) | High | High | Duty-cycle discipline pre-partner; alerting hygiene; M7 unfreezes hiring (D3 budgets 2–3 seniors when hiring starts) |
| R5 | GCP quota walls on fresh account | High | Medium — days of stall | Staged quota filings in Sprint 0 and at each scale-out; track headroom per sprint review |
| R6 | Free-compute abuse post-open-signup | High (if opened) | High | Invite-only through V1; abuse controls are a GA entry criterion |
| R7 | Estimate ≠ invoice drift as pricing evolves | Medium | High — breaks the product's soul | One pricing table, three consumers; invariant tests at all layers (§10.2) |
| R8 | Spec debt resolved ad-hoc instead of via rulings | Medium | Medium — grammar erosion | The S-process: yaml amended first, clients regenerated, conflicts surfaced never silently fixed (agent-guide §3) |
| R9 | Effort estimates are founder-optimistic | High | Medium | Sprint-2 recalibration checkpoint; MVP scope is deliberately narrow so slip → later date, not broader scope |
| R10 | §5 prices stale after 2026-10-07 | Certain | Low | Re-verify before financial decisions (§8.5) |

### Assumptions
- 2 founding engineers + AI-agent leverage; no hires until M7 (D11).
- 2-week sprints; ranges ±40% until Sprint 2 calibration.
- The five committed clients are available for onboarding at M4 and willing to convert to paid at M7 pricing.
- Console repo is the frontend of record; no screen redesigns during integration (frame gallery remains spec).
- Canon fixtures remain the seed/demo/QA world (ADR-026); S5/S6 rulings land before QA baselines freeze.
- Payment provider decision (Razorpay vs Stripe vs both) made by Sprint 11.
- The 12 email templates ride an established provider (build-vs-buy: buy — comms infra is a never-build, ADR-003).

---

## 14 · Effort summary

| Epic | Name | EW | Phase |
|---|---|---|---|
| E1 | Platform substrate (Cell-0) | 8 | MVP |
| E2 | Identity, RBAC, events spine | 7 | MVP |
| E3 | Projects, estimates, Postgres | 9 | MVP |
| E4 | Deploy & branched previews | 9 | MVP |
| E5 | CLI v0 | 5 | MVP |
| E6 | Baseline observe + metering | 4 | MVP |
| **MVP total** | | **42 EW ≈ 6 sprints @ ~5.5–6 EW loaded** (fits the ~90-day target with zero slack — R9 applies) | |
| E7 | Auth hardening | 4 | V1 |
| E8 | Console integration | 10 | V1 |
| E9 | Data layer (Valkey/Storage/Queue) | 8 | V1 |
| E10 | Observability + notifications + emails | 8 | V1 |
| E11 | Billing & subscription | 9 | V1 |
| E12 | Policies, templates, dashboards | 7 | V1 |
| E13 | AI plane | 8 | V1 |
| E14 | Data-plane depth (wave 1) | 5 | V1-tail |
| E4.8 | Custom domains & TLS (F5) | 1 | V1 |
| T4.9 | Masking-by-policy V1 depth (F14) | 3.5 | V1 |
| US-11.7 | Hard spend cap (F9 flagship) | 0.5 | V1 |
| E5.6–7 | TS SDK v0 + docs | 1.5 | V1 |
| **V1 total** | | **65.5 EW ≈ 10–11 sprints** | |
| **Grand total to v1 GA** | | **~108 EW ≈ 16 sprints ≈ 8 months** with the stated team | |

**Revision note (2026-07-18 review):** gaps closed — custom domains/TLS (F5) was specced in module M5 and DB tier 4 but owned by no epic (now E4 T4.8); TS SDK and documentation had workstream mentions but no tasks (now E5 T5.6/T5.7); Sprint 16 gained an explicit external security review. Duplication check: the E6-metering vs E11-billing split and the E8-lite vs E8 slice overlap are intentional and stand.

---

*Next actions:* (1) the Sprint-0 founder session — setup items P1–P5 + spec rulings S1–S7 in one sitting; (2) commit the S3 mechanical yaml fixes; (3) Sprint 1, starting with the substrate spike (ADR-0003).
