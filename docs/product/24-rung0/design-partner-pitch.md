# Design-partner pitch — Rung 0 (v1, 2026-07-13)

The pitch for the 30–50 conversations (INF-001 §4). Consumes
17-brand/messaging.md — never authors it. One enemy per telling,
the wedge first, the platform only if they pull. Superlatives banned.
Everything shown is canon or the live composer; nothing is asserted
that can't be demonstrated on screen.

---

## Who this is for

The developer who signs off on production at a 3–30 person product
company — senior/staff engineer or the founder who still deploys.
Qualifying scars (any one): a cloud bill they couldn't explain, a
staging environment that drifted from production, PR reviews done
against toy data, an incident whose cause took a war room to find.
People without the scar are not the audience; thank them and move on.

## The opener — one enemy, named

Pick the enemy that matches their scar (from messaging.md; never mix):

- **Bill shock:** "Every cloud tells you what you spent. None tells you
  what you're *about* to spend."
- **The mystery deploy:** "You reviewed the code. Did you review it
  against anything resembling production?"
- **Silent limbo:** "When your deploy fails at 2am, does the error tell
  you what to do next — or who to wake?"

## The wedge — shown, not described

> Open a PR. Steloit gives it a working preview environment with a
> **branch of your production database — masked by policy** — and the
> bot comments the price where the reviewer reads:
>
> `db: branch of production (masked · policy) · $0.07/day`
>
> Merge, and it's gone. Zero footprint. Nobody else does this loop
> whole.

Then the second beat, only after the wedge lands: **the estimate is the
workflow** — nothing provisions, and nothing bills, before you accept a
shown number, and the invoice is that number. Demo: the composer, live,
unauthenticated — price their actual stack in front of them.

## What the alpha actually is (candid, per D11)

One path, end to end: **push repo → see the cost estimate → approve →
deployed service + Postgres → open a PR → get the branched preview.**
CLI-first; the full console comes later. Postgres, previews, deploys —
cache, queues, object storage, and the AI layer are roadmap, not alpha.

**The honesty block (read verbatim, do not soften):**
- Invite-only alpha, small cohort, direct line to the founder.
- Runs in Mumbai (asia-south1) from day one — no region migrations,
  ever (A1.7).
- Platform is 24/7 from the day the first design partner onboards
  (A1.6).
- Durability, printed like everything else: in a disk or zone failure,
  up to [RPO — ≤5 min recommended, **pending ratification A1.3**] of
  the most recent writes may be lost during alpha; full quorum lands
  with general availability.

Saying this out loud is the pitch: a platform whose *sales pitch*
prints its failure modes is the product thesis in one move.

## What design partners get / what we ask

**Get:** shaping rights on the alpha path (weekly 30 min with the
founder) · locked founding pricing when billing turns on · named
credit in the changelog if wanted · the Mumbai cell's first slots.

**Ask (the ladder — always ask for the next rung up):**
1. 45 minutes of their real workflow (the interview).
2. A signed design-partner LOI: named team, named app, intent to run a
   real workload on the alpha.
3. A scheduled onboarding date against the sprint's day-90 target.
4. A pre-order / deposit at founding pricing — the only signal that
   fully counts (INF-001 §4: costly commitments, not politeness).

## What we never say

No "blazingly/seamless/magical" (voice.md, banned) · no "everything
platform" lead (messaging.md — the wedge first, always) · no invented
numbers — canon or the live composer only · no promising alpha scope
beyond D11's path, whatever they ask for (that request is *evidence*,
log it; it is not a commitment).
