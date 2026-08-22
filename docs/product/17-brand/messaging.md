# Messaging framework

Brand owns positioning, messaging, tone, audiences, proof, and taglines — this page is that ownership, downstream of the constitution (18-philosophy). The marketing site and all campaigns are consumers of this document, never authors of it.

**Revised Aug 2026 (positioning v2, founder-directed — `docs/plan/positioning-v2.md`).** The v1 framing led with certainty ("Know before you deploy") and wedged on the PR preview with masked data. Both survive as capabilities; neither leads any more. What changed is the problem we say we solve. Prior framing is recorded in `docs/plan/positioning-v2.md` and `docs/plan/product-wedge-review.md` — read them before relitigating.

## The thesis (settled Aug 2026)

> AI made writing software dramatically faster, and made it possible for people who are not
> infrastructure engineers to produce real applications. The bottleneck moved. It is no longer
> writing the code — it is deploying, configuring, scaling, monitoring, securing, and paying for
> what the code needs to run.

Steloit is the infrastructure layer for that world: **AI-native infrastructure.**

**The AI framing is the *why now* and the *mechanism* — never a qualifier on the customer.** "AI-native" is about us: the platform reads the application and works out what it needs. We do **not** define the product as "infrastructure for software built with AI" — the infrastructure does not care how the code was written, that phrase fences out hand-written codebases with the identical problem, and it dates the moment all software is built with AI. The thesis above carries the AI story; definitional lines carry the need. *(Founder ruling, 2026-08-22.)*

## The hierarchy

- **North Star (internal):** Steloit exists so that the people who build software can run it in production and keep it there — without becoming infrastructure experts.
- **Design principle (internal, never a headline):** *Show your work.* Unchanged, and load-bearing: the more the platform decides on the developer's behalf, the more it must be able to prove what it decided and why.
- **External promise:** **Build with AI. Deploy and run it without becoming an infrastructure expert.**
- **The order of the story** — never re-ranked in a telling:
  1. **Primary: infrastructure.** We run the application. That is the product.
  2. **Core value: abstraction.** Deploy and operate without infrastructure complexity.
  3. **Differentiator: AI-native understanding.** The platform reads the application and works out what it needs — it does not hand you a form.
  4. **Important capability: cost visibility and predictability.** Know what it will cost before it exists; set a bound that is actually enforced.
  5. **Secondary capabilities:** monitoring, scaling, security, deployment automation, previews, backups.
- **The two copy tests.** Every sentence completes one of:
  - "…so you don't have to ___." (abstraction — the primary test)
  - "…so you know ___." (certainty — retained for cost, policy, and outcome copy)

  Completing neither means the sentence is off-thesis. Testing, QA, and code quality complete neither: **Steloit is not a code-testing or production-readiness product, and no campaign may imply it is.**

## Audience & enemy

**Primary:** the developer shipping applications they built with AI — solo builders, two-to-ten-person teams, and capable developers who are not infrastructure specialists. They can produce more software than they can operate. They are not disqualified by inexperience; the product exists because the operating half is genuinely hard and they should not have to learn it.

**Secondary (retained, not demoted in value):** the senior engineer or small platform team who signs off on production. They are the buyer inside a team, they read pricing pages like contracts, and they are the reason the certainty proof points still matter.

**The enemy is named, always specific, never a competitor's name:**

- **Infrastructure homework** — the flagship enemy. VM sizing, Kubernetes, load balancers, DNS, SSL, secrets, backups, connection pools, autoscaling rules: work nobody set out to do, standing between a working application and a running one.
- **The gap between "it works" and "it's live"** — the app was finished on Friday and shipped three weeks later.
- **Bill shock** — no cloud tells you what you are *about* to spend.
- **Silent limbo** and **the mystery deploy** — retained; still real, still ours.

Every campaign leads with one enemy and kills it on screen.

## The story hierarchy (every telling walks it in order)

**Problem → Abstraction → Demo → Implementation.**

- **Problem:** the application exists; running it does not. The infrastructure work is the part nobody signed up for, and getting it wrong is expensive in both directions — outages one way, a surprise bill the other.
- **Abstraction (the differentiator):** point Steloit at the repository. It reads the application, works out what it needs, recommends the infrastructure with reasons, prices it, and — once a human accepts — provisions, deploys, wires, and operates it. The developer never sizes a VM.
- **Demo (the wedge):** the repo-to-running loop, shown end to end on one screen. Canonical demo line, at the accept moment:

  ```
  acme/storefront → web · postgres 2 GB · valkey
  projected $41–58/mo · cap $75/mo enforced
  ```

  Then it runs: URL live, TLS issued, database bound, backups on, metrics flowing — with no configuration screen in between. That absence *is* the demo.
- **Implementation (never positioning):** cells, CNPG, snapshots, branching, masking, reconcilers, gVisor. These words belong in docs and changelogs, not in headlines. They are *how*; they are never *why*.

The platform reveals itself after the wedge lands — never lead with "everything platform," and never lead with a mechanism.

## Honesty rules on cost (binding)

Cost is our strongest capability and the easiest place to lie by implication. Three rules:

1. **Projected cost is a range, and is always shown as one.** Before a shape exists, repository analysis yields a *projection* — "$41–58/mo" — and it is labelled projected, never estimated. Ranges are stated with what drives them.
2. **The estimate is exact and is a contract.** Once a shape is accepted, one arithmetic applies: the estimate's line grammar equals the invoice's, to the cent ($383 + $99 = $482). Unchanged.
3. **We never say "fixed", "flat", or "predictable pricing".** The one thing we can guarantee is the **cap**: a bound you set that the platform enforces — at the bound, provisioning pauses and overage stops accruing; nothing is deleted; never alerts-only. Say that, and only that.

## Proof points (each must be *shown*, not asserted)

1. **Repo to running, in one flow** — a real GitHub repository, detected stack, recommended infrastructure with reasons, projected cost, one accept, a live URL. The whole thesis in one demo.
2. **Nothing to configure** — TLS, private networking, backups, and monitoring are on at provisioning time, by default. The proof is the *absence* of setup screens; walk the flow and show there is nowhere to go wrong.
3. **The estimate rail is the pricing calculator** — the real create surface, unauthenticated, price updating live. Trust as a demo.
4. **The cap is real** — set a cap, watch an estimate that would cross it get refused with the math. Competitors' caps are alerts, retrofits, or "coming soon".
5. **One arithmetic** — $383 + $99 = $482 on every surface; the invoice expands to the meter rows behind it.
6. **The four laws** — "no auto-apply path exists in the API" is a sentence competitors cannot say, and it is *more* load-bearing now that AI proposes the infrastructure itself. The platform recommends; a human accepts; the accepted estimate is the contract.
7. **One grammar** — two product pages side by side, sameness visually undeniable; the CLI reading identically across products.
8. **Safety is never gated** — the plan table's absences are the argument.
9. **The incident story** (secondary) — deploy #142 → evidence-cited diagnosis → #143 through gates, canary −47%. Run it live when the room is senior.

**Demoted from flagship, retained as capability:** the PR preview environment with masked production-shaped data. It is real, it ships, and it is excellent — it is no longer the wedge, and no telling opens with it. Masking-by-policy (F14) keeps its investment level; it is now positioned as *safety when a preview carries real data*, not as the differentiator.

## Rules of engagement

Proof over claims: real UI, canon numbers (19-canon only), no stock imagery, no orbs/padlocks/circuit boards (brand.md). Speed is *demonstrated*, never conceded and never the message. Superlatives banned even here (voice.md). Sticker slogans are product invariants verbatim: "no silent limbo" · "estimate first" · "calculators, not sales".

**Two things we do not say, ever:** that we test, score, or certify anyone's code; and that pricing is fixed. The first is a different product; the second is a claim the architecture cannot back.

## Taglines (parked for a naming pass — not yet decided)

Leading candidates under v2: **"Build with AI. Run it anywhere but in your head."** · **"Infrastructure without the homework"** (names the enemy; user-state) · **"Infrastructure that reads your app"** (the mechanism, plainly) · **"Infrastructure without surprises"** (v1's leader; now reads as the cost capability, not the whole promise). *Struck:* "AI-native infrastructure for software built with AI" — the trailing qualifier fences and dates (see §The thesis). Decision criteria on file; do not ship any of these without the naming pass.
