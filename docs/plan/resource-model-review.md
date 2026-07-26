# Resource Model & Hierarchy Review — Adversarial First-Principles Pass

**Status:** Proposed for founder decision · 2026-07-18 · The final architecture review. Same governance: analysis; nothing frozen/human-only changes until ratified.
**Method:** ADR-0003-style attack on org→project→environment→service. One evidence sweep (13-platform hierarchy comparison) + the strategy corpus (ADR-001/002/013, the nine primitives, GOV-002 §1–2).
**Verdict up front:** **✅ Keep the current hierarchy — unchanged.** This is the strongest confirm of any review, because the evidence shows the model is not one option among many but the **convergent target that comparable platforms migrate toward from flat** (Render, AWS, Terraform, Azure all *retrofitted* exactly these layers because flat didn't scale). Steloit has it as a day-one design decision, avoiding the migration pain the others suffered. The core question — "can every current and future managed product fit this hierarchy?" — is **yes**, and the product-family review *strengthened* the fit by shrinking the surface. The review's deliverable is not a change; it's the canonical **ownership matrix** and **inheritance table** (Parts 7–8), consolidated for the first time, and the future-fit proof (Part 12).

---

## The core question — yes

**Every current and future managed product fits org→project→env→service** because the model has exactly three extension points, and every product is one of them (this is the governed-resource contract, ADR-0006, applied to the tree):
1. **A Service** — a provisioned capability in an environment (postgres, valkey, web, worker, and any future managed product).
2. **A Binding** — a managed connection, internal or to an external provider (storage, AI, future integrations).
3. **A capability of an existing Service** — a feature surfaced on a product (queue = pgmq on Postgres; branches/backups tabs).

Nothing in the plausible future needs a fourth kind. Queue, Object Storage, Observability, Feature Flags, Cron, Search (Part 12) each resolve to one of the three without a hierarchy change. **If a proposed thing fits none of the three, that is the signal it is an Org-level concern or doesn't belong** — the exact test the constitution already states (GOV-002 §1.1).

---

## Part 1 · The hierarchy — keep all four levels; add none; remove none

**Org → Project → Environment → Service is the right *depth* — the evidence is one-directional.** A middle grouping layer between the billing boundary (Org) and the running resource (Service) is **universal for multi-service platforms**: Vercel Project, Railway/Render Project, GCP Project, Azure Subscription/RG, K8s Namespace, Terraform Workspace, GitHub Repo. The only platforms without one are **single-primitive** products where the resource *is* the unit (Fly = apps, PlanetScale = a database, Supabase = a backend-in-a-box). Steloit is multi-service → the layer is mandatory. GCP states it as an axiom: "the project resource is the fundamental organizing entity."

**Rejected additions:**
- **Workspace** — already rejected (ADR-001); 95% never need it; the discipline confirmed by evidence is "keep mandatory depth shallow, make deeper nesting *optional*" (GCP folders, AWS OUs, Azure MGs are all optional). Steloit's open slot for enterprise grouping is exactly this pattern.
- **Folder/Team** — not hierarchy layers: Folders/labels are *attributes* of projects (grouping without security/billing semantics); Teams are an *identity* overlay (grouping of users for RBAC), not a resource layer. Both already in the model.
- **Cluster/Region/Tenant/Deployment as top-level layers** — Cluster (cell) is beneath the grammar (D8, invisible); Region is an environment property (ADR-004); Tenant maps to Project (D7); Deployment is a child of Service (primitive 5). None is a hierarchy layer.

## Part 2 · Organization — thin; owns everything that isn't operational

Confirms GOV-002. The Org owns **identity (members/teams/roles), billing (plans/invoices/budgets), policies, org-level secrets, audit, API keys, domains-at-org-level, and the provider credentials for Bindings**. It is deliberately thin — "nothing operational lives here; developers spend 95% of their time inside Projects." **Users own nothing directly** — a user spans orgs; ownership must survive a user leaving, so all resources roll up to the Org via the Project (ADR-002). Storage/AI Binding *credentials* live in org/project Secrets (reusable); the Binding *wiring* is per-env (Part 6).

## Part 3 · Project — the atom; the evidence for why it must exist

**The deepest attack — "could Environment or Service hang directly off Org?" — fails decisively, and the proof is historical.** Platforms that started flat *added* the Project layer because flat broke:
- **Render (the cleanest dev-cloud case):** flat until Projects+Environments GA **2023-07-21**. Their own post-mortem: services were "a long list," "naming was the only way to distinguish… developers appended '-staging'," the platform "didn't have a way for teams to express how their services were related." They retrofitted exactly Project + Environment — *Steloit's model*.
- **AWS:** flat single-account until Organizations GA **2017**. **Terraform Cloud:** flat Org→Workspace until Projects (~2022–23), to tame "workspace sprawl." **Azure:** Management Groups added ~2018 over subscription sprawl. **Supabase:** added branching (2024) to relieve one-project-per-environment heaviness.

**The pattern is unmistakable:** flat scales to a point → teams invent naming conventions (`-staging`) → the platform formalizes the convention into a real layer. **Naming conventions are the smoke; the Project/Environment layer is the fix.** Steloit having it day-one is not over-engineering — it is skipping the migration every flat platform was forced into.

**Why Project specifically (ADR-002, validated):** it is the unit of cost, permission, topology, and deletion — the rollup the certainty wedge depends on ("$383 org = Σ projects"; "what does my app cost / consist of / who can touch it" has no answer without it). App-centric organization (GCP/Vercel/Railway/Render/GitHub) "wins decisively for developer understanding"; the whole flat→grouped migration history is the market moving *from* service-centric *to* app-centric — which is ADR-002's thesis stated as market direction.

## Part 4 · Environment — a filter for topology, a cheap container for state (ADR-013, validated)

**The evidence validates ADR-013's exact synthesis.** The research finds two camps — env-as-container (separate resources per env) vs env-as-filter (same topology, env selects values) — and the principled rule: **the state/data/isolation boundary is a container; the topology is filtered.** ADR-013 is precisely this: *"shape is per-project (the topology/rail doesn't reshuffle on env switch — the filter), presence and scale are per-environment (isolation — the container)."* And the modern collapse of the container-vs-filter cost tradeoff — "previews and DB branches are the cheap copy-on-write container" (Neon/PlanetScale) — **is exactly ADR-0003's branched-preview model.** Steloit's environment is the sophisticated synthesis, not a naive pick.

**Environment owns** (env-scoped values, not topology): **secrets/variables (per-env values), domains (per-env), policies (the env-sensitive narrowing layer — the whole point), and cost visibility.** Previews and DB branches *are* environments (`kind=preview`, ephemeral, branched DB per ADR-0003). **Spend caps** are Org-level (F9 — the org sets a bound); a per-project/per-env budget, if ever wanted, is a **Policy attached at that level** (policies attach at any level) — no hierarchy change, the model already accommodates it.

## Part 5 · Service — the capability; Deployment is a first-class child resource

A **Service** is a provisioned capability inside an environment (postgres, valkey, web, worker — the frozen surface). **Deployment is a resource** (primitive 5: "an immutable release of a *compute* service"), owned by the Service — so web/worker services have a deployment history (multiple deployments, rollback); data services provision, they don't deploy. This is correct and unchanged: Deployment is a child resource of Service, not a peer layer.

## Part 6 · Bindings — edges in the environment topology, source-service-owned

A **Binding** is an edge between a source Service and a target (an internal Service in the same env, or an external provider — storage/AI, ADR-0004). It **belongs to the source Service** (created from it, injects config into it) and **lives in the Environment** (the topology is per-env — prod's web binds prod's postgres). **Not shared across environments** — each env has its own topology (that is the isolation). **Inheritance:** the *credential* (a Secret / provider-config) can be org/project-scoped and reused; the *Binding* (the wiring) is per-env. External Bindings are governed identically (a resource in the tree, ADR-0006). No hierarchy change; Bindings already fit as env-topology edges.

## Part 7 · Canonical ownership matrix

| Resource | Owned by | Notes |
|---|---|---|
| **User** | (global) | Spans orgs; owns nothing directly; is a *member* of orgs |
| **Organization** | — (root) | Identity/billing/policy boundary |
| **Membership / Team / Role** | Organization | The identity overlay; RBAC |
| **Billing / Subscription / Invoice / Budget** | Organization | Money; the spend *cap* is org-level (F9) |
| **API Key (org)** | Organization | Scoped machine credential |
| **Personal Token** | User (used within an Org's scope) | Acts as the user; re-evaluated at use |
| **Policy** | Any level (Org / Project / Env) | Attaches where it applies; inherits (Part 8) |
| **Secret** | Org / Project / Environment | Scoped; closest wins; Binding creds live here |
| **Domain** | Environment (service-attached) | Per-env |
| **Audit / Event** | Organization | One append-only spine; `/audit` is a view |
| **Project** | Organization | The atom; unit of cost/permission/topology/deletion |
| **Environment** | Project | Named isolated instance; filter+container (Part 4) |
| **Service** | Environment | postgres/valkey/web/worker |
| **Deployment** | Service (compute only) | Immutable release; many per service |
| **Binding** | Source Service (lives in Env) | Edge; internal or external target |

**The invariant:** everything rolls up to the Organization *through the Project* — no resource is orphaned when a user leaves; deleting a Project cascades its whole topology; the Org total is the sum of its Projects.

## Part 8 · Inheritance — policies and secrets inherit (closest wins); nothing else does

| Concern | Inherits? | Rule |
|---|---|---|
| **Policies** | **Yes** | Org → Project → Environment; **closest wins**; org sets floors/ceilings; a child may only *narrow* (tighten-only, the AWS-SCP model, ADR-0006), never widen |
| **Secrets** | **Yes** | Org → Project → Env; closest scope wins at resolution |
| **Roles / permissions** | Down the tree | A grant at org level applies beneath it; a grant scoped to a project/env narrows it |
| **Region** | Project → Environment | Env sets the home region (ADR-004); services inherit; overrides are explicit priced exceptions |
| **Service shape / topology** | **No** | Shape is per-project (same across envs — the filter); presence/scale are per-env |
| **Deployments / Bindings / data** | **No** | Env-local; each env has its own |

Inheritance exists *only* where it reduces repetition without hiding truth (policies, secrets, roles, region). Topology and data deliberately do **not** inherit — that is the isolation guarantee.

## Part 9 · API design — nested paths mirror ownership; resources carry stable prefixed ids

Confirms Architecture v1.2 §9 + the x-conventions. **Paths mirror the tree** (`/orgs/{org}/projects/{project}`, `/projects/{project}/envs`, `/envs/{env}/services`, `/services/{service}/...`) — the URL *is* the ownership path, teaching the hierarchy on sight (GOV-002: "the hierarchy is identical across CLI breadcrumb, API path, and Terraform"). **Resources carry immutable prefixed ids** (`org_ prj_ env_ svc_ dep_ bnd_ …`) so a resource is addressable both by its ownership path and by its stable id; `?env=` is the filter for project-scoped lists (env-as-filter). Nested for creation/ownership; id-addressable for direct access. No change.

## Part 10 · Database model — `parent_id` FKs, immutable ids, cascade rules

Confirms models.md. Every table carries a **stable prefixed id** (never reused) and a **parent FK** mirroring the tree: `projects.org_id`, `environments.project_id`, `services.env_id`, `deployments.service_id`, `bindings.source_id` (+ `target_type`/`target_id`/`provider`, ADR-0004), `secrets`(scope: org|project|env), `policies`(org_id, project_id nullable, env nullable). `cell_id` on every resource (invariant 1). Deletion cascades down the tree with the final-backup + typed-confirm guards (F2); `ON DELETE RESTRICT` on bindings (dependents named). No change.

## Part 11 · UI/UX — the hierarchy is learnable and scales

The console already renders it (ADR-011: rail = the project's actual products; env is the crumb pill filter; the Environments page is the M3 parity matrix). It is **teachable** ("your app = a Project; its instances = Environments; its parts = Services") and **scales to thousands of services** because the rail shows only *one project's* products at a time and env is a filter, not a reshuffle (ADR-013) — you never see the whole fleet at once, you see one app. Enterprise customers understand it because it is the same shape as GCP/Azure/AWS-Organizations they already run. No change.

## Part 12 · Future products — all fit without a hierarchy change

| Future thing | Fits as | Hierarchy change? |
|---|---|---|
| Queue | Postgres capability (pgmq) — ADR-0004 | No |
| Object Storage | Storage **Binding** (external) — ADR-0004 | No |
| Observability | Cross-cutting product on the events spine (already primitive 9) | No |
| Feature Flags | A Service, or a capability/config surface | No |
| Cron | A capability of a Worker service (schedules) | No |
| Search | Postgres capability (FTS) first, later a Service | No |
| A brand-new managed product | A **Service** in an environment | No — new matrix rows, same tree |
| A new external integration | A **Binding** target type | No |

**The core question answered concretely:** every plausible future product is a Service, a Binding, or a capability — the three extension points — so none requires touching the hierarchy. This is *why* the model is future-proof: it extends by adding instances of existing primitives, never by adding primitives (ADR-001's discipline).

## Part 13 · Business review

- **Billing:** the Project is the cost rollup; the Org total = Σ Projects (the one-arithmetic wedge). Clean.
- **Support:** "which project / which env" localizes every ticket; the tree is the diagnostic index.
- **Pricing:** plans gate at the Org; usage meters at the Service; nothing about pricing needs a hierarchy change.
- **Enterprise:** the optional-deeper-nesting slot (ADR-001) absorbs enterprise grouping when a deal needs it; the tree matches what enterprises already run (AWS/GCP/Azure).
- **BYOC / self-hosting:** the whole tree is control-plane state that travels unchanged to a BYOC cell (only the data plane relocates, ADR-0005); the hierarchy is what makes "same everywhere" true.
- **Migration:** because Steloit is *not* flat, there is no future flat→grouped migration to suffer (the Render/AWS/TF tax is pre-paid).
- **DX:** app-centric ("think in your app, not in databases") is the market-winning model — it is the wedge's native frame.

## Part 14 · Recommendation — ✅ Keep the current hierarchy (no change)

**Org → Project → Environment → Service, with Deployment/Binding/Secret/Policy/Event as the supporting primitives, is correct and stays.** The evidence is not merely "no reason to change" — it is "this is the convergent target every comparable platform migrated toward, and Steloit already has it." Attacks considered and rejected:
- Remove Project (flat / Org→Service) → **rejected**: the exact model Render/AWS/Terraform abandoned; kills the cost/permission/topology rollup the wedge needs.
- Make Environment a mere filter with no ownership → **rejected**: the isolation/state boundary must be a container (env owns secrets/domains/policies/values); ADR-013's filter-for-topology + container-for-state synthesis is validated.
- Add a Workspace/Folder layer → **rejected** (ADR-001): mandatory depth stays shallow; deeper nesting stays optional.
- Make Deployment a top-level layer → **rejected**: it is a child of Service (primitive 5).

**Not a "small refinement," not a "redesign" — a clean Keep.** I looked hard for a genuine structural refinement and found none; the only additions are documentation (the consolidated ownership matrix + inheritance table above), which is a reference artifact, not a model change.

## Part 15 · Ripple — documentation only; nothing structural

**Migration effort: ~zero (nothing changes).** This review is a confirmation.

- **ADR-037** (product log — drafted for stamp): record that the resource model was adversarially reviewed and *confirmed* (the flat→retrofit evidence; the three-extension-point future-fit; the ownership matrix as canonical). Reaffirms ADR-001/002/013 rather than superseding them.
- **Optional reference:** the ownership matrix (Part 7) + inheritance table (Part 8) may be linked from AGENTS.md's map as the canonical resource-model reference (they consolidate what is scattered across GOV-002). No new context pack needed.
- **No change** to: GOV-002, the nine primitives, ADR-001/002/013, Architecture v1.2, INF-001, the enum, the roadmap, the product surface, or any frozen surface.

**What does NOT change:** everything. The model was right from the constitution; the evidence confirms it is the convergent industry target; the product-family review already tuned the surface to fit it better. This is the foundation the platform is built on, and it holds.
