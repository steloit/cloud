# Spec-change proposals — findings from building the console

**Provenance:** `steloit/console` main `9dd8dfa`→`85e96ca` (the six-pass design audit, 2026-07-11). Every finding below lives as a comment or `disabledReason` in the console source; representative citations are `console:src/...` paths. Nothing here was resolved locally — per `22-agents/agent-guide.md` §3/§4 ("spec change first" · "propose the owner-level change") each item is filed against its owner for human sign-off. `00-sources/`, the constitution, and the ADR log change by human decision only; the ADR drafts in §7 are *proposals*, not entries.

**Resolution rules the console applied while waiting:** fixtures win for data facts (ADR-026), frame copy wins for microcopy, and every conflict is carried as an in-code finding. Gated verbs are visible-but-disabled with the gap named (B6 grammar) — the `disabledReason` strings in the UI are the canonical statement of each missing capability.

**Cross-checked against the ADR rejection list:** nothing below re-proposes a new primitive (ADR-001), a never-build class (ADR-003), AI auto-apply (ADR-005), env-as-container (ADR-013), the create-dialog (ADR-014), new rail items (ADR-017), inline forms (ADR-019), the FAB (ADR-020), or template linking (ADR-021). Where a proposal *touches* one of these, the tension is stated.

---

## 1 · The headline: the data plane vs the API's own promise

`08-api/openapi.yaml` (info.description, lines 5–8) promises:

> "CLI (`steloit …`) is a thin client of the same endpoints; **no console-only capabilities exist.**"

But the D-series depth tabs — specced in full by the gallery — require an entire data plane the API doesn't carry. Today every one of these is a designed surface whose verbs are honestly gated:

| Capability | Frames | Console evidence |
|---|---|---|
| SQL execution + table data + pg_stat_statements | D1, D2, D4 | `svc.$service.sql-editor.tsx:15`, `tables.tsx:14`, `insights.tsx:18` |
| Key browse / TTL edit / CLI exec (Valkey) | D3, D6, D14 | `data-browser.tsx:196`, `cli.tsx:11` |
| Messages: ready/in-flight lists, payloads, redrive, discard, purge | D8, D18 | `messages.tsx:16`, `overviews.tsx:309`, `tabs/queue.tsx:118` |
| Object list/upload/signed URLs, public-access request | D7, D16 | `objects.tsx:72`, `overviews.tsx:244` |
| Backups: list, snapshot-now, restore-to-branch, drills | D5 | `backups.tsx:21` |
| Branches: list/create/promote/delete | W5, D5 | `branches.tsx:143` |
| Shell exec (web/worker) | D20 | frame-fixed transcript only |
| FLUSHALL / flush | D14 | `tabs/valkey.tsx:310` |

**Decision needed (ADR draft in §7.1):** either the v1 API grows a data-plane section, or the promise is scoped ("no console-only *control-plane* capabilities") and the D-tabs are re-labeled as a later API phase. The console can render either outcome; it cannot honestly render the current contradiction. Note: operating one's own provisioned services is not an ADR-003 never-build class — this is product depth, not IaaS.

## 2 · 08-api: proposed new endpoints

All paths follow the spec's own `x-conventions` (plural nouns, `:verb` action sub-resources, prefixed ids, problem+json with required `remediation`, cursor pagination, `?env=` filter, `x-streamable` where live). Grouped by area; each row cites one representative gated surface.

### 2a · Data plane (pending the §1 decision)

| Proposed | Blocks today | Evidence |
|---|---|---|
| `POST /services/{service}/queries` (SQL exec) + `GET /services/{service}/tables` | D1/D2 run + table viewer | `sql-editor.tsx:15` |
| `GET /services/{service}/insights` (pg_stat_statements) | D4 rows | `insights.tsx:18` |
| `GET /services/{service}/keys` + `PATCH /keys/{key}` (TTL) + `POST /services/{service}/commands:exec` | D3 browse, TTL edit, D6 CLI | `data-browser.tsx:196` |
| `GET /services/{service}/messages?state=` + `POST /messages/{msg}:redrive` · `:discard` + `POST /services/{service}:purge` | D8/D18 | `messages.tsx:16` |
| `GET/POST /services/{service}/objects` (+ signed-URL grant) | D7 | `objects.tsx:72` |
| `GET /services/{service}/backups` + `POST /backups/{bkp}:restore` (into a new branch, never in place) | D5 | `backups.tsx:21` |
| `GET/POST /services/{service}/branches` + `POST /branches/{branch}:promote` + `DELETE` | W5/D5 | `branches.tsx:143` |
| `POST /services/{service}/shell:exec` (audited, TTL'd unlock) | D20, D6 write-unlock | `cli.tsx:54` |
| `POST /services/{service}:flush` | D14 | `tabs/valkey.tsx:310` |

### 2b · Control plane & lifecycle

| Proposed | Blocks today | Evidence |
|---|---|---|
| `POST /bindings/{binding}:rotate` | D11 rotate | `bindings-tab.tsx:187` |
| `POST /domains/{dom}:recheck` + per-domain resource (`DELETE`, make-primary) | U5/D21 | `domain-drawer.tsx:60` |
| `GET/PATCH/DELETE /schedules/{sch}` + `POST /schedules/{sch}:run` | D17 per-row actions, manual run | `schedules.tsx:161`, `overviews.tsx:566` |
| `GET/PATCH/DELETE /lifecycle-rules/{rule}` | D15 per-row actions | `lifecycle.tsx:144` |
| `POST /deployments/{dep}:pause` · `:abort` | DP2 controls | `deploy.$dep.tsx:28` |
| `DELETE /envs/{env}` (preview teardown, U6 typed-confirm) | DP3 | `deploy.previews.tsx:117` |
| `PATCH /envs/{env}` (rename — the ADR-037 escape hatch for teams that want a different word than `production`; rename consequences stated: `?env=` deep links use names) | ADR-037 / implicit-environment-ux.md | founder-ratified 2026-07-18 |
| `POST /projects:batch` or estimate-linked multi-create | AI1 review-and-create | `create.tsx:569` |
| `DELETE /orgs/{org}/api-keys/{key}` | G4 revoke (list+create only today) | `api-keys.tsx:200` |
| `POST /orgs/{org}:transfer` + `POST /projects/{project}:transfer` | G5/G1 transfer | `settings.general.tsx:90` |
| Template revisions (`GET/POST /templates/{tpl}/revisions`) | T2 shape editing | `templates.tsx:55` |
| Git integration (`/orgs/{org}/git` connect/disconnect) | G12 | `settings.git.tsx:111` |
| Cell ops (`POST /cells/{cell}:drain` · `:detach`) | X2 (connect is already spec'd) | `settings.cells.tsx:60` |

### 2c · Observe & notifications

| Proposed | Blocks today | Evidence |
|---|---|---|
| **Notifications family** — `GET /me/notifications`, `POST /notifications/{ntf}:read`, approval threads, webhook routes + test delivery | N1–N3 bell/inbox, U8 webhook, policy Review/Deny (highest-frequency gap: 8 surfaces) | `notifications/data.ts:6` |
| `GET/POST /projects/{project}/silences` | O5 Silences | `alerts.tsx:164` |
| `GET /projects/{project}/alert-firings` | O5 History | `alerts.tsx:116` |
| Logs: `?around=` context param, saved searches, message deep-links | O3 | `logs.tsx:177,232` |
| Metrics: `?compare=` and `?group_by=` params | O2 compare/split-by | `metrics.tsx:302` |
| `GET /envs/{env}/traces` (list; spans for more than one trace) | O6 selection | `traces.tsx:17` |
| Dashboards: fork/duplicate/delete-widget — LANDED in T12.7 as `POST /dashboards/{dash}/fork` · `/duplicate`, `DELETE /dashboards/{dash}/widgets/{wdg}` (sub-path form for stdlib-mux compat). **Share grants DEFERRED**: no shape in §2c and no restricted-share member model exists (restricted=owner-only, T12.6) — needs a founder decision on the grant model before implementing. | DB2/3/5/7 | `dashboards.$dashId.tsx:77` |

### 2d · Account, auth & billing

| Proposed | Blocks today | Evidence |
|---|---|---|
| **Auth section** — sign-in/session, password change, MFA/WebAuthn, recovery codes, session list/revoke | A-series + P3/P4 (today the console fakes a local session seam — `login.tsx:10`) | `security.tsx:16`, `sessions.tsx:13` |
| `PATCH /me` notification settings + avatar upload | P6, P2 | `account.notifications.tsx:149` |
| Leave-org / account-delete (grace-windowed) | P2 | `profile.tsx:90` |
| `PUT /orgs/{org}/billing/budget` | B1 set-budget | `billing.overview.tsx:192` |
| Billing exports (PDF/CSV), invoice line→usage breakdown (schema carries `usage_ref` already), disputes/billing threads | B3/B4 | `invoices.tsx:39,256` |
| Payment-method management (replace/add-backup — POST exists, no update/delete) | B5 | `payment.tsx:60` |
| Project-scoped membership & roles (today "the API only models org membership") | G-project members | `$project.settings.members.tsx:23` |
| Per-service AI policy override (within ADR-005's laws — an override *tightens*, never loosens) | AI3 per-service | `svc.$service.ai.tsx:326` |
| Assistant activity ledger (`GET /assistant/activity`) | AI7 | `assistant.activity.tsx:10` |

## 3 · 08-api: schema amendments

| Schema | Change | Evidence |
|---|---|---|
| `PATCH /services` body | add `name` (rename re-derives binding env-vars — consequence documented in the console's rename modal) | `tabs/postgres.tsx:621` |
| `PATCH /services` body | add `vars` (env-var CRUD) and a mode/restart contract for shape changes (D14) | `tabs/web.tsx:155` |
| `Token.scope` | enum too narrow (`full`\|`read_only`) for the frame's scope strings | `api-keys.tsx:32` |
| `/invites/{invite}/renew` | **spec defect:** path param `{invite}` missing → generated client has `path?: never` | `invites/hooks.ts:23` |
| `DELETE /invites/{invite}` | missing entirely (console calls it via raw client) | `settings.members.tsx:72` |
| `Widget.viz` | enum (`area`,`list`) diverges from the DB8 drawer's `bar`/`log stream` | `dashboards.$dashId.tsx:161` |
| `PATCH /assistant/insights/{ins}` | add snooze `duration` | `assistant.insights.tsx:137` |
| Domains / lifecycle-rules / schedules | add ownership (`service_id`) — fixtures carry none; the console scopes them by hand to D21 api / D15 assets / D17 jobs + S5 worker | `mocks/handlers.ts:272` |
| Schedule / LifecycleRule | add run history (`last_run_at`, `next_run_at`, outcome) | `schedules.tsx:20` |
| Policy | expose archived versions; add changed-by; scope finer than org/project if G7's chips are real | `policies.new.tsx:85` |
| Dashboard create body | project scoping beyond the current project id | `new-dashboard-modal.tsx:98` |
| Create-service body | more than one lifecycle rule; a catalog endpoint for type-block options | `type-blocks.tsx:730` |
| W2 create | accept a `template` param (template-prefilled create) | `template.$tpl.tsx:161` |

## 4 · 19-canon: fixture additions & corrections

Additions must keep every arithmetic invariant green (`19-canon/canon.md` §invariants) and conform to schemas (ADR-026).

**Additions (sections that don't exist):** billing usage/invoices/payment-methods (`world.ts:190`); domains/lifecycle-rules/schedules *with ownership* (`world.ts:438`); deploy `dep_141` (frames reference it, fixtures lack it — `deploy.$dep.tsx:124`); message payload for `msg_9f224`; spans for at least one more trace; widget data for the DB5/prebuilt dashboards beyond checkout-health/PostgreSQL-Health/Infrastructure/Cost (`dashboards.index.tsx:43`); alert-firing history (the Jul 2 incident as data); a canon cost series for O2's Cost tab (`metrics.tsx:221`).

**Corrections (fixture-internal defects):** B3 invoice lines don't sum to their printed subtotal (`world.ts:194`); B2 egress 41.3 GB contradicts the quotas fixture 87/100 (`world.ts:193`); internal-tools services total $96 vs the gallery card's $41 — the name↔cost swap (`world.ts:467`).

**Two canon decisions (ADR drafts §7.2–7.3):** whether canon mode should be *stateful* for mutations (today renames/deletes echo once and fixtures win on refetch — the DB8 precedent, ~8 sites, `handlers.ts:70`), and whether the X1 gateway becomes a canon service (today it's a client-side exemplar because adding $34 breaks `7 services = $208` — a new invariant or a revised one is required, `gateway-tabs.tsx:77`).

## 5 · 00-sources: frame ↔ fixture/spec conflicts (human decision only)

Each needs a ruling: update the frame, update the fixture, or record the divergence as intended.

| # | Conflict | Citations |
|---|---|---|
| 1 | Typed-confirm grammar: G1 frame says "type the project **slug**", the U6 contract types the **name** | `$project.settings.general.tsx:122` |
| 2 | Alert rule names: frame `api-p95-burn` (+6-rule set) vs fixtures `p95-slo` (+4 rules) | `alerts.tsx:18,119` |
| 3 | DP1 frame shas (`a41f2c`, `88ba02`) vs fixtures' `git_sha` (`a71c9e2`, `b3f19d0`); #141 in frames, not fixtures | `deploy.$dep.tsx:39,124` |
| 4 | Events actor: frame says asha, fixtures differ | `events.tsx:19` |
| 5 | B-series numerics: B7 quota "6.2M/50M", budget 38.1%, env costs, billing dates (frame Mar/Apr vs fixtures' July cycle) | `quotas.tsx:29`, `overview.tsx:350`, `payment.tsx:32` |
| 6 | U2 bindings: frame shows api→assets selectable, fixtures already bind it | `bindings-tab.tsx:309` |
| 7 | X1 gateway: sub-tab frames don't exist; "3 routes" never named | `gateway-tabs.tsx:18` |
| 8 | O2 category tabs (Databases/Queue/Network/Cost) unspecced by any frame | `metrics.tsx:18` |
| 9 | Frames show bare page titles ("SQL Editor") vs the design system's "Area · Thing" grammar (~30 pages; console follows the grammar) | `account.tokens.tsx:105` |

## 6 · 01-design-system: adopt what the console had to invent

Per agent-guide §3 ("never invent a component… design-system change first") these were built out of necessity during the audit and should be ratified or replaced by the owner (ADR-010 pioneer check applies):

1. **`--scrim` token** (`rgba(6,9,12,.44)`) — the value already lives verbatim in the overlay recipes; it was never a token. → add to `15-assets/tokens.css` + `tokens.json`.
2. **Type ramp** — a discrete scale `10 / 10.5 / 11 / 11.5 / 12 / 12.5 / 13 / 14` px (10px floor), materialized from the gallery's own values; design-system.md says "8–22px" loosely. → document the ramp; the console's audit found 41 sub-10px uses worth prohibiting.
3. **Chart height tiers** `sm 80 / md 120 / lg 160` — no chart-height scale exists anywhere in the spec.
4. **Dot tones `none` (hair) and `ai` (assist)** — the six-mark vocabulary lacked a neutral/read dot and an assistant-provenance dot; both were needed by N1's design.
5. **Sprite additions** — the inventory (35 ids) lacks: up-caret/trend arrow (▲), alert triangle (⚠), star (★), and a small select-caret distinct from `s-chevd`; these render as unicode text today. Also: `icons.md` says 24×24 grid while `21-playbooks/new-product.md` Step 1 says 20px/1.5px — reconcile.
6. **Tooltip primitive** — none exists; every hint is native `title=` (invisible to keyboard users). The B6 disabled-reason grammar deserves a real component.
7. **Org-avatar gradient** — `linear-gradient(135deg,#E36C4B,#B34A2E)` appeared inline in four gallery-derived spots; the console extracted `.orggrad`. → token or documented exception.
8. **Title grammar ruling** — see §5.9; if "Area · Thing" is confirmed, the frames should be annotated so future readers don't re-diverge.

## 7 · Proposed ADR drafts (for `18-philosophy/decisions.md`, human sign-off)

**7.1 — Data-plane API scope.**
*Decision (proposed):* v1 grows a data-plane section (§2a) behind the same auth, with destructive verbs policy-gated and audited; the "no console-only capabilities" promise stands.
*Context & alternatives:* the D-series frames spec a full data plane the API lacks (§1); alternative is scoping the promise to the control plane and re-labeling the D-tab verbs as phase-2 API.
*Why:* the promise is load-bearing for the CLI story ("CLI verbs fall out of the noun-verb grammar", playbook §4); a console that shows what the CLI can't do breaks it.
*Consequences:* ~20 endpoints; canon needs data-plane fixtures; not an ADR-003 violation (operating owned services ≠ IaaS product).
*Refs:* openapi.yaml info.description; D1–D22 frames; console findings A1–A17.

**7.2 — Canon statefulness.**
*Decision (proposed):* canon mode stays stateless for fixture entities (mutations echo; fixtures win on refetch) and stateful only for user-created entities — ratifying the console's existing behavior (stateful createdServices/createdEnvironments; echo for renames/deletes of canon objects).
*Refs:* ADR-026; `console:src/mocks/handlers.ts:70`.

**7.3 — X1 gateway canonization.**
*Decision (proposed):* either (a) the gateway joins canon as an 8th ecommerce service and the invariant becomes `208 + 34 = 242`, with frame updates to every $208 mention, or (b) X1 stays a documented exemplar (status quo, client-side synthetic service).
*Refs:* X1 frame; `console:src/features/services/gateway.ts`; canon.md invariants.

**7.4 — Auth surface.**
*Decision (proposed):* the API gains an auth/session section (§2d) — today sign-in exists only as a console-local seam, which is itself a "console-only capability" in the inverse direction.
*Refs:* `console:src/lib/session.ts`, `login.tsx:10`.

## 8 · 02-information-architecture: route registrations

Routes the console added minimally (each carried as a finding) that the URL map should adopt: `/forgot-password`, `/reset-password`, `/$org/new-project`, `/$org/$project/new-env`, `/$org/$project/environments`, `/$org/notifications`, `/account/sessions`, `/$org/$project/deploy/$dep` (per-deployment detail), `/$org/$project/instances/$product`, org-level `/$org/assistant/*`. Citations: `forgot-password.tsx:11`, `new-env.tsx:23`, `environments.tsx:20`, `deploy.$dep.tsx:24`, `assistant.tsx:8`.

## 9 · Priorities

By number of blocked surfaces: **notifications endpoints** (8 surfaces) → **the §1 data-plane decision** (~20 surfaces across all D-tabs) → **gateway canonization** (6) → **dashboard fixture coverage** (6) → **auth section** (the whole A/P plane rests on a local seam) → **the two spec defects** (invite renew path param; invite DELETE missing) which are mechanical and unblock generated-client cleanliness immediately.

## 10 · Findings from implementation (Phase 2 review loop, 2026-07-19)

| Finding | Evidence | Proposed change |
|---|---|---|
| **List operations declare no `cursor`/`limit` query params** (listProjects, listEnvironments, listServices, listDeployments, listPersonalTokens, listInvites, listMembers…) while api-conventions mandates cursor pagination and every List schema carries `next_cursor` — no client can request page 2 | surfaced by the SDK pagination review (PR #233); generated ops have `query?: never` | add `cursor` (+ optional `limit`) query params to every list op in openapi.yaml; until ruled, SDK iterators stop after page one on cursorless ops (guarded, never a duplicate loop) |
| **Problem emission drifts from the schema**: the server's `problem` package writes `reasons: []string` and a `retry_after_s` body field; the contract says `reasons: [{code, message, remediation}]` and carries 429 timing in the `Retry-After` header only | services/api/internal/platform/problem/problem.go vs openapi.yaml `Problem` | server-side conformance fix (structured reasons; drop the body field — header stays); SDK tolerates both shapes during the window |
| **Repo-link management has no contract surface** — G3's "per-service repo map" (link/unlink a repo+branch to a service, list links, installation status) exists only at the store layer; the console's G3 screen and the CLI need endpoints | T4.1 built the storage + webhook ingress; no ops exist in openapi.yaml | add /services/{service}/repo-link (PUT/DELETE/GET) or equivalent — owner-level shape call |
| **Concurrent owner-mutation race** (medium): leaveOrg / removeMember / changeMemberRole / deleteAccount can, under overlapping READ-COMMITTED txns on the SAME org, each see the other's uncommitted owner row and both commit → zero live owners. The `members_keep_owner` trigger's NOT EXISTS check isn't serialized. | QA finding on T7.6 | serialize owner-count mutations per org (e.g. `pg_advisory_xact_lock(hashtext(org_id))` at the start of each path, or `SELECT … FOR UPDATE` on the org's owner rows) + a purge-job re-check — a deliberate locking change, its own task |
| **G6 flagged-resources on leave/remove is shape-only** until E10 ships ownable resources (dashboards etc.); leaveOrg's 204 carries no list and removeMember returns an empty one — the "owned resources flagged, never reassigned" promise becomes real when ownable resources exist. | reviewer finding on T7.6 | populate FlaggedResource once E10 lands ownable resources |
| **leaveOrg does not route through the two-layer Authorizer** — membership itself is the grant and the WHERE-clause fences foreign orgs (404), but no policy can gate leaving and no denial is audited. | reviewer note on T7.6 | owner decision: is "leave" ever policy-gateable? if so, add a permission + route it |
| **WebAuthn/passkeys (T7.1 remainder) is browser+domain gated**: webauthnRegister/Login begin/finish need a bound relying-party origin (the deployed content domain — P2/domains) and a browser authenticator to exercise; they are NOT in the strict server yet (nothing generated goes unimplemented). TOTP + recovery is the buildable, testable MFA core. | T7.1 | implement WebAuthn with go-webauthn once the origin is bound (P2) + a browser E2E in E8; the LoginResult already lists webauthn first (passkeys-first, ADR-0006) |
| **totpVerify returns 204 but recovery codes are minted at verify**: the contract's verify is 204 (no body), so the reveal-once recovery codes are surfaced via a follow-up `recovery:regenerate` call rather than the verify response. A client enrolling must call regenerate to see them. | T7.1 | consider widening totpVerify's 200 to carry the first recovery-code set, or document the two-step enroll → regenerate flow |
| **Org display name is unbounded and unvalidated** (`orgs.go` CreateOrgFull/UpdateOrg; openapi PATCH /orgs has no maxLength/pattern): only the slug is charset-checked. A crafted name reaches any surface that renders it. T10.4 escapes it at the email boundary (html.EscapeString), but the root fix is input validation (length cap + no control chars) — a follow-up E2 task. | T10.4 review | add name length/charset validation in UpdateOrg + the openapi schema |
| **CreateInvite is not rate-limited** (only login is): an authenticated `members.invite` holder can drive templated emails from the trusted noreply@steloit.app domain (spam/phishing amplification). Add a per-org/per-actor rate limit on invite creation. | T10.4 review | rate-limit CreateInvite (E7/E10 follow-up) |
| **Noop email provider in production is a silent drop**: a prod deploy missing RESEND_API_KEY logs at info and drops all mail. There is no environment flag to fail-fast on. Add one (or gate on COOKIE_SECURE) to refuse boot when prod + no key. | T10.4 review | fail-fast startup guard when prod && RESEND_API_KEY empty |
| **The events spine is org-scoped (events.org_id NOT NULL REFERENCES orgs), but account-level auth facts have no org** (password reset, and future: login-from-new-device, security notices). T7.2 resolved this by NOT putting reset on the org spine — the reset-token row is its own durable trigger (mailer.AccountSource), reusing the same idempotent Dispatch + email_deliveries ledger. Open question for a spine-model ruling: should the spine gain account-scoped events (org_id nullable, ~27-site ripple + a GOV-002 primitive change) so account security actions have a first-class audit trail? Currently password-reset request/complete are NOT audited (they can't go on the org spine). | T7.2 | founder/spec ruling on account-level events + a personal security-audit log |
| **Password reset does not clear MFA**: if an attacker enrolled their own TOTP during a compromise, a reset revokes sessions+tokens but leaves attacker MFA — the victim can be locked out post-reset. Standard practice keeps MFA separate (recovery via recovery codes), but the interaction with attacker-enrolled MFA warrants a product decision. | T7.2 review | decide whether reset should also disable MFA / notify on MFA change |
| **Reset token travels in a URL query param** (`?token=`): leaks via Referer, browser history, access logs. Bounded by single-use + 1h TTL (standard), but the E8 /reset-password route should load no third-party assets and scrub the token from history after consumption. | T7.2 review | frontend hardening in E8 |
| **The invoice generator has no production caller**: `invoice.Close(orgID, period)` is exercised only by its test — nothing invokes it on the billing anchor, so no `open` invoices are ever produced in a running system. The monthly-close scheduler (a periodic sweep closing each org's *previous* period as its anchor passes, tracking closed periods so it fires once) is separate infra. | T11.3 review | file the monthly-close scheduler as a follow-up (candidate T11.6), landing with the T11.4 payment layer that consumes `open` invoices |
| **No proration on mid-period plan change**: `Close` uses the org's plan at close time for the whole-period fee line — an upgrade/downgrade mid-period over- or under-bills the fee (metered lines are unaffected; they're already priced per-accrual). | T11.3 review | proration is a follow-up gated on real billing-anchor semantics (when the anchor + plan-change timestamps are both first-class) |
| **Invoice lines can't attribute to a project**: `GetQuotaUsage` aggregates the meter per `(org, meter, period)` with no project dimension, so every invoice line is emitted with `project_id = nil`. B3/B4's per-project usage breakdown is structurally impossible until the meter carries a project key. | T11.3 review (arch) | add a project dimension to the rollup/quota_usage before promising per-project invoice attribution — owner-level data-model call |
| **Invoice `tax` is always nil**: the schema/migration provision `tax jsonb` ("GST etc.") but `Close` hardcodes `Tax: nil`. For an India-domiciled org this is an invoice-completeness gap. | T11.3 review (arch) | compute tax in the T11.4 payment/tax layer (GST/GSTIN), not the generator |
| **Read surface masks corrupt `lines` jsonb**: `invoiceToAPI` unmarshals lines under `err == nil && len > 0` — a corrupted jsonb drops all lines but still returns `total_cents`, so a total appears with no lines instead of a 500. Also `int64 → int` cent narrowing is lossless only on 64-bit. | T11.3 review (QA) | surface the unmarshal error (or log+alert); pin/bound the 64-bit cent cast — code hardening follow-up |
| **`/webhooks/{wbh}:test` is unroutable by Go's stdlib mux**: a `:verb` custom-method glued to a path parameter panics oapi-codegen's generated router ("wildcard segment must end with }"). It is the ONLY such path; all other custom methods put `:verb` on a literal segment. T10.3 handles it with a pre-strict manual route (the SSE pattern) and keeps it out of the strict include list. | T10.3 | either bless the pre-strict shim as the convention for `{param}:verb`, or restructure the path to `/webhooks/{wbh}/test` so it generates natively |
| **T10.3 bell/email routing has no production caller**: `notify.Router.Notify` (fan an event to org members' bells + email) is exercised only by its test — no domain event calls it yet, so nothing rings the bell in a running system. The webhook route IS wired (its outbox runs off the spine in `main`). Wiring specific domain actions to `Notify` needs the P6/U8 frame-verbatim microcopy (titles) the router must not invent. | T10.3 | a follow-up that maps notification-worthy actions → titles (from frames) and calls `Notify` at each site, or a spine-scan projection once titles are data |
| **No dedicated `webhooks.manage` permission**: webhooks reuse `api_keys.manage` (owner+admin) as the closest integration-credential class. Finer granularity (a role that manages webhooks but not API keys) isn't expressible. | T10.3 | add `webhooks.manage` to the rbac matrix if the product wants it distinct from API keys |
| **Webhook outbox is single-replica-safe only**: `ListPendingWebhookEvents` has no `FOR UPDATE SKIP LOCKED` and `ClaimWebhookDelivery`'s `ON CONFLICT DO UPDATE` re-claims an in-flight `pending` row (needed for crash recovery, but it can't tell "crashed" from "another replica is mid-send"). Two API replicas both running `RunOutbox` can double-deliver. The signed payload carries a stable event `id` so receivers can dedupe (at-least-once is standard webhook semantics), but horizontal scale needs a lease/`claimed_at` + `SKIP LOCKED`. | T10.3 review (arch) | add a delivery lease before running >1 replica (the River-backed worker follow-up is the natural home) |
| **No webhook-per-org cap or per-target rate limit**: an authenticated owner can register N webhooks at the same victim URL; each spine event fans out N× — a third-party DoS amplification primitive. Delivery is attempt-capped per `(webhook,event)` but not per destination. | T10.3 review (QA) | cap webhooks-per-org and/or add a per-host delivery rate limit before GA |
| **Hard cap: soft-overage accrual stop not wired** (T11.6): the accept-gate cap refuses new provisioning that would cross the monthly bound, but usage-based overage (egress etc.) keeps accruing past the bound in the rollup — feature-specs §31 says "soft-overage stops accruing" at the cap. The provisioning half is done; the metering half is a follow-up. | T11.6 | stop/flag overage accrual for a capped-out org in the T6.3 rollup (or gate metering emit) — its own task, touches metering |
| **Hard cap: concurrency race (TOCTOU) at the accept/scale gate** (T11.6, HIGH): `enforceBudget` reads the run-rate then the write (InsertService/UpdateServiceShape) happens without an enclosing serializable tx or per-org lock — two concurrent same-org creates can each pass the check and both commit, overshooting the cap (bounded by the concurrent burst, but NOT self-correcting — the over-cap committed run-rate persists until the org scales down or deletes). For a flagship "impossible by construction," this window should be closed. | T11.6 review (arch+QA) | wrap the cap-check + the committing write in one tx holding `pg_advisory_xact_lock(hashtext(org_id))` (store has `WithTx`); the estimate `Accept` stays a pre-check burn-guard, the locked check is authoritative — its own task (touches the provisioning write path) |
| **Hard cap: run-rate counts suspended/failed services** (T11.6, LOW): `SumOrgMonthlyEstimate` excludes only `deleting`, so a `failed`/`suspended` service still inflates committed run-rate and can refuse a legitimate new create. Conservative (over-counts → never under the cap), intentional — flagged for a product ruling on whether non-running states should count. | T11.6 review (arch) | product ruling on which service states count toward the cap |
| **Hard cap: scaling/override-only PATCH bypasses the run-rate model** (T11.6, LOW): a PATCH that changes only `scaling`/`override` (shape nil) pins more instances / wider autoscale bounds and raises *actual runtime* cost, but does not reprice `monthly_estimate_cents`, so the cap-guarded committed number and run-rate stay flat. A metering-fidelity gap in the run-rate model, not a hole in enforcing the number the cap guards. | T11.6 review (arch) | reprice on scaling/override changes (or fold instance count into the run-rate) so the cap tracks true committed cost |
| **Hard cap: alert-threshold delivery not wired** (T11.6): `budgets.alert_thresholds` (e.g. 80%) persist but nothing sends the banner+bell+email at the threshold. | T11.6 | wire threshold crossings to T10.3 routing + T11.5 `ShouldWarn` — a follow-up |
| **ClusterImagePolicy is authored but wired to nothing** (T1.2 review): `infra/k8s/policy/cluster-image-policy.yaml` needs the sigstore policy-controller installed and the enforced namespaces labeled (`policy.sigstore.dev/include=true`) — today no Terraform applies any of it, so invariant-11/keyless-signing enforcement is documentation-only. | T1.2 review (QA) | wire policy-controller install + namespace labels into the cnpg/gke-cell layer, landing WITH the first real dev apply (before any workload runs) |
| **T1.2 deferred-apply bundle** (tracked so the first applier isn't surprised): the staged apply is REQUIRED (kubernetes_manifest plans against the live API; CNPG CRs need CRDs — sequence documented in infra/README.md); AC-4's live snapshot→recovery round-trip evidence lands then; `terraform state list` at that point must confirm no legacy `zfs-storage` pool state. Also INFO: google_client_config puts a ~1h OAuth token in state (state bucket is private+versioned, acceptable). | T1.2 review | execute with the T1.4/T3.4 apply |
| **Template PATCH strips unknown contents keys silently (200) while the estimate path rejects them (422)** (T12.4 residual): PriceAll runs on the already-projected shape, so a PATCH with junk keys succeeds minus the junk. Making PATCH price the un-projected decode first would reject loudly, matching "never silently stripped". | T12.4 review (arch) | make PATCH loud — small handler change, own PR |
| **Template instantiation compensation failures are silent + untested** (T12.4 residual): the teardown calls (DeleteBindingsForProject/HardDeleteProject) swallow errors — a failed compensation strands the project with zero signal; the compensation path has no test. | T12.4 review (arch) | log+event on compensation failure; add a fault-injected test |
| **Downgrade gate can't evaluate the canon's "cells" reason** (Q9): qa.md #4 names "12 members & 2 cells" as the Business→Pro blockers; cells aren't data until E1 runtime (US-1.1 cell_id rows), so the 409 evaluates members + projects today. | Q9 | add the cells dimension to the changePlan gate when cells become queryable |
| **orgs.plan vs subscriptions.plan divergence (FIXED in Q9, recorded for history)**: the T11.2 state machine updated subscriptions.plan only; every plan gate reads orgs.plan — a cancel-at-anchor left orgs gated as paid forever. commit() now SyncOrgPlan-converges on every state write. | Q9 | done — regression held by TestQAScenario4's convergence assertion |
| **Pending-downgrade apply skips the B4 gate at the anchor** (Q9 review M3/M4): members/projects are validated when SCHEDULING a downgrade; the anchor-time apply re-checks nothing, so an org that grew past the target's limits mid-wait still lands over-limit. | Q9 review | re-validate at apply (hold + notify on violation) — needs count queries in the subscription package, own task |
| **The lifecycle sweep's SQL now() is unwarpable** (Q9 review): ListSubscriptionsToAdvance selects on DB now(), so the harness exercises AdvanceLifecycle directly and the selection predicate (incl. the pending_plan_at arm) is untested under warp. | Q9 review | inject the comparison instant as a query arg so the sweep itself warps |
| **subscriptions↔orgs plan convergence is two statements, not one tx** (Q9 review M2): the CAS + SyncOrgPlan pair can interleave/partially fail (now LOUD via slog, no longer silent). | Q9 review | wrap in one tx (store.WithTx) — small follow-up |
| **Enterprise-target changePlan untested** (Q9 review): only PlanRank covers the enterprise tier; a changePlan to/from enterprise (custom-priced, PlanFeeCents !ok) has no test. | Q9 review | add enterprise-edge tests with the role-matrix batch |
| **ChangePlan has no full status gate for upgrades** (Q9 review Med1): a suspended/grace (delinquent) org can upgrade — dunning state is preserved (no dodge) but the higher plan's entitlements apply via SyncOrgPlan with no payment event. Cancelled rows ARE refused. | Q9 review | gate upgrade-while-delinquent on a successful charge when T11.4/Stripe lands |
| **Billing-role authz on subscription ops untested** (Q9 review INFO): the matrix gives subscription.change to billing=Y/admin=N (intended per matrix); no test pins it. | Q9 review | add role-matrix tests for changePlan/cancel |
| **Console realtime component coverage needs a DOM test env** (T8.2): vitest runs node-env with no testing-library, so RealtimeMount/ProvisioningWatcher (toast-on-ready, invalidation, env-switch teardown, StrictMode double-mount) are untested at the component level; the engine + registry are fully unit-tested. Also: bell bumps on EVERY spine event until the N1 slice adds notification-worthy filtering; degraded-transport state (poll mode) has no UI affordance yet (onModeChange is exposed, unconsumed). | T8.2 review | add jsdom + @testing-library when the next console slice needs it; N1 slice owns the filter + offline affordance |
| **T12.3 deps incomplete** (found in T13.3): T12.3's "gate ALL /assistant/* (404 empty-equivalent)" AC depends on the assistant HTTP surface existing to gate — its declared `deps: [T12.1]` omits the E13 endpoint tasks (T13.3 + insights/proposals). The enforcement primitive `Service.AIAssistantEnabled` now exists; T12.3 becomes wiring it across every assistant handler + deciding the multi-org/org-key resolution contract. | T13.3 | add the E13 surface tasks to T12.3 deps; T12.3 wires the primitive + resolution |
| **listThreads has no org param** (T13.3): the contract carries no org selector, so listThreads unions the user's threads across all their orgs (omitting AI-disabled ones). It also evaluates each org at project scope "" — a thread created under a PROJECT-OVERRIDE enable in an org whose org-level policy is disabled is hidden from the list (rows persist, no Law-4 deletion), because the GET has no project param to scope by. A `?org=`/`?project=` filter would need an owner-level openapi change. | T13.3 | add `?org=`/`?project=` to listThreads for per-scope visibility |
| **Dashboard layout/pos jsonb unbounded** (T12.6 review): name (≤200) and widget query (≤4096) are capped, but the layout and widget pos jsonb blobs have no size bound — a storage/DoS surface relying on the global body-size limit. | T12.6 review | add jsonb size caps if the global limit proves insufficient |
| **personal↔restricted visibility ungated** (T12.6 review F8): while restricted is owner-only the transition is harmless, but it becomes a share_org bypass when a restricted-share member surface lands. Commented in-code. | T12.6 review | add the share_org gate to the restricted transition when restricted-share ships |
| **Dashboard "renders only viewer-accessible projects and says so" is unimplemented data-side** (T12.6): the backend enforces the object-model axes (visibility + born-filtered scope), but the org-wide data filtering + the "and says so" annotation have no schema field on Dashboard — it's a render/data-plane concern for widget-query execution. `restricted` visibility is owner-only until a restricted-share member surface exists. Dashboards live in the provisioning module (helper reuse), not an observe module. | T12.6 | wire viewer-scoped widget-query filtering + a filtered-indicator field in the observe/data task; add restricted-share members if the product needs them |
| **copyToPersonal (fork/duplicate) is non-transactional** (T12.7 review, minor): CreateDashboard then a per-widget AddWidget loop — a mid-loop failure leaves a partial personal dashboard. Low blast radius (owner-deletable personal object). | T12.7 review | wrap in one tx (store.WithTx) alongside the T12.4 instantiate compensation cleanup |
| **US-11.5 non-blocking test gaps** (QA, recorded): cancel-invariant is pinned for `services` (bindings/deployments untouched follows structurally — cancel writes only the subscriptions row); double-cancel / reactivate-after-reactivate idempotency (both 409 via ErrBadTransition, unswept); `nextAnchor` date-rollover fuzz target; the cancelled-while-dunning `dunning{day,next_retry}` wire projection (state omitted) is untested. | US-11.5 review | add when the billing surface next gets touched; none change behavior |
| **[PARTIAL — primitive added, no call site yet] never-gated ENFORCEMENT-path** (US-11.1 review): billing.GateCapability is the enforcement primitive (consults IsNeverGated before a plan gate; table-tested), but it has NO caller — no capability→plan gate exists yet (the only PlanGated site is the project-LIMIT gate, not a capability). It binds the first real capability gate when one lands. Original: US-11.1 pins the never_gated list (set-equality) + two safety handlers structurally, but nothing asserts the T11.5 quota evaluator / plan-gate call site actually CONSULTS IsNeverGated before problem.PlanGated — the list can be correct yet unhonored. Structural handler coverage is 2-of-7 (MFA, self-deletion). | US-11.1 review | pin the gate-site→IsNeverGated consultation in US-11.2/T11.5 coverage; extend structural scan to backups/tls/policies/alerts handlers as they land |
| **[PARTIAL — primitive added, not wired] soft-overage halt at cap**: quota.ClampOverageToCap is the halt primitive (table-tested incl. boundary), but it is NOT yet wired into the rollup/Evaluate path — a running service's soft overage is not yet clamped in production. Wiring needs the semantic call (US-11.7 says 'running services untouched', so WHERE soft overage is billed-vs-halted at the cap is a design decision). Original: US-11.7 enforces the cap at PROVISION time (402); AC2's "soft-overage stops accruing when the cap is reached" is the metered-quota side (T11.5 evaluator) — a running service's soft overage (egress, etc.) is not yet halted when the org hits its monthly cap. | US-11.7 | wire the cap into the quota evaluator's soft-overage path in US-11.2 |
| **cap `committed` counts cancelled/suspended services** (US-11.7 review QA, medium): SumOrgMonthlyEstimate excludes only `deleting` services, so a `cancelled_at_anchor`/`suspended` service still counts toward the committed monthly total the cap projects against — with US-11.5 (cancel≠delete keeps services running+billing) this is arguably CORRECT (they still incur cost), but it's unpinned; a `deleting` service correctly frees its budget. | US-11.7 review | pin the status→committed edges (deleting frees; cancelled/suspended still count) in a test; confirm the cancel-still-counts semantic is intended |
| **B3 invoice fixture numbers need the S5 ruling** (US-11.6): inv_2026_06's lines (Σ 41492¢) don't sum to its printed total (47700¢), and its GST tax (13208¢ = 31.8%, not 18%) is inconsistent. US-11.6 enforces the §74 lines-sum rule everywhere and DEFERS this one fixture to S5 (which number is authoritative — itemization vs printed total). The console invariant test excludes inv_2026_06 with an explicit marker; remove it when S5 rules. | US-11.6 | S5 rules the authoritative B3 numbers; then drop the B3_PENDING_S5 exception |
| **Per-plan meter allowances (Free/Pro egress/events/AI/builds) are canon-unsourced** (US-11.2): plans.json $note defers them (only Business 100 GB egress is pinned). The 80% warning mechanism is proven for the canon allowance, but a rollup-time auto-trigger that warns every over-80% meter across plans needs the allowance data. The "~$1.62 at current pace" forecast + B8 "no upsell when overage is cheaper" are console/forecast concerns. | US-11.2 | source the per-plan meter allowances (founder/canon) then wire the rollup-time warning + the forecast/upsell UI |
| **Template refresh BROADENS to the full source env, ignoring the original selection** (T12.5 review, medium): a template persists source_project/source_env but NOT the captured service ids, so RefreshTemplate calls captureFrom(env, nil) = whole env — a service added to the source (or one deliberately EXCLUDED at capture) reappears on refresh. Pinned by TestTemplateRefreshVersioning; whether refresh should honor the original selection (persist the service ids) or intentionally re-capture the full source is a T2 design question. | T12.5 review | T2 owner rules: persist the capture selection and honor it on refresh, OR document full-env re-capture as intended |
| **Urgent (security/paging) bypass: no SOURCE yet + email-off-override undecided** (US-10.2 review): NotifyInput.Urgent bypasses QUIET HOURS (tested, matches the AC + the openapi "never gate escalation paging" note), but (a) no security/paging notification is emitted anywhere to set it, and (b) it still respects a member's email-OFF pref — glossary/design-spec P6 ("escalation paging is org policy; quiet hours don't silence it") arguably implies a security page should override personal email-off TOO, not just quiet hours. Both are decisions the E7/auth wiring task must make explicitly. | US-10.2 review | the E7/auth source sets Urgent:true AND rules whether Urgent overrides email-off (org-policy escalation) — record the ruling, don't let it slip |
| **Console `biome check` fails on main (pre-existing)**: unused imports/suppressions in command-palette.tsx, overlay.tsx, type-blocks.tsx, snav-product.tsx — `pnpm --filter console lint` exits 1 on a clean checkout, so lint is not effectively enforced for the console in CI. | T8.2 (observed) | fix the four files + ensure the CI job actually gates on console lint |
| **Warn-mode policies are skipped, not counted** (G9 "warn counts violations"): `policy.Engine.Evaluate` `continue`s past a `warn`-posture row without emitting any violation event, so the warn-first telemetry story (author a policy in `warn`, watch what it *would* block before flipping to `enforce`) has no data. | T12.2 | emit a spine violation event (or a counter) when a `warn` policy's kind would have denied — its own task, touches the events spine |
| **Notification `kind` enum cannot be pinned inline** (US-10.3 enrichment): frames P6/N2 define the vocabulary `[alert, proposal, approval, deploy, lifecycle]`, but `NotificationList.data[].kind` is a bare string and **shipped rows already violate that domain** — `EmitQuotaWarning` writes `billing` (`quota_warning.go:29`) and the migration documents `security`. Narrowing the field would make `listNotifications` emit contract-invalid values for existing rows. US-10.3 therefore ships **no contract change**; `Classify` emits frame kinds for new rows only. | US-10.3 enrichment (arch review) | rule the legacy `billing`/`security` → frame-kind mapping AND the backfill-vs-tolerant-read, then pin the enum. "Map to the nearest frame row" is improvisation (§5 rule 10) and was rejected |
| **`deleteWebhook` / `PATCH /webhooks/{wbh}` absent from the contract**: create + list + test exist; there is no way to remove or disable a webhook through the API, so a compromised or obsolete endpoint can only be stopped in the database. Adding an operation is a proposal, never inline (Trap T2). | US-10.3 enrichment | add the delete (and likely disable) operations to `openapi.yaml`, then a task to implement |
| **[SECURITY] `testWebhook` leaks webhook existence across orgs**: the pre-strict shim calls an **unscoped** `GetWebhook(ctx, wbh)` then `authz.Require` on the webhook's own org (`webhooks_http.go:112-121`), so a member of another org receives **403, not 404** — distinguishing "exists" from "does not exist". Trap T8 requires 404 on id-addressed reads for non-members (billing is the sole documented exception). Found while specifying US-10.3; not fixed there (out of scope). | US-10.3 enrichment (QA review) | return `notFoundError` when the caller is not a member of the webhook's org; pin with an HTTP-level test — the shim's manual auth path has no coverage at all |
| **[PARTIAL — primitive exists, not wired] notification email route is dead in production**: `cmd/api/main.go:229` builds `notify.NewRouter(queries, kek)` with **no `.WithEmail(...)`**, so `Router.email` is nil and `Notify`'s email branch never fires. The bell/webhook routes work. This means US-10.2's quiet-hours *email* semantics and T10.3's email fan-out are tested but unobservable in a running system — a §5 rule 1 "primitive with no enforcement path". | US-10.3 enrichment (QA review) | wire the mailer into the router in `main`, and add a wiring assertion so a nil sender cannot ship silently |
| **Bell projection extends multi-replica exposure to email**: the webhook outbox's known single-replica limitation is bell-safe (rows dedupe on the partial unique index) but **not email-safe** — email has no ledger and no lease, so two replicas running the projection would send duplicate mail. US-10.3 mitigates with `SELECT … FOR UPDATE` on the cursor row, which serializes replicas; a lease is still the general fix. | US-10.3 enrichment (QA review) | fold the bell cursor into the same lease design as the webhook delivery lease |
