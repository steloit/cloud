# Steloit — Production Technical Architecture

**Status:** **FROZEN — Architecture v1.2** · v1 approved 2026-07-18 (ADR-0001); v1.1 = **ADR-0003** (CNPG + CoW snapshots replace Neon OSS; INF-001 §A4); v1.2 = **ADR-0004** (product surface narrows to postgres/valkey/web/worker; storage & AI become Bindings; queue → Postgres capability; GPU removed; INF-001 §A5), all founder-ratified · deltas require a superseding ADR in `docs/adr/`; measured triggers, never anticipated scale.
**Governing constraints:** INF-001 (D1–D11 + A1–A3) decides infrastructure shape; GOV-002 decides product shape; `docs/product/08-api/openapi.yaml` is the API contract; the console (`apps/console`) is built. This document decides everything they left open, as **one opinionated recommendation per layer**.

---

## 1 · Backend: Go

**Decision:** Go (latest stable, currently 1.23+) for all three backend deliverables — `services/api` (the control-plane modular monolith), `services/cell-agent` (the reconciler), and `apps/cli` (`steloit`). This also resolves Sprint-0 item SP0-2 (CLI language).

**Why it wins for this product specifically:**
- **The data plane is Kubernetes + CNPG/ZFS.** The reconciler (D9) is a controller: watch desired state, converge actual. Go is the native language of that entire ecosystem (client-go, controller-runtime, CNPG itself); every pattern the cell-agent needs has a decade of prior art in Go and approximately none anywhere else.
- **The CLI is the alpha's primary client** (D11: CLI-first). Go ships single static cross-platform binaries with instant startup — exactly what `steloit` must be. A Node CLI drags a runtime; Rust doubles iteration cost for no user-visible gain.
- **"Cheap on capacity, never on shape" (INF-001) applies to our own footprint.** The core pool has a floor of one small node (A1.6); a Go API server idles at tens of MB where Node/JVM idle at hundreds. Our own efficiency is margin.
- **One backend language across api + cell-agent + CLI** means shared internal packages (problem+json, ids, money, contracts) and agents that move between the three without a context switch.
- **AI-agent leverage is excellent in Go:** small interface surface, explicit errors, gofmt-enforced uniformity, and a culture of boring code — the properties that make agent output reviewable.

**Rejected:** TypeScript backend (keeps one language with the console, but loses the k8s/controller ecosystem, static-binary CLI, and footprint; TS stays for console + SDK — two languages total, each where it is strongest) · Rust (iteration cost, thin k8s control-plane ecosystem, hiring pool) · Java/Kotlin (footprint, startup, culture mismatch).

## 2 · Frontend: keep Vite + React 19 + TanStack (not Next.js)

The console is complete on Vite + React 19 + TanStack Router (file-based, `?env=` filter) + TanStack Query + thin Zustand + Tailwind v4 over LATTICE tokens + generated Hey API client + MSW canon mode. It is a pure authenticated SPA: no SEO, no SSR, no per-request rendering — Next.js would add a server runtime and a migration for zero product benefit. **Decision: keep the stack exactly as built.** Next.js remains correct for the marketing site (`steloit/website`), which is a separate plane with separate needs. Deploy as a static bundle (GCS + Cloud CDN behind the LB); the API is the only server.

## 3 · Database: PostgreSQL — everywhere, one operator *(v1.1, ADR-0003)*

- **Customer databases:** **CloudNativePG-operated vanilla PostgreSQL — one cluster per project-environment** (single-instance for free/dev tiers; replicas are a paid knob), on an **OpenEBS ZFS-LocalPV** storage node pool. **Branching = CSI VolumeSnapshot (ZFS copy-on-write, instant, delta-priced) → CNPG recovery → hibernated single-instance cluster**; branch metadata, routing, lifecycle, and cleanup live in *our* control plane (Xata OSS is the Apache-2.0 reference implementation; pgstream where masked data-sync feeds are needed). Scale-to-zero = CNPG declarative hibernation + idle detection; wake-on-connect via the cell gateway. PITR = WAL archiving to GCS with `archive_timeout` ≤ 5 min (meets A1.3's RPO by construction); restore is always to a **new** branch. Pin CNPG ≤1.30 (in-tree Barman) until the barman-cloud plugin's failover/restore issues close — a tracked version knob. D3's branching *requirement* is unchanged; the engine changed (INF-001 §A4 records why — the Neon OSS upstream went dark and self-hosting was never supported).
- **Control-plane database:** also a **CNPG cluster** (single instance, PITR to a separate GCS bucket per invariant 10) — one operator to know everywhere, replacing v1's hand-rolled pod+WAL-G. **Knob-turn at M7:** add a replica or move to Cloud SQL — capacity, not shape.
- One database technology for the whole company: control plane, customer product, and the queue substrate are all Postgres. MySQL/other: no reason exists. **Branching is a product capability, not a database capability** — no customer-facing contract exposes CNPG, ZFS, snapshots, or any substrate concept (D8).
- **Queue is a Postgres capability (v1.2, ADR-0004/A5.2), not a product:** `pgmq` inside the customer's CNPG database (branches with the DB; consumed by a worker that scales to zero). No separate queue service, no broker, no NATS. The A3.1 queue apparatus and risk R3 are retired. Internal control-plane jobs remain on River (§12) — same Postgres-native philosophy.
- **Cache (Valkey) is optional and never default (A5.1):** provisioned only on explicit add, idle-suspended, hard-quota'd — because a cache can't scale to zero and a default pod per project is a permanent idle floor.

## 4 · Cache: Valkey

D5 (locked): Valkey, per-project pods for the customer product, never shared. Internally the control plane starts with **no cache tier at all** — Postgres + in-process caching is correct at alpha scale; an internal Valkey pod is a later knob with a measured trigger (p95 on hot reads), not a day-one component. Redis rejected on licensing (the reason Valkey exists).

## 5 · Queue & events

- **Internal background jobs: River** (Postgres-native job queue for Go). Jobs enqueue **transactionally with the domain write** (same DB, same tx — a failed provisioning can never leave an orphaned job), scale-to-zero friendly, no new infrastructure. Cron = River periodic jobs.
- **The events spine is a Postgres table + SSE fanout** (GOV-002 primitive 9) — not a broker. No Kafka, no NATS internally; A1.2 keeps brokers as last-resort for the *customer* queue product too (WAL/CDC-derived signals preferred, A3.1 design review pending, task E9-4).

## 6 · Object storage & AI: Bindings, not products *(v1.2, ADR-0004)*

The managed `Product` surface is exactly **`[postgres, valkey, web, worker]`**. Object storage and AI are **external-provider Bindings** — the Binding primitive (GOV-002 #6) extended so a target can be an external provider (type + provider + region + secret-ref) instead of an internal service. One shared mechanism, zero new stateful infra:

- **Storage Binding:** connect the customer's own S3/GCS/R2 — credentials (Secrets), config injection, allow-policy, lifecycle, audit, estimate-at-bind. Steloit never proxies bytes, never bears egress (A5.3). No managed storage product (D4 dormant).
- **AI Binding:** govern the app's LLM-provider connection — allow-policy, credentials, config injection, estimate-at-bind, cost visibility (provider usage API), lifecycle audit, **soft** spend control. **No proxy, no routing, no hard in-line caps** — that is the gateway commodity, never built (A5.4). Distinct from the four-laws assistant (§13/ADR-005).
- **GPU:** removed from the surface (unbuildable on our infra, specialist vertical); a future GPU need is a Binding to Modal/Replicate, v4+.

### 6b · Internal object storage
Steloit's OWN artifacts (images, WAL archives, backups) use plain GCS buckets in the cell project — internal infrastructure, not a customer product. Preview/content that must be served on the customer-content eTLD+1 (A2.4) still applies to *preview environments* (E4), not to a storage product. There is no per-customer managed bucket to proxy (A5.3).

## 7 · Search: later, and Postgres-first when it comes

No search infrastructure now. Console-side filtering is client/API-level; log search is Loki's job. When product search ships (GOV-002 places it v1.5+), it starts as **Postgres FTS + pg_trgm inside the existing substrate** — consistent with the product's own roadmap ("Search ships as Postgres FTS first"). OpenSearch/Meilisearch: not at any currently-planned scale; introducing a fourth stateful system for search would violate cheap-on-capacity for no partner-visible gain.

## 8 · Observability: the OTel + Grafana OSS stack, single-replica alpha

D5 locked Loki + OpenTelemetry; completing the stack:

| Concern | Choice | Notes |
|---|---|---|
| Collection | **OpenTelemetry Collector** per cell | Stamps tenancy labels (`project_id`, `environment_id`) — never trusted from customer workloads (D7) |
| Metrics | **Prometheus** (single replica, short retention) | Mimir is the scale knob, trigger-based |
| Logs | **Loki** (single binary mode) | Routed away from Cloud Logging's paid tier |
| Traces | **Tempo** (single binary) | The product sells traces (O6); same query path we expose |
| Profiling | **Go pprof** built-in; Parca later if ever | Zero-cost now |
| Internal dashboards/alerting | **Grafana + Alertmanager**, founders-only | Customer-facing observability is served by *our* API only (D8 — grammar surface; Grafana never leaks to customers) |
| Uptime | Synthetic probe of the alpha path → pages a founder (task O4) | Eats our own F6 rules once E10 ships |

## 9 · API: REST + SSE. No gRPC, no GraphQL, no WebSockets (yet)

- **REST** (`/v1`, openapi.yaml) is the single public contract — console, CLI, SDKs, *and the cell-agent* all speak it. "No console-only capabilities" extends to "no internal-only protocols": the agent uses the same authenticated API + a reconciler-scoped token, which keeps one client stack, one auth path, and D8's grammar everywhere.
- **SSE** for everything live (`x-streamable`: events, logs, metrics tails) with cursor resume. It is one-directional, HTTP-native, proxy-friendly, and already contract-specced.
- **gRPC: no.** Its payoff (binary perf, streaming, codegen) duplicates what REST+SSE+oapi-codegen already give us at our scale, and adds a second contract to keep honest.
- **GraphQL: never** — a second query surface would fork the one-grammar promise.
- **WebSockets: not yet.** The only real candidate is interactive shell exec (D20, data-plane wave 2) — decide there, via ADR, when it ships.

## 10 · Authentication: own the users, adopt the protocols

Per D5 ("authN adopted, authZ built") and the E2/E7 task specs:
- **Own:** the `users` table, sessions (server-side, argon2id), personal tokens + org API keys (reveal-once, hash-stored), MFA (TOTP + WebAuthn), recovery codes. The auth section enters openapi.yaml first (S1 ruling), then is implemented — contract-first like everything else.
- **Adopt:** OIDC for social login (later) and **Dex as the SSO broker at Business tier** (SAML/OIDC federation without building an IdP).
- **Rejected:** Clerk/Auth0/Better Auth — an external SaaS in the trust-critical path of a platform whose product *is* infrastructure trust, plus per-MAU pricing against our unit economics. AuthZ is the two-layer evaluator (matrix + policies) and is always ours.

## 11 · SQL layer: sqlc + pgx + golang-migrate

**Decision:** raw SQL, type-checked — **sqlc** generating Go from checked-in queries, over **pgx** (no database/sql indirection), with **golang-migrate** for sequential, versioned, never-edited-after-apply migrations.

**Why:** it is the same philosophy as the rest of the repo — *the artifact is the source of truth and code is generated from it* (openapi.yaml → clients; queries.sql → data layer). Agents write SQL that reviewers actually read; the schema in `docs/product/09-data-models/models.md` maps 1:1 to migrations with no ORM dialect in between; `EXPLAIN` works on what ships. **Rejected:** GORM (reflection, silent query generation — exactly what agent code must not hide) · ent (graph abstraction that fights models.md's explicit design) · Prisma/Drizzle (TS-only — wrong side of the language split).

## 12 · Background jobs: River + reconciler loops

Two patterns, deliberately distinct:
- **Control-plane work** (emails, invoice close, preview-env reaper, backtests, insight scans): **River** jobs, transactional with their triggering write, idempotent handlers, periodic jobs for cron.
- **Infrastructure convergence**: never a "job" — always the **reconciler** (D9): desired state in Postgres, cell-agent converges with level-triggered loops + watch, status written back. If a change can be expressed as desired state, it must be (a job that imperatively provisions is a defect, per the provisioning pack's mistake bank).

## 13 · AI services: a module, not a service

The assistant lives **inside `services/api` as the `assistant` module**. The eight read-only tools are internal function calls that receive the viewer's RBAC context (Law 3 enforcement is a function argument, not a network hop); the resolver, proposal store, and insight scan share the monolith's data layer; inference is a **bought API behind a swappable interface** (D5) so the provider is a config change. Law 4 (disable) is a policy check, not a deployment boundary — a separate AI service would add latency, a second deploy, and an inter-service auth surface for zero law-enforcement benefit. **Trigger to revisit:** if insight scans' resource profile measurably interferes with API latency, the *scan worker* (River jobs) moves to its own deployment — the module boundary already permits it.

## 14 · Deployment

Per the migration/E1 plan, confirmed: **zonal GKE Standard** (free mgmt tier), core pool (floor 1) runs `api` + the CNPG operator/control DB + observability (customer clusters live on the ZFS storage pool, hibernating to zero); scale-to-zero gVisor pool runs customer workloads + builds (Kaniko/BuildKit, signed with cosign, provenance from first build). Console = static GCS+CDN. Terraform for everything; GitHub Actions → build → sign → **reconciler-applied manifests** (the control plane deploys itself the way it deploys customers). Workload identity everywhere, zero static keys, secrets via GCP Secret Manager with KMS envelope (satisfies D5's "KMS envelope"; OpenBao is the self-hosted option if a trigger ever demands it). Environments: founder dev us-central1 (destroyable, duty-cycled) · partner-facing born asia-south1 (A1.7).

## 15 · Repository architecture (confirms workflow Part II A5)

```
apps/console        TS/React SPA (built)
apps/cli            Go — the steloit CLI (thin client of /v1)
services/api        Go modular monolith — ONE deployable
  cmd/api/main.go
  internal/platform/   config · db (pgx/sqlc) · problem (RFC9457) · ids · money · river · sse
  internal/identity/   users · sessions · tokens · rbac (matrix+policy evaluator)
  internal/orgs/       orgs · members · invites · projects · environments
  internal/provisioning/ estimates · services · bindings · secrets · drivers/ · cells
  internal/billing/    metering · quotas · subscriptions · invoices · payments
  internal/observe/    events spine · metrics/logs/traces query · alerts · notifications
  internal/assistant/  tools · resolver · threads · insights · proposals
  internal/httpapi/    generated server (oapi-codegen) + handlers wiring modules
services/cell-agent  Go reconciler (controller-runtime patterns; talks REST /v1)
packages/contracts   generated TS client (console/SDK) — from docs/product/08-api/openapi.yaml
packages/canon       fixtures + invariants (JSON + TS utils; Go tests read the same JSON)
```

**Module boundaries (enforced, not aspirational):** modules may import `platform` and *interfaces* of other modules — never another module's internals; cross-module effects flow through the events spine or explicit interfaces. Enforced by depguard/import-boundary lint in CI. The split-into-services escape hatch exists per module (each has its own store + no lateral imports), used only on measured triggers.

## 16 · Coding standards

- **Layout & dependency direction:** handler → service → store, dependencies point inward; `internal/platform` depends on nothing above it; no module imports `httpapi`. Generated code (`sqlc`, `oapi-codegen`) is committed, never edited, regenerated in CI (drift fails the build).
- **Testing strategy (the pyramid we actually run):** (1) unit tests beside code; (2) store/integration tests against real Postgres (dockerized in CI); (3) **contract tests** — generated server stubs guarantee handler↔openapi conformance; (4) **canon invariants** imported into Go and TS from `packages/canon` — same numbers, both languages; (5) the 10 QA scenarios as the E2E backbone (API-level + Playwright), landing with their epics; (6) fault injection + clock-warping per the canon-testing pack. Coverage is not a KPI; the verify blocks are.
- **Migrations:** golang-migrate, sequential numbered SQL, never edited after apply; expand→migrate→contract for breaking changes; every migration reversible or explicitly marked irreversible in review.
- **API versioning:** `/v1` additive-only; the S-process (yaml first, founder ruling for new surface) is the only path to contract change; breaking changes mean `/v2` and we plan never to need it.
- **Error handling:** one `platform/problem` package; every error path returns catalog-registered problem+json with required `remediation`; internal errors wrapped with `%w`, logged with the event id that the 5xx body carries.
- **Configuration:** 12-factor env vars into one typed config struct validated at boot (fail fast, print effective non-secret config); no config files in prod; secrets only via workload identity/Secret Manager.
- **Feature flags:** no flag SaaS. Three native mechanisms only — plan/capability gates (the billing matrix, already product), org policies (already product), and build-time slice flags in the console (exist). A runtime kill-switch, if ever needed, is a row in a `settings` table behind an ADR.

## 17 · Implementation toolchain (the final eight)

- **HTTP framework: stdlib `net/http`** (Go 1.22+ ServeMux, method+path patterns) — no chi, no echo. The router's job is done by the generator: oapi-codegen emits the route wiring from openapi.yaml, so a routing library would be a dependency doing nothing. Middleware = plain `func(http.Handler) http.Handler` chains in `platform/httpmw` (request-id, auth, RBAC context, problem-recovery, logging).
- **Dependency injection: manual constructor wiring** in one composition root (`cmd/api/main.go`): build config → pgx pool → stores → services → handlers, explicitly, in order. Wire/Fx rejected: codegen/reflection magic to save ~60 readable lines in a 7-module monolith is complexity with no payoff, and explicit wiring is exactly what agents and reviewers parse best.
- **Configuration: `caarlos0/env` v11** into the one typed struct (§16), `required` tags + a boot-time `Validate()`; fail fast, print effective non-secret config. Viper/koanf rejected (files, watching, surface we don't want).
- **Logging: stdlib `log/slog`** — JSON handler in prod, text in dev. Conventions: snake_case keys; every log in a request path carries `request_id` (+ `trace_id` from OTel context) and the in-scope ids (`org_id`, `project_id`, `env_id`, `service_id`); errors log once, at the boundary, with the `event_id` the 5xx body carries; levels error/warn/info/debug; logger injected, never global-constructed outside `platform`.
- **Testing stack:** unit = stdlib `testing` + `testify/require` · integration = **testcontainers-go** (real Postgres; sqlc queries and River jobs run against it) · API/contract = `httptest` over the generated strict server + oapi-codegen request/response validation middleware, with golden problem+json bodies · E2E = the 10 canon scenarios as a Go harness against a compose stack, plus Playwright (already in console) for browser flows · plus the §16 pyramid (canon invariants both languages, fault injection, clock-warping).
- **Developer tooling:** lint = **golangci-lint** (govet, staticcheck, errcheck, gosec, **depguard enforcing §15 module boundaries**) + Biome (TS, exists) · format = **gofumpt** + goimports · security = **govulncheck** (deps) + gosec (lint) + **gitleaks** (secrets) + Trivy on images (beside cosign) · dependency updates = **Renovate** (one config for gomod + pnpm) · pre-commit = **lefthook** running gofumpt/golangci-lint/Biome/`validate.mjs` on staged files. CI runs the same set — hooks are convenience, CI is the gate.
- **OpenAPI toolchain (one pipeline, drift-checked):**
  `docs/product/08-api/openapi.yaml` (S-process is the only way to change it)
  → `make gen` runs all generators:
  ① **oapi-codegen** strict-server + types → `services/api/internal/httpapi/gen/` (handlers implement the generated interface — conformance by construction)
  ② **oapi-codegen** client → `packages/contracts/go/` (imported by `apps/cli` and `services/cell-agent` — the agent speaks `/v1` like everyone)
  ③ **Hey API** client → `packages/contracts/ts/` (consumed by `apps/console`, exists today as `gen:api`)
  ④ TS SDK = ③ + the ergonomics layer per `20-clients/sdk.md`
  → CI regenerates all four and **fails on diff** (generated code is committed, never edited).
- **ADR process:** every architectural delta from v1 = one ADR in `docs/adr/` (`NNNN-title.md`, status header, cites the §/decision it supersedes and its measured trigger). v1 itself is ADR-0001; the engineering OS (workflow, tasks, packs, sync) is ADR-0002. Per the repo's no-duplication rule, ADRs record *decisions and pointers*, never restate this document.

## 18 · Decision index (for Context Packs / AGENTS.md)

Go backend (api · cell-agent · CLI) — resolves SP0-2 · Vite/React console stands, no Next.js · Postgres everywhere (CNPG cluster per project-env + ZFS CoW snapshot branching per ADR-0003/INF-001 A4; CNPG control DB, Cloud SQL at M7) · **product surface = [postgres, valkey, web, worker] only (v1.2, ADR-0004)**; Valkey optional/idle-suspend, no internal cache tier yet · queue = pgmq Postgres-capability (no service, no broker, R3 retired) · **storage & AI = external-provider Bindings, not products; GPU removed** · River for internal jobs, reconciler for convergence, no broker · internal artifacts on GCS · search later, Postgres-first · OTel + Prometheus/Loki/Tempo/Grafana single-replica · REST+SSE only; gRPC/GraphQL rejected; WS deferred to D20 ADR · auth owned (sessions/tokens/MFA), Dex for SSO, no auth SaaS · sqlc+pgx+golang-migrate · assistant is a module, inference bought (D5) · GKE zonal + gVisor + cosign + Terraform + reconciler-deploys-itself · modular monolith with lint-enforced boundaries · stdlib net/http + manual DI + caarlos0/env + slog · testify/testcontainers/httptest + canon scenarios · golangci-lint/gofumpt/govulncheck/gitleaks/Renovate/lefthook · one `make gen` OpenAPI pipeline (oapi-codegen server + Go client, Hey API TS client, SDK), drift fails CI.
