# Getting started — the alpha loop

This guide takes a partner from a personal token to a deployed-on-paper service:
**token → `auth login` → project → estimate → create → bind → deploy → `events -f`** —
first with the CLI, then the same loop with `@steloit/sdk`.

## Where the platform is right now (the P1 boundary, honestly)

The **control plane is real**: every step below creates real rows, walks real status
machines, enforces the estimate gate at the API layer, and lands on the events spine and
the audit ledger. What does **not** happen yet: nothing physically provisions. The
reconciler (cell-agent) and the build pipeline are P1-gated, so

- a created service stays `◌ provisioning` (its C4 step timeline is born with the row,
  `allocate` active, and waits for the reconciler);
- a created deployment stays `queued` (it is numbered and marked on the spine — the
  `#N + sha` every chart can render — but no image builds);
- bindings are `pending` (credential material is minted and referenced; injection
  happens at the next real deploy);
- **no billing starts** — metering starts at `ready`, and nothing reaches `ready` yet.

Everything you script against today keeps working unchanged when the data plane lands —
the API contract is the product.

## 0 · Prerequisites

- A running Steloit API and its base URL. The CLI defaults to
  `https://api.steloit.com`; pass your deployment's URL explicitly with
  `steloit auth login --api <url>` (persisted) or `STELOIT_API_URL`.
- A **personal token** (`stp_…`). Create one in the console under your profile
  (Tokens — the plaintext is shown exactly once), or, connected already, with
  `steloit token create <name>`. Tokens act as *you*: they carry your roles and shrink
  the moment your roles do.
- The CLI: `cd apps/cli && go build -o steloit .` (or `go run .` in place).

## 1 · Connect

```console
$ steloit auth login --api https://api.your-deployment.example
Paste your personal token (stp_…): ····
✓ connected as you@partner.example · token stored (only a hash lives server-side)

$ steloit connect verify
✓ connected just now · personal token · you@partner.example · manage under your profile
```

The token is verified against `/v1/auth/session` **before** it is stored (`0600`, in
`~/.config/steloit/config.json`), and no command ever echoes it back.

## 2 · Project (and the org you need first)

Commands that create things need an **org** in context. There is no `steloit org`
command yet — take the `org_…` id from the console, or from the API directly:

```console
$ curl -s -H "Authorization: Bearer $STELOIT_TOKEN" "$STELOIT_API_URL/v1/orgs"
{"data":[{"id":"org_…","slug":"acme","name":"Acme","plan":"free",…}]}
```

Then:

```console
$ steloit project create shop --org org_…
✓ shop created · prj_… · production environment born with it (ADR-037)

$ steloit init --project prj_… --org org_…
✓ linked /work/shop/.steloit → prj_… (env production)
  commands here now resolve context from this link; flags still override
```

From here on, commands run inside the repo need no context flags. Every project is born
with a `production` environment — omitted `--env` means `production`, always.

## 3 · Estimate, then create

Nothing provisions without an accepted estimate — the CLI shows it, you accept it, and
the **API enforces the gate** (a create without a live, matching `est_…` id is refused):

```console
$ steloit db create appdb --size dev --storage 10
prj_…/production ·
  appdb          $24/mo
  total          $24/mo · billing starts when the instance is ready — not now
Create appdb at $24/mo? [y/N] y
◌ provisioning appdb · svc_…
```

`--yes` accepts a *shown* estimate; no flag skips seeing it. The service now exists as a
control-plane row in `provisioning` — at the P1 boundary it stays there (see above).

The CLI ships the `db` (PostgreSQL) noun today. For a compute service (`web`/`worker`)
use the SDK or the API directly (same estimate gate) — the snippet in §6 creates one.

## 4 · Bind

Wiring is free, and credentials are minted at bind:

```console
$ steloit bind svc_web… svc_db…
✓ bound · APPDB_URL injected · credentials minted · $0 · effective next deploy
```

The binding is `pending` until the next deploy; the injected variable name is
deterministic (`<TARGET>_URL`), and its value is always masked in reads.

## 5 · Deploy and watch

```console
$ steloit deploy --service svc_web… --sha 4f2a91c
prj_…/production ·
◌ deployment #1 queued · dep_…

$ steloit events -f
AT        KIND        ACTION              ACTOR
10:41:22  lifecycle   service.created     you@partner.example
10:42:05  deploy      deployment.created  you@partner.example
…
```

`events` reads the env's spine — every state change is a row there, deploy markers
included. `-f` live-tails over SSE and resumes from the last frame's `id:` cursor on
reconnect: no gap, no duplicate. (`steloit logs` arrives with E6; `steloit dev` with E4 —
both say so instead of pretending.)

## 6 · The same loop from code (`@steloit/sdk`)

```js
import { Steloit, fmtMoney } from "@steloit/sdk";

const client = new Steloit({
  apiUrl: process.env.STELOIT_API_URL,
  token: process.env.STELOIT_TOKEN,   // stp_…
  org: "org_…", project: "prj_…", env: "env_…", // context carried, not repeated
});

// estimate → inspect → create: the estimate id IS the method signature
const dbEst = await client.estimates.create({
  services: [{ product: "postgres", name: "appdb", shape: { size: "dev", storage_gb: 10 } }],
});
console.log(`appdb: ${fmtMoney(dbEst.monthly_total_cents)}/mo`); // $24/mo
const db = await client.services.create({
  product: "postgres", name: "appdb",
  shape: { size: "dev", storage_gb: 10 }, estimateId: dbEst.id,
});

// acceptance is one-shot: a second create needs its own estimate
const webEst = await client.estimates.create({
  services: [{ product: "web", name: "site", shape: { size: "standard-1", instances: 1 } }],
});
const web = await client.services.create({
  product: "web", name: "site",
  shape: { size: "standard-1", instances: 1 }, estimateId: webEst.id,
});

await client.bindings.create(web.id, { target: db.id });      // $0, pending until deploy
const dep = await client.deployments.create({ service: web.id, gitSha: "4f2a91c" });

// live tail with cursor resume (SSE under the hood)
for await (const frame of client.events.stream()) {
  console.log(frame.data); // the contract's Event shape
}
```

Errors are typed problem+json with `remediation` preserved: catch `QuotaExceededError`
(`overagePriceCents`, `requiredPlan`), `ConflictError` (`reasons[]` — all blockers),
`PermissionDeniedError`, `RateLimitedError` (`retryAfterS` from the `Retry-After`
header). A runnable version of this loop lives in `examples/node-alpha-loop/`.

## What to read next

- `docs/dev/api.md` — every live operation, the error model, pagination, SSE.
- `docs/dev/cli.md` — every shipped command, the safety grammar, exit codes.
- `docs/product/08-api/openapi.yaml` — the contract itself (shapes are generated from it).

Sources: apps/cli/internal/cli/** · packages/sdk/src/steloit.ts · docs/product/08-api/openapi.yaml ·
services/api/internal/provisioning/{services.go,deployments.go} · services/api/internal/estimates/{engine.go,pricing.json,service.go} ·
services/api/oapi-server.cfg.yaml · docs/product/20-clients/{cli.md,sdk.md}
