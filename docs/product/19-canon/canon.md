# Canon — the one demo world

Every demo, screenshot, test, Storybook story, doc example, CLI example, and AI prompt context uses **this world and no other**. The rule (elevated from brand.md): **no demo data exists outside canon, anywhere.** `fixtures.json` beside this file is the machine-readable form — every object conforms to a schema in `08-api/openapi.yaml`, so a schema change that breaks canon fails loudly in CI, and the same file seeds API mocks, Storybook, docs, and the ten QA scenarios in `16-qa/qa.md`.

Facts below are **frame-fixed** (visible in the gallery — changing one is a canon change requiring frame updates) unless marked *(representative)* — chosen here to complete the dataset where no frame pins the value.

## The cast

| Entity | Facts |
|---|---|
| **Acme** (org) | 12 members · **Business $99/mo** · home region `aws · ap-south-1` · org total **$482/mo** = $383 resources + $99 plan · MTD **$171.42** · forecast $482 ± $6 · budget $450 · GST 18% w/ GSTIN · two BYOC cells |
| **Borealis** (org) | **Pro** — the subscription-lifecycle org (B9–B12): trial 2/5 seats, dunning day-8 frame, cancel/reactivate. Project `starter` from `store-baseline` |
| **initech** (org) | Day-zero: 0 projects · $0/mo (E5); a `store-baseline` consumer |
| **asha** | The incident's protagonist: investigates, creates `checkout-health` (Jul 2, 14:31), applies `prp_7c31a2` → deploy #143 |
| **marco** | Requested public CDN on assets (pending approval in G7/N1); dismissed a worker scale-down insight (reason logged) |
| **priya** | Appears in W12 audit as `priya via assistant` — the applied-via-assistant precedent |

## Acme's projects

| Project | Cost/mo | Status | Notes |
|---|---|---|---|
| **ecommerce** | **$208** | warn | the canon project — all product frames run on it |
| internal-tools | $96 | ok | two products (M1's adaptive-rail example); hosts `events-db` *(fleet view DB2; project assignment representative)* |
| mobile-api | $41 | ok | its "billing began" is B1's lifecycle marker ◇ |
| analytics *(representative name)* | $38 | provisioning | SW2's provisioning row |
| sandbox *(representative name)* | $0 | ok | SW2's $0 row |

## ecommerce — six services + one Storage Binding, three environments

**Services** (production; db ×3 is the rail count badge — frames still show ×2, reconciled via S9): `api` Web **$61** · `worker` Worker **$22** · `db-main` PostgreSQL Standard **$58** (192/200 connections) · `db-reports` PostgreSQL Dev **$24** (consumed read-only by worker via `bnd_worker_dbreports`) · `cache` Valkey **$22** · `jobs` **PostgreSQL Dev + 4 GB $21** (pgmq — the Jobs product; P4 canon ruling 2026-07-18, ADR-038/039) — Σ **$208**. **External:** `assets` → **Storage Binding** (`bnd_api_assets`, provider aws-s3; the provider's ~$9 shows at bind and never enters Steloit totals — A5.3).

**Environments:** `production` $199.10 · `staging` $6.70 · `preview/pr-142` $2.20 (Σ $208) — preview carries `preview-minimal v2` (the dedicated `jobs` database + worker excluded by template policy — a policy choice, not a limitation: the branch-coherence story, queue state branching with previews, is fully realized when Jobs rides in `db-main`; canon demonstrates the dedicated-instance resolution) and the `branch-data-masking` flag, expires by policy. `pr-142` has a masked branch of db-main linked to it; steloit-bot's PR comment: URL + "db: production data (masked · policy) · $0.07/day · capped".

**In-flight (never in any total):** admin (binding *issued · activates at ready*), sessions, emails, mailer, env `production-us` (C8) — *uploads and gpu-encoder retired with ADR-0004 (P4 ruling)*. Depicted at their moment of creation only.

## The incident — one story, every lens (Jul 2, 2026, IST)

| Time | Event | Where it's visible |
|---|---|---|
| 13:58 | deploy **#142** "gift cards" ships — with an unindexed `orders` query and a missing gift-card receipt path | ◆ marker on every chart |
| 14:02 | alert `api p95 > 800ms` fires — p95 hits **812 ms** | O1/O5, bell |
| 14:03 | 2 × `order.paid` dead letters; **642 ms** SeqScan log lines begin | O3, D8, jobs panel |
| — | trace `tr_8814`: **79% of the request is one span** — the orders query; route isolated to `GET /orders` | O6, O2 split-by-route |
| 14:31 | asha creates `checkout-health` (DB7) — the view she wished existed | Dashboards |
| ~14:43 | the O-series / W3 "moment" frames | canon snapshot time |
| 14:50 | **#143** `fix/orders-query` promotes: applies `prp_7c31a2` as migration `0142_add_idx`; canary 33%; p95 **431 ms (−47%)**; alert trends to resolve; DLQ replayed after | DP1/DP2, every chart's #143 marker |

History context: **#140** rolled back · auto · gate `error rate 2.4×`. #142's annotation: `p95 alert followed · evt_9d21c4`.

## The four proposals (one per product panel)

`prp_7c31a2` PostgreSQL — index on orders (the incident's fix; applied via #143) · `prp_ca11e0` Valkey — eviction policy + maxmemory (AI6; AI9's follow-up thread attaches here) · `prp_ap77a4` Web — api off-peak scale window, window-bound and auto-reverting (AI7 re-cast per the P4 ruling; frame reconciles via S9) · `prp_qu20b9` Queue — recognizes the dead letters as a *code* fix, routes through #143 before replay (AI8).

## Templates, dashboards, quotas, cells

**Templates:** `store-baseline` v4 · 6 services + 1 Storage Binding · $208 · used by initech and borealis/starter — the `store` gallery template is the canonical origin of ecommerce · `api-worker-pair` v2 · $95 · used 4× · `analytics-starter` v1 · restricted. T3's capture example: `checkout-stack` (api, worker, jobs, cache) = **$126/mo** (jobs now the $21 Postgres).

**Dashboard:** `checkout-health` — ecommerce-scoped (org badge was the error), personal→shared, five widgets: api p95 (with #143 marker), order.paid errors, DLQ depth, MTD spend, deploy list.

**Quota canon:** egress **87/100 GB**, warned at 80% — banner + bell + email with the math (~**$1.62** overage); recommendation is a calculator, "do nothing" valid.

**Cells:** Acme Production Cell (regional · aws ap-south-1 · healthy) · Acme EU Cell (regional · gcp europe-west4 · upgrade available) — regional/dedicated cells in Steloit's account (Business+, ADR-022 as refined); cross-account BYOC is Enterprise/v3 (ADR-035).

## Arithmetic invariants (tests import these; nobody retypes them)

- `61 + 22 + 58 + 24 + 22 + 21 = 208` (ecommerce services → project; the assets Storage Binding is external — provider ~$9, never in Steloit totals)
- `199.10 + 6.70 + 2.20 = 208` (environments → project)
- `208 + 96 + 41 + 38 + 0 = 383` (projects → org resources)
- `383 + 99 = 482` (resources + plan = org total — the number on every surface)
- Estimate line grammar == invoice line grammar; money is integer cents end-to-end.
