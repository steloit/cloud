# Design-partner pitch — Rung 0 (v2, 2026-08-22 · positioning v2 · `docs/plan/positioning-v2.md`)

The pitch for the 30–50 conversations (INF-001 §4). Consumes
17-brand/messaging.md — never authors it. One enemy per telling,
the wedge first, the platform only if they pull. Superlatives banned.
Everything shown is canon or the live composer; nothing is asserted
that can't be demonstrated on screen.

---

## Who this is for

Anyone who has an application they built — increasingly with AI — and
who has to get it running and keep it running. Two shapes, same pitch:

- **The builder.** A solo developer or 2–10 person team shipping
  applications faster than they can operate them. They are competent
  developers; they are not infrastructure engineers, and they should
  not have to become ones.
- **The developer who signs off on production** at a 3–30 person
  product company — senior/staff engineer or the founder who still
  deploys. Inside a team, this is usually the buyer.

Qualifying signals (any one): an application finished weeks before it
was live · time spent on Kubernetes, load balancers, TLS, or scaling
rules that had nothing to do with the product · a cloud bill they
couldn't explain · an incident whose cause took a war room to find.

**Do not disqualify on inexperience.** A builder who says "I don't
really understand any of this" is the audience, stated plainly — not a
weak lead. Disqualify only on *fit*: no application, or no intention to
run one.

## The opener — one enemy, named

Pick the enemy that matches what they've lived (from messaging.md;
never mix):

- **Infrastructure homework** (the default opener): "How long between
  your app working and your app being live? What were you actually
  doing in that gap?"
- **Bill shock:** "Every cloud tells you what you spent. None tells you
  what you're *about* to spend."
- **Silent limbo:** "When your deploy fails at 2am, does the error tell
  you what to do next — or who to wake?"

## The promise, then the wedge — shown, not described

One sentence before the demo, so the demo has a frame (Problem →
Abstraction → Demo — the mechanism is never the message):

> Build it however you like, with whatever AI you like. Steloit runs
> it — **without you becoming an infrastructure expert.**

Then show it, on *their* repository if they'll give you one:

> Point Steloit at a GitHub repo. It reads the application, works out
> what it needs, and shows you the infrastructure and the cost before
> anything exists:
>
> `acme/storefront → web · postgres 2 GB · valkey`
> `projected $41–58/mo · cap $75/mo enforced`
>
> Accept. It provisions, deploys, wires the database, issues TLS,
> turns on backups and monitoring. There is no configuration screen
> in between — walk them through and show them there is nowhere to go
> wrong.

Then the second beat: **the estimate is the workflow, and the cap is
real** — nothing provisions, and nothing bills, before a human accepts
a shown number; the invoice is that number; and a spend bound you set
is *enforced* — at the cap, provisioning pauses, nothing is deleted,
never alerts-only. Demo: the composer, live, unauthenticated — price
their actual stack in front of them, then set a cap and watch an
over-the-bound estimate get refused with the math.

Third beat, only if they pull: **previews.** Open a PR and get a
working preview environment with production-shaped data, masked by
policy, priced on the comment. Real, shipped, and no longer the lead —
it is a capability, not the pitch.

**Two things to say precisely.** The projected number is a *range* and
you say so; the estimate for an accepted shape is exact and binds; the
cap is the only guarantee. And Steloit does not test, score, or certify
their code — it works out what the code *needs*. If they hear "AI code
review", correct it on the spot.

(Implementation vocabulary — branching, snapshots, PITR, CNPG — stays
out of the pitch. If they ask how: "copy-on-write branches under the
hood; you get the masked data and the price." What they should
remember is that they didn't have to configure anything.)

## What the alpha actually is (candid, per D11)

One path, end to end: **connect repo → see the cost → approve →
deployed service + Postgres → open a PR → get the branched preview.**
CLI-first; the full console comes later. Postgres, previews, deploys —
cache, queues, object storage, and the AI layer are roadmap, not alpha.

Be precise about the alpha's edge of the v2 promise: **application
understanding is staged.** The estimate gate, provisioning, deploy, and
the cap are real in the alpha; automatic stack detection and
infrastructure recommendation land against this cohort's repositories,
not before them. Do not demo a capability against a repo it hasn't been
run on — offer to run it on theirs, live, and show the result whatever
it is. That *is* the pitch.

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
numbers — canon or the live composer only · **no "fixed" or
"predictable" pricing** (a projection is a range, an estimate is exact,
only the cap is guaranteed) · **nothing implying we review, test, or
score their code** · no promising alpha scope beyond D11's path,
whatever they ask for (that request is *evidence*, log it; it is not a
commitment).
