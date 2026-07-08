# The Steloit Product Philosophy

This is the constitution above the specs. The rest of this package records *what* was decided; this document records *why*, in a form that governs decisions not yet made. The existing specs are its precedent — every rule here is already embodied in shipped frames and invariants, cited where it lives. If a future decision conflicts with this document, that conflict is a finding to surface and resolve, never to paper over.

---

## North Star

> **Steloit exists to eliminate uncertainty in operating modern infrastructure. Every product, feature, API, and interaction should help developers know more, guess less, and remain in control.**

Everything below derives from this sentence, in order. Each layer answers a different audience's question, and each inherits from the one above:

```
North Star            eliminate uncertainty
  ↓
Internal principle    Show your work.          → how the team builds
  ↓                   (What did we stop hiding?)
External promise      Know before you deploy.  → what customers gain
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

Developers operate infrastructure under uncertainty. What will this cost? What is actually running? Why did the deploy fail? Where did this request go? Who changed this? Why doesn't the invoice match what I saw yesterday? Can I trust AI with production?

Every platform answers these questions *after* something happens — the invoice arrives, the deploy breaks, the audit is requested. **Steloit answers them before they become problems.** We are not building another cloud platform; we are building the cloud platform that removes uncertainty.

Our innovation is rarely new machinery. Competitors already have rollout gates, query plans, retry queues, and pricing formulas — internally, hidden. **Steloit's innovation is exposing the machinery.** That is defensible because it compounds across the platform and cannot be copied with a feature: matching it requires redesigning a platform's whole relationship with its users.

---

## 2. Internal philosophy — *Show your work*

For engineers and designers. It says **how to build**: every feature exposes its reasoning instead of hiding it. Generalized: **every important decision leaves behind inspectable evidence** — a user can always learn not only *what* happened, but *why*, *when*, and *because of what*. Deployments, pricing, AI, scaling, governance, and observability are not six features with this property; they are six instances of this one rule.

The philosophy has a generative question — the one that turns it from a value into a roadmap:

> **What did we stop hiding?**

Don't ask "what feature should we build?" Ask "what is hidden today that developers deserve to see?" That question produced the rollout timeline, the query plan, the invoice math, the deployment gates, the AI evidence, the retry history, the branch lineage. It will produce the next ten. The chain reads: *Show your work → What did we stop hiding? → Know before you deploy.*

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

## 3. External promise — *Know before you deploy*

For customers. It says **what they gain** — phrased as the user's state ("now I know"), never the vendor's virtue ("we're honest"). Claims of honesty invite skepticism; a state of knowledge is falsifiable on contact with the product, which is where we win.

The before-moment leads the marketing because it is when the buyer decides: estimate before provision, policy dry-runs, alert backtests, template prices at birth. But the promise runs the whole lifecycle — know **before** (estimate, dry-run, backtest), know **during** (provisioning steps, rollout gates, live canary), know **after** (invoice reconciles to the sidebar, audit log, why-it-fired). The promise must visibly not expire at the deploy button.

Billing is the proof, not the product. Nobody wakes up wanting honest billing; they wake up needing to deploy an application. The unit of first deployment is therefore **the app, not a service** — a web service, its database, its wiring, one price on the confirm. Certainty must arrive at the speed of the fastest competitor: an estimate that feels like power, never like a checkout interstitial.

## 4. Company strategy — *We don't ship products that break the grammar*

For product leadership. It says **what not to build**.

The moat is not any product — everyone has Postgres, queues, compute. The moat is that every product behaves like a Steloit product: **deployed, estimated, observed, billed, secured, and audited the same way.** One mental model, compounding as the catalog grows.

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

1. **"…so you know ___?"** — What does the user know after this that they didn't before? If nobody can complete the sentence, the feature is off-thesis.
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
