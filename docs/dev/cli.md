# `steloit` CLI reference

The CLI is a **thin client of the same `/v1` API the console uses** — a command is a
rendering of an operation in `openapi.yaml`, never a second implementation. This page
documents the commands actually registered in the shipped binary (`apps/cli`); the full
grammar the CLI grows into is `docs/product/20-clients/cli.md`.

Run it from the repo: `cd apps/cli && go run . <command>` (or build with
`go build -o steloit .`).

## Grammar

```
steloit <noun> <verb> [args] [--project <p>] [--env <e>] [--org <o>] [flags]
```

Flags may appear anywhere. `--flag value` and `--flag=value` both work. Bare boolean
flags: `--json --quiet --yes --help -f`.

### Context resolution (the ladder)

Context mirrors the console crumb: `--org / --project / --env` are the context bar as
flags. Resolution order, **per field**:

1. **Explicit flags** — always win.
2. **Repo link** — `steloit init` writes `.steloit` at the repo root; the CLI walks
   upward from the working directory to find it.
3. **Profile default** — `default_org` / `default_project` / `default_env` in the config
   file (no command sets these yet; edit the file).

The resolved context is **echoed before state-changing output** (`ecommerce/production ·`)
— context is worn, not guessed.

### The never-guess-env rule (ADR-037)

Environment resolution accepts an `env_…` id verbatim or a name resolved against the
context project. When no env is worn:

- **1 environment** in the project → used, never asked for.
- **≥2 environments** → **read** commands default to `production` (the echo says so);
  **state-changing** commands never guess: on a TTY you get a numbered picker, non-TTY
  exits `2` with *"pass `--env <name>` — state-changing commands never guess"*.

Omitted env always means `production` — implicitness is chrome, the API stays explicit.

## Output modes

- **Human (default):** aligned tables, ids and money in plain copyable text. Status is
  always mark **and** word — the six marks, identical on every surface:

  `✓ ready · ◌ provisioning · ! degraded · ✕ failed · ○ suspended · · deleting`

- **`--json`:** the raw API response bytes, **verbatim** (snake_case, `*_cents`,
  `{data, next_cursor}`) — scripts parse this, never the human tables.
- **`--quiet`:** ids only, one per line (pipe fodder).
- Color maps semantic tokens and is never the sole carrier. `NO_COLOR` and
  `STELOIT_COLOR=0` disable it; color is currently opt-in via `STELOIT_COLOR=1`.
- Money renders from integer cents, one arithmetic everywhere: `$58/mo`.

## Safety grammar

- **Estimate before provision:** every `create` prints the estimate lines and total, then
  prompts `Create <name> at $N/mo? [y/N]`. `--yes` accepts a **shown** estimate — no flag
  skips seeing it. Billing starts when the instance is `ready`, not at accept.
- **Destructive actions:** blast radius stated, then `--confirm <exact-name>` required —
  the service's exact name, not its id. **No `--force` exists.** Data-destructive
  commands print the recovery path ("final backup restorable 30 d").
- **Reveal-once:** `token create` prints the secret exactly once ("shown once — we store
  only a hash"); it is never retrievable again, including via `--json` (which is the same
  reveal, once). The stored login token is never echoed back by any command.

## Errors and exit codes

API errors render as problem+json in three lines: **what happened** (title — detail) ·
**why/where** (403's denying role/policy, 409's blockers, 429's retry timing) · **what to
do next** (`→ remediation`, always present).

| Exit | Meaning | HTTP |
|---|---|---|
| 0 | ok | 2xx |
| 1 | generic failure | 5xx, transport |
| 2 | usage (bad args, unresolvable context, unconfirmed destroy) | — |
| 3 | permission | 401, 403 |
| 4 | not found (also the honest-stub code) | 404 |
| 5 | conflict/blocked — all reasons listed | 409 |
| 6 | payment/quota — with the math | 402 |
| 7 | rate-limited | 429 |

## Configuration

`~/.config/steloit/config.json` (overridable via `STELOIT_CONFIG`; respects
`XDG_CONFIG_HOME`), written `0600` because it holds the token:

```json
{
  "api_url": "https://api.steloit.com",
  "token": "stp_…",
  "default_org": "org_…",
  "default_project": "prj_…",
  "default_env": "production"
}
```

`STELOIT_API_URL` overrides `api_url` per invocation. `steloit auth login --api <url>`
persists a new base URL. Note: the contract's `servers:` URL is
`https://api.steloit.app/v1` while the shipped default is `https://api.steloit.com` —
recorded drift; set the URL explicitly for your deployment.

## Commands

### auth

- **`steloit auth login [--token stp_…] [--api <url>]`** — store a personal token.
  Without `--token`, prompts for a paste (or pipe: `steloit auth login < tokenfile`).
  The token is **verified against `/v1/auth/session` before it is stored**; rejected
  tokens exit `3`, unreachable API exits with the URL to check. Success:
  `✓ connected as <email> · token stored (only a hash lives server-side)`.
- **`steloit auth logout`** — forget the stored token locally
  (`revoke it under your profile to kill it server-side`).
- **`steloit connect verify`** — check this machine's connection: re-verifies the stored
  token and prints `✓ connected just now · personal token · <email>`. `--json` gives
  `{"connected":true,"email":…}`.

### init / version

- **`steloit init --project <prj_id> [--org <org_id>] [--env <name>]`** (project id may
  also be the first positional) — binds the repo to a project by writing `.steloit`;
  commands run inside the repo then resolve context from the link (flags still override).
- **`steloit version`** — print the CLI version (`--json`: `{"version":"…"}`).

### project

- **`steloit project create <name> [--region aws/…]`** — requires org context
  (`--org` or a default). `✓ <name> created · prj_… · production environment born with
  it (ADR-037)`.
- **`steloit project list`** — table `ID NAME ENVS COST` (cost as `$N/mo`).
- **`steloit project inspect [<prj_id>]`** — the API shape, verbatim (defaults to the
  context project).

### env

- **`steloit env create <name> [--region aws/…]`** — requires project context.
- **`steloit env list`** — table `ID NAME REGION` (regions display provider-faceted:
  `aws · ap-south-1`; flag values use `aws/ap-south-1`).

### db (PostgreSQL)

- **`steloit db create <name> [--size dev|standard|performance] [--storage <GB>]
  [--ha=true]`** — the estimate-first create: prints each estimate line and the total
  (`billing starts when the instance is ready — not now`), prompts, then creates with the
  accepted `est_…` id — the API enforces the gate. Default size `dev`.
  *(Parser note: `--ha` is not registered as a bare boolean — pass `--ha=true` or
  `--ha true`; a trailing bare `--ha` errors. Recorded as a finding.)*
- **`steloit db list`** — services in the env filtered to `postgres`; table
  `ID NAME STATUS COST` with the six marks.
- **`steloit db inspect <svc_id>`** — the API shape, verbatim.
- **`steloit db destroy <svc_id> --confirm <exact-name>`** — states the blast radius and
  the recovery path, requires the exact **name** (there is no `--force`); on success:
  `· <name> deleting · final backup will be recorded (restorable 30 d)`.

### bind

- **`steloit bind <source-svc-id> <target-svc-id> [--scope read_only|read_write]`** —
  `✓ bound · <TARGET>_URL injected · credentials minted · $0 · effective next deploy`.

### deploy

- **`steloit deploy --service <svc_id> [--sha <git-sha>]`** — creates a deployment in
  the worn env: `◌ deployment #N queued · dep_…`.
- **`steloit deploy list`** — immutable history: `# ID SHA STATE ACTOR`.
- **`steloit deploy rollback <dep_id>`** — `✓ rollback created · redeploys <sha> ·
  migrations don't auto-revert`.

### token

- **`steloit token create <name> [--scope read_only|full]`** — prints the `stp_…`
  plaintext **once** on stdout (the warning goes to stderr, so piping stdout captures
  only the secret).
- **`steloit token list`** — `ID NAME PREFIX SCOPE`; never the secret.
- **`steloit token revoke <tok_id>`** — `✓ revoked — live requests with it stop now`.

### events

- **`steloit events [--kind <kind>] [-f]`** — the env's event spine (deploy markers
  included): table `AT KIND ACTION ACTOR`. With **`-f`** it live-tails over SSE and
  **reconnects resume from the last frame's `id:` cursor** — no gap, no duplicate.
  `--json -f` prints raw frame data lines.

### Honest stubs (shown, not hidden — with the reason)

- **`steloit logs`** — `✕ logs need the observability pipeline (E6) — not available yet
  → steloit events shows the env's lifecycle spine today` (exit 4).
- **`steloit dev`** — `✕ steloit dev arrives with the deploy epic (E4) — local runs with
  injected config` (exit 4).

Nouns in the CLI spec that have **not** landed yet (no command exists): `valkey`, `web`,
`worker`, `add`, `metrics`, `traces`, `alert`, `template`, `policy`, `usage`, `billing`,
`key`, `cell`, `ask`, and `project delete` / `env inspect|delete` / `deploy promote` /
`db scale|backup|branch`. Create non-Postgres services via the API/SDK for now.

Sources: apps/cli/internal/cli/{run.go,commands.go,nouns.go,auth.go,context.go,api.go,config.go,tail.go} ·
apps/cli/internal/output/output.go · `go run . --help` output · docs/product/20-clients/cli.md
