# SDK & client design guide

SDKs are products with owners and quality bars (GOV-002 §3.5): **generated from the OpenAPI spec to guarantee parity, hand-polished ergonomics on top**. TypeScript, Python, Go first; Java, Rust, C# follow. This document owns the philosophy every language binding must satisfy — the one-grammar test applied to code.

## 1. The two-layer rule

- **Generated core** — every operation, schema, and enum from `08-api/openapi.yaml`, regenerated on spec change, never hand-edited. If a capability isn't in the spec, no SDK exposes it (and no SDK omits one that is: **parity is per-endpoint, audited in CI**).
- **Ergonomics layer** — handwritten, thin, per-language: builders, iterators, context carriers. It may *wrap* the core; it may never *bypass* it or add semantics the API doesn't have. A convenience that changes behavior is a spec change first.

## 2. One grammar, native dress

The API's shape is the SDK's shape; only casing is translated:

| | TypeScript | Python | Go |
|---|---|---|---|
| Call | `client.services.create(…)` | `client.services.create(…)` | `client.Services.Create(ctx, …)` |
| Fields | `monthlyEstimateCents` | `monthly_estimate_cents` | `MonthlyEstimateCents` |
| Enums | string unions | `StrEnum` | typed consts |

Resource nouns and verbs match the CLI exactly (`services.create` ≡ `steloit <product> create`) — a developer fluent in one client is fluent in all three surfaces. **Context is carried, not repeated**: clients accept `{org, project, env}` at construction or per-call, mirroring `--project/--env`; env is part of every project-scoped call signature (env-as-filter reaches the type system).

## 3. Non-negotiable behaviors (each maps to a platform law)

- **Money is integer cents, typed.** `MonthlyCost` wraps an integer; no float APIs exist. Formatting helpers reproduce `fmtMoney`. (One arithmetic.)
- **Errors are typed problem+json, remediation preserved.** `QuotaExceededError.overagePriceCents`, `PermissionDeniedError.denyingPolicy`, `ConflictError.reasons[]` — the fields the console renders are the fields code catches. SDKs never swallow `remediation`. (Every failure names a way forward.)
- **Retries: idempotent reads only**, exponential backoff, `Retry-After` honored on 429. **Mutations never auto-retry**; a failed proposal-apply never auto-reapplies (Law 1 in the transport layer).
- **Pagination is an iterator** over `{data, next_cursor}` — `for await (const svc of client.services.list())`; the envelope never leaks into user code, but `--raw` access to it exists.
- **Reveal-once survives the SDK.** `tokens.create()` returns the secret in the response object and nowhere else — never cached, never logged, redacted in any debug/telemetry output.
- **Estimate-before-provision is the method signature.** `services.create({estimateId})` requires an accepted estimate — the SDK offers `estimates.create()` → inspect → pass, and no shortcut that hides the price exists.
- **Streaming** — `logs.tail()` / `events.stream()` wrap SSE with reconnect; the query string is the shared query language, verbatim.
- **No AI auto-apply helper, deliberately.** `proposals.apply(id)` exists (a normal human-credentialed call, audited "via assistant"); a loop-and-apply convenience does not and must never be added. (The four laws bind library authors too.)

## 4. Runtime helpers (the second half of the SDK)

Per GOV-002 §3.5, SDKs also serve *running code*: typed access to injected bindings and secrets — `binding("db-main").url` instead of `process.env.DATABASE_URL` string-typing — resolved from the one injection mechanism, identical under `steloit dev` locally and in deployment. Helpers are read-only over injected config; they never fetch secrets over the network at runtime.

## 5. Quality bar

Every SDK ships with: examples that run against **canon fixtures** (19-canon — the only permitted demo data); generated reference docs from the same OpenAPI descriptions (one wording everywhere); semantic versioning where a spec-breaking change is an SDK major; and a parity report in CI (spec operations × languages, no gaps, no extras).
