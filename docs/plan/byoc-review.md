# BYOC Review — Adversarial First-Principles Pass

**Status:** Proposed for founder decision · 2026-07-18 · Same governance as ADR-0003/0004: analysis; nothing touching INF-001, ADR-022, or frozen surfaces changes until ratified.
**Method:** ADR-0003-style attack on Bring-Your-Own-Cloud. One evidence sweep (19-company landscape) + the strategy corpus + the certainty wedge.
**The finding up front:** BYOC's *placement is already correct* — the roadmap has it at v3/Enterprise, and INF-001 already gates it ("GCP-only until a paying enterprise demands otherwise"; "a dedicated cell is the future enterprise SKU"). No BYOC code exists. So the honest verdict is **✅ Enterprise feature, already correctly placed — confirmed, with four sharpenings that make it cheaper and safer.** The value here is not moving BYOC; it's (a) reframing *why* the enabling architecture exists so we don't over-invest in it, and (b) installing a cheaper ladder that absorbs most "BYOC" demand without true BYOC.

---

## The reframe that shapes everything: the residency ladder

The single most important evidence finding: **most "BYOC demand" is not sovereignty demand.** The market answers "keep my data safe" with a ladder of increasingly expensive rungs, and true BYOC sits at the top — most demand is satisfied lower down:

| Rung | Ask | Cheapest honest answer | Steloit today |
|---|---|---|---|
| 1 | "Data in my region/country" (residency) | Region/cell selection **in the vendor's account** (Snowflake, Atlas, Datadog all stop here) | **Regional cells already exist** (A1.7: asia-south1; cells are regional by design) |
| 2 | "Off the public internet" (network isolation) | PrivateLink / VPC peering / PSC into a **managed** data plane (Confluent Dedicated, Neon, Vercel Secure Compute) | v3 "private networking" — a knob on the existing cell |
| 3 | "My encryption keys" (key custody) | BYOK / CMEK, data still in vendor account (Atlas) | Fits the Secrets/KMS model; later |
| 4 | **"Data in an account I own" (true sovereignty)** | **True BYOC** — data plane in the customer's account | v3, deal-gated |

Confluent's CEO put the case against skipping straight to rung 4 bluntly: traditional BYOC is "often the worst of both worlds… to install them you had to grant the vendor full privileges in your account." **The discipline this installs: on every "BYOC" ask, force the qualifying question — is it *region*, *network*, *keys*, or genuinely *my account*? Only the last needs true BYOC, and Steloit's own regional cells + v3 private networking already cover rungs 1–2 cheaply.**

---

## 1 · Should BYOC exist? Yes — eventually, as an enterprise deal-closer

**Who wants it:** mid-market → enterprise with a hard *data-sovereignty mandate* (defense, government, regulated finance/health where "the bytes must be in our account" is a compliance line, not a preference), or an *economic* driver (burning down a committed cloud spend / avoiding egress at scale). **Who never asks:** startups and SMBs — they want zero-ops managed; BYOC is *more* ops for them, the opposite of what they buy Steloit for.

**PMF or enterprise feature:** unambiguously enterprise. It is a **sales-unlock that closes contracts otherwise blocked by compliance**, not a driver of product-market fit. The evidence is categorical (§3).

## 2 · Is BYOC part of the wedge? No — with one preservable tangent

The wedge is **certainty** (cost/policy/outcome before provisioning) for the scarred senior engineer at a 3–30-person company. That persona wants managed and zero-ops; BYOC is irrelevant-to-hostile to them. **Removing BYOC entirely would not weaken the wedge differentiation at all** — the wedge is certainty, not deployment location.

The **one** connection worth preserving: *"start managed, move to your own account later with zero application changes — you're never locked in."* That is a certainty-adjacent, anti-lock-in story. But it is an **enterprise-sales narrative delivered at v3**, not a wedge message — and it is preserved *for free* by the portable architecture we keep for other reasons (§8). Do not put BYOC in wedge messaging.

## 3 · Market research — the category verdict

| Category | BYOC? | Evidence |
|---|---|---|
| **Data / streaming / OLAP** | **Yes** | Redpanda (flagship), WarpStream, Aiven, Databricks (classic compute), CockroachDB (Preview Apr 2026, ~11yr post-founding), ClickHouse (committed-contract only), PlanetScale (Enterprise) — data-gravity + regulatory weight + egress economics |
| **App-hosting PaaS — our closest analog** | **None** | Railway (own metal), Render (SaaS), Fly.io (own fleet), Vercel (SaaS; "Secure Compute" peers into a *Vercel-owned* account), Cloudflare (own edge). **The opinionated uniform runtime *is* the product; BYOC dissolves it.** |
| **Observability** | **Mostly no** | Datadog is SaaS-or-nothing (even FedRAMP is Datadog-hosted); Grafana/Elastic/Temporal split "ship to vendor" vs "self-host the whole stack" — neither is BYOC |

**Two patterns decide this review:**
1. **Timing:** BYOC is a *late enterprise unlock* everywhere except Redpanda/WarpStream — and those two *bet the company on BYOC as their architectural thesis* (diskless/stateless streaming into your VPC). Everyone else added it years post-founding, gated to the top tier / a signed contract. Confluent didn't build it, publicly criticized it, then *acquired* WarpStream.
2. **The nearest analog category (app-hosting PaaS) offers zero BYOC.** That is the most relevant precedent for Steloit and it points one way.

## 4 · Customer analysis — small by count, concentrated in the enterprise tail

By count: single-digit-percent of customers would use *true* BYOC — all enterprise, all with a sovereignty mandate or a cloud-commit economic driver. By revenue: potentially meaningful (enterprise ACV is large). But the demand is **disproportionately AWS** (that's where enterprise accounts and commits live), and **Steloit is GCP-only** — so a GCP-only BYOC serves a small slice of an already-small population. Most of what *reads* as BYOC demand from mid-market is really rung-1/2 residency/networking, which regional cells + private networking absorb.

## 5 · Business impact

- **Revenue:** unlocks enterprise deals blocked by compliance — high ACV — **but only once enterprise pipeline exists (post-PMF, post-revenue).** Zero value before that.
- **Margins:** a control-plane/platform fee on infra the customer pays for (ADR-022's per-cell fee) — pure-software margin in principle, but **eaten by support** in practice.
- **Support burden:** the largest negative. The N-cloud × arbitrary-customer-config debugging matrix; a *degraded* product (ClickHouse BYOC loses SQL console, autoscaling, CMEK); SLAs held only by contractually locking customers out of their own VPC resources (Redpanda). This is why every vendor gates BYOC behind sales — it is white-glove by nature.
- **Enterprise sales / trust / churn:** BYOC is often *the* unlock; it raises trust for the security-conscious and increases lock-in. All real — all enterprise-phase.

**Net: improves the business *later* (enterprise phase); actively *hurts* it if built early** (a 2-founder team's scarcest resource spent on a white-glove enterprise feature with no buyers).

## 6 · Operational impact — managed vs BYOC

BYOC is dramatically harder on every axis: multi-cloud (AWS+GCP+Azure differences vs our GCP-only), debugging inside customer accounts we don't control, cross-account IAM, the customer's networking/CIDR/SCP, DR across accounts, a feature-reduced product, and SLA-by-lockout. Managed keeps all of this in *our* account where we already operate it. For a pre-revenue team the gap is not incremental — it is a different company's operational maturity (Databricks/Redpanda took years).

## 7 · Product experience — managed → BYOC migration is the one real advantage

If the architecture is genuinely portable (D6: the same grammar, API, and provisioning pattern everywhere; only the *cell* relocates), then **migrating from managed to BYOC is a data-plane relocation with zero application changes** — "you're never locked in; your app doesn't change when your infra location does." That *would* be a competitive advantage and it *should* be first-class **when BYOC ships (v3)**. It is the anti-lock-in tangent from §2, and it is preserved *for free* by the portable architecture — which we keep for isolation/regions regardless (§8). Build the narrative at v3; build nothing early.

## 8 · Architecture — the D6 split already answers it (and the free-rider reframe)

**If BYOC exists, only the data plane (the cell) relocates.** Postgres, Web, Workers, Valkey all run *inside* the cell → all support BYOC by construction. Storage/AI Bindings are already external (the customer's provider) → identical everywhere. Billing stays centralized (control plane). The control plane (orgs, IAM, estimate engine, provisioner, billing, the reconciler's desired-state store) stays at Steloit. **The D6 two-plane split already specifies this; there is no new architecture to design.**

**The reframe that matters (and prevents over-investment):** INF-001 partly justifies the day-one shape cost — cells, the reconciler, `cell_id` on every row — as "avoid an enterprise-phase BYOC rewrite." But that architecture is required *anyway* for tenant isolation (D7), regional cells (residency, A1.7), DR, and scale caps (D6). **BYOC is a free rider on architecture we need regardless — not its justification.** The consequence is precise: **carry the portable *shape* (cheap, and we're keeping it), but build zero BYOC-*specific* machinery** — no multi-cloud abstraction, no cross-account IAM, no AWS/Azure drivers — until a signed deal pays for it. This keeps us honest to "cheap on capacity, never on shape": the shape is portable; the capacity (a BYOC cell) stays at zero until demand.

## 9 · Security — the real cost surface, deferred to v3

True BYOC's hard part is that **Steloit's control plane must hold standing privileges inside the customer's account** to reconcile. Doing it *safely* is real security engineering, not config: least-privilege **tag-scoped** roles, **external-ID** confused-deputy protection (the Databricks pattern everyone imitates), customer-granted and auditable, and a **metadata-only control plane** where possible (the WarpStream model: "we cannot read the data in your topics"). The industry-acknowledged risk (per Confluent's CEO) is that naive BYOC *trades* "data leaves my account" for "a third party holds a high-privilege role in my account." This is precisely the kind of surface a 2-founder team should not stand up speculatively — it is a v3, deal-funded, security-reviewed build.

## 10 · Recommendation — ✅ Enterprise feature (placement already correct; four sharpenings)

BYOC is an **enterprise feature**, correctly placed at v3 and demand-gated. Not core strategy (it's off-wedge), not V2 (it's later/enterprise), not remove (the sovereignty-mandated enterprise deal is real and the portable architecture makes BYOC nearly free to *enable* when the time comes). The review **confirms** the existing placement and adds four disciplines:

1. **Free-rider reframe (§8):** the cell/reconciler/`cell_id` architecture is justified by isolation/regions/DR/scale — BYOC rides free. Carry the portable shape; build **zero BYOC-specific machinery** pre-v3.
2. **The residency ladder (top of doc):** absorb rungs 1–2 (residency via regional cells; network isolation via v3 PrivateLink) cheaply; force the qualifying question on every "BYOC" ask; reserve true cross-account BYOC (rung 4) for genuine sovereignty mandates.
3. **Hard demand-gate — now an explicit AND checklist, not a vibe.** The qualitative "when an enterprise asks" is replaced by **five exit criteria (all must hold)** and a **canonical routing decision tree** — both codified in ADR-0005 as standing policy: signed contract *requiring* in-account deployment · the deal funds the fully-loaded first-year build+ops+security-review · sovereignty *provably un-substitutable* by regional cell / dedicated cell / PrivateLink / BYOK · single named cloud (AWS-first, no multi-cloud abstraction until a second signed deal) · security model in funded scope. Failing any one → do not build; route to the cheapest sufficient rung. This is what stops future teams treating every enterprise request as a BYOC request.
4. **Preserve the anti-lock-in narrative (§7)** as a v3 enterprise-sales story ("managed → your account, zero app change"), delivered by the portable architecture we keep anyway. Keep it out of wedge messaging.

**If BYOC is already the correct strategy — it is.** The honest output of this attack is that the constitution got the placement right; what it lacked was the residency ladder and the free-rider discipline, which is what this review adds.

## 11 · Ripple (small — confirmatory + two refinements)

**Migration effort: ~zero code (nothing built); ~1–2 hrs doc-work.**

- **ADR-0005** (new, `docs/adr/`): record the BYOC review — enterprise feature, v3, signed-contract-gated; the residency ladder; the free-rider reframe; AWS-first; anti-lock-in-as-enterprise-narrative; no pre-v3 BYOC machinery.
- **ADR-022 refinement** (product log, human-only — drafted for stamp): distinguish **regional/dedicated cells** (Steloit's account, customer's region — the "cells at Business+" the console already shows, satisfying residency) from **true cross-account BYOC** (Enterprise, contract-gated, sovereignty). Today ADR-022 conflates "cells" and "BYOC"; they are different rungs.
- **INF-001:** *no amendment needed* — it already gates BYOC correctly. Optionally a one-line note pointing to ADR-0005's ladder (offered, not required).
- **Roadmap:** confirm BYOC at v3; add the residency-ladder note (rungs 1–2 are earlier/cheaper); fix the minor wording where BYOC is loosely listed under "Deferred to V1" (it is v3).
- **Positioning/messaging:** add the anti-lock-in line to the *enterprise* narrative (v3), explicitly **not** the wedge.
- **Context pack (`provisioning.md`):** one note — the cell/reconciler architecture is BYOC-*ready* by design but BYOC is demand-gated to v3; do not build cross-account/multi-cloud machinery until a signed deal; run the residency ladder on any BYOC ask.
- **No architecture change, no enum change, no product-surface change.** F13/X2/X3 stay as the v3 spec.

**What does NOT change:** the alpha path, the wedge, the product surface (postgres/valkey/web/worker), Architecture v1.2, the GCP-only pre-revenue posture. This review removes future risk (over-investing in BYOC) and installs a cheaper demand ladder; it does not touch the promise.
