# Positioning v2 — infrastructure for the AI-generated software era

**Status:** Founder-directed decision · 2026-08-22 · **applied to the docs; awaiting its entry in the product decision log** (`docs/product/18-philosophy/decisions.md`, human-decision-only) — draft entry in §Findings, item 5. **No ADR number is pre-assigned here:** ADR-042 is taken (INF-001 A6) and 043/044 are mid-ratification in `claudedocs/ratification-proposal-adr-043.md`, so the number is assigned by whoever appends the entry.
**Register:** this is a **product/positioning** decision, so it does not live in `docs/adr/` (engineering ADRs only, per that register's README). It follows the precedent of the other founder-decision records in `docs/plan/` — `product-wedge-review.md`, `developer-first-review.md`, `catalog-structure-review.md`.
**Scope:** supersedes the positioning hierarchy in `17-brand/messaging.md` v1, amends `18-philosophy/product-philosophy.md` §1 and §3, and supersedes `docs/plan/product-wedge-review.md` §7 as the *standing* narrative ruling — that document's evidence and its analysis of branching remain valid and are not withdrawn.
**Trigger:** founder decision. Not a measured architecture trigger — **no architecture, API, or product-surface change is authorised here** (see §Non-consequences).

## Context

Positioning v1 answered "why would a developer trust a cloud?" with *certainty*: the North Star was *eliminate uncertainty in operating modern infrastructure*, the external promise was *Know before you deploy*, and the wedge was a PR preview environment carrying masked production-shaped data at a stated price. That framing was evidence-backed (`product-wedge-review.md`, ~70 primary sources: bill shock ranked as the #1 developer pain) and it produced the estimate gate, the enforced cap, and the one-arithmetic invariants — all of which ship and all of which stand.

What changed is the problem developers are having. AI has made software creation dramatically faster and has put it within reach of people who never intended to become infrastructure engineers. Applications now get *written* far faster than they get deployed, configured, secured, monitored, scaled, and paid for. The binding constraint moved from writing the code to running it.

Against that problem, v1 led with the wrong half. Certainty is what makes a platform *trustworthy*; it is not what makes it *necessary*. And the v1 wedge (previews for code review) sits adjacent to testing — a category we are explicitly not in.

## Decision

**Steloit is AI-native infrastructure.** External promise: *Build with AI. Deploy and run it without becoming an infrastructure expert.*

**The AI framing is our *why now* and our *mechanism* — never a qualifier on the customer.** "AI-native" describes how the platform works: it reads the application and decides. "For software built with AI" was struck from every definitional line (founder challenge, 2026-08-22): the infrastructure does not care how the code was written, the qualifier fences out hand-written codebases with the identical problem, and it dates badly — when all software is built with AI the phrase says nothing. The AI-era bottleneck belongs in the thesis, which explains why this problem became urgent; it does not belong in the definition of what we sell. The promise keeps *"Build with AI"* because it **invites** rather than fences: it is addressed to someone already building that way and excludes nobody.

1. **The message hierarchy is fixed and is not re-ranked in any telling:**

   | Rank | Layer | Content |
   |---|---|---|
   | 1 | **Primary** | Infrastructure. We run the application. |
   | 2 | **Core value** | Deploy and operate without infrastructure complexity. |
   | 3 | **Differentiator** | AI-native understanding and automation of the application's infrastructure. |
   | 4 | **Important capability** | Cost visibility and predictability. |
   | 5 | **Secondary** | Monitoring, scaling, security, deployment automation, previews, backups. |

2. **The canonical loop** — the thing every demo, doc, and campaign walks:
   `repository → understand the application → detect stack and requirements → recommend infrastructure → project the cost → [human accepts] → provision → deploy → configure (database · cache · networking · TLS · secrets · backups) → monitor → scale → operate`.

3. **Certainty is retained in full, demoted in rank.** *Know before you deploy* becomes the promise of capability #4, not of the platform. Every certainty invariant is unchanged and non-negotiable: estimate-before-provision enforced at the API layer, one arithmetic, the enforced cap, problem+json with `remediation`, the audit spine.

4. **Two cost objects, never conflated.** A **projection** is a *range* derived from application understanding before a shape exists (`projected $41–58/mo`); it is labelled projected and is never a guarantee. An **estimate** is exact, attaches to an accepted shape, and equals the invoice. The **cap** is the only cost guarantee we make. **"Fixed", "flat", and "predictable pricing" are banned claims** (`17-brand/voice.md`) — the architecture backs a bound and an exact estimate, so those are what we claim.

5. **We are not a testing, QA, code-quality, or production-readiness product,** and no surface may imply it. *Application understanding* determines what an application **needs to run** — it makes no claim about whether the code is good. This fence is registered in the glossary and is binding on copy.

6. **The preview-with-masked-data loop is demoted from wedge to capability.** It ships, its investment level is unchanged (F14 masking-by-policy stays F-level), and it stays out of the opening of every telling. Branching, snapshots, and PITR remain implementation vocabulary, never positioning — unchanged from the wedge review's ruling #1.

7. **The audience widens and stops excluding.** Primary: developers shipping applications they built with AI, including those who are not infrastructure specialists. Secondary, retained as the in-team buyer: the engineer who signs off on production. The v1 instruction that "people without the scar are not the audience" is **struck**.

## Non-consequences (explicit)

This record changes what we *say*. It authorises no change to what we *build*:

- **Architecture v1.2 stays frozen** (ADR-0001/0003/0004/0005). Product surface remains `[postgres, valkey, web, worker]`; storage and AI remain Bindings; queue remains a Postgres capability. A positioning change is not evidence, and ADR-040 forbids architecture deltas without implementation or customer evidence.
- **The four AI laws are untouched and are now more load-bearing, not less.** "AI-native" describes how infrastructure is *proposed*, never how it is applied: no auto-apply path exists in the API, every recommendation cites its evidence, retrieval stays inside the viewer's RBAC, and the platform remains whole with AI disabled. The accepted estimate remains the contract between the recommendation and what gets provisioned.
- **No new endpoint, component, product, or console surface** is created by this decision. The repo→recommend→price→accept loop is the existing Composer / describe-to-provision path (C1, AI1, GOV-002 §5.1 and §7.2) told in a new order — not new machinery.
- **The roadmap sequence is unchanged.** The D11 alpha path already begins at "push repo → cost estimate → approve → deployed service + Postgres"; this ADR re-weights its telling, not its build order.

## Consequences

- `17-brand/messaging.md` rewritten (positioning owner). `18-philosophy/product-philosophy.md` §1, §3, §7.1, §9 amended on the record. `17-brand/voice.md` gains two banned-claim classes. `18-philosophy/glossary.md` registers the operating gap, infrastructure homework, application understanding, stack detection, recommended infrastructure, and projection.
- `24-rung0/design-partner-pitch.md` and `interview-script.md` re-aimed: the interview now leads with the gap ("how long between working and live, and what were you doing?"), and the screener no longer disqualifies on inexperience. **The interview instrument is now testing a different hypothesis** — v1 interview logs measured the certainty thesis and are not comparable to v2 logs on the gap question.
- Marketing, docs, and campaign copy consume the new hierarchy; none of them may re-rank it.

## Findings raised, not resolved here

Recorded per the task protocol (conflicts are findings, never judgement calls). Each needs a human decision; none blocks the decision.

1. **`GOV-002 §7` says "AI is not the product; AI improves the developer experience."** This ADR names AI-native understanding the *differentiator*. The reconciliation used throughout — AI proposes the infrastructure, a human accepts the estimate, and the platform stays whole with AI disabled — is consistent with all four laws, but `GOV-002` is `00-sources/` and human-decision-only. **Founder ruling needed** on whether §7's sentence is amended or the reconciliation is recorded as the binding reading.
2. **`GOV-002 §10`'s closing line and the console's A1–A4 frames** carry v1's "One project. One platform. Infinite possibilities." It is not off-message and needs no urgent change, but the frame gallery is frozen and any console copy change requires a founder frame ruling. **No console copy was changed.**
3. **"Projection" is a new noun in the pricing grammar.** The glossary registers it as *proposed*, fenced against "estimate". Pricing grammar is owned by frozen authority (`08-api/openapi.yaml`, the estimate rail's canon); if projections ever reach an API response or a console surface, that needs its own engineering ADR and an openapi change, not this record.
4. **`INF-001 D3`'s rationale still calls branching "the category-defining feature"** and "the flagship". That was already stale after the wedge review; it is now doubly so. INF-001 is `00-sources/` — flagged, not edited. The *decision* D3 encodes (branching capability on the substrate) is unaffected either way.
5. **A product-log entry belongs in `docs/product/18-philosophy/decisions.md`** (human-decision-only). Draft entry, for the founder to paste — **assign the next free number at paste time** (042 is taken; 043/044 are pending in `claudedocs/ratification-proposal-adr-043.md`):

   > **ADR-0NN · Positioning v2 — infrastructure for the AI-generated software era.** The problem we lead with moves from *uncertainty in operating infrastructure* to *the operating gap* AI-accelerated software creation opened: applications get written far faster than they get run. Promise: *Build with AI. Deploy and run it without becoming an infrastructure expert.* Hierarchy: infrastructure → abstraction → AI-native understanding → cost visibility → everything else. Certainty and every certainty invariant are retained, demoted in rank. The masked-preview loop demotes from wedge to capability. We are explicitly not a testing/QA/production-readiness product. Alternatives considered: keep v1 (rejected — certainty explains trust, not need); lead with AI (rejected — Law 4, and it makes the platform sound like a feature). *Consequence:* copy tests are now two ("…so you don't have to ___" primary, "…so you know ___" retained); a projection is a range, an estimate binds, only the cap is guaranteed. → docs/plan/positioning-v2.md · 17-brand/messaging.md · 18-philosophy/product-philosophy.md §1, §3

## Refs

`17-brand/messaging.md` · `18-philosophy/product-philosophy.md` §1, §3, §7, §9 · `18-philosophy/glossary.md` · `24-rung0/` · `docs/plan/product-wedge-review.md` (superseded as narrative ruling; evidence retained) · `docs/adr/` ADR-0001/0004/0005 (frozen architecture, unaffected) · ADR-005 / `12-ai/` (four laws, unaffected)
