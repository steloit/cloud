# Product-Family Review — Adversarial First-Principles Pass

**Status:** Proposed for founder decision · 2026-07-18 · Same governance as ADR-0003/the wedge review: this is analysis; nothing touching the enum, INF-001, GOV-002, or frozen architecture changes until ratified.
**Method:** ADR-0003-style attack on every remaining product family (cache, object storage, queue, web, workers, GPU workers, AI gateway). Three evidence sweeps (competitive/commercial, developer discourse, funding) + the strategy corpus + the prior demand ranking.
**Frame:** the wedge review already established the top customer pains — cost certainty, reliability, agent-DBs, migration fear, previews. **None of the seven families here is on that list.** So the burden of proof flips: each must *earn* its place, and the default for anything that neither completes the wedge nor makes the "one-platform" reveal credible is **defer or cut**.

---

## 0 · The finding that reframes everything

**The product enum over-committed beyond the constitution.** GOV-002 §3.8's own "Build (core)" list is: *Postgres, Valkey, Storage, Queue, Container compute, workers, cron, unified observability, IAM/secrets/audit, the AI assistance layer.* The API enum is `[postgres, valkey, storage, queue, web, worker, gpu-worker, ai-gateway]`. **`gpu-worker` and `ai-gateway` are in the enum but nowhere in GOV-002's build list, have zero build tasks, and no roadmap placement** — `ai-gateway` leaked in via the ecommerce canon demo (the X1 exemplar), `gpu-worker` is a bare enum entry. They are reserved surface with no constitutional mandate. That's a defect this review closes without needing a single market datapoint.

## 1 · The cross-cutting pattern the evidence keeps repeating

Every family here sits in a market with the **same shape**: the primitive is commoditized (often given away free or absorbed into Postgres), and the money has moved to the *ends* — demand-aggregation/billing at scale, or up-stack observability/governance/DX. A generalist 2-founder cloud can win at neither end for a *secondary* product. Translated to a rule: **build the secondary only if it (a) instantiates the Postgres pioneer pattern at near-zero marginal cost AND (b) makes the one-platform reveal materially more credible to a design partner.** Everything else is scope that competes for the founders' scarcest resource against the actual wedge.

---

## 2 · Data — Valkey (cache)

**Exists / who needs it:** every web app caches; it's genuinely ubiquitous (unlike the other secondaries). But — **nobody switches platforms for managed cache.** HN treats ElastiCache/Upstash/Heroku Redis as interchangeable; self-host covers workloads to ~1M visits/mo; at hyperscale custom beats managed ~10×. Cache is something a platform must *have*, never something anyone *chooses you for*.

**Market/competitive:** Upstash proved the serverless-per-request niche but shows **weak momentum** (no funding/acquisition surfaced; recent discourse is customers *leaving* on cost at geographic scale). Valkey is the one clear trend — AWS/Google/Oracle/Linux-Foundation-backed, "rapidly overtaking Redis." If we build cache, Valkey (D5) is unambiguously right.

**Architecture attack (the real issue):** D5 says "Valkey, per-project pods, never shared." **Cache cannot scale to zero** — it's stateful-hot, memory reserved 24/7. So a dedicated pod per project is a *permanent idle-cost floor on every project including the free tier* — the exact opposite of CNPG (which hibernates) and structurally hostile to our scale-to-zero economics. Upstash sells per-command pricing precisely because it runs **shared multi-tenant with keyspace/ACL isolation**, not dedicated instances. D5's "per-project pods, never shared" is, for cache specifically, the costly choice.

**Operational/economics:** ~2 EW to instantiate off the pioneer pattern; but it adds a second stateful system to operate during the ₹0/trial phase, for a bundle-completer no partner pulls on for the wedge.

**Attack it / defend it:** *Against:* off-wedge, idle-cost trap, second stateful system, nobody switches for it — defer. *For:* it's the most-used secondary (every app caches), cheap to instantiate, and its absence dents the "more than a database" signal on day one of the reveal.

**Recommendation: 🔄 Simplify.** Keep Valkey as the cache product, but **fix the lifecycle before building: provision-on-add (a project gets a cache pod only when it asks for one) + idle-suspend + hard memory quotas** — never a pod per project by default. This requires amending D5's "per-project pods, never shared" for cache. **Timing: keep in V1 only if the provision-on-add lifecycle is in scope; otherwise it drops cleanly to V2.** My lean is V1-simplified — it's the one secondary worth the small cost.

---

## 3 · Data — Object Storage

**Exists / who needs it:** apps need blobs (uploads, assets). But object storage is *the* canonical commodity with a universal API (S3), and **there is no evidence anyone picks a dev platform for its blob store** — they point their app at S3/R2/B2 directly.

**Market/competitive:** R2's zero-egress was a genuine, durable disruption (667-pt launch; egress called "the ultimate cloud moat"). That's the warning, not the invitation.

**Architecture attack (the margin trap):** D4 says "proxy GCS, serve every URL through our domain/CDN." The sharpest evidence line: proxying means **you become the party that pays egress at provider rates, at the system boundary, on every read, for all tenants.** R2 exists *because* egress is where the margin and moat live. Proxying S3/GCS = thin-or-negative margin on a commodity nobody switches for — unless the backend is itself zero-egress (R2/B2), in which case the customer could just use R2 directly. INF-001 already flags egress as a risk; the storage proxy is where that risk concentrates.

**Strategy/economics:** 3 EW, the **least** wedge-relevant of all seven families, and the worst cost-control profile. It is the clearest "not in V1" of the data secondaries.

**Attack it / defend it:** *Against:* off-wedge, margin trap, standard API means zero switching benefit — the strongest defer/don't-build case of the three. *For:* the "replaces R2" bundle line, and a *certainty* angle exists ("no surprise egress bills") — but that's a V2 flourish, and honest egress pricing on a proxied backend is exactly the margin we can't control.

**Recommendation: ⏳ Move to V2** (keep the grammar/enum slot; build later, and when built, **revisit D4** — a zero-egress backend like R2/B2, or storage-as-integration/binding, may beat proxying GCS). Out of V1 entirely.

---

## 4 · Data — Queue

**Exists / who needs it:** background jobs are real. But **"standalone managed queue" is arguably a dead product category**, squeezed from both sides:
- **From below — commoditized into Postgres.** pgmq (241 pts), River (360 pts), `SKIP LOCKED`; "Postgres gives me everything I want," "performance is a non-issue below ~10M items/day." Supabase shipped pgmq *inside* Postgres as "Supabase Queues," not as a separate SKU. If you offer managed Postgres, the customer already has the substrate.
- **From above — out-competed by durable execution.** Temporal ($5B, $300M Series D 2026), Inngest, Trigger.dev, DBOS, Restate, Microsoft's pg_durable (474 pts) win everything complex enough to justify a dedicated tool — and the value is the **SDK/DX/orchestration layer, not the queue primitive** (tellingly, most are themselves Postgres-backed).

**Architecture attack (deletes an entire apparatus):** the queue product is the single most-flagged item in our own plan — the A3.1 WAL-signal design review (E9-4), the NATS fallback, risk R3, the whole A1.2 scale-to-zero constraint. **All of that complexity exists only to build a *separate* branch-coherent queue service.** Collapse the product to **pgmq inside the customer's CNPG database** and every bit of it evaporates: the queue rows branch with the DB for free (they're *in* the DB), and the consumer is a worker that scales to zero like any worker — the same problem we already solve for compute, not a special queue problem. No safekeeper/CDC, no NATS, no design review, no R3.

**Note on internal jobs:** we already chose **River (Go/Postgres)** for the control plane's own jobs (Architecture v1 §12). Customer "queue capability" = pgmq-in-their-DB. Both Postgres-backed, consistent, boring.

**Attack it / defend it:** *Against:* dead category; the standalone-queue half is the commoditized half; durable execution is a real company's whole business (Temporal) → ADR-003 says *integrate, don't build*. *For:* a documented pgmq capability is near-zero cost and rounds out the bundle — worth doing as a *capability*, not a *product*.

**Recommendation: 🔄 Simplify to a capability + ⏳ V2.** Kill the "queue product": **delete E9-4 (design review), rewrite E9-5 to "enable pgmq in the customer's Postgres + document the worker-consumer pattern"** (near-zero build, ships in V2 alongside workers, since a queue without worker compute to drain it is half a product). Durable execution is explicitly **never-build / integrate-later** (Temporal/Inngest via Bindings, per GOV-003 boundary logic). This retires R3 and the A1.2/A3.1 constraint's entire relevance to the customer product.

---

## 5 · Compute — Web

**Exists / who needs it:** the compute half of the wedge — push repo → deployed service → the preview loop only exists because we run the app. Core.

**Market/competitive:** app-hosting *delivery* is commoditized (Railway/Render/Fly all raising but none profitable; Fly.io ~1/30th of a GPU specialist's revenue and hasn't raised since 2023). The universal 2026 finding: **the differentiator moved up-stack to workflow/DX — preview envs, integrated per-environment databases, git-deploy.** That is *exactly the certainty wedge.*

**Why choose Steloit:** not for the hosting — for the certainty around it (estimate-before-deploy, the masked branched-DB preview, the cap). Web compute is the **vehicle for certainty, not chosen on hosting merits** — and GOV-002 §1.4 keeps "connect your external host + Steloit data" a permanent first-class mode, so we're not betting the company on winning app-hosting.

**Recommendation: ✅ Keep (alpha).** Keep the build pipeline deliberately minimal (buildpacks/Dockerfile, gVisor) — do not out-engineer Railway on hosting; win on the workflow.

---

## 6 · Compute — Workers

**Exists / who needs it:** long-running / non-HTTP background compute. Real, but the evidence reframes it: a worker is **"a service that doesn't bind a port"** — Render/Railway treat it as a first-class *service type* with its own lifecycle/billing, **not a separately marketed product.** So it's a thin variant of web compute, not a heavy new surface.

**Strategy:** already correctly V2 (GOV-002 v2; ADR-006 data-before-compute). Pairs naturally with the pgmq queue capability (§4), which is why both belong in the same V2 wave.

**Recommendation: ✅ Keep as V2** — an "existing decision already best" case, made *lighter*: it's a service type, not a product. No change beyond noting it drains the pgmq queue.

---

## 7 · Compute — GPU Workers

**Exists / who needs it:** GPU inference/training. But the evidence is one-directional against a generalist owning it, and against *us* specifically:

- **We literally cannot build or demo it on our infra.** New GCP accounts have GPU quota **hard-locked at 0**, behind a "contact sales" process that won't engage individuals; the $300 trial is "largely unusable for GPU." A `gpu-worker` surface would be a dead UI backed by nothing.
- **Every generalist stays out, and the one entrant is leaving:** Fly.io's docs — the only generalist that shipped real GPU — now read "GPUs are deprecated and will be unavailable after August 1." Railway/Render/Vercel/Supabase: none.
- **It's a capital-intensive specialist war** (Modal $4.65B, Baseten $5–11B, Together $8.3B, Fal ~$4.5B) fought on reserved fleets and custom cold-start engineering. Two founders on trial credits have no wedge.
- **Idle economics are structurally fatal** at small scale: 19–90s cold starts; reserved GPUs idle money below ~40–50% utilization.
- **Not in GOV-002's build list.** It was never sanctioned.

**Attack it / defend it:** *Against:* all of the above. *For:* the AI-app tailwind — but the correct form of that is a **pass-through binding to a specialist (Modal/Replicate/Fal)**, post-PMF, never owned capacity.

**Recommendation: ❌ Remove** `gpu-worker` from the enum. If GPU ever matters, it's a v4+ *integration*, not a product surface we reserve today.

---

## 8 · Platform — AI Gateway

**Exists / who needs it:** route/meter an app's LLM calls. But it's the textbook commodity-in-the-middle:

- **Two VC-funded routing-first startups (Martian, Unify) both abandoned routing** — the clearest possible market verdict.
- **Cloudflare and Vercel give a full-featured AI gateway away at $0 markup;** LiteLLM is free open-source (54k stars) and the self-host default. The commodity's price is zero.
- OpenRouter is the one big winner (~100T tokens/mo, $1.3B) but it's a **payments-float + demand-aggregation business**, not gateway software; its take is a Stripe spread.
- Real money sits only at the *ends* (observability/governance — Portkey/Helicone/Kong; or billing-at-scale — OpenRouter). The middle (routing/unified-API/caching/rate-limiting) is given away.
- **Not in GOV-002** (which has an *assistant layer*, ADR-005 — a different thing). It leaked in via the X1 canon exemplar and is *not even canonized* (pending decision S6/7.3).

**The defensible remnant collapses into certainty:** the one valued gateway feature is **spend caps on tokens** — and that is not a product, it's **F9's cap/metering applied to a Binding to an external LLM provider.** "Cap your AI spend" ships as *certainty everywhere*, not as a gateway we build and operate.

**Recommendation: ❌ Remove** `ai-gateway` as a product; resolve S6/7.3 by **not canonizing X1** as a real service (keep it a documented client-side exemplar, or cut it). AI token spend flows through the existing metering/cap via a provider binding — no new surface.

---

## 9 · Decision summary (for ratification)

| Family | Current | Recommendation | Net effect |
|---|---|---|---|
| **Web** | Alpha | ✅ **Keep** | Wedge vehicle; keep pipeline minimal |
| **Workers** | V2 | ✅ **Keep as V2** | Reframed: a service type, not a product; pairs with pgmq |
| **Valkey (cache)** | V1 | 🔄 **Simplify** | Provision-on-add + idle-suspend (amend D5); V1-if-lifecycle-in-scope, else V2 |
| **Object Storage** | V1 | ⏳ **Move to V2** | Off-wedge + egress margin trap; revisit D4 (proxy) when built |
| **Queue** | V1 | 🔄 **Simplify → capability + ⏳ V2** | pgmq-in-DB; **delete E9-4, R3, the A3.1 apparatus, NATS** |
| **GPU Workers** | enum-only | ❌ **Remove from enum** | Unbuildable on our infra; specialist vertical; never sanctioned |
| **AI Gateway** | enum + X1 | ❌ **Remove as product** | Commodity; spend-cap remnant → F9 binding, not a product |

**The shape of the win:** V1's data-layer (E9) drops from ~8 EW to ~2 EW (Valkey, simplified); object storage and the queue product leave V1; **risk R3 and the entire A3.1 queue-design-review apparatus are retired**; no new stateful system (NATS) ever enters the architecture; two un-sanctioned enum entries are cut. V1 becomes *the wedge, fully realized* — postgres + web + previews + the certainty layer (cap, masking, billing, observe, console, AI assistant) — with cache as the one bundle-completer. Same product promise, smaller and more operable surface. The ADR-0003 pattern exactly.

---

## 10 · Ripple analysis (only if ratified)

**Migration effort: ~zero code (nothing is built); ~2–3 hrs agent doc-work.** Same as the Postgres and wedge reviews.

- **INF-001 Amendment A5** (founder-ratified — 00-sources is human-only): D5 cache clause (per-project pods → provision-on-add/idle-suspend; shared-with-isolation permitted); D5 queue clause (pgmq-in-customer-DB, not a separate service; NATS struck); note A1.2/A3.1 now bind *internal jobs (River)* only, superseded for the customer product. New §2 nothing; this is a scope-narrowing amendment.
- **GOV-002 §3.8**: annotate the build list — GPU/AI-gateway were never on it (no change to GOV-002 text needed; the enum is what's wrong).
- **openapi.yaml + models.md** (S-process): `Product` enum → `[postgres, valkey, storage, queue, web, worker]`. Removes `gpu-worker`, `ai-gateway`. *(superseded: the ratified final is the 4-value enum `[postgres, valkey, web, worker]` — §11d / ADR-034; this line records the review's mid-document proposal only)*
- **ADR-0004** (docs/adr) + **ADR-034** (product log): record this review's outcomes, superseding the relevant slices of the implementation plan and D5.
- **Architecture v1.2**: §3/§5 note (customer queue = pgmq-in-DB; cache = provision-on-add); enum note.
- **Roadmap (implementation-plan.md)**: E9 rescoped to Valkey-only in V1; E9-2/E9-3 (storage) → V2; E9-4 **deleted**; E9-5 → V2 pgmq-capability; E9-6/CK-M6 ("data layer complete") redefined; **risk R3 removed**; Sprint 8–9 lightened; effort totals down ~6 EW.
- **Tasks**: close/retag E9-2, E9-3, E9-4, E9-5, E9-6, CK-M6; add cache-lifecycle AC to E9-1; E14 T14.5 (object list) → V2-gated. Resolve **S6** (X1 de-canonized). Canon: the ecommerce demo's ai-gateway service (X1) becomes a documented exemplar, not a canon service (arithmetic already excludes it — the $208 invariant never counted it).
- **Context Packs**: `provisioning.md` (cache provision-on-add + idle-suspend; queue = pgmq-in-DB, no queue-as-service); `ai-plane.md` unaffected (assistant ≠ gateway).
- **Messaging**: the "one platform replaces Neon + Upstash + R2 + a queue vendor" line softens to what V1 actually delivers (Postgres + cache); R2/queue framed as roadmap/integration. The *certainty* narrative is untouched — this only right-sizes the bundle claim to be falsifiable-on-contact (per messaging.md's own test).

**What does NOT change:** the alpha path, the five-partner wedge, the certainty narrative, ADR-0003's substrate, Architecture v1.1's stack, the estimate-gate/cap/masking, Postgres and web. This review removes scope; it does not touch the promise.

---

## 11 · Founder-refined final decisions (2026-07-18)

Rows 1,2,4 (web keep / workers V2 / GPU remove) and the queue collapse (row 5→pgmq-in-DB) are approved as written. Two rows were sent back for a Binding reframe. Evaluated below; both reframes **win**, and together they produce the review's cleanest architectural output.

### 11a · Storage — 🔁 **Replace managed storage with a Storage Binding** (adopt the reframe; supersedes §3's "V2")

The reframe asks: instead of *building* object storage, make it a **Binding** to the customer's own S3/GCS/R2. Evaluated against the prior "build in V2":

| | Managed storage (build, V2) | **Storage Binding (integrate)** |
|---|---|---|
| Egress economics | We pay egress at provider rates on every read — the margin trap | **Zero** — customer's app ↔ their bucket; we never touch the bytes |
| New stateful infra | Yes (buckets, proxy, CDN, content-domain) | **None** — rides Secrets (#7) + Bindings (#6) + config injection |
| Constitutional fit | Object storage is a commodity with a standard API and gravity (S3) → GOV-002 says *integrate, don't build* | **Exactly the heuristic** |
| Certainty story | "one bill" includes storage | Governed integration: estimate-at-bind (provider's price shown), allow-policy, credential hygiene + rotation, topology, lifecycle-audit — honest ("we don't mark up your storage") |
| Effort | 3 EW + ongoing egress risk | ~part of the shared external-Binding mechanism below |

**Honest tradeoff:** the Binding forfeits "storage on *our* bill" — storage cost lives on the customer's provider invoice. That marginally weakens the "one bill" bundle line, but it's *more* falsifiable-on-contact (no marked-up egress to explain) and it permanently sidesteps the margin trap. **Recommendation: Storage Binding is the V1 storage answer; a managed storage product is deferred indefinitely** (v4+, and only ever on a zero-egress backend like R2/B2). `storage` leaves the Service enum.

### 11b · AI — 🔁 **Replace "AI Gateway" with an "AI Binding"** (adopt the reframe; supersedes §8's "remove entirely")

The reframe asks: make AI a **Binding** to a provider (OpenAI/Anthropic/…), governed like any other — config, credentials, spend visibility, policy, audit — *without* a routing product. Evaluated:

**The critical distinction that makes this work — and its one honest boundary.** An AI *Gateway* sits in the request path (proxy) and routes/caches/hard-caps traffic — the commodity Cloudflare gives away and Martian/Unify abandoned. An AI *Binding* is control-plane only: it governs the *connection*, not the *traffic*. That boundary is exactly what keeps it simple and on-thesis, but it has a real consequence you must accept:

- **What the no-proxy AI Binding delivers (all real, all cheap):** provider + model **allow-policy** (governance — "this org may use Anthropic, not X"); **credentials in Secrets**, scoped per env, rotated, never in code; **config injection** (`ANTHROPIC_API_KEY`, base URL) via the existing Binding mechanism; **estimate-at-bind** ("gpt-x costs $/1k tokens" — the wedge, extended to the scariest new bill); **cost visibility** surfaced from the provider's usage API into the unified billing view; **lifecycle audit** on the events spine; **soft spend control** — suspend/rotate the binding at a threshold.
- **What it deliberately does NOT do (this is the guardrail, not a gap):** proxy traffic, route/failover across providers, or enforce *hard, real-time, in-line* spend caps + per-call observability. Those require sitting in the request path — i.e., building the gateway commodity. If a customer needs a hard real-time cap, that's the provider's own budget feature; Steloit's soft cap (usage-API + key control) is the honest control-plane answer.

**Verdict:** materially simpler than a gateway, materially better than removing AI. In 2026 every app has an LLM dependency; governing it like a database — with policy, credential hygiene, cost visibility, and estimate-at-bind — extends the certainty narrative to AI **without** entering the commodity. Adopt. (Distinct from the AI *assistant* — the four-laws module, ADR-005/E13 — which is unchanged. Assistant = Steloit's own AI; AI Binding = the customer's app's AI dependency, governed.)

### 11c · The unifying output — one mechanism replaces two products

Storage Binding and AI Binding are **the same architectural move: extend the Binding primitive (#6) to external providers.** A Binding already manages credentials, injects config, and rotates — the only change is that the *target* can be an external provider (type + provider + region + secret-ref) instead of an internal service. **One small capability (~1.5–2 EW, shared) replaces two would-be managed products** (object storage, AI gateway), adds zero stateful infra, and is maximally on-thesis. This is the ADR-0003 pattern again: same promise, less to build and operate.

### 11d · The final enum

`Product` (managed services we actually build): **`[postgres, valkey, web, worker]`** — from eight to four. Removed: `gpu-worker` (cut), `ai-gateway`→AI Binding, `storage`→Storage Binding, `queue`→a *capability of Postgres* (pgmq extension, surfaced as a Postgres tab like branches/backups — not a standalone service). New: external **Binding target types** `[storage, ai]` (provider-backed). Alpha builds `postgres`+`web`; V1 adds `valkey` (simplified) + external Bindings; `worker` + the pgmq queue-capability are V2.

**Net vs the original plan:** ~4 EW lighter in V1, two fewer stateful systems ever operated, risk R3 and the entire A3.1 queue apparatus retired, four products become four (down from eight) plus two integration Bindings — and the certainty narrative now reaches storage and AI, honestly. Final recommendation stands; the prepared amendments follow in **ADR-0004** (drafted, ratification-ready).
