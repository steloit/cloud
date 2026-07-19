---
id: api-conventions
owns: [docs/product/08-api/**, packages/contracts/**]
see: [canon-testing, events-spine]
---

# API conventions

The REST contract is `docs/product/08-api/openapi.yaml` (51 ops, v1). **Contract-first, always:**
amend the yaml → regenerate clients (`packages/contracts`, console `pnpm gen:api`) → implement.
Never hand-write API types; never add an endpoint the spec lacks (spec change first — the S-process).

## The rules (x-conventions, condensed)

- Base `https://api.steloit.app/v1`; version in path. JSON keys `snake_case`.
- Ids are prefixed + opaque: `org_ prj_ env_ svc_ dep_ bnd_ est_ pol_ evt_ tok_ …`. Never leak
  foreign-org existence — unknown/foreign id → **404, not 403**.
- **Money = integer cents**, fields `*_cents` (ADR-025). Timestamps RFC 3339 UTC, fields `*_at`.
- Pagination: cursor only — `?limit=&cursor=` → `{data: [], next_cursor}`. Every project-scoped
  list accepts `?env=` (env-as-filter, ADR-013); **omitted `?env=` always means `production`**
  (ADR-037: every project is born with it; `POST /projects` returns it; no `/envs/default` alias —
  the API is always explicit, implicitness is console chrome only).
- Errors: RFC 9457 problem+json; `remediation` is **required on every error** — each failure names
  a next step. Extensions: 422 `errors[]` · 409 `reasons[]` (ALL blockers, not the first) ·
  402 `required_plan` / `overage_price_cents` (soft overage proceeds only with `confirm=true`) ·
  429 honors `Retry-After` · 5xx carries `event_id`.
- Custom actions are colon sub-resources (`…/lifecycle-rules:dryRun`, `/alert-rules:backtest`).
  Live reads declare `x-streamable` and accept `Accept: text/event-stream`.
- Status vocabulary: services walk `provisioning|ready|degraded|failed|suspended|deleting`
  (ADR-024 — never `running`/`deleted`). Metering starts at `ready`.

## Known contract gaps (carry as findings, never improvise)

The findings ledger is `docs/product/claudedocs/spec-change-proposals.md` — ~40 proposed
endpoints, 13 schema amendments, pending founder rulings (S1–S7 in tasks/e0-setup/). If your task
hits a missing endpoint: check the ledger first; if listed, your task depends on its ruling.

## Mistake bank

- Enforcing a business rule only in a client — every rule lands in the API handler.
- Returning 403 for another org's resource id (existence leak; use 404).
- First-blocker-only 409s — `reasons[]` carries ALL blockers.
- Hand-editing generated types in `packages/contracts` (regenerate instead).
- Forgetting `remediation` on an error path — schema requires it; tests should too.
- Untagged test structs decoding snake_case API JSON — Go's case-insensitive match does
  NOT fold underscores (`OveragePriceCents` ≠ `overage_price_cents` silently reads zero);
  always write explicit `json:` tags in assertions (caught by CI on T2.7).
- Action-style op paths with a colon AFTER a wildcard (`/orgs/{org}:leave`) — Go 1.22+
  `net/http.ServeMux` rejects them (a `{wildcard}` must be the whole segment) and the
  strict server PANICS at mount, not build; this only trips CI (local skips the mount
  without Docker). Use a sub-resource (`/orgs/{org}/leave`) instead. Colons on LITERAL
  segments (`/me/notifications:read`, `/alert-rules:backtest`) are fine (caught on T7.6).
- One-time codes (TOTP, magic links, OTP) with no single-use tracking — a valid code is
  replayable for its whole validity window (TOTP: ±1 step ≈ 90s). Record the consumed
  step/nonce and reject re-use ATOMICALLY (`UPDATE ... WHERE last_step < $step` :execrows,
  0 rows = replay). RFC 6238 §5.2 requires it; rate-limiting doesn't stop replay of an
  observed code (caught on T7.1 by the QA agent, not CI).
- A fallback factor gated behind the primary factor's availability — recovery codes that
  only work if the TOTP secret decrypts aren't a fallback (device loss / KEK rotation =
  permanent lockout). Try the independent factor even when the primary is absent; consume
  paths that need no KEK must not sit behind one (T7.1).
- Multi-statement auth mutations on the pool instead of one tx — a partial failure
  (enable set, secret row missing) can brick every future login. Wrap enable/disable/
  rotate flows in `s.db.Begin` + `q.WithTx(tx)` like `orgs.go` (T7.1).
- **Integration tests silently SKIP locally without a container runtime** — `newWorld`
  calls `t.Skipf` and the suite still prints `ok`, so a runtime bug (org-brick, wrong
  status) passes local review and only surfaces in CI. Start a runtime and RUN them before
  claiming done: `colima start`, then `DOCKER_HOST=unix://~/.colima/default/docker.sock
  TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock TESTCONTAINERS_RYUK_DISABLED=true
  go test ./internal/...` (T12.1 — the reviewers found the bug by tracing, not running).
- An AUTHORING surface that writes rows a separate EVAL layer consumes must agree with it on
  the key vocabulary AND the enforcement semantics. T12.1's authoring accepted arbitrary
  keys the eval Engine fail-closed on (self-brick), and spelled the ai key `ai-assistant`
  while eval registered `ai_assistant`. Reconcile spelling verbatim (the DB key must equal
  the registered kind), make warn-first mean warn-never-denies, and refuse promote-to-enforce
  for a kind with no evaluator.
- Attaching a child resource by a client-supplied parent id (policy.project_id, etc.) WITHOUT
  checking the parent belongs to the caller's org — an FK validates existence, not ownership.
  A cross-org id passes FK + unique and creates a cross-tenant row (+ CASCADE coupling +
  existence oracle). Validate parent.org_id == the resource's org (T12.1).
- Appending the audit event BEFORE the state tx orphans it on failure, and a unique-violation
  race then surfaces as 500. Insert first (map 23505→409, 23503→422 like `invites.go`), then
  append + link the event AFTER commit — the spine never records a change that didn't commit (T12.1).
- Hiding a foreign resource as 404 by matching only `membership:` denials misses the org-key
  path: an org-key foreign-scope denial is `key:…`, so it leaks existence via 403. Treat both
  `membership:` and `key:` (no-standing) denials as 404; only a `role:` (member-lacks-perm)
  denial is an honest 403 (T12.1).
