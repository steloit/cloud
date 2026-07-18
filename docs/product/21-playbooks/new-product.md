# Playbook — shipping a new product without breaking the grammar

The constitution says *we don't ship products that break the grammar* (18-philosophy §4). This playbook is how you actually comply — the steps, in order, each citing the authority it instantiates. It exists because the moment of maximum drift risk is a team shipping a product a year from now, under deadline, who never read the design history. GOV-002 §6.2 named the need: "the grammar is documented as an internal spec that every new product must satisfy — the platform's constitution." This is that spec's operating procedure.

**The prime directive (GOV-002 §6.1):** a new service type *ships with Binding support or it doesn't ship*. Everything below assumes yes.

## Step 0 — The admission tests (before any design)

0. **The Developer-First test (ADR-040) — before anything:** what developer problem does this solve? Is it a Product, Capability, Binding, or Infrastructure Service (the State Test, ADR-038, answers)? Which execution model provides the best developer experience? Does it strengthen the certainty story? The State Test decides what may exist; the catalog decides how it is *sold* — as an outcome-named intent (ADR-039). Review order throughout: DX first, implementation simplicity second, architectural elegance third.
1. **Primitive test (X1):** is it a Service — provisioned, holds secrets, emits events, costs money, bound by others? Then it takes a **service slot on the rail**, never a new rail zone. A new *zone* requires a new *primitive* (there are nine; adding one is a deliberate, versioned act — GOV-002 §9.2).
2. **Cardinality:** **fleet** (pointable instances — Postgres, Valkey, Web, Worker) or **capability** (one endpoint, config-shaped — Storage, AI Gateway)? This decides: count badge and instance switcher (fleet, rendered only at n>1) vs. neither, ever (capability). See spec §Extensibility.
3. **GOV-002 §3's three tests:** Project test (makes a Project more capable?), Coherence test (expressible in the nine primitives and standard grammar?), Focus test (serves "default platform for starting a project"?). Fail any → escalate as a *product* question; do not proceed to design.
4. **The review checklist (18-philosophy §7):** what does the user know after this? What did we stop hiding? Which question does it own?

## Step 1 — Identity

- **Glyph:** one metaphor, owned forever (spec §1.4; 20px grid, 1.5px stroke). Add to `01-design-system/icons/sprite.svg` + inventory. The glyph appears identically in rail, ⌘K, topology, CLI help.
- **Name + terminology:** register every new noun in the glossary before it appears in UI. One word per concept; check it doesn't collide with an owned term (the two "templates" needed a fence — don't create a third).

## Step 2 — Create surface (C1 grammar)

A new product is **a new card + type block on the one create surface — never a new page** (C1's revision note is the precedent). The card is an **intent** (outcome-named, ADR-039/040): the Composer resolves it to named resolutions with stated semantics; dependencies are satisfied by composition, never surfaced as homework; the accepted estimate is the contract and stamps the `intent:` tag. Deliverables:
- Product card: one-liner, from-price, availability honesty (beta pill, "not in <region> yet — request it" where true — C7's pattern).
- **Type block** in the shared skeleton `name → type block → pre-wire bindings → included defaults → live estimate → confirm` (C2). Region is inherited from the environment, never chosen per service.
- `/estimates` support **before** provision support: the estimate line for this product must speak the invoice line grammar. Nothing provisions before estimate acceptance; metering starts at `ready`; failed provisioning never bills (C4).
- Deep links: `?add=<product>` and ⌘K "add <product>" land preselected.

## Step 3 — The product's pages (Overview grammar, W4 · S1–S5)

Generate from the template — consistency is structural, not reviewed-in:
1. **Identity header** — glyph, name, status label, metadata chip row, copyable service id; one contextual primary + Docs.
2. **Vitals strip** — the product's four vital signs with deltas + **cost always last** ("$X/mo · of $Y project").
3. **Attention panel** — "Needs attention" (amber) or "Activity" (calm); the label is part of the signal and always truthful. Fresh instance shows the S6 empty state: honest zeros, two bootstrap actions, ambient facts, the promise.
4. **Connect panel** — injected env-var pills, one copyable CLI line, relationship list with role pills, security promise footer.
5. **Act row** — four quick-action tiles; destructive ones in danger treatment with the guard named.

Sidebar sections in the fixed order **Overview/Metrics/Logs → Browse → Manage**, Settings always last. Metrics/Logs reuse the D9/D10 engines (category tabs may grow product-specific panes). Browse tools are the type-tab slot promoted to a section; they run under the caller's **binding role**, writes audited. Settings page follows the settings grammar: blast radius stated before apply; danger zone dependency-blocked, typed-confirm, recovery path.

## Step 4 — Platform integration (nothing optional here)

- **Bindings:** credentials minted at bind, deterministic `<TARGET>_URL` injection, topology edge with role label.
- **Events:** every state change emits to the one pipeline — that's what makes it appear in Observe, markers, the bell, and audit for free.
- **Status:** the six-mark vocabulary only (`provisioning|ready|degraded|failed|suspended|deleting`); color never alone.
- **API:** resources added to `08-api/openapi.yaml` following x-conventions; CLI verbs fall out of the noun-verb grammar (`steloit <product> <verb>`); empty states teach the CLI command.
- **Billing:** meter defined with its price printed in Usage; soft/hard limit behavior declared; scale-to-zero honesty where supported.
- **RBAC:** actions map to existing matrix rows where possible; a genuinely new permission is a matrix change first (with the two-layer evaluation unchanged).
- **AI panel (if any):** inside the one `.prop` proposal component bound to the platform proposal schema — teams *cannot* ship AI features outside it (spec §6). Obeys all four laws; disappears cleanly under the `ai-assistant` policy.

## Step 5 — Canon + QA (the proof)

- Add the product to `19-canon/fixtures.json` — an instance in the canon world with realistic costs that keep every arithmetic invariant green, plus its landing-page "why-two rationale" if fleet (M4–M8's pattern).
- Extend `16-qa/qa.md`: the consistency checklist must pass unmodified; add product-specific canonical scenarios.
- Run the coverage matrix: Create / Overview / Metrics / Logs / Browse / Manage each has a designed home ("grammar" reuse is fine and preferred — mock at full fidelity only what's genuinely new).

## Step 6 — The exit question

Before launch, answer in writing (it becomes the ADR): **what question does this product own, what did we stop hiding, and which grammar rules did it stress?** If it stressed one, that's either a defect in the product's design or a proposed amendment to the grammar — escalated as a grammar question, never resolved locally with a bespoke screen (GOV-005's rule, still true).

---

*Pioneer rule (GOV-005, kept): at any moment exactly one product may be pioneering a new pattern; all others instantiate. Two pioneers at once is how design systems fork.*
