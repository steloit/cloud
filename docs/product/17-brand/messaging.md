# Messaging framework

Brand owns positioning, messaging, tone, audiences, proof, and taglines — this page is that ownership, downstream of the constitution (18-philosophy). The marketing site and all campaigns are consumers of this document, never authors of it.

## The hierarchy (settled Jul 2026)

- **North Star (internal):** Steloit exists to eliminate uncertainty in operating modern infrastructure.
- **Design principle (internal, never a headline):** *Show your work.*
- **External promise:** **Know before you deploy.** Phrased as the user's state, not our virtue — "honest cloud" invites skepticism; "you already know what this costs" is falsifiable on contact, which is where we win. The full promise runs the lifecycle: know **before** (estimate, dry-run, backtest) · **during** (gates, canary vs baseline, provisioning steps) · **after** (invoice reconciles to the sidebar, audit, why-it-fired).
- **The test for any copy:** complete "…so you know ___." Can't complete it → off-thesis.

## Audience & enemy

**Primary:** the developer who signs off on production — senior/staff engineers and small platform teams who have been burned. They buy proof, not promises; they read pricing pages like contracts. **The enemy is named, always specific, never a competitor's name:** bill shock · silent limbo · the mystery deploy · the invoice that doesn't match yesterday · the AI you can't audit. Every campaign leads with one enemy and kills it on screen.

## The wedge

**Preview environments with a real database branch per PR — masked by policy, priced per day, stated on the PR comment** ("db: branch of production (masked · policy) · $0.07/day"). Concrete, demoable, priced-in-the-open; nobody does the whole loop. The platform reveals itself after the wedge lands — never lead with "everything platform."

## Proof points (each must be *shown*, not asserted)

1. **The estimate rail is the pricing calculator** — the real create surface, unauthenticated, price updating live. Trust as a demo.
2. **One arithmetic** — $383 + $99 = $482 on every surface; the invoice expands to the meter rows behind it.
3. **The incident story** — deploy #142 → evidence-cited diagnosis → #143 through gates, canary −47%. Six products, zero context switches; run it live.
4. **The four laws** — "no auto-apply path exists in the API" is a sentence competitors cannot say.
5. **One grammar** — two product pages side by side, sameness visually undeniable; the CLI reading identically across products.
6. **Safety is never gated** — the plan table's absences are the argument.

## Rules of engagement

Proof over claims: real UI, canon numbers (19-canon only), no stock imagery, no orbs/padlocks/circuit boards (brand.md). Speed is *demonstrated*, never conceded and never the message — certainty at the speed of the fastest competitor. Superlatives banned even here (voice.md). Sticker slogans are product invariants verbatim: "no silent limbo" · "estimate first" · "calculators, not sales".

## Taglines (parked for a naming pass — not yet decided)

Leading candidates: **"Infrastructure without surprises"** (best acquisition: user-state, names the enemy, falsifiable) · **"The cloud that shows its work"** (best brand line; vendor-voiced) · a first-person "I know" construction (unexplored). Decision criteria on file; do not ship any of these without the naming pass.
