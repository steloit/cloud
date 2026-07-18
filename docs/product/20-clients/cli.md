# CLI specification — `steloit`

The CLI is a first-class product (GOV-002 §3.5) and a **thin client of the same API** the console uses — no CLI-only capabilities, no console-only capabilities, ever. The console teaches the CLI by existing (every empty state renders the equivalent command); the CLI is the console's script. One grammar, two costumes.

This document owns the CLI's grammar and output conventions. Command *behavior* is owned by the API spec (`08-api/openapi.yaml`) — a CLI command is a rendering of an operation, never a second implementation.

## 1. Grammar

```
steloit <noun> <verb> [args] [--project <p>] [--env <e>] [--org <o>] [flags]
```

- **Nouns are the primitives and products**; verbs are the shared lifecycle set (`create, list, inspect, scale, backup, destroy` — GOV-002 §6.2) plus product verbs. A developer fluent in `steloit db …` is fluent in `steloit valkey …`. *(ADR-0004/A5: `storage` and `queue` are no longer product nouns — queue capabilities surface under `steloit db` (pgmq) and storage under Binding commands. ADR-039/040 adds the intent door `steloit add <intent>` — task T5.3.)*
- **Context mirrors the console crumb exactly**: `--org / --project / --env` are the context bar as flags. Resolution order: explicit flags → repo link (`steloit init` wrote `.steloit`) → profile default. The resolved context is **always echoed** on state-changing commands (`ecommerce/production ·`) — context is worn, not guessed.
- **Env is a filter here too**: `--env staging` re-scopes the same command; omitting it uses the linked default and says so.
- Regions render provider-faceted everywhere: display `aws · ap-south-1`, flag value `aws/ap-south-1`.

## 2. Output conventions

- **Human mode (default):** aligned tables, one record per row, ids and money in plain copyable text. Status renders as the six marks — the same vocabulary as every other surface (spec §1.2):

  `✓ ready · ◌ provisioning · ! degraded · ✕ failed · ○ suspended · · deleting`

- **`--json`:** raw API response shapes, verbatim (snake_case, `*_cents`, `{data, next_cursor}`) — the schemas in openapi.yaml are the contract; scripts parse this, never the human tables.
- **`--quiet`:** ids only, one per line (pipe fodder).
- Color maps the semantic tokens (ok/warn/err/prov) and is never the sole carrier — the mark and the word always present. `NO_COLOR` and non-TTY output degrade losslessly.
- Money renders via the same arithmetic everywhere: `$58/mo`, from integer cents.

## 3. Safety grammar (designed friction survives the terminal)

- **Estimate before provision:** every `create` prints the estimate rail's lines and total, then prompts. `--yes` accepts a *shown* estimate; there is no flag that skips seeing it. Nothing provisions before acceptance; metering starts at `ready`.
- **Destructive actions:** blast radius stated (dependents named, exactly U6's list), then `--confirm <exact-name>` required. No `--force` exists. Data-destructive additionally prints the recovery path ("final backup restorable 30 d").
- **Reveal-once:** `steloit token create` prints the secret exactly once with the same contract as U7 ("shown once — we store only a hash"); it is never retrievable again, including via `--json`.
- Every state-changing command lands in the audit log with actor = the token's user.

## 4. Errors

problem+json rendered as three lines: **what happened** (title + detail) · **why/where** (the named policy or missing role for 403 — E3's grammar) · **what to do next** (remediation, always present). Exit codes: `0` ok · `1` generic · `2` usage · `3` permission (403) · `4` not found · `5` conflict/blocked (409, all reasons listed) · `6` payment/quota (402, with the math) · `7` rate-limited (auto-retries idempotent reads honoring Retry-After; mutations never auto-retry).

## 5. The shared query language, verbatim

`steloit logs 'level:warn+ source:postgres "duration"' --env production --since 1h -f`

The exact string works in the console's log bar, ⌘K, and alert rules — anything findable is alertable, from any surface. `-f` live-tails (SSE). Same for metrics: `steloit metrics 'service:api metric:p95' --range 24h --compare`.

## 6. Command inventory (canon commands are frame-fixed)

| Noun | Verbs | Notes / canon examples |
|---|---|---|
| `project` | create, list, inspect, delete | `steloit project create ecommerce` (GOV-002 §5) |
| `env` | create, list, inspect, delete | C8's flow: `steloit env create production-us --region aws/us-east-1 --clone-shape production` |
| `db`, `valkey`, `web`, `worker` | create, list, inspect, scale, backup, destroy + product verbs | `steloit db branch db-main --from production` (W5) · `steloit --project ecommerce --env production db list` |
| `bind` | (verb-first form) | `steloit bind worker jobs` — consumer config injected, effective next deploy (GOV-002 §6.1) |
| `deploy` | —, list, rollback, promote | `steloit deploy` · promotion prints the DP1 diff before running |
| `logs`, `metrics`, `traces`, `events` | (query-first) | §5 grammar; deploy markers included in output |
| `alert` | create, list, backtest | `steloit alert backtest 'service:api metric:p95' --gt 800ms --window 5m --days 7` |
| `template` | save, list, inspect, delete | **canon:** `steloit template save checkout-stack --from ecommerce/production --services api,worker,jobs,cache` (T3) |
| `policy` | create, list, inspect | **canon:** `steloit policy create --dry-run` (G9) — dry-run prints the impact preview |
| `usage`, `billing` | export, overview, quotas | **canon:** `steloit usage export` (B2) |
| `token`, `key` | create, list, revoke | reveal-once (§3) |
| `cell` | connect, list, inspect | X2's stepper, terminal form |
| `ask` | (free text) | **canon:** `steloit ask "why is my database slow?"` (AI2) — answers cite evidence ids; proposals are links back into the console/`--json`; the CLI never applies |
| `init` | — | binds the repo to a project; writes the context the flags would carry (GOV-002 §3.5) |
| `dev` | — | local containers or tunneled dev services with identical config injection (GOV-002 §3.5) |
| `connect` | verify | A9's moment: `✓ laptop-cli connected just now · device token · manage under your profile` |

## 7. What the CLI never does

Never renders AI output outside the proposal grammar; never auto-applies anything (Law 1 has no terminal exception); never prints a secret twice; never hides a gated action — it shows it disabled with the reason, like B6; never invents output not derivable from an API response.
