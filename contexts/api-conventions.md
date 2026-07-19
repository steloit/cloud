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
