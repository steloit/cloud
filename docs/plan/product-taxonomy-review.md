# Product Taxonomy Review — Capability, Service, or Binding?

**Status:** **RATIFIED — ADR-038 accepted; the product taxonomy is FROZEN** · founder-approved 2026-07-18 · The State Test (with the why-state philosophy sentence and the Jobs≠Messaging scope clause) is the canonical product-classification rule. From this point forward, every proposed product begins by answering the State Test before any implementation discussion. No frozen surface changed — this review validated the taxonomy and gave it its principle.
**The verdict up front:** **Neither model as posed. Model A's taxonomy is correct — but for a reason neither model states: the dependency is *semantic*, not implementational. Model B mistakes "implementation ≠ product" for "therefore independent product," and its swap-optionality is an illusion that would forfeit our differentiation. The UX problem Model A creates (Postgres as homework) is real and is solved by the catalog selling *intents* that the composer resolves — which AI1 already specifies. The deliverable is the classification principle (the State Test) that decides every future case mechanically.**

---

## 0 · Correcting the premise (the current model, precisely)

The challenge's taxonomy listing mis-bins a few things worth fixing before arguing: **Secrets and Observability are platform primitives** (GOV-002 #7, #9 — they belong to the tree, not to Postgres); **Branching and Backups are per-service capabilities** (every service has backups; branching is Postgres's signature); **Email/Payments/Identity providers are template/binding integrations** (GOV-002 §3.8 integrate-column), alongside the Storage and AI Bindings (ADR-0004). The contested set is exactly three: **Jobs, Search, Vector.**

## 1 · The central question, answered directly

**"Is an implementation dependency the same thing as a product dependency?" — No. But the challenge offers only two possibilities (implementation detail vs independent product), and the truth for these three is a third thing: a *semantic* dependency.**

Test each, honestly:

- **Does Jobs require PostgreSQL because customers think it does?** No. **Because pgmq is our MVP?** Also no — that alone would never justify the coupling; the founder's instinct is right that MVP implementation must not define product identity. **The real reason:** the *differentiated semantics* of Steloit's queue only exist inside the customer's database — **transactional enqueue** (enqueue in the same transaction as the business write — the property that eliminates the dual-write problem every external queue has) and **branch-coherence** (your preview environment includes your queue state, because the queue *is* rows). Move Jobs out of the database and those aren't implementation losses — they're *product* losses. The features die.
- **Vector:** embeddings are rows *about the app's data*. Their value is joining with it, staying transactionally consistent with it, branching with previews. A separate vector store re-creates the sync-pipeline pain that pgvector's entire market victory came from eliminating. The dependency is the feature.
- **Search:** search-over-*your-rows* has the same shape today (FTS). Unlike the other two, Search has a known future point where the semantics genuinely diverge (multi-source indexes, independent scaling) — and GOV-002 already plans exactly that: dedicated Search as a *new service* at v4. That is the correct evolution: **when the semantics stop being "of your database," the thing earns a new name — it doesn't inherit the old one with swapped internals.**

## 2 · The principle (the deliverable): the State Test

**A thing is a Service iff its state has an independent lifecycle and an independent isolation/scaling boundary. It is a Capability iff its state is *of* a parent service — it branches, backs up, joins, and transacts with the parent's data. It is a Binding iff its state lives outside the platform.**

**Why state is the deciding factor (the philosophy behind the rule):** state is the architectural boundary because lifecycle, backup, branching, scaling, permissions, billing, and ownership all ultimately follow state. Therefore the location and ownership of state — not implementation, not UI — is the canonical classifier for every managed product. (Founder addition, 2026-07-18.)

Implementation dependency is irrelevant to classification; **semantic entanglement is identity.** Applied: Jobs/Search/Vector state is *of the customer's database* (branches/joins/transacts with it) → Capabilities. Postgres/Valkey/Web/Worker have independent lifecycles and isolation boundaries → Services. Storage/AI state lives at an external provider → Bindings. Every future case resolves mechanically: "does its state branch with something else's?" decides it.

### Scope clause (founder challenge, 2026-07-18 — incorporated): Jobs ≠ Messaging

The Jobs capability is scoped **exactly** to *database-native background execution*: work items whose state belongs to the application's PostgreSQL — transactional enqueue, branch-coherence, joins with application data, DLQ, at-least-once execution. **Messaging/event-streaming is a different category, not a bigger queue** — the boundary is state-semantics, not throughput: a streaming platform's first-class state is *the log itself* (retention, replay, ordering, consumer groups, event sourcing), with its own lifecycle and scaling boundary. It therefore **passes the State Test as a Service in its own right** — the challenge strengthens the test by being its first correct application to a future case. Consequences, binding on the future:
1. **"Jobs" is never backend-swapped to a broker.** The capability's database-native semantics *are* its contract; a Kafka-backed thing calling itself Jobs would be a lie about state.
2. **If streaming demand arrives, the new-managed-product gate routes it up a ladder:** first a **Messaging Binding** (connect the customer's Confluent/MSK/EventBridge — Kafka is the definition of an ecosystem with gravity; GOV-002 §3.8 says integrate), and only on evidence a managed **Streams** service — **under its own name**, via its own ADR, passing the State Test it naturally satisfies. Same pattern as the BYOC residency ladder and the Storage/AI Bindings: cheapest sufficient rung first.
3. A5.2's "no broker" strike applies to the *Jobs/queue* scope and is untouched — it never forbade a future, separately-named, evidence-gated streaming product; it forbade smuggling a broker in as the queue's implementation.

## 3 · Why Model B fails (three independent ways)

1. **The swap-optionality is an illusion.** pgmq→Kafka/SQS is not an implementation-preserving swap: transactional enqueue and branch-coherence *cannot survive it* — no external broker can enqueue in your database's transaction. pgvector→Pinecone loses joins and consistency; FTS→OpenSearch loses transactional freshness. To keep the "stable public API" Model B promises, you must **promise only the lowest common denominator** across possible backends — i.e., discard precisely the semantics that differentiate us and that the wedge (branch-coherent previews) depends on. Premature abstraction here doesn't preserve optionality; **it forfeits the differentiation to buy a flexibility we'd never honestly exercise.**
2. **It re-opens a ratified, evidence-backed decision.** ADR-0004 established with market evidence that the standalone managed queue is a **dead category** (commoditized into Postgres below, out-competed by durable execution above) — and the same research found the standalone-serverless-primitives vendor (Upstash) showing weak momentum with customers churning on cost. Model B rebuilds the rejected products with extra steps: enum 4→7, three new S/D-family console surfaces, three estimate shapes, three sets of quotas — for products with zero demand evidence.
3. **It is the AWS taxonomy.** "Everything is a separate service" (SQS, OpenSearch, Kendra…) is the fragmentation experience GOV-002 §0 names as the enemy: *"competitors make you think in databases, buckets, and instances; Steloit makes you think in your app."* Model B doesn't fix our taxonomy — it adopts our competitor's.

**The decisive external precedent — MongoDB Atlas:** Atlas Search and Atlas Vector Search run a *completely different engine* (Lucene) under the hood, yet are sold as **capabilities of your cluster**, scoped to your collections — hugely successful, and proof of two things at once: (a) capability-of-the-database is a winning product location even when the internal engine differs, and (b) **the abstraction seam Model B wants belongs *inside* the capability, not at the product level.** If Steloit's Search tab is someday powered by an attached engine instead of FTS, it can be — transparently, still scoped to that database's data, still branch-aware — without ever becoming a standalone product. Supabase (pgmq, pgvector, pg_cron as database features) is the same pattern at the dev-platform tier; Render/Railway sell none of these; Cloudflare Queues is standalone only because Cloudflare has no app-database product to attach to.

## 4 · Why Model A as-posed also fails — and the fix

Model A's genuine flaw is UX, not taxonomy: **"provision PostgreSQL first" as customer homework is a product-identity leak in the opposite direction** — it makes the implementation the *user's* problem. The customer who says "I just want a job queue" must never be told to go learn about databases.

**The fix is already designed: the catalog sells intents; the composer resolves them.** AI1 (describe-to-provision) is specified as exactly this: *"map intent → services[] each with why + shape + estimate."* The create canvas (C1) is the one room. So:

> Customer picks **"Jobs"** in the catalog/canvas → the composer proposes: *a managed Postgres shaped for jobs (pgmq enabled, small) — "your queue state branches with previews and enqueues transactionally with your data" — plus a Worker to drain it* → estimate shown → one accept provisions everything. If a Postgres already exists: *"enable Jobs on db-main, or create a dedicated one?"*

The dependency is satisfied **by composition, never by error message.** The customer asked for an outcome and received it, priced. What appears in the rail is honest (a Postgres carrying the Jobs capability — and they got a full database for free, which is a gift, not a leak). This is also exactly how the canon world already renders it: `jobs` in the ecommerce project *is* a Postgres instance carrying queue semantics.

**"I only want a Vector database"** resolves the same way — and more pointedly: a Postgres with pgvector *is* a vector database, one whose vectors can also join, transact, and branch. **"I only want Search"** likewise. The persona these intents don't serve — someone shopping for a bare commodity queue/vector/search endpoint with no app on the platform — is the customer ADR-0004's evidence says not to chase.

## 4b · The two-plane rule (founder refinement, 2026-07-18 — ratified as ADR-039)

**Architecture answers "what is this?" (the State Test, frozen). The catalog answers "what problem does this solve?" (intents). The planes never leak into each other.**

- **Catalog plane:** flat and outcome-first — **Jobs, Search, Vector are catalog peers of PostgreSQL**; parentage is never displayed as structure. A catalog entry is an *intent*, not a product identity. Rationale (founder): the customer with an existing PostgreSQL isn't buying Jobs because they can't install pgmq — **they're buying the managed experience** (queue UI, retries, scheduling, DLQ, observability, lifecycle, branch-coherent previews). The outcome+experience is the product; the backing is the Composer's concern. One intent entry also means future backends never duplicate catalog entries under multiple services.
- **The bridge:** the Composer fans an intent out to resolutions ("create a new PostgreSQL for it / enable on existing `db-main` / *(future)* use your connected Kafka"), and **the accepted estimate is the reconciliation contract** between intent and architecture — the customer sees "Jobs — runs on managed Postgres `jobs` · $X/mo" before accepting, and the invoice matches that line (one arithmetic). Intent-first cataloging creates no billing ambiguity because the estimate translates planes at the moment of consent.
- **The guard (preserves the ADR-038 scope clause):** intents fan out to **named products with stated semantics — never to backend dropdowns.** When Kafka-class execution someday joins the fan-out, the Composer offers *differently named* resolutions with tradeoffs stated ("database-native Jobs — transactional, branch-coherent" vs "Streams — high-throughput, replay; no transactional coupling"), and the customer picks an outcome variant whose semantics they've seen: **estimate-before-provision extended to semantics.** "Backend" is never a customer-facing word; nothing named Jobs ever silently changes semantics; the *intent* simply gains more answers over time.
- **Console honesty for free:** the rail shows the architecture truthfully via instance naming (canon's queue-shaped Postgres is literally named `jobs`) with capability tabs prominent — the outcome reads first, the architecture remains inspectable.

## 4c · Product-first pricing presentation + the generalized exact-sum invariant (founder refinements, 2026-07-18 — part of ADR-039)

**Label by outcome, account by cost components — same numbers at every level.** Estimates and invoices default to the **product view** and expand, B3-style, one level deeper than today: *product line → cost components → meter rows.* The customer buys the outcome they came for; the machinery stays one interaction away, never gated, never approximated. This is not a transparency concession — it is the console's existing grammar (ten-second truth first, expansion for depth) applied to commerce.

**The generalized invariant (founder wording, adopted):** *Every Product expands into the complete set of cost components that generated its price.* For infrastructure products those components are managed services; for platform products they may be shared compute, token usage, storage, bandwidth, or other metered resources. The invariant is not that every product maps to services — it is that **every price is completely explainable and expands to its underlying cost model without hidden arithmetic.**

**Guards:**
1. **Exact-sum rule, generalized:** the product line equals the sum of its cost-component lines, byte-for-byte, at every expansion level. No blending, no rounding across levels, no "from $X."
2. **Terminal condition (closes the loophole):** expansion bottoms out at **published unit price × metered quantity** or a **stated plan allowance** — never an opaque composite ("platform fee"). Unified rule: *every line on any Steloit surface is either a metered quantity at a published unit price, or a stated plan allowance; nothing else exists.* (This collapses F9's two axes under one auditable rule and subsumes the B3 line→meter contract.)
3. **No value markup hiding in grouping:** grouping is presentation; value capture stays on the plan axis; the infra/meter axis stays cost-transparent.
4. **Ranges are honest and bounded:** shape costs are exact; usage components show workload-informed ranges **with the cap** (US-11.7) — a range with a hard bound is a stronger promise than fake precision. "Search: $9/mo base · usage est. $3–8 · capped at your $25."
5. **Mechanism — the intent tag:** services (and meter groups) carry the intent they were provisioned from (`intent: search`, stamped by the Composer at creation) — the structural grouping key that lets estimates/invoices label by outcome while accounting by component; survives renames; one column.

API/CLI stay explicit (components + meters; the product label is a grouping key, `--json` returns the full structure). Any future *platform* product still passes the standing gates (State Test first, new-managed-product gate, ADR-0004 boundaries) — this section governs how its price is *presented and explained*, not whether it may exist.

## 5 · Deliverables, point by point

1. **Model chosen:** Model A's taxonomy + the intent catalog (call it **"capabilities with a front door"**), + the Atlas seam (engine evolution happens *inside* a capability, scoped to the parent's data, never as a silent backend swap of a standalone product).
2. **The principle:** the **State Test** (§2). Service = independent lifecycle + isolation boundary · Capability = state is *of* the parent (branches/joins/transacts with it) · Binding = state lives outside the platform. Semantic entanglement, not implementation, is identity.
3. **Dependencies:** represented as **composition in the composer**, never prerequisites in the UX. The API stays truthful (a capability is enabled on a service; `?env=`-style honesty); the catalog is intent-level.
4. **Jobs without PostgreSQL:** the §4 flow — propose the shaped service + worker, price it, one accept. Never an error, never homework.
5. **Public catalog:**
   - **Services:** PostgreSQL · Valkey · Web · Worker
   - **PostgreSQL capabilities (catalog-visible as outcomes):** Jobs & Queues · Search · Vector · Branching · Backups/PITR — each marketed by its outcome, each honest about its home ("runs inside your database; branches with your previews")
   - **Platform:** Secrets · Observability · Policies · Audit (primitives, every product inherits)
   - **Bindings:** Storage (S3/GCS/R2) · AI providers · via templates: email/payments/auth
   - **Roadmap honesty:** dedicated Search/Vector engines may arrive at v4 as *new services with new names* if customer pull warrants — never as backend swaps of the capabilities. Likewise **Messaging/Streams** (Kafka-class): out of Jobs' scope forever; ladder = Messaging Binding first, managed Streams service only on evidence, own name, own ADR (§2 scope clause).
6. **Long-term consequences:**

| | Model A + front door (chosen) | Model B |
|---|---|---|
| Good | Differentiated semantics preserved (transactional enqueue, branch-coherence, joins); zero new surfaces; coherent "think in your app" story; Atlas-proven evolution path; ADR-0004 boundary intact | Clean-sounding catalog; serves the bare-commodity shopper |
| Bad | The "only a queue, no platform" shopper is deliberately unserved (accepted: dead-category evidence); capability discoverability depends on the catalog/composer doing its job (S-ruling: catalog surfaces capabilities as first-class entries) | Forfeits differentiation to lowest-common-denominator semantics; 3 new product surfaces with zero demand evidence; re-opens ADR-0004; adopts the AWS fragmentation taxonomy; "swap later" is semantically impossible anyway |

## 6 · Ripple (small; nothing frozen moves)

- **ADR-038 (product log, drafted for stamp):** the **State Test** as the canonical Service/Capability/Binding classifier + the **Jobs-scope clause** (database-native background execution only; Messaging/Streams is a separate category that passes the State Test as a Service — Binding first, managed service only on evidence, always its own name) + "catalog sells intents, composer resolves them, dependencies satisfied by composition never homework" + the Atlas seam rule (engine evolution inside a capability, scoped to parent data; a semantic divergence gets a *new* service name, per GOV-002's own v4 plan). Confirms ADR-0004 (A5.2's no-broker strike scoped precisely: it governs the queue, not a future named streaming product); extends the new-managed-product gate in AGENTS.md with the State Test as its first question.
- **Catalog/canvas note (C1/AI1 tasks + frontend pack):** Jobs/Search/Vector appear as catalog intents; the composer's proposal flow per §4. (AI1's spec already covers the mechanics.)
- **`provisioning.md` pack:** add the State Test + never-homework rule to the mistake bank ("classifying by implementation instead of state semantics"; "surfacing a dependency as a prerequisite instead of composing it").
- **No change** to the enum, Architecture v1.2, ADR-0004, GOV-002, or any frozen surface — this challenge *validates* the frozen taxonomy and adds the principle + front door that make it customer-proof.
