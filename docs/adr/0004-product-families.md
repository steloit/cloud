# ADR-0004 · Product-family review: bindings over managed products; enum from 8→4

**Status:** **Accepted** · founder-ratified 2026-07-18 · INF-001 Amendment A5 applied (§A5; product ADR log ADR-034) · supersedes slices of the implementation plan, D5, and the Product enum · Architecture v1.2.
**Trigger (measured, per ADR-0001's rule):** founder-requested adversarial first-principles review of every remaining product family; three evidence sweeps (2026-07-18). Full analysis: `docs/plan/product-family-review.md`.

## Decision set

| Family | Decision |
|---|---|
| **Web** | ✅ Keep (alpha). The wedge's compute vehicle; pipeline stays minimal. Win on workflow, not hosting. |
| **Workers** | ✅ Keep as V2 — a compute *service type* ("a service that doesn't bind a port"), not a standalone product; drains the pgmq queue-capability. |
| **Valkey (cache)** | 🔄 Simplify. Keep in V1 as an **optional** service, **never provisioned by default**: provision-on-add + idle-suspend + hard quotas. Amends D5's "per-project pods, never shared." |
| **Queue** | 🔄 Collapse to a **capability of Postgres** (pgmq extension, surfaced as a Postgres tab). Remove the separate queue product. Retire E9-4 (design review), risk R3, the A1.2/A3.1 apparatus (now internal-jobs/River guidance only), and the NATS fallback. Queue-capability + workers = V2. |
| **GPU Workers** | ❌ Remove from the enum. Unbuildable on our infra (GCP trial GPU quota = 0, sales-gated); specialist vertical; never in GOV-002. Future GPU = integration/binding to Modal/Replicate, v4+. |
| **Object Storage** | 🔁 Replace with a **Storage Binding** (connect customer's S3/GCS/R2). No managed storage product in V1; deferred indefinitely (v4+, zero-egress backend only). Sidesteps the egress margin trap; on GOV-002's integrate-don't-build heuristic. |
| **AI Gateway** | 🔁 Replace with an **AI Binding** (govern the app's LLM-provider connection: policy, credentials, cost visibility, estimate-at-bind, audit, soft spend control). **No proxy, no routing, no hard in-line caps** — that is the gateway commodity, explicitly never-build. Distinct from the four-laws assistant (ADR-005/E13, unchanged). |

**Unifying architecture:** Storage Binding and AI Binding are one move — **extend the Binding primitive (#6) to external providers** (target = provider + region + secret-ref instead of an internal service id). One ~1.5–2 EW capability replaces two managed products, adds zero stateful infra.

**Final `Product` enum:** `[postgres, valkey, web, worker]` (was `[postgres, valkey, storage, queue, web, worker, gpu-worker, ai-gateway]`). New external **Binding target types:** `[storage, ai]`. Alpha: postgres+web. V1 adds valkey (optional) + external Bindings. V2: worker + pgmq queue-capability.

**Consequences:** V1 ~4 EW lighter; two fewer stateful systems ever operated; risk R3 + the A3.1 queue apparatus retired; certainty narrative extended to storage and AI, honestly; migration cost ~zero (nothing built). The product promise is untouched — this removes scope and moves two products to the integration side of Steloit's own boundary.

---

## Drafted INF-001 Amendment A5 (for founder to apply — 00-sources is human-only)

> **Amendment A5 — 2026-07-18** (founder-ratified; full evidence + candidate analysis: `steloit/cloud` `docs/plan/product-family-review.md` + `docs/adr/0004-product-families.md`; product ADR log ADR-034).
>
> **A5.1 — D5 cache clause.** Valkey remains the cache substrate. The "per-project pods, never shared" implication is amended: a cache is an **optional** service, **provisioned only on explicit add**, **idle-suspended**, and hard-quota'd — never a pod per project by default. Shared-multi-tenant-with-strong-isolation is a permitted implementation if idle economics require it. Rationale: cache cannot scale to zero; a default dedicated pod per project is a permanent idle-cost floor hostile to §3 scale-to-zero economics.
>
> **A5.2 — D5 queue clause.** The customer queue is **not a separate service or broker**. Queue capability is **pgmq inside the customer's Postgres** (branches with the database for free; consumed by a worker that scales to zero like any compute). The A1.2/A3.1 scale-to-zero-queue constraint and the NATS-JetStream fallback are **struck for the customer product** and retained only as guidance for internal control-plane jobs (River, Architecture v1 §12). *(amended: supersedes the queue portions of A1.2, A2.2, A3.1 for the customer product.)*
>
> **A5.3 — Object storage → integration.** No managed object-storage product is built in the planning horizon. Storage is delivered as a **Storage Binding** to the customer's own provider (S3/GCS/R2): credentials, config injection, policy, lifecycle, audit — never proxying the bytes, never bearing egress. A managed storage product may return only via a future ADR on a zero-egress backend. D4 ("proxy GCS") is dormant, not deleted — it applies only if managed storage is ever revived.
>
> **A5.4 — AI → integration (Binding, not gateway).** AI is governed as an **AI Binding** to an external provider: allow-policy, credentials-in-Secrets, config injection, estimate-at-bind, cost visibility (provider usage API), lifecycle audit, soft spend control. Steloit **does not** proxy LLM traffic, route/failover across providers, or enforce hard in-line spend caps — those are the AI-gateway commodity, added to the §3.7 never-build list in spirit. The four-laws assistant (ADR-005) is unaffected.
>
> **A5.5 — Bindings extend to external providers.** The Binding primitive (GOV-002 #6) may target an external provider (type + provider + region + secret-ref), not only an internal service. This is the shared mechanism for A5.3 and A5.4. Grammar isolation (D8) holds: no substrate/provider concept leaks into a customer surface beyond the provider name the customer chose.

---

## Ripple (on ratification)

- **openapi.yaml + models.md** (S-process): `Product` enum → `[postgres, valkey, web, worker]`. Binding schema gains an external-target variant (`target_type: service|storage|ai`, provider, region, secret_ref). Remove `gpu-worker`, `ai-gateway`, `storage`, `queue` as products.
- **GOV-002**: no text edit needed; the enum was the defect. AI-gateway-style traffic proxying joins the §3.7 never-build set in spirit (recorded here).
- **Architecture v1.2** (`docs/architecture.md`): §3/§5 notes (queue = pgmq-in-DB; cache = optional/idle-suspend; storage + AI = external Bindings; enum). Stamp v1.1→v1.2.
- **Roadmap** (`implementation-plan.md`): E9 rescoped to **Valkey (optional) + external Bindings (storage+AI)** in V1; delete E9-2/E9-3 (managed storage), E9-4 (queue review); E9-5 → V2 pgmq-capability; E9-6/CK-M6 redefined; **remove risk R3**; Sprint 8–9 lightened; effort down ~4 EW.
- **Tasks**: close/retag E9-2, E9-3, E9-4, E9-5, E9-6, CK-M6; add cache-lifecycle AC to E9-1; new tasks: external-Binding mechanism, Storage Binding, AI Binding; E14 T14.5 (object list) → dropped (no managed storage). Resolve **S6** (X1 de-canonized — the ecommerce demo's ai-gateway becomes a documented AI-Binding exemplar; the $208 invariant already excludes it). Queue frames (S4/D8/D17/D18) re-home under the Postgres service as a tab (V2).
- **Context Packs**: `provisioning.md` (cache optional/idle-suspend; queue = pgmq-in-DB; external Bindings for storage/AI); `ai-plane.md` gains an AI-Binding note (distinct from the assistant); `billing.md` (AI-Binding soft spend control + estimate-at-bind).
- **Messaging**: "one platform replaces Neon + Upstash + R2 + a queue vendor" → right-sized to what V1 delivers (Postgres + optional cache) plus **governed Bindings** to your storage and AI (integrate, don't resell). Certainty narrative untouched; the bundle claim becomes falsifiable-on-contact.

**Migration effort:** ~zero code; ~2–3 hrs agent doc-work. Same as ADR-0003/A4.

**What does NOT change:** alpha path, five-partner wedge, certainty narrative, ADR-0003 substrate, Architecture v1.1 stack, estimate-gate/cap/masking, Postgres, web, the four-laws assistant.
