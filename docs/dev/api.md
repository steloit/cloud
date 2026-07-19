# Steloit API reference — live operations

This documents the operations that are **live in the strict server today** — the
`include-operation-ids` list in `services/api/oapi-server.cfg.yaml` is the source of that
list (45 operations). The full contract is `docs/product/08-api/openapi.yaml` (51+ ops);
anything not listed here exists in the spec but is not yet served. The contract, not this
page, is authoritative for shapes — SDK types and `--json` output are generated from it.

## Base URL and auth

- The contract declares `https://api.steloit.app/v1` (`servers:` in openapi.yaml). The
  shipped CLI and SDK currently default to `https://api.steloit.com` — a recorded drift;
  when running against your own deployment, always set the base explicitly
  (`steloit auth login --api …`, `STELOIT_API_URL`, or `new Steloit({apiUrl})`).
- **Auth**: `Authorization: Bearer <token>` — a personal token (`stp_…`, acts as you,
  shrinks the moment your roles do) or an org API key (explicit least-privilege scopes).
  The auth endpoints also establish a server-side **session cookie**; `getSession`,
  `logout`, and the `/me/*` operations accept either credential.
- Operations marked *public* below require no credential (`security: []` in the spec).

## Conventions (the contract's `x-conventions`, condensed)

- JSON keys are `snake_case`. Ids are prefixed and opaque: `org_ prj_ env_ svc_ dep_ bnd_
  est_ evt_ tok_ inv_ mbr_ ses_ …`. An unknown **or foreign-org** id returns **404, never
  403** — existence is never leaked.
- **Money is integer cents**, fields suffixed `_cents` (ADR-025). Render as dollars from
  cents: `5800` → `$58/mo`. Timestamps are RFC 3339 UTC, fields suffixed `_at`.
- Mutating POSTs marked below accept `Idempotency-Key` (≤255 chars): 24 h dedupe on
  (principal, endpoint, key); a replay returns the original response with
  `Idempotent-Replay: true`; same key + different body → 409.
- Every project-scoped list accepts `?env=`; **omitted `?env=` always means `production`**
  (ADR-037 — every project is born with it).

## The error model (documented once — it applies to every operation)

Every error is RFC 9457 `application/problem+json` using the contract's `Problem` schema:

```json
{
  "type": "https://api.steloit.app/problems/quota-exceeded",
  "title": "Included quota exceeded",
  "status": 402,
  "detail": "…",
  "remediation": "…"
}
```

- `type`, `title`, `status`, `detail`, `remediation` — **`remediation` is required on
  every error, product-wide**: each failure names a next step, never a dead end.
- **422** carries `errors: [{field, message}]` — render inline under the field.
- **409** carries `reasons: [{code, message, remediation}]` — **ALL blockers**, never just
  the first.
- **402** carries `required_plan` (plan gate) or `overage_price_cents` (soft quota — the
  request proceeds only when retried with explicit `confirm=true`).
- **403** `detail` names the missing role or the denying policy; denials are audited.
- **429** carries its timing in the **`Retry-After` header only** — never a body field.
  Clients auto-retry idempotent reads honoring it; mutations never auto-retry.
- **5xx** carries `event_id` — an incident-grade reference.

### The estimate gate (enforced at the API layer, never only in a client)

`createService` refuses to provision without an accepted estimate:

- missing `estimate_id` → **422** (`estimate_id: required — nothing provisions without an
  accepted estimate`)
- estimate missing, **expired (TTL: 1 hour)**, **already used (acceptance is one-shot)**,
  or priced for another environment → **409** with the remediation *"Create a fresh
  estimate for this environment and accept it — nothing provisions without one."*
- the estimate did not price the exact shape being created → **409** (*"the estimate does
  not cover this shape … the estimate IS the contract"*)

One estimate authorizes one create. Estimate the shape, inspect the lines, pass the
`est_…` id.

### Seat and plan 402s

- `createInvite` — seat limit is a **soft** quota: 402 with `overage_price_cents`
  ($7/seat, prorated); retry with `?confirm=true` to accept the overage.
- `createProject` — 402 when the plan's project limit is reached, with the reason.
- `createService` — 402 for a plan-gated product or a hard quota (hard quotas fail loudly;
  soft quotas keep working and bill — both warn at 80% with the math).

## Pagination

List responses are `{data: [], next_cursor}` — cursor pagination is the platform
convention. **Recorded contract gap** (findings ledger,
`docs/product/claudedocs/spec-change-proposals.md` §10): most list operations declare **no
`cursor`/`limit` query parameters** in openapi.yaml, so page 2 is unreachable on them
until the spec ruling lands; the SDK's iterators deliberately stop after page one on
cursorless operations rather than refetch page one forever. The exceptions that *do*
accept `?cursor=` today: `listEvents` (also `?kind=`) and `listAuditEvents` (also
`?actor=`, `?action=`).

## Server-sent events (live reads)

`GET /envs/{env}/events` is `x-streamable`: send `Accept: text/event-stream` to switch
the same URL from a JSON list to a live stream. The stream:

- replays the backlog from `?cursor=` (or from the beginning), then goes live;
- emits frames as `id: <cursor>` / `event: <kind>` / `data: <Event JSON>` — **the `id:`
  of each frame is an opaque resume cursor**: on reconnect, pass the last seen `id` as
  `?cursor=` for gapless, duplicate-free resume;
- sends a `: ping` comment heartbeat every 15 s.

Requires `observe.read` on the environment; a plain GET (no `Accept` header) returns the
JSON page with `next_cursor`.

## Live operations

**P1 boundary, stated once:** rows are **desired state** (D9). The control plane —
records, status machines, the estimate gate, events, audit — is fully real; the
reconciler (cell-agent) and build pipeline that converge physical infrastructure are
P1-gated. Until they land, services remain `provisioning` and deployments remain
`queued`. Nothing here talks to infrastructure yet.

### Auth

| Operation | Method + path | Auth | Purpose (spec summary) | Key errors |
|---|---|---|---|---|
| `signup` | `POST /auth/signup` | public · Idempotency-Key | Create the user + first session | 409 email already registered (`reasons[]` + remediation) · 422 password policy · 429 |
| `login` | `POST /auth/login` | public | Password login → server-side session; MFA-enrolled users receive `mfa_required` | 401 `auth_failed` — identical body for unknown email (no account disclosure) · 429 |
| `logout` | `POST /auth/logout` | session/bearer | Revoke the current session | 401 no active session |
| `getSession` | `GET /auth/session` | session/bearer | Current user + session — the console boot call (and the CLI's token check) | 401 |

### Personal tokens

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createPersonalToken` | `POST /me/tokens` | bearer/session | Personal token: acts as YOU, carries your roles and shrinks the moment they do; **plaintext returned exactly once**, only a hash stored | 422 |
| `listPersonalTokens` | `GET /me/tokens` | bearer/session | Prefix + metadata only — never the secret | — |
| `revokePersonalToken` | `DELETE /me/tokens/{tok}` | bearer/session | Revoked immediately (204) | 404 |

### Organizations

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createOrg` | `POST /orgs` | bearer | Create organization; slug immutable after create | 409 |
| `listMyOrgs` | `GET /orgs` | bearer | Organizations I belong to | — |
| `getOrg` | `GET /orgs/{org}` | bearer | One organization | 404 |
| `updateOrg` | `PATCH /orgs/{org}` | bearer | Rename, change default region. Slug never changes | 403 · 404 |
| `deleteOrg` | `DELETE /orgs/{org}` | bearer | Owner only; 90-day billing-data rule enforced server-side (202) | 409 blockers |

### Members, invites, API keys

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `listMembers` | `GET /orgs/{org}/members` | bearer | Members with role and MFA posture (+ `seats` block: included/used/overage price) | — |
| `changeMemberRole` | `PATCH /orgs/{org}/members/{member}` | bearer | Role change applies without re-login; audited before → after; tokens shrink immediately | 409 last Owner cannot be demoted |
| `removeMember` | `DELETE /orgs/{org}/members/{member}` | bearer | Sessions + tokens revoked immediately; owned resources flagged (returned as `flagged_resources`), never silently reassigned | 404 |
| `createInvite` | `POST /orgs/{org}/invites?confirm=` | bearer | Invite by email + role; 7-day expiry; dedupe | **402 soft seat overage** (`overage_price_cents`, retry `confirm=true`) · 409 already member/invited |
| `listInvites` | `GET /orgs/{org}/invites` | bearer | Pending invites | — |
| `revokeInvite` | `DELETE /orgs/{org}/invites/{invite}` | bearer | Admin revoke: invalidates the link, audited (204) | 404 |
| `getInvitePublic` | `GET /invites/{invite}` | **public** | Inviter, org, role (explained by consequences), email hint, status | 410 expired/used/revoked — distinguishes the failure states with a way forward |
| `acceptInvite` | `POST /invites/{invite}` | session | Accept — session email must match the invited address; grants access instantly, audited | 403 wrong account |
| `declineInvite` | `DELETE /invites/{invite}` | session | Decline — notifies inviter, invalidates the link (204) | 404 |
| `renewInvite` | `POST /invites/{invite}/renew` | **public** | Request a new link for an expired invite — notifies the inviter (202) | 404 |
| `createApiKey` | `POST /orgs/{org}/api-keys` | bearer | Org automation key: explicit least-privilege scopes; same reveal-once/hash contract as tokens | 403 |
| `listApiKeys` | `GET /orgs/{org}/api-keys` | bearer | Keys — prefix + metadata only | — |

### Audit and events

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `listAuditEvents` | `GET /orgs/{org}/audit?actor=&action=&cursor=` | bearer | Append-only compliance ledger; actor column distinguishes humans, tokens, and `user via assistant` | 403 |
| `listEvents` | `GET /envs/{env}/events?kind=&cursor=` | bearer | The spine: deploys, scaling, lifecycle — every chart marker is one of these rows. **SSE-streamable** (see above) | 403 `observe.read` · 404 |

### Projects and environments

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createProject` | `POST /orgs/{org}/projects` | bearer | Create project (optionally from template); **production env created by default** (ADR-037) | 402 plan project-limit |
| `listProjects` | `GET /orgs/{org}/projects` | bearer | Projects with health, monthly cost, env count, last deploy | — |
| `getProject` | `GET /projects/{project}` | bearer | One project | 404 |
| `updateProject` | `PATCH /projects/{project}` | bearer | Rename (with redirect promise), transfer | 404 |
| `deleteProject` | `DELETE /projects/{project}` | bearer | Blocked while services/envs exist unless cascade acknowledged; final backups taken (202) | 409 blockers |
| `createEnvironment` | `POST /projects/{project}/envs` | bearer | New environment: own home region (org default = prefill), clone-shape or start empty | 422 |
| `listEnvironments` | `GET /projects/{project}/envs` | bearer | Environments with per-env cost and parity info | 404 |

### Estimates

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createEstimate` | `POST /estimates` | bearer · Idempotency-Key | Price a shape **before anything provisions** — the estimate-before-provision law. Estimate line grammar == invoice line grammar. Acceptance is one-shot, env-fenced, TTL 1 h | 422 shape errors |

### Services

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createService` | `POST /envs/{env}/services` | bearer · Idempotency-Key | Provision (`estimate_id` required). Metering starts at `ready`; failed provisioning never bills | **the estimate gate** (above) · 402 plan/hard quota · 409 name taken |
| `listServices` | `GET /envs/{env}/services` | bearer | Services in the env (status vocabulary: `provisioning\|ready\|degraded\|failed\|suspended\|deleting`, ADR-024) | 404 |
| `getService` | `GET /services/{service}` | bearer | One service, with the C4 `provisioning_steps` timeline | 404 |
| `updateService` | `PATCH /services/{service}` | bearer | Shape/scale changes state blast radius pre-apply; temporary overrides require a reason and auto-expire 24 h | 409 · 422 |
| `deleteService` | `DELETE /services/{service}` | bearer | Takes a final backup (restorable 30 d); 202 | 409 names dependents that will knowingly break |

### Bindings

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createBinding` | `POST /services/{service}/bindings` | bearer | Bindings are wiring — **$0**; credentials minted at bind, rotated at unbind; effective next deploy; `status: pending` until then. Env vars deterministic `<TARGET>_URL`, values masked in reads | 409 duplicate binding |
| `listBindings` | `GET /services/{service}/bindings` | bearer | Bindings of a service | 404 |
| `deleteBinding` | `DELETE /bindings/{binding}` | bearer | Unbind — rotates credentials immediately (204) | 404 |

### Deployments

| Operation | Method + path | Auth | Purpose | Key errors |
|---|---|---|---|---|
| `createDeployment` | `POST /envs/{env}/deployments` | bearer | Deploy / promote. States: `queued → building → migrating → canary → verifying → live` (P1: records are created, numbered, and marked on the spine; the build pipeline that walks the states is P1-gated) | 404 · 422 |
| `listDeployments` | `GET /envs/{env}/deployments` | bearer | Immutable history — id, number, git sha, actor, state, annotations | 404 |
| `rollbackDeployment` | `POST /deployments/{dep}/rollback` | bearer | Rollback = redeploy of the previous image; migrations don't auto-revert (expand-contract) | 404 · 409 |

Sources: services/api/oapi-server.cfg.yaml · docs/product/08-api/openapi.yaml ·
services/api/internal/platform/problem/problem.go · services/api/internal/estimates/service.go ·
services/api/internal/provisioning/{services.go,deployments.go} · services/api/internal/events/{sse.go,events.go} ·
contexts/api-conventions.md · docs/product/claudedocs/spec-change-proposals.md §10
