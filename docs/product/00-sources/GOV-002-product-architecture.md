# Steloit Product Architecture Specification

**Document class:** Internal — Product & Platform Architecture
**Status:** v1.0 — Blueprint
**Source of truth:** *Steloit — The Developer Cloud Platform* (approved vision document)
**Audience:** Product, Engineering, Design, and Platform leadership

---

## 0. How to Read This Document

The vision document answers *why Steloit exists* and *what it should feel like*. This document answers *what Steloit is made of* — the primitives, products, boundaries, workflows, and evolution path that turn the vision into a buildable, coherent platform.

Three rules governed every decision here:

1. **The Project is the atom of the platform.** Every product, page, permission, and price rolls up to a Project. If a proposed feature cannot be explained as something that happens *to* or *inside* a Project, it is either an Organization-level concern or it does not belong in Steloit.
2. **One mental model, everywhere.** A developer who learns how PostgreSQL works on Steloit has implicitly learned how Valkey, Storage, Queues, and Compute work. Consistency is a feature we design, not an accident we hope for.
3. **Fewer primitives, more leverage.** Every abstraction in this document must earn its existence. When two concepts could be merged, they were merged. When a concept could be deferred, it was deferred.

The document is organized as: **Primitives → Products → Boundaries → Information Architecture → Lifecycle → Cross-Product Experience → Intelligence → Evolution → Long-Term Strategy.**

---

# 1. Core Platform Primitives

Steloit is built on **nine primitives**. Everything a developer touches — in the Console, CLI, SDKs, API, or Terraform — is one of these nine things or a view over them. New products must be expressible in these primitives; if they aren't, the primitive set is revisited deliberately, not extended casually.

| # | Primitive | One-line definition |
|---|-----------|--------------------|
| 1 | **Organization** | The boundary of identity, billing, and policy |
| 2 | **Project** | An application; the primary resource of the platform |
| 3 | **Environment** | A named, isolated instance of a project (production, staging, preview) |
| 4 | **Service** | A provisioned capability inside an environment (a Postgres database, a cache, a web app) |
| 5 | **Deployment** | An immutable release of a compute service |
| 6 | **Binding** | A managed connection between two services (credentials, network, config) |
| 7 | **Secret** | A versioned, scoped, encrypted value |
| 8 | **Policy** | A declarative rule attached to any level of the hierarchy (access, cost, retention, network) |
| 9 | **Event** | Anything that happened: audit entries, alerts, lifecycle changes, notifications |

## 1.1 Why exactly these nine

**Organization exists** because someone has to own things, pay for things, and set rules. It is deliberately thin: identity, membership, billing, and policy — nothing operational lives here. Developers should spend 95% of their time inside Projects, not org administration.

**Project exists** because it is the vision's core bet: infrastructure organized around *applications*, not around service types. Competing platforms make you think in databases, buckets, and instances; Steloit makes you think in "my e-commerce app." The Project is the unit of provisioning, deployment, cost, observability, and eventual deletion. It is the noun in every sentence a developer says about Steloit.

**Environment exists** because "my app" is never one thing — it is production, staging, and the branch you're testing. Without Environments as a first-class primitive, developers recreate them badly with naming conventions (`ecommerce-prod`, `ecommerce-staging`) and lose the platform's ability to reason about promotion, cost separation, data isolation, and preview lifecycles. Environments are cheap to create, isolated by default, and share the project's *shape* (which services exist) while differing in *scale* (how big they run).

**Service exists** as the general term for anything provisioned inside an environment. A Postgres database, a Valkey cache, a bucket, a queue, and a web application are all Services with the same lifecycle verbs (create, configure, scale, back up, destroy) and the same page anatomy in the Console. This is the abstraction that makes the platform learn-once: the *category* differs, the *grammar* doesn't.

**Deployment exists** — separately from Service — because compute has a property data services don't: it changes with every release. A Deployment is an immutable artifact + configuration snapshot. Rollback, canary, promotion, and "what exactly is running right now" all fall out of making deployments first-class rather than mutating services in place.

**Binding exists** because "connecting service A to service B" is the single most repeated, most error-prone act in application infrastructure. On other platforms it means copying connection strings into env vars by hand. On Steloit, a Binding is a managed object: it provisions least-privilege credentials, opens the network path, injects configuration into the consumer, rotates credentials on schedule, and is visible on both services' pages. Bindings are how "everything feels like one platform" becomes mechanically true instead of aspirational.

**Secret exists** as its own primitive (not merely a feature of services) because secrets outlive and cross-cut services: third-party API keys, signing keys, webhook secrets. They are versioned, scoped to org/project/environment, and injected via the same mechanism Bindings use — so developers have exactly one way configuration reaches running code.

**Policy exists** so that governance never becomes a parallel product. RBAC rules, spend limits, backup retention, network restrictions, and region constraints are all Policies attached at org, project, or environment level, evaluated with clear inheritance (closest wins, org can set floors/ceilings). Enterprises get compliance; individual developers never see a policy unless one applies to them.

**Event exists** because observability, auditing, alerting, and notifications are all views over the same underlying truth: things that happened. Modeling them as one primitive with one pipeline prevents Steloit from growing four inconsistent history systems.

## 1.2 The deliberate non-primitive: Workspace

The task of a "workspace model" was considered and **rejected as a separate primitive**. Introducing Workspaces between Organizations and Projects (as some platforms do) adds a level of hierarchy that 95% of customers never need and that confuses the core story ("create a project" becomes "create a workspace, then a project"). Instead:

- **Small teams:** Organization → Projects. Flat, simple.
- **Large organizations:** Projects support **labels** and **folders** (a purely organizational grouping with no security or billing semantics of its own), and Policies + Teams provide the actual governance. A "workspace" for the Payments division is just a folder plus a team plus policies — composable from existing primitives rather than a new one.

If, at enterprise scale, folders prove insufficient (e.g., delegated administration demands a hard boundary), a Workspace primitive can be introduced *between* Organization and Project without breaking any existing model — the hierarchy was designed to leave that slot open. This is the correct order: earn the complexity, don't presuppose it.

---

# 2. The Resource Hierarchy

```
Organization
├── Members, Teams, Roles                (identity)
├── Billing (plans, invoices, budgets)   (money)
├── Policies                             (governance)
├── Org-level Secrets                    (shared credentials)
├── Audit Log                            (events)
│
└── Project  ("ecommerce")
    ├── Project settings, labels, folder
    ├── Project-level Secrets
    ├── Project Policies (inherit + override within org limits)
    │
    └── Environment  ("production", "staging", "preview/pr-142")
        ├── Private network (implicit, per-environment)
        ├── Environment Secrets
        │
        └── Services
            ├── Data:     PostgreSQL │ Valkey │ Object Storage │ Queue │ (later: Vector, Search)
            ├── Compute:  Web App │ API │ Worker │ Cron Job     (later)
            ├── Bindings between services
            └── Per-service: config, metrics, logs, backups, deployments (compute only)
```

## 2.1 Rules of the hierarchy

1. **Identity and money live at the Organization. Everything operational lives at or below the Project.** A developer never needs to leave a project to do their job; an admin never needs to enter one to do theirs.
2. **Every Environment is isolated by default.** Separate credentials, separate private network, separate data. Nothing in staging can reach production unless a Binding explicitly crosses (and crossing environments requires elevated permission — this is a Policy default).
3. **Services never exist outside an Environment.** There is no such thing as a floating database. This makes cost attribution, cleanup, and mental modeling trivial: delete the environment, everything in it goes; delete the project, all environments go (with safeguards — see Lifecycle).
4. **Configuration flows downward, never sideways.** Org secrets are visible to projects that are granted them; project secrets flow to environments; environment secrets flow to services via Bindings/injection. There is exactly one direction and one mechanism.
5. **The hierarchy is identical across all surfaces.** `steloit --project ecommerce --env production db list`, the Console breadcrumb `Acme / ecommerce / production / db-main`, the API path `/orgs/acme/projects/ecommerce/envs/production/services/db-main`, and the Terraform resource tree all express the same shape. Learning one surface is learning all of them.

## 2.2 The infrastructure model beneath the hierarchy

Developers see Projects; the platform runs **Cells**. A Cell is a regional unit of Steloit-operated infrastructure (a Kubernetes-based control + data plane in one region of one underlying provider). This internal concept matters to product architecture for three reasons:

- **Region is an Environment property, not a Service property.** All services in an environment live in the same region by default, which makes intra-environment latency, private networking, and data residency simple and predictable. Multi-region is a later, deliberate feature (v3+), not an accidental capability.
- **Steloit Cloud and Steloit BYOC are the same architecture.** A BYOC deployment is a Cell whose data plane runs in the customer's AWS/GCP/Azure account while the Steloit control plane remains the management brain. Because the product model (Projects, Environments, Services) is defined *above* the Cell layer, the developer experience is byte-identical across deployment models — which is precisely the vision's promise.
- **Self-Hosted (future) is a Cell whose control plane also moves to the customer.** The architecture pre-commits to this separation now so the enterprise phase doesn't require a rewrite.

## 2.3 The networking model

Networking is designed to be **invisible until needed, explicit when it matters**:

- Every Environment gets an implicit **private network**. Services within an environment can reach each other only through Bindings; a database with no bindings is reachable by nothing.
- **Nothing is public by default** except compute services that explicitly expose an HTTP endpoint (web apps, APIs), which receive a `*.steloit.app` URL plus custom domain support with managed TLS.
- Data services (Postgres, Valkey, Storage, Queues) are private-only by default. Public access to a database is an explicit, policy-gated, loudly-labeled toggle — supported because developers sometimes need it, discouraged because they usually shouldn't.
- **Cross-environment and cross-project connectivity** exists only as an explicit Binding type ("shared services" — e.g., one analytics database consumed by three projects), requiring permissions on both sides.
- Enterprise **private networking** (VPC peering, PrivateLink-style connectivity, IP allowlists) arrives with BYOC and is expressed as Policies + network attachments on Environments — the same primitives, richer options.

## 2.4 The security model

Security is structural, not bolted on:

- **Identity:** humans (org members, via email or SSO), machines (API keys and service tokens, scoped to org/project/environment with expiry), and services themselves (every service has an identity used for Bindings — the foundation for a zero-trust internal model).
- **RBAC:** a small set of comprehensible roles — Org: *Owner, Admin, Billing, Member*; Project: *Admin, Developer, Viewer* — assignable to users or Teams, at org, folder, or project scope. Custom roles are an enterprise feature (v3), not a v1 complexity.
- **Least privilege by construction:** Bindings mint scoped credentials (a worker bound to Postgres gets a role limited to what it declared it needs), and rotation is automatic. The secure path is the default path and also the *easy* path — the only reliable security strategy.
- **Audit:** every state-changing action, by human or machine or AI assistant, is an Event in the immutable org audit log, from day one — not retrofitted for enterprise later.
- **Secrets:** encrypted at rest per-organization, versioned, injected at runtime (never baked into images), access-logged.

---
# 3. The Product Ecosystem

The ecosystem is organized into **six product families**, each existing for a distinct reason. Within each family, individual products were accepted or rejected against three tests:

- **The Project test:** does it make a Project more capable, or is it a detour?
- **The Coherence test:** can it use the nine primitives and the standard service grammar, or does it demand its own mental model?
- **The Focus test:** does building it serve "default platform for starting a new project," or does it serve a different company's mission?

## 3.1 Family A — Data (the foundation)

Data services are the beachhead because they are the stickiest, most-dreaded-to-assemble part of any stack, and because they deliver value *before* Steloit hosts any compute — a developer on Vercel can adopt Steloit for their entire data layer on day one.

| Product | Why it exists | Notes |
|---|---|---|
| **Managed PostgreSQL** | The default database of modern development; the anchor product. HA, PITR, read replicas, automated backups. | **Branching and cloning are the signature features** — a database branch per preview environment turns Environments from a convenience into a superpower. |
| **Managed Valkey** | Sessions, caching, rate limiting, pub/sub — nearly every app needs it within weeks. Open governance (vs. Redis licensing) aligns with developer trust. | Ships as cache-first defaults; durable/streams modes are explicit configurations. |
| **Object Storage** | File uploads are universal. S3-compatible API means zero SDK lock-in and instant ecosystem compatibility. | Signed URLs, lifecycle policies, versioning, CDN-backed public buckets. |
| **Queue** | The fourth leg of the standard backend. Explicitly a *simple, reliable job/message queue* (at-least-once, DLQs, delays, schedules) — **not Kafka**. | Deep Binding integration: a Worker bound to a Queue auto-receives consumer config. |
| **Vector Database** *(later)* | AI applications are a declared Phase-1 audience; embeddings storage is becoming as standard as caching. | Ships first as **managed pgvector inside PostgreSQL** (zero new product surface), graduates to a dedicated service only when scale demands it. This sequencing is deliberate: meet the need without expanding the ecosystem prematurely. |
| **Search** *(later)* | Full-text/faceted search is a common need Postgres eventually outgrows. | Same pattern: Postgres FTS first, dedicated service when justified by real demand. |
| **Analytics Database** *(later)* | Columnar/OLAP for product analytics and event data. | v4 territory; premature before compute and observability mature. |

## 3.2 Family B — Compute (the completion)

Compute exists because the vision is "build, deploy, and operate" — without it, Steloit is a better Neon-plus-Upstash, not a developer cloud. It ships *after* data deliberately: data services generate trust and revenue while the harder compute problem is built properly.

| Product | Why it exists | Notes |
|---|---|---|
| **Web Services / APIs** | The core deployable: containerized HTTP services from a Dockerfile, buildpack, or Git repo. HTTPS, custom domains, autoscaling, zero-downtime deploys. | One product, not two — "web app" vs "API" is a routing detail, not a product boundary. |
| **Workers** | Long-running background processors; the natural consumer of Queues. | Identical deployment model to web services, minus the HTTP endpoint. |
| **Cron Jobs** | Scheduled execution; every real application needs it. | A scheduling wrapper over the same container runtime — near-zero marginal product surface. |
| **Functions** *(later, v4)* | Event-driven, scale-to-zero handlers for glue code and webhooks. | Deferred because functions demand a distinct runtime, cold-start engineering, and DX investment; containers cover 90% of early needs. |
| **Edge Runtime** *(later, v4+)* | Global low-latency execution. | Deferred hardest: it requires global infrastructure that conflicts with the single-region Cell model until multi-region matures. Building it early would be building CDN infrastructure instead of a developer platform. |

**Rejected: raw VMs / generic Kubernetes-as-a-product.** Exposing VMs or raw K8s clusters would import exactly the operational complexity Steloit exists to abolish. Kubernetes is an internal implementation detail (Cells), never a product surface. Customers who need raw infrastructure are AWS customers; that is fine.

## 3.3 Family C — Operations (the promise)

The vision states every service includes monitoring, logs, alerts, and backups "with no additional setup required." Architecturally, this means Operations is **one product with several views, not four products**:

| Product | Why it exists | Notes |
|---|---|---|
| **Observability** (metrics + logs + alerts, unified) | Fragmented observability is half of why multi-vendor stacks hurt. Every service emits metrics and logs into one pipeline (the Event primitive); the Console shows them *in context* on each service's page and *correlated* at the environment level. | Deliberately one product: separate "Monitoring," "Logging," and "Alerting" products would recreate the Datadog-shaped fragmentation Steloit criticizes. Alerts are rules over the same streams, with sensible defaults pre-installed per service type. |
| **Backup & Restore** | Data safety is table stakes and a top-3 anxiety for anyone leaving a big cloud. Automatic for every stateful service; restore is a first-class, *rehearsable* workflow (restore-to-new-branch). | Backup policies are Policies — set retention at the org level once, applies everywhere. |
| **Usage & Cost Analytics** | Cost transparency is a pillar of the pricing philosophy. Real-time cost per service, per environment, per project; budgets and spend alerts are Policies. | Cost is shown *at provisioning time* (as in the vision's project-creation flow) and *continuously* after. |
| **Distributed Tracing** *(later, v4)* | Valuable, but a deep specialty. | OpenTelemetry-compatible export ships early (integrate with Honeycomb/Grafana); native tracing waits until compute adoption justifies it. Shipping a mediocre APM would violate the coherence bar. |

## 3.4 Family D — Security & Governance

| Product | Why it exists | Notes |
|---|---|---|
| **IAM** (members, teams, roles, API keys) | Someone must control who does what. | Ships at v1 in simple form; custom roles, SCIM, SSO at v3. |
| **Secrets Manager** | One place, one injection mechanism for all configuration credentials. | A primitive exposed as a product; described in §1 and §2.4. |
| **Audit Log** | Trust and compliance. | Built on Events from day one; export and retention policies at enterprise tier. |
| **Private Networking** *(v3)* | Enterprise connectivity: peering, allowlists, private endpoints. | Arrives with BYOC, where it is genuinely needed. |

**Rejected as a v1–v2 product: customer-facing Identity/Auth (user login for *their* apps).** It appears in the vision's Phase 6 and stays there. Auth-as-a-service is an enormous, security-critical product category (Auth0, Clerk) with unforgiving stakes. Building it before the platform's core is mature would be a focus-destroying detour. Until v4+, Steloit integrates: templates ship with Clerk/Auth.js/Keycloak patterns pre-wired.

## 3.5 Family E — Developer Experience (the multiplier)

These are not features around the platform; they *are* the platform's interface, and they are products with owners, roadmaps, and quality bars.

| Product | Why it exists | Notes |
|---|---|---|
| **REST API** | Everything is API-first — the API is the platform; Console and CLI are its first two clients. | Versioned, OpenAPI-specified, no Console-only capabilities, ever. This is enforced by building Console and CLI on the public API. |
| **CLI** | The vision names it a first-class product; it is where developers live. | Grammar mirrors the hierarchy: `steloit <noun> <verb>` with `--project/--env` context. Includes `steloit init` (bind a repo to a project) and `steloit dev` (run locally against real or local-emulated services). |
| **Console** | The visual surface: exploration, observability, administration, and the guided provisioning flow from the vision. | Detailed in §5 (Information Architecture). |
| **SDKs** (TypeScript, Python, Go first; Java, Rust, C# following) | Programmatic control + runtime helpers (typed access to bindings and secrets). | Generated from the OpenAPI spec to guarantee parity; hand-polished ergonomics on top. |
| **Terraform / Pulumi providers** | Teams standardize on IaC; meeting them there removes an adoption veto. | Providers map 1:1 to the primitives — another payoff of a small primitive set. |
| **Templates** | The fastest route from zero to deployed. `steloit init --template saas-starter` creates a project with services, bindings, and example code pre-wired. | First-party and curated community templates. Templates are the delivery vehicle for best practices and, later, the seed of any marketplace. |
| **Local development** | The gap most platforms ignore. `steloit dev` provides local containers or tunneled connections to real dev environments, with identical config injection. | Parity between local and deployed is a coherence requirement, not a nice-to-have. |

**Rejected for now: a third-party Marketplace.** A marketplace before the platform has gravity is an empty mall — it adds surface area, review burden, and quality risk with no demand behind it. The Template gallery is the v1 answer; a real marketplace (partner services provisionable as Bindings) is a v4+ decision to be made from strength.

## 3.6 Family F — Platform Intelligence

The AI Assistant is a **cross-cutting layer, not a destination product** — detailed fully in §7. It appears here to fix its architectural position: intelligence lives *inside* every product's surface (a panel on the Postgres page, a flag on the CLI, an explanation on an alert), plus one conversational entry point. There is no "AI tab."

## 3.7 What Steloit will never build

Stated now to prevent a thousand future debates:

- **A general-purpose IaaS.** No raw VMs, no exposed Kubernetes, no "EC2 but ours." The moment Steloit sells undifferentiated infrastructure, it competes on price with hyperscalers and loses its reason to exist.
- **A CI/CD product.** Git hosting and CI are solved, beloved, and deeply entrenched (GitHub, GitLab). Steloit integrates natively — Git-push deploys, PR preview environments, first-party GitHub Actions — and the deploy step is Steloit's; the pipeline is not.
- **Email/SMS sending, payments, DNS registration, website builders, ML model training.** Each is a real company's whole business. Templates and Bindings integrate best-of-breed providers instead.
- **A low-code/no-code app builder.** Steloit's user is a developer. Diluting that focus dilutes everything.

## 3.8 Boundary summary

| **Build (core)** | **Integrate (native, first-class)** | **Never build** |
|---|---|---|
| Postgres, Valkey, Storage, Queue | Git providers & CI (GitHub, GitLab) | Raw VMs / exposed K8s |
| Container compute, workers, cron | Error tracking & APM early (Sentry, OTel exporters) | CI/CD pipelines |
| Unified observability, backups | External IdPs via SSO (Okta, Entra) | Email/SMS/payments infrastructure |
| IAM, secrets, audit, networking | Payment/email/SMS providers via templates | DNS registrar, website builder |
| Console, CLI, SDKs, API, IaC, templates | Data ecosystems (Fivetran, dbt) via standard protocols | ML training infrastructure |
| AI assistance layer | Customer-app auth (Clerk etc.) until v4+ | Low-code builders |

The integration philosophy: **integrate where ecosystems already have gravity; build where fragmentation is the pain.** Databases are fragmented across vendors — build. Git has a center of gravity — integrate. This single heuristic explains every row above.

---
# 4. Information Architecture

The IA has one job: make the Project-centric model *visible and navigable*, so the product's structure teaches its philosophy.

## 4.1 Primary and secondary objects

- **Primary objects** (things with their own pages, URLs, and lifecycle): Organizations, Projects, Environments, Services, Deployments.
- **Secondary objects** (things that live on a primary object's page): Bindings, Secrets, Backups, Alerts, Domains, Policies, API keys, audit entries, invoices.

The rule: primary objects appear in navigation and search as destinations; secondary objects appear as tabs, panels, and rows *in context*. Secrets never get a top-level nav item at the project level — they are a tab of the environment, because that's where they apply. Context beats catalog.

## 4.2 Navigation model

```
┌────────────────────────────────────────────────────────────────────┐
│  [Org: Acme ▾]   [Project: ecommerce ▾]   [Env: production ▾]      │  ← context bar
│                                          🔍 ⌘K    🔔    👤        │
├──────────────┬─────────────────────────────────────────────────────┤
│ PROJECT      │                                                     │
│  Overview    │   The canvas: everything shown is scoped to the     │
│  Services    │   selected Org / Project / Environment context.     │
│  Deployments │                                                     │
│  Observe     │   Switching environment never changes *where*       │
│  Data        │   you are — only *which instance* you're seeing.    │
│  Settings    │                                                     │
├──────────────┤                                                     │
│ ORGANIZATION │                                                     │
│  Projects    │                                                     │
│  Members     │                                                     │
│  Billing     │                                                     │
│  Audit       │                                                     │
│  Policies    │                                                     │
└──────────────┴─────────────────────────────────────────────────────┘
```

**The context bar is the IA's spine.** Org → Project → Environment is always visible, always switchable, and always the answer to "what am I looking at?" It maps exactly to the CLI's `--project/--env` flags and the API's URL structure — the three surfaces are the same tree wearing different clothes.

**Environment switching is a filter, not a navigation.** You stay on the Postgres metrics page and flip production → staging; the page re-renders for the other environment. This one interaction decision makes environment parity effortless to inspect and drift obvious.

## 4.3 The page hierarchy

**Home = the Projects list** (per organization). Cards show health, monthly cost, active environments, and last deployment. For a new user, home is the guided project-creation flow from the vision — the product's front door *is* its core concept.

**Project Overview** answers three questions in ten seconds: *Is my app healthy? What is it made of? What changed recently?* — a service topology map (services as nodes, Bindings as edges), current status, cost this month, recent deployments and events. The topology map matters: it is the visual proof of the "one platform" promise, impossible to render on any multi-vendor stack.

**Every Service page shares one anatomy**, regardless of type:

| Tab | Content |
|---|---|
| Overview | Status, key metrics sparklines, connection info, bindings |
| Metrics | Full dashboards (type-specific, layout-consistent) |
| Logs | Live tail + search, same query grammar everywhere |
| *Type tab(s)* | Postgres: Branches, Query insights · Storage: Buckets/objects · Queue: Messages/DLQ · Compute: Deployments |
| Backups | Snapshots, PITR, restore actions (stateful services) |
| Settings | Config, scaling, network exposure, danger zone |

A developer who has used one service page has used them all. This anatomy is a design-system contract, enforced in review: new service types must fit it or justify an exception in writing.

**Observe (environment level)** is the correlated view: metrics, logs, and alerts across all services in one timeline. Its defining interaction is **pivot, don't switch tools** — from an alert to the culprit service's metrics to its logs at that timestamp to the deployment that preceded it, in four clicks, without ever leaving the environment's context.

## 4.4 Global search (⌘K)

One search across primary objects, secondary objects, documentation, and actions:

- Type a service name → jump to it (fuzzy, scoped to current context first, then global).
- Type a verb → command palette: "create database," "roll back api," "invite member."
- Type a question → the AI Assistant answers with platform context (§7).

Search is the escape hatch that keeps deep hierarchy shallow: nothing on the platform is more than one ⌘K away.

## 4.5 Notifications

Notifications are a **view over the Event primitive**, filtered by relevance and routed by preference:

- **In-product inbox** (🔔): deploy results, alerts, backup failures, budget warnings, mentions.
- **Channels:** email, Slack, and webhooks — configured once per project or org, not per service.
- **Severity discipline:** alerts page; digests inform. The platform is aggressively quiet by default — notification fatigue is a product failure, and every default notification must justify its interruption.

## 4.6 Settings model

Settings appear at exactly three levels, matching the hierarchy: **Organization** (identity, billing, org policies, SSO), **Project** (name, folder, members overrides, project policies, integrations like GitHub), **Environment/Service** (config, scaling, exposure). The test for where a setting lives: *at what level would two reasonable teammates expect it to differ?* Billing can't differ per project → org. Scaling differs per environment → environment. No setting appears at two levels.

---

# 5. The Project Lifecycle

The lifecycle below is the platform's core loop. Every stage is available in Console, CLI, and API with identical semantics.

```
   CREATE ─→ PROVISION ─→ CONNECT ─→ DEPLOY ─→ OBSERVE
      ▲                                            │
      │              ┌── ITERATE (branch/preview) ◄┤
      │              ├── SCALE                     │
   TEMPLATE          ├── PROTECT (backup/restore)  │
                     ├── PROMOTE (env → env)       │
                     └── RETIRE (archive/destroy)  │
```

**1. Create.** `steloit project create ecommerce` or the Console flow. The developer names the application, selects services (or describes the app and reviews AI recommendations — the vision's flow), and sees the estimated monthly cost *before anything exists*. Creation from a Template pre-wires services, bindings, and example code. A `production` environment is created by default; nothing else is presumed.

**2. Provision.** Selected services materialize with production-grade defaults: backups on, monitoring on, private networking on, sensible sizes. The "no additional setup" promise is kept *here*, at provisioning time, by defaults — not later by configuration.

**3. Connect.** Bindings are created between services (worker ↔ queue, api ↔ postgres) and to the developer's code: `steloit init` links the repo, and injected configuration (`DATABASE_URL` etc.) flows through the one injection mechanism. `steloit dev` brings the same configuration to the local machine.

**4. Deploy** *(once Compute exists; before then, this stage is "connect your external host," which the platform treats as a legitimate, documented mode — Steloit-as-data-layer is a supported way of life, not a degraded one).* Git push or `steloit deploy` produces an immutable Deployment: build → release (migrations hook) → health-checked rollout → instant rollback available. Preview environments per pull request — with **branched databases**, not shared ones — are the flagship of this stage.

**5. Observe.** Dashboards, logs, and default alerts exist from the first minute without setup. The developer's monitoring story starts at "already done" and gets customized from there.

**6. Iterate.** The loop: branch an environment (with data branches), test, open a PR, get a preview, merge, deploy. The lifecycle is a cycle, and stages 3–6 repeat daily.

**7. Scale.** Vertical resizing, read replicas, autoscaling ranges for compute. The AI layer *proposes* scaling from observed load; humans (or explicit auto-scale policies the human set) decide. Costs of every scaling action are shown before confirmation.

**8. Protect.** Continuous backups by default; retention via Policies; restore as a rehearsable action (restore into a fresh branch to verify, then promote). Disaster recovery is a workflow developers can practice, not a document they hope is right.

**9. Promote.** Configuration and deployments move staging → production through explicit promotion, showing a diff of what will change. Environments make promotion meaningful; diffs make it safe.

**10. Retire.** Archive (stop compute, snapshot data, stop most billing, keep restorable) or destroy (explicit confirmation, name-typing for production, final snapshot retained per policy, grace period before irreversibility). Preview environments retire automatically when their PR closes — lifecycle hygiene is automated where intent is unambiguous.

The lifecycle's design principle: **expensive things are explicit, reversible things are easy, irreversible things are slow.**

---

# 6. Cross-Product Experience

Coherence is enforced by mechanisms, not by hope. Five mechanisms make N products feel like one platform:

**1. The Binding fabric.** Every "X talks to Y" flows through Bindings: credential minting, network path, config injection, rotation, and topology visualization. Adding a queue to an app is `steloit queue create jobs && steloit bind worker jobs` — and the worker restarts with consumer config injected. No copied connection strings, ever. When a new service type ships, it ships with Binding support or it doesn't ship.

**2. The shared service grammar.** All services share lifecycle verbs (`create, list, inspect, scale, backup, destroy`), page anatomy (§4.3), CLI shape, API shape, and metric/log conventions. The grammar is documented as an internal spec ("What it means to be a Steloit Service") that every new product must satisfy — the platform's constitution.

**3. The single Event pipeline.** One stream feeds service logs, environment observability, alerts, notifications, audit, and AI context. Cross-product debugging ("show me everything that happened in production between 14:02 and 14:10") is a query, not an integration project.

**4. Identity everywhere.** One login, one API key model, one RBAC evaluation for every product. A `Developer` on a project has coherent, predictable powers across Postgres, Storage, Compute, and Logs without per-product permission systems.

**5. One bill, itemized by the hierarchy.** Costs roll up service → environment → project → org, visible at every level, with provisioning-time estimates and running actuals side by side. Pricing coherence is product coherence: a developer should never need a spreadsheet to understand what Steloit costs.

Worked example — the promise in miniature: *a request is slow.* Alert fires (Events) → notification links to the API service's metrics (grammar) → latency spike correlates with Postgres connection saturation, visible because both services share the environment timeline (pipeline) → pivot to the database's query insights → the AI panel explains the offending query and proposes an index (§7) → developer applies it via a migration in a preview branch (lifecycle) → promotes. Six products touched; zero context switches; one platform.

---
# 7. Platform Intelligence

The vision's constraint is exact: *AI is not the product; AI improves the developer experience.* The architecture makes that constraint mechanical.

## 7.1 The four laws of Steloit intelligence

1. **AI proposes; humans dispose.** Every AI capability terminates in a *reviewable artifact* — a recommendation, a plan, a diff, a draft migration — never in a silently applied change. The apply button belongs to the developer.
2. **Everything AI does is explained and auditable.** Every recommendation carries its reasoning ("recommended Valkey because you described session management") and its evidence (the metrics or schema it examined). Every AI-initiated action, once approved, lands in the audit log attributed as `user X via assistant`.
3. **AI reads broadly, writes narrowly.** The assistant may read metrics, schemas, logs, and configuration within the caller's permissions — never more. It can *draft* changes to anything; it can *apply* nothing without the same permission checks and confirmations a human faces. It never touches security settings, IAM, network exposure, or destructive operations even as one-click proposals — those it only *describes how* to do.
4. **The platform must be fully excellent with AI turned off.** Intelligence is a layer, and org-level Policy can disable it entirely (an enterprise requirement and an honesty check: if any workflow *requires* the AI, the workflow is underdesigned).

## 7.2 Where intelligence lives

| Surface | Capability |
|---|---|
| **Provisioning** | The vision's flow: describe the app → recommended services with reasons and cost → developer edits and confirms. |
| **PostgreSQL** | Schema generation, query explanation, slow-query diagnosis, index recommendations, migration drafting — each producing artifacts (SQL, migration files) the developer reviews. |
| **Valkey / Storage / Queue** | Caching-strategy suggestions, memory analysis; lifecycle-policy and cost recommendations; DLQ pattern diagnosis. |
| **Observability** | Anomaly annotation ("latency up 4× since deploy #142"), alert triage summaries, plain-language incident timelines assembled from the Event pipeline. |
| **Cost** | Idle-resource detection, right-sizing proposals with predicted savings, "what changed in this bill" explanations. |
| **Platform Assistant (⌘K / `steloit ask`)** | Conversational entry point over all of the above plus documentation. Answers "why is my app slow?" with *this app's* data, and answers "how do I add a read replica?" with the exact command — or a prepared, confirmable action. |

## 7.3 Where intelligence must never interfere

- **No autonomous production changes** — no auto-applied scaling, indexes, config, or "self-healing" mutations outside explicit, human-authored auto-scale policies.
- **No AI in the security path** — IAM, secrets values, network exposure, and deletion flows are AI-description-only zones.
- **No dark patterns** — the assistant never nags, never upsells, and its cost recommendations are allowed to *reduce* Steloit's revenue. Trust is the product.
- **No ambient data use** — customer data and schemas are used to serve that customer's session, governed by policy, never to train shared models without explicit opt-in.

The strategic view: intelligence compounds the platform's structural advantage. Because Steloit sees the whole project — schema, traffic, logs, cost, topology — its assistant can reason about the *application*, while single-product vendors can only reason about their silo. §9 develops where this goes; the laws above are permanent regardless.

---

# 8. Product Evolution

The versioning model is deliberate: **v0 → v0.5 → v1 → v1.5 → v2 → v2.5 → v3 → v4**, where integers are *promises to the market* (a new definition of what Steloit is) and halves are *deepening releases* (making the current promise excellent). This rhythm — expand, then deepen — prevents the classic platform failure of stacking new products on unfinished ones. It intentionally re-sequences the vision's Phase 2 (Operations): baseline observability cannot wait for a separate phase, because "everything included" is the launch promise, not a later feature.

---

### v0 — The Wedge *(private alpha)*

- **Vision:** prove the core loop: project → provision → connect, in minutes.
- **Products:** Projects, Environments (production + dev), **Managed PostgreSQL**, Secrets, Bindings (to external apps), CLI, minimal Console, REST API. Baseline metrics/logs on the database.
- **Why this version exists:** everything in Steloit rests on two bets — the Project abstraction feels right, and Steloit can run Postgres excellently. v0 tests both with maximum focus and minimum surface.
- **Technical dependencies:** first Cell (single region), control plane, identity core, Event pipeline skeleton, billing metering (even before charging).
- **User value:** a production-grade Postgres with backups and monitoring, provisioned inside a coherent project model, in under five minutes.
- **Risks:** operating stateful Postgres reliably is the hardest engineering on the roadmap — if v0 loses data, the company is over; mitigate with boring proven architecture (streaming replication, continuous WAL archiving, restore drills as CI). Second risk: the Project model confusing solo developers → mitigated by making single-service projects feel weightless.
- **Success criteria:** time-to-first-connection < 5 min; zero data-loss incidents; alpha users voluntarily pointing a real app at it; qualitative signal that "project" clicks.
- **Postponed:** every other service; Console polish; SDKs. **Why wait:** each additional surface divides the attention the database's reliability needs.

---

### v0.5 — The Data Layer *(public beta)*

- **Vision:** the complete backend data layer from one platform.
- **Products introduced:** **Valkey, Object Storage, Queue**; the guided/AI-assisted provisioning flow with cost estimates; Console reaches full service-page anatomy; TypeScript + Python SDKs; database branching (alpha).
- **Why:** the four-service data layer is the vision's Phase 1 and the minimum credible version of "stop assembling vendors." Shipping three services *after* the grammar exists (v0) forces the grammar to be real.
- **Dependencies:** Binding fabric generalized across service types; the internal "Steloit Service" spec written and enforced.
- **User value:** one platform replaces Neon + Upstash + R2 + a queue vendor, with one bill and one CLI. *(refined by ADR-0004/A5: Steloit builds Postgres + Valkey; R2/S3/GCS storage and LLM providers are governed **Bindings** (your bucket/provider, your bill, our wiring + policy + estimate); queue is a Postgres capability (pgmq). "One platform, one CLI, one control plane" holds; "one bill" holds for what we operate — external Bindings surface the provider's price honestly rather than reselling it.)*
- **Risks:** breadth diluting Postgres quality (mitigate: separate service teams, shared platform team); Valkey/Queue commoditization pressure (answer: the *integration* — bindings, unified observability — is the differentiation, not the engines).
- **Success criteria:** >50% of active projects use ≥2 services (the platform thesis, measured); branching NPS; beta retention.
- **Postponed:** compute, alerts customization, Terraform. **Why wait:** the promise being made is "data layer," and it must be excellent before the next promise.

---

### v1 — The Platform Promise *(GA, revenue)*

- **Vision:** the easiest place to provision and operate backend infrastructure — the vision's Phase 1 + operational baseline, generally available and paid.
- **Products/features introduced:** unified **Observability** (correlated environment view, default + custom alerts, notification channels); **Backup & Restore** as a rehearsable workflow; **database branching GA** + preview-environment support for externally-hosted apps; Teams + project RBAC; **Terraform provider**; Go SDK; Free/Pro/Business plans with **Usage & Cost analytics**; Template gallery (first-party).
- **Why:** GA is when "no additional setup required" must be demonstrably true, and when money changes hands the operational products stop being optional.
- **Dependencies:** Event pipeline at production scale; billing hardened; support org.
- **User value:** a startup runs its entire data and operations layer on Steloit with confidence: monitored, backed up, access-controlled, cost-transparent.
- **Risks:** GA reliability bar (SLA commitments) — mitigate with error budgets gating feature work; pricing complexity undermining the transparency pillar — mitigate with the estimate-before-provision flow as a hard requirement for every SKU.
- **Success criteria:** paid conversion; net revenue retention > 110%; support ticket themes shifting from "how do I" to "can it also"; SLA attainment.
- **Postponed:** compute (the next integer promise), SSO/custom roles, vector DB. **Why wait:** each belongs to a later promise; shipping them now would fragment the GA message.

---

### v1.5 — Depth Before Breadth

- **Vision:** make the data platform the best in the world, not just the most integrated.
- **Introduced:** Postgres query insights + AI query/index assistance GA; **pgvector as managed Postgres capability** (the AI-startup answer without a new product); Valkey clustering; Storage CDN integration + signed-URL ergonomics; Queue schedules/DLQ tooling; Java/Rust/C# SDKs; Pulumi; audit log GA for Business tier.
- **Why:** the deepening beat. v2's compute launch will consume enormous attention; the data layer must be strong enough to carry the brand through it.
- **Risks:** organizational temptation to skip this release for compute headlines — resist; platforms die of unfinished foundations.
- **Success criteria:** churn reduction; measured expansion within existing projects; "best Postgres DX" showing up in unprompted user language.
- **Postponed:** everything compute — one more release of patience.

---

### v2 — The Developer Cloud *(the second promise)*

- **Vision:** projects become *deployable*. Build, deploy, and operate — the full sentence.
- **Products introduced:** **Web Services/APIs, Workers, Cron Jobs**; Deployments (immutable, rollback, promotion with diffs); Git integration (push-to-deploy, **PR preview environments with branched databases** — the flagship feature of the entire roadmap); custom domains + managed TLS; `steloit dev` local development GA.
- **Why:** this is the vision's endgame ("create a project, deploy the application") and the moment Steloit's category changes from "great data platform" to "developer cloud." It comes fourth, not first, because compute without a mature data + observability layer is just another PaaS.
- **Dependencies:** build system, rollout orchestration, ingress/edge routing, Deployment primitive; observability extended to application runtime.
- **User value:** the multi-vendor stack from the vision's problem statement collapses to one platform; previews-with-data-branches deliver a workflow no fragmented stack can copy.
- **Risks:** the largest engineering expansion on the roadmap (builds, networking, runtime security) — mitigate by launching with containers-from-Dockerfile/buildpacks only, no exotic runtimes; risk of neglecting data-layer-only customers — mitigate by keeping "Steloit as your data layer" an explicitly supported mode forever.
- **Success criteria:** % of projects with ≥1 compute service after 6 months; preview-environment weekly usage; end-to-end (Git-push → live) time < 3 minutes; compute reliability at parity with data SLAs.
- **Postponed:** Functions, Edge, autoscaling sophistication. **Why wait:** containers cover the overwhelming majority of early workloads; functions/edge are distinct runtimes deserving their own deepening cycle.

---

### v2.5 — The Complete Application Platform

- **Vision:** deepen compute, complete the common application stack.
- **Introduced:** autoscaling maturity; deployment strategies (canary/blue-green); **dedicated Vector DB** (graduating from pgvector where scale demands) and **Search** service; OpenTelemetry export GA; AI incident summaries and cost optimizer GA; template ecosystem opened to curated community contributions.
- **Why:** the deepen beat after the v2 expand beat; also where the AI-startup audience gets purpose-built primitives, timed to real demand rather than hype.
- **Success criteria:** vector/search attach rate among AI-product customers; canary adoption; community template submissions.
- **Postponed:** enterprise capabilities — deliberately next.

---

### v3 — The Enterprise Platform *(the third promise)*

- **Vision:** the same simple platform, deployable where enterprises need it — the vision's Phase 5.
- **Products introduced:** **Steloit BYOC** (AWS first, then GCP/Azure) — data plane in the customer's account, Steloit control plane, *identical* CLI/API/Console; **SSO (SAML/OIDC) + SCIM**; custom roles; **Private Networking** (peering/private endpoints/allowlists); compliance program (SOC 2 first, expanding); org Policies GA (spend, region, retention floors); multi-region environments (initial: cross-region replicas + DR for Postgres).
- **Why:** enterprises are the vision's Phase 3 customers and BYOC is the durable answer to data-ownership and compliance objections — a moat pure-SaaS competitors struggle to cross. It comes *after* v2 because BYOC multiplies operational complexity across everything already built; the surface must be stable first.
- **Dependencies:** the Cell abstraction proving out (this is why it was designed in §2.2 from day one); remote-cell fleet operations; compliance groundwork started at v1.
- **User value:** platform-engineering teams give their developers Steloit's DX inside corporate cloud accounts, keeping data ownership, networking, and compliance.
- **Risks:** BYOC support burden (customer-account variability) — mitigate with strict prerequisites and automated cell health management; enterprise sales motion maturity — a company-building risk beyond product scope, flagged here deliberately.
- **Success criteria:** BYOC cells in production at reference customers; enterprise ARR; DX-parity audits between Cloud and BYOC showing zero divergence; SOC 2 attained.
- **Postponed:** self-hosted (control plane in customer hands) — the vision marks it future; it waits until BYOC operations are boring. Analytics DB, event bus — v4.

---

### v4 — Platform Services *(the expansion, vision Phase 6)*

- **Vision:** adjacent services that naturally extend the platform — added only where the "naturally extend" test passes.
- **Candidates (in likely order):** **Functions** (event-driven compute, now justified by a mature event fabric); **Event Bus** (formalizing the internal Event pipeline into a customer-usable primitive — the most "natural" extension on the list); **Workflow Engine** (durable orchestration atop queues + functions); **Analytics Database**; **Identity & Auth for customer apps** (last, hardest, only with dedicated security investment); Edge Runtime (contingent on multi-region maturity); Marketplace (partner services provisionable as Bindings — the v3.x template ecosystem's graduation, if and only if platform gravity has arrived); **Self-Hosted** deployment.
- **Why this version exists:** it is intentionally a *portfolio of options, not commitments*. By v4 the platform has the primitives, the distribution, and the operational maturity to expand in several directions; choosing among them should be driven by observed customer pull, not a five-year-old document. The architecture's job — done in §1–§3 — is to guarantee that whichever are chosen fit the existing grammar.
- **Success criteria:** each candidate gets its own v0-style wedge criteria before graduating to the platform; the meta-criterion is that every addition *increases* platform coherence scores (measured by cross-service adoption), never dilutes them.

---

## 8.1 Evolution principles (the rules behind the sequence)

1. **Alternate expand and deepen.** Integer versions change what Steloit *is*; half versions make it excellent. Never two expansions in a row.
2. **Trust before breadth.** Stateful data services — where failure is unforgivable — mature first; compute — where failure is a bad deploy — comes second; enterprise — where failure is contractual — waits for both.
3. **Operations ship with, not after.** Re-sequenced from the vision's phases: baseline observability and backups accompany every service at its launch, because "included" is the brand.
4. **Every audience gets a version.** Indie devs (v0–v1), startups (v1–v2), scale-ups (v2–v2.5), enterprises (v3) — matching the vision's customer phases exactly.
5. **Postponement is a feature.** Every "not yet" above protects the quality of a "now."

---

# 9. Long-Term Platform Strategy (Years 5–10)

## 9.1 How Steloit grows

Growth follows the **land–expand–anchor** shape the architecture was built for: land with one service in one project (usually Postgres — the wedge never stops working); expand across services within the project (the Binding fabric and single bill make each addition cheaper than a new vendor); anchor as the org's default ("new project" at the company means `steloit project create`). The strategic metric across all years is **services-per-project and projects-per-organization** — coherence, measured.

## 9.2 How products evolve

Individual products deepen along a common arc: *managed → insightful → assisted*. Managed Postgres becomes Postgres-with-query-intelligence becomes Postgres-where-performance-regressions-are-diagnosed-before-you-notice. Crucially, the arc never crosses into *autonomous* without explicit, human-authored policy — the four laws of §7 are permanent, because a decade of trust is worth more than any demo.

The primitive set is the other constant. Ten years of new products should be expressible in Organizations, Projects, Environments, Services, Deployments, Bindings, Secrets, Policies, and Events. Pressure to add primitives is treated as a signal to think harder, and any addition (Workspace being the pre-identified candidate) is a deliberate, versioned act.

## 9.3 How the platform expands

Three axes, in order of confidence:

1. **Deeper into the application** (high confidence): the v4 portfolio — functions, event bus, workflows, analytics, eventually identity. Each is adjacent, each fits the grammar.
2. **Broader across geography** (high confidence): multi-region evolves from replicas/DR (v3) toward active-active data and globally distributed compute — the point where an Edge Runtime finally becomes natural rather than premature.
3. **Downward into deployment models** (medium confidence): Cloud → BYOC → Self-Hosted is a progression of the *same* Cell architecture, and the ten-year enterprise story is "one product model, any substrate" — potentially including sovereign and air-gapped environments, which the Event/Policy/audit foundations were designed to survive.

The expansion discipline is the §3 heuristic, permanently: build where fragmentation is the pain, integrate where ecosystems have gravity, never sell undifferentiated infrastructure.

## 9.4 How enterprise capabilities emerge

Enterprise features are architected as **intensifications of existing primitives, not a parallel product**: SSO intensifies Identity; compliance intensifies the audit Event stream; governance intensifies Policies; private networking intensifies the Environment network; BYOC intensifies the Cell. This is the structural defense against the disease that kills developer platforms — the enterprise fork, where the product enterprises buy stops being the product developers love. On Steloit there is exactly one platform; enterprises configure more of it.

## 9.5 How AI evolves

Three horizons, one constraint:

- **Assistant** (now): contextual help, recommendations, diagnosis — §7 as specified.
- **Copilot** (mid): the assistant participates in workflows end-to-end — drafting a full "add search to this app" plan spanning provisioning, bindings, migration, and deploy, executed as a reviewable, promotable change set through the *existing* preview-environment machinery. AI plans become just another thing that flows through the lifecycle.
- **Operator-with-a-license** (far): developers may *delegate* narrow, bounded operational authority through explicit Policies ("may add read replicas up to $X/mo when saturation exceeds Y") — automation as human-authored policy, executed and audited by the platform, with AI proposing the policies themselves. Autonomy is never assumed; it is always *granted*, scoped, and revocable.

The constraint that survives all three horizons: the developer can always answer *what is running, why, and who decided* — from the audit log, in plain language.

## 9.6 How developer workflows change — and what must not

Over ten years, expect: infrastructure specified increasingly as intent (a project's "shape" declared in a versioned manifest — IaC converging with the Project primitive itself); preview environments becoming the default unit of all change (app *and* data *and* infrastructure branched together); operations shifting from dashboards-watched to exceptions-surfaced.

What must not change is the sentence at the heart of the vision: **create a project, choose what you need, deploy, and focus on building software.** Every capability added over the decade must be invisible until wanted. The measure of success in year ten is that the first-five-minutes experience is *simpler* than it is today, while the ceiling above it has risen without limit.

---

# 10. Closing: The Tests This Architecture Must Keep Passing

1. **The five-minute test** — from zero to a provisioned, monitored, connected service in under five minutes, at every stage of company growth.
2. **The one-grammar test** — a developer fluent in one Steloit service is fluent in all of them, across Console, CLI, API, and IaC.
3. **The topology test** — every project can draw itself: services and bindings, one picture, no vendor seams.
4. **The trust test** — nothing irreversible is easy, nothing automated is unexplained, nothing intelligent is unaccountable.
5. **The coherence test** — every new product increases the value of every existing one, or it doesn't ship.

The vision promised one project, one platform, infinite possibilities. This architecture is the machine that keeps that promise as the possibilities arrive.

*— End of specification —*
