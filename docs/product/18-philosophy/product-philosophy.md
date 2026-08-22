# The Steloit Product Philosophy

This is the constitution above the specs. The rest of this package records *what* was decided; this document records *why*, in a form that governs decisions not yet made. The existing specs are its precedent — every rule here is already embodied in shipped frames and invariants, cited where it lives. If a future decision conflicts with this document, that conflict is a finding to surface and resolve, never to paper over.

> **Amended Aug 2026 — positioning v2 (founder-directed; `docs/plan/positioning-v2.md`).** §1 and §3 are rewritten. The
> v1 North Star was *eliminate uncertainty in operating modern infrastructure*, and the v1 external
> promise was *Know before you deploy*. Certainty did not stop being true; it stopped being the
> whole answer. The problem the platform leads with is now the **operating gap** that AI-accelerated
> software creation opened. Certainty is retained, in full, as the capability that makes the new
> promise safe to accept. §§2, 4–8 are unchanged — the grammar, question-ownership, and
> show-your-work rules were never a function of the wedge. Amendment recorded on the record, as
> this document requires of itself.

---

## North Star

> **Steloit exists so that the people who build software can run it in production and keep it there — without becoming infrastructure experts. Every product, feature, API, and interaction should remove infrastructure work from the developer, or prove what the platform did on their behalf.**

The North Star is stated as a *need*, not as a market segment: it does not matter whether the code was written by hand, by a team, or by an AI. What AI changed is the **volume and the urgency** — far more software is now produced than can be operated — which is why §1 leads with it. That is the why-now, not a qualifier on who we serve.

Everything below derives from this sentence, in order. Each layer answers a different audience's question, and each inherits from the one above:

```
North Star            close the operating gap
  ↓
Internal principle    Show your work.          → how the team builds
  ↓                   (What did we stop hiding?)
External promise      Build with AI. Deploy    → what customers gain
                      and run it without
                      becoming an infra expert.
  ↓
Company strategy      Don't break the grammar. → what not to build
  ↓
Information arch.     Every surface owns one   → where things belong
                      primary question.
  ↓
Platform arch.        The grammar is code,     → why it survives growth
                      not memory.
  ↓
Implementation        specs · components · API · tests
```

**When principles conflict.** They will — speed vs. certainty, simplicity vs. proof, grammar vs. innovation. The hierarchy is the tiebreaker: higher layers take precedence over lower ones. We optimize for certainty over convenience, grammar over novelty, ownership over duplication. Most conflicts dissolve rather than resolve — the document's own precedents show the shape: speed vs. certainty became *certainty at the speed of the fastest competitor* (§3); grammar vs. innovation became the primitive test (§4). Look for the synthesis first. If a conflict genuinely cannot be resolved within these principles, the constitution — not the individual feature — is amended, on the record.

---

## 1. Why Steloit exists

**The cost of writing software collapsed. The cost of running it did not.** AI made software creation dramatically faster, and put it in reach of people who never intended to become infrastructure engineers. The result is a widening gap: far more applications get written than get deployed, configured, secured, monitored, scaled, and paid for correctly. That gap is now the bottleneck, and it is the problem Steloit exists to close.

The work on the far side of that gap is the same work it always was — VM sizing, Kubernetes, networking, load balancers, database provisioning, cache configuration, TLS and DNS, secrets, backups, monitoring, scaling rules, and cloud billing. None of it is why anyone built the application. **A developer should not have to understand any of it to run their software in production.** That sentence is the product.

Steloit closes the gap by *understanding the application*: point the platform at a repository, and it works out what the application is, what it needs to run, what that will cost, and how to operate it once it is live. The developer stays in charge — they accept a recommendation and a price — but they are never handed infrastructure homework as the price of admission.

### The uncertainty problem, retained

Closing the gap is only worth anything if the result can be trusted, and infrastructure is where trust goes to die: What will this cost? What is actually running? Why did the deploy fail? Who changed this? Why doesn't the invoice match what I saw yesterday? Can I trust AI with production?

Every platform answers these questions *after* something happens — the invoice arrives, the deploy breaks, the audit is requested. **Steloit answers them before they become problems.** This was the v1 North Star and it is now the discipline that makes the v2 promise safe: the more the platform does on the developer's behalf, the less excusable it is for the platform to be opaque. Automation without evidence is just a different kind of surprise.

Our innovation is rarely new machinery. Competitors already have rollout gates, query plans, retry queues, and pricing formulas — internally, hidden. **Steloit's innovation is understanding the application well enough to make the infrastructure decisions, and then exposing every one of them.** Both halves are required; either alone has been built before.

---

## 2. Internal philosophy — *Show your work*

For engineers and designers. It says **how to build**: every feature exposes its reasoning instead of hiding it. Generalized: **every important decision leaves behind inspectable evidence** — a user can always learn not only *what* happened, but *why*, *when*, and *because of what*. Deployments, pricing, AI, scaling, governance, and observability are not six features with this property; they are six instances of this one rule.

The philosophy has a generative question — the one that turns it from a value into a roadmap:

> **What did we stop hiding?**

Don't ask "what feature should we build?" Ask "what is hidden today that developers deserve to see?" That question produced the rollout timeline, the query plan, the invoice math, the deployment gates, the AI evidence, the retry history, the branch lineage. It will produce the next ten. The chain reads: *Show your work → What did we stop hiding? → what did the platform decide for me, and why?*

| Surface | The work it shows |
|---|---|
| Deployment | rollout gates live, canary vs baseline, auto-abort contract stated in place (DP2) |
| Pricing | the math — per-service estimate lines before anything exists (W2, C1) |
| Invoice | every line expands to the usage rows behind it (B3) |
| AI | evidence → reasoning → proposal, always in that order (W10) |
| Alerts | why it fired — the rule speaks the query language verbatim (O5) |
| Database | branch lineage, per-branch freshness and cost (W5) |
| Scaling | why it scaled — annotated events on the shared timeline (D22) |
| Governance | preview before enforce; dry-run against current state (G9) |

This phrase is not marketing. It never appears as a headline; it appears as behavior.

## 3. External promise — *Build with AI. Deploy and run it without becoming an infrastructure expert.*

For customers. It says **what they gain** — phrased as the user's state, never the vendor's virtue. The promise is falsifiable on contact: either they got their application running without learning Kubernetes, or they didn't.

The promise runs a loop, not a moment:

```
repository → understand the application → detect stack and requirements
           → recommend infrastructure → project the cost
           → [human accepts] → provision → deploy
           → configure (database · cache · networking · TLS · secrets · backups)
           → monitor → scale → operate
```

Every arrow is the platform's work, not the developer's. The one place a human is required is the accept — and that is deliberate, permanent, and the subject of the four laws (§9).

**Certainty is the capability that makes the promise acceptable.** *Know before you deploy* is no longer the headline; it is the reason anyone would let a platform make these decisions. It runs the same lifecycle it always did — know **before** (projection, estimate, dry-run, backtest), **during** (provisioning steps, rollout gates, live canary), **after** (invoice reconciles to the sidebar, audit log, why-it-fired) — and it must visibly not expire at the deploy button.

**Two cost objects, never conflated.** A **projection** is a range derived from reading a repository before any shape exists ("$41–58/mo"); it is labelled projected and never presented as a guarantee. An **estimate** is exact, attaches to an accepted shape, and equals the invoice ("one arithmetic", §9). The only cost guarantee we make is **the cap**: a bound the customer sets and the platform enforces. We do not say "fixed", "flat", or "predictable pricing" — the architecture backs a bound and an exact estimate, and those are what we claim.

Billing is the proof, not the product. Nobody wakes up wanting honest billing; they wake up needing to run an application. The unit of first deployment is therefore **the app, not a service** — a web service, its database, its wiring, one price on the confirm. Certainty must arrive at the speed of the fastest competitor: an estimate that feels like power, never like a checkout interstitial.

## 4. Company strategy — *We don't ship products that break the grammar*

For product leadership. It says **what not to build**.

The moat is not any product — everyone has Postgres, queues, compute. The moat is that every product behaves like a Steloit product: **deployed, estimated, observed, billed, secured, and audited the same way.** One mental model, compounding as the catalog grows.

**The catalog principle (ADR-040).** Developers buy outcomes, not infrastructure. The public catalog sells Products — outcome-named intents — and the Composer resolves each into an execution model, priced, in the estimate the customer accepts. Infrastructure powers the experience and never defines it: no catalog entry exists because the infrastructure exists, though an infrastructure name may be a Product when it is itself the outcome developers seek (PostgreSQL). **Execution models are replaceable; semantic contracts are not** — engines may change invisibly, meanings may not.

**The grammar is the shared language of Steloit.** Concretely, it is:
- the **context model** — Org → Project → Environment, env-as-filter (`02-information-architecture`)
- the **page anatomy** — the service page contract, product overview zones, settings grammar (spec §4.3, Overview grammar)
- the **interaction tiers** — modal / drawer / page, no inline forms (`06-interactions`)
- the **status language** — six marks, one meaning of green, shape + label never color alone (spec §1.2)
- the **pricing arithmetic** — estimate-before-provision, one arithmetic, soft-bills / hard-fails-loudly
- the **API conventions** — problem+json, snake_case, `xxx_` ids, one query language across logs, alerts, ⌘K, CLI (`08-api`)
- the **terminology and microcopy** — the frames' words, verbatim
- the **visual language** — LATTICE tokens, mono-means-copyable, violet-means-AI (`01-design-system`, `17-brand`)

When this document says a product "fits the grammar," it means all eight — not a style match.

The enemy of this strategy is not a competitor; it is Conway's law. Platforms fragment when autonomous teams invent local grammars and nothing with authority stands in the way. So the grammar has an admission test, not a taste test: a new product must compose the existing anatomy (the service page contract, the create-surface skeleton, the C1 card + type block). A new rail *zone* requires a new *primitive*, not merely a new product (the X1 rule). A product that cannot fit the grammar is not ready to ship — even if a competitor ships its equivalent first. Shipping slower than a bolt-on is a cost this strategy accepts on purpose.

## 5. Information architecture — *Every surface owns one primary question*

For designers. It says **where things belong**.

Every page exists to answer a concrete developer question. Nothing exists because competitors have it.

| Surface | Primary question | Secondary answers |
|---|---|---|
| Canvas (C1) | What am I deploying? | what it costs, how it's wired |
| Observe | What happened? | why, what's affected |
| Billing | What am I paying for? | how it compares to the estimate |
| Deploy | What is shipping? | what changed, what gates it |
| Templates | How do I repeat this? | what it will cost this time |
| Audit log | Who decided this? | — |
| Assistant | What should I do? | why you're suggesting it |

Ownership rules:
- A proposed surface must name its primary question, and no existing surface may already own it. If one does, improve the owner instead.
- Secondary answers are welcome but always **link to the owner** — as page content, never as navigation (the single-nav-home rule). The cost cell on a product overview is the pattern: ambient answer, one canonical home.
- The same rule structures the docs: organize by developer question, not by product noun.

## 6. Single source of truth — *show it once*

The complement to question ownership: **every concept has one canonical owner; other surfaces summarize or reference it, never recreate it.** Consistency comes from ownership, not from duplication kept in sync.

Billing owns the ledger; DB4 *watches* spend and links back. Observe owns the incident; Deploy, the topology, and the product overviews all point at the same event ids rather than retelling it. The estimate rail owns the arithmetic; every cost cell elsewhere is a reference to it. When two surfaces seem to need the same information, the resolution is a summary-plus-link on one and canon on the other — never two implementations of the truth.

The corollary for entry points is the spec's own pattern: **many doors, one room.** The create surface has three doors (rail `+`, ⌘K, a product page's New button) and they all land on the same canvas; templates and describe-to-provision are two entrances to the same review. Doors may multiply freely — they carry context, they cost nothing. Rooms may not: a second surface for the same job forks the grammar, splits the audit trail, and doubles the maintenance of truth.

## 7. The product review checklist

Run every proposed feature through these questions. A feature that can't answer them isn't ready.

1. **"…so you don't have to ___?"** or **"…so you know ___?"** — What infrastructure work does this remove, or what does the user know after it that they didn't before? A feature that completes neither sentence is off-thesis. (Note which one it completes: the first is the primary test, the second the certainty test.)
2. **What did we stop hiding?** — Prefer exposing existing machinery over adding new surfaces. Show-the-work features are cheaper and more on-brand than new-dashboard features.
3. **Which question does this own?** — And confirm no existing surface owns it (§5).
4. **Does it pass the grammar?** — Composes the anatomy, uses the interaction tiers, one arithmetic, status language, microcopy discipline (§4).
5. **What do the failure, empty, and limit states say?** — Every one must name a way forward (frame A7 is the bar). No silent limbo.
6. **Does the platform remain whole without it?** — Mandatory for anything AI-adjacent (Law 4); a healthy question for everything else.

## 8. Platform architecture — the grammar is code, not memory

Consistency survives headcount only when breaking it is harder than following it. Guidelines are remembered; primitives are imported.

- **Shared shells are the only way to build their surface class.** The proposal card is the precedent — one component, one platform-wide schema; teams *cannot* ship AI features outside it. Generalize this: the service page shell, the create-surface skeleton, the settings grammar, the switcher menu, the overlay tiers (modal/drawer/page) are each a single implementation that new products fill with slots, never redraw.
- API types are generated from `08-api/openapi.yaml`, never hand-written. Money is integer cents, rendered by `fmtMoney`, in mono.
- Microcopy is copied verbatim from the frames — the words are spec.
- The invariants in `16-qa/qa.md` are automated tests, not review-time suggestions. The audits the design passed are the audits the code must keep passing.

## 9. Things we never do

The constitution. When the team grows, nobody will remember every discussion — they will remember these.

- **We never hide the arithmetic.** One arithmetic everywhere; the estimate speaks the same grammar as the invoice.
- **We never gate safety behind a plan.** TLS, backups, MFA, policies, alerts, dunning protections — on every tier, always.
- **We never let AI act without a human.** No auto-apply path exists in the API. Suggestions end in proposals; humans apply them; applications are audited.
- **We never bill before `ready`,** and we never delete data for billing reasons within 90 days.
- **We never ship a product that breaks the grammar.**
- **We never duplicate ownership of a developer question.**
- **We never build two rooms for one job.** Many doors, one room — entry points may multiply, canonical surfaces may not.
- **We never make the developer think in infrastructure to buy an outcome.** The catalog sells Products; the Composer composes; dependencies are satisfied by composition, never surfaced as homework.
- **We never require infrastructure expertise to run an application.** Anything the platform can work out from the application, it works out — VM sizing, networking, TLS, backups, scaling rules. Where a decision genuinely needs the developer, we ask one question in their vocabulary, never in the cloud's.
- **We never present a projection as a guarantee.** A range is labelled a range; an estimate is exact and binds; only the cap is enforced. "Fixed pricing" is a sentence we cannot back, so it is a sentence we do not write.
- **We never silently change a product's semantics.** Execution models are replaceable; semantic contracts are not — a semantic change ships as a new named variant, consented through its estimate.
- **We never leave a failure state without a way forward.** No silent limbo, no dead ends.
- **We never capture secrets in templates, and we never reveal a token twice.**
- **We never use magic where proof is possible.** If we can show the evidence, the math, or the state, we do — "trust us" is a last resort, and a design smell.

---

## How this document is used

Every artifact in the company inherits from the level above it, never the other way around:

```
        Philosophy      (this document)
           ↓
        Platform        (the grammar: shells, tiers, tokens, one API convention)
           ↓
        Products        (compose the platform; admitted by the primitive test)
           ↓
        Features        (pass the review checklist)
           ↓
        UI              (frames are spec; microcopy verbatim)
           ↓
        Code            (generated types, shared primitives, automated invariants)
```

A decision made at a lower level never silently overrides a higher one. When reality argues that a principle is wrong — and occasionally it will — the principle is amended *here*, on the record, the way the specs record their reversals.

**This document is a constitution, not a wiki.** It should change rarely and stay short — if it exceeds ten pages, it has started absorbing content that belongs in specs. Product specs, API docs, and implementation details evolve constantly; this is the stable foundation every future decision traces back to.

One test to carry out of this document:

> **If a decision makes Steloit harder to understand, harder to predict, or harder to trust — it is probably the wrong decision.**
