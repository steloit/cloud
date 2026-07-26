# PDS-SHELL — Platform Shell & Project Experience

**Document class:** Product Design Specification (per DES-000)
**Document ID:** PDS-SHELL
**Status:** Draft v1.0 → for Gate G-A (§§1–5) and Gate G-B (§§6–14)
**Owner:** Shell Design Lead (Platform Experience track)
**Braided with:** PDS-001 (Managed PostgreSQL) — combined gates per DES-009 §5.5
**Pinned authorities:** GOV-001 · GOV-002 · GOV-003 (Approved, incl. R1 resolutions) · FND-001 (Review r3) · FND-002 (Review r2) · FND-003 (Review r2, co-evolving) · FND-004 (Draft r1) · FND-007 (Draft r1) · FND-009 (Review r1) · FND-010 (Draft r2, estimate contract) · PRD-012 (Draft r2, recommendation-flow scope) · DES-001/002/003/004/005/007/008 (Approved) · EXP-001 (skeleton) · EXP-003 (grammar boards v1) · EXP-006 (Approved)
**Version applicability:** v0 core (marked ⓿), v0.5 additions (marked ⓵). Items marked ⓶ are v2 slots designed as frames only.

---

## §1 — Design Intent **[M]**

The Shell is the platform's body language. Before a developer has read a word of documentation, the Shell has already taught them Steloit's model: *you are in an organization, looking at a project, through the lens of an environment, at the services inside it.* Every Shell decision serves that one lesson.

**The sentence a developer should say after first use:** *"I understood the whole platform from one screen."*

**The one thing this design must get right:** the context model (G1). Org → Project → Environment must be so legible, so persistent, and so cheap to switch that developers stop thinking about *where they are* within their first session — the way developers stop thinking about which git branch they're on because the prompt always shows it.

**The one failure it must avoid:** becoming a portal. The Shell is chrome around work, not a destination. If a developer spends time *in the Shell* rather than *through it*, the design has failed. Density over decoration; verbs over dashboards-about-dashboards; the daily loop (J2) in under ten seconds.

**Emotional register (per DES-001):** calm, dense, honest, fast. The Shell never celebrates itself. It celebrates exactly twice: first successful provision, and first successful connection — the two moments GOV-001's promise is kept.

---

## §2 — Authorities & Delta Statement **[M]**

### 2.1 Citation spine

| Authority | What this PDS renders from it |
|---|---|
| GOV-002 §1–2 | The nine primitives; the hierarchy; "identity and money at the Org, everything operational at the Project"; environment isolation; region-as-environment-property |
| GOV-002 §4 | The IA: context bar spine, environment-switch-as-filter, primary/secondary object rules, page hierarchy, settings placement test |
| GOV-002 §5 | Lifecycle stages Create/Provision/Connect/Retire (Shell-owned stages); "expensive explicit, reversible easy, irreversible slow" |
| GOV-001 | The project-creation flow with service selection, AI recommendation, and estimated monthly cost before provisioning |
| FND-001 | Object identities, states, soft-delete grace semantics, folders/labels |
| FND-002 | URL grammar, async-operation pattern, error taxonomy (catalog in §9.3) |
| FND-003 | Standard service states rendered by topology and service cards |
| FND-004 | Binding-as-edge rendering; external-runtime connect flow (R2 resolution) |
| FND-007 | Policy attachment points surfaced in settings frames |
| FND-009 | Region selection at environment creation; production identity |
| FND-010 | The provisioning-time estimate contract → the wizard estimate panel |
| PRD-012 | The recommendation flow: describe → recommended services + reasons + cost → user edits → confirm. Nothing else. |
| DES-002 | G1 (context), G3 (danger), G4 (cost), G5 (forms), G6 (empty/loading/error), G7 (time), G8 (production identity), G9 (AI presence), G10 (density/keyboard/speed), G11 (theming) — instantiated, never restated |
| DES-003 | J1, J2, J3, J7 (steps 3–4), J9 participation |
| EXP-001/003/006 | Nav skeleton; CLI grammar; first-run flow |

### 2.2 Delta statement — everything this PDS decides that no authority already determines

1. The Console URL grammar's concrete form (§4.3) within FND-002's rules.
2. The navigation tree's final v0/v0.5 composition — including the resolution of GOV-002 §4.2's illustrative "Data" nav item into a Services-list filter, not a destination (§4.4, with rationale).
3. Context persistence, deep-link precedence, and context-conflict recovery rules (§6.4).
4. The ⌘K palette's three modes, scoping, ranking, and result anatomy (§6.7).
5. The notification inbox's category set, grouping, and quiet-by-default thresholds (§6.8).
6. The project creation wizard's step structure, both paths (manual / describe), and the estimate panel's layout obligations (§6.5).
7. The topology view's rendering rules, interactions, and non-visual traversal (§6.6).
8. The four settings frames' composition and the placement of every v0/v0.5 setting (§6.9).
9. Project archive/destroy and environment destroy flows' concrete tiers and copy (§13, §9).
10. First-run experience: empty states, the guided path, and the two celebration moments (§6.10).
11. Shell keyboard map within G10's scheme (§6.11).
12. The connect (post-provision) experience for external runtimes — R2's resolution rendered (§6.5, screen SH-19).

Everything not on this list is inherited. Reviewers: treat any §6+ content not traceable to this list or an authority as a defect.

---

## §3 — Scope, Non-Goals & Pioneer Declaration **[M]**

### 3.1 In scope (v0 ⓿ unless marked)

App frame and navigation · context bar with org/project/environment switchers · projects home · project overview (dashboard) · services list · topology view · environments management · project creation wizard incl. AI-recommendation path ⓵ and template entry point stub ⓵ · post-provision connect experience · ⌘K palette (search + commands ⓿, ask ⓵) · notification inbox · org / project / environment / user settings frames · first-run experience · project archive/destroy, environment create/destroy · shell-level empty/loading/error/permission states · shell keyboard model.

### 3.2 Non-goals

PostgreSQL's service page and type tabs (PDS-001) except the wizard's Postgres configure step, co-owned and cross-referenced · Observe section contents (PDS-005; nav slot only) · member/role/API-key management contents (PDS-007; settings frames provide slots) · billing pages (PDS-009; org settings provides slot) · account signup/login/MFA (PDS-ACC; this PDS begins post-authentication) · Deployments nav item ⓶ (frame reserved, hidden pre-v2) · template gallery (PDS-TPL; wizard shows its entry point ⓵) · marketing site.

### 3.3 Pioneer declaration (the complete license — nothing else may be minted here)

| # | Pattern minted | Registry name |
|---|---|---|
| P-1 | Context bar & context model rendering | `pattern/context-bar` |
| P-2 | Provisioning wizard (workflow 1's concrete form) | `pattern/provisioning-wizard` |
| P-3 | Cost-estimate panel (G4's first rendering) | `pattern/estimate-panel` |
| P-4 | Topology map | `pattern/topology-map` |
| P-5 | ⌘K palette | `pattern/command-palette` |
| P-6 | Notification inbox | `pattern/notification-inbox` |
| P-7 | Settings frame (three-scope composition) | `pattern/settings-frame` |
| P-8 | Async progress block (FND-002 async ops rendered) | `pattern/async-progress` |
| P-9 | Connect block (credentials + injected-config instructions) — *co-minted with PDS-001* | `pattern/connect-block` |

---

## §4 — Object Model & Information Architecture **[M]**

### 4.1 Object map

Objects the Shell **owns** (primary, per GOV-002 §4.1) vs **renders** (owned elsewhere, displayed here):

```
OWNED                                          RENDERED
Organization ──┬── Folder (grouping only)      Service (FND-003; as card/node)
               ├── Project ──┬── Environment   Binding (FND-004; as edge/row)
               │             │                 Secret (FND-006; as env-settings slot)
               │             └── Label         Policy (FND-007; as settings rows)
               ├── Notification (view of Event)Cost figure (FND-010; as chip/panel)
               └── Settings scopes (org/proj/env/user)   Member/Team (FND-005; as slots)
```

| Object | FND-001 identity | States (FND-001) | Notes |
|---|---|---|---|
| Organization | `org` slug, immutable after 30d rename window | `active, suspended` | Suspension renders per FND-010/SEC-007 triggers; Shell shows banner + read-only mode |
| Project | `project` slug unique in org; rename allowed, slug redirect kept 90d | `active, archived, deleting(grace 7d), deleted` | Archive = compute stopped/services suspended, snapshot retained, restorable; per FND-001 |
| Environment | `env` slug unique in project; `production` created by default, protected flag on | `active, deleting(grace 72h for production→n/a: production cannot be deleted while project active — see §13), deleted` | Region fixed at creation (FND-009) |
| Folder | org-scoped name | — | No security/billing semantics (GOV-002 §1.2); pure grouping |
| Label | key:value on project | — | Filter chips on projects home |
| Notification | event-derived, per-user read state | `unread, read, dismissed` | Categories in §6.8 |

### 4.2 Rendered-object contracts

- **Service card/node:** icon (service-type family, DES-004), name, one-line status (G2 vocabulary verbatim: `provisioning, ready, degraded, suspended, deleting`), primary metric sparkline (metric chosen by the owning PDS), cost chip (month-to-date, FND-010). Click → service page (owning PDS's territory).
- **Binding edge/row:** direction (consumer → target), scope summary, status (inherits G2), created-by. Click → binding side-panel (§6.6).

### 4.3 URL grammar (Console)

Base: `app.steloit.com`. Pattern mirrors FND-002's API paths with slugs:

```
/{org}                                  → redirect to /{org}/projects
/{org}/projects                         → projects home (SH-02)
/{org}/settings/{section}               → org settings (SH-14)
/{org}/{project}                        → redirect to default env overview
/{org}/{project}/{env}                  → project overview (SH-04)
/{org}/{project}/{env}/services         → services list (SH-05)
/{org}/{project}/{env}/services/{svc}…  → service pages (owning PDS)
/{org}/{project}/{env}/topology         → topology (SH-06)
/{org}/{project}/settings/{section}     → project settings (SH-15)  [env-independent]
/{org}/{project}/{env}/settings/{sec}   → environment settings (SH-16)
/{org}/{project}/environments           → environments mgmt (SH-07) [env-independent]
/account/{section}                      → user settings (SH-17)      [org-independent]
/new                                    → wizard (SH-08…) [org context from bar]
```

Rules: environment is a **path segment, not a query param** — deep links always carry full context; env-independent pages omit the segment; unknown segments resolve per §6.4's conflict rules. Slugs render in monospace everywhere (DES-004 mono system).

### 4.4 Navigation tree (final, v0/v0.5)

```
[Context bar: Org ▾ / Project ▾ / Env ▾ ......... ⌘K  🔔  👤]
PROJECT (visible when project context set)
  Overview            /{org}/{project}/{env}
  Services            …/services
  Topology            …/topology
  Observe ⓵           …/observe            (slot; contents PDS-005)
  Deployments ⓶       (hidden until v2)
  Settings            /{org}/{project}/settings
ORGANIZATION (collapsed group, always present)
  Projects            /{org}/projects
  Members             /{org}/settings/members     (contents PDS-007)
  Billing ⓵           /{org}/settings/billing     (contents PDS-009)
  Audit ⓵             /{org}/settings/audit
  Policies ⓵          /{org}/settings/policies
```

**Delta ruling — the "Data" item (delta 2):** GOV-002 §4.2's sketch showed a `Data` nav entry. Rendering it as a destination would give the same objects (services) two homes, violating the primary-object rule ("primary objects appear in navigation… as destinations" — one destination). Ruling: **Services is the single list; a `Data | Compute ⓶ | All` filter bar within it provides the grouping the sketch gestured at.** The sketch is illustrative per GOV-002's own framing; EXP-001's skeleton is amended accordingly (change request EXP-001-CR-3 filed).

**Topology as nav item (delta 2, continued):** the topology map is the project overview's hero *and* a full-screen destination. Both exist: overview embeds the compact map; the nav item opens the full canvas. One object, one component, two zoom levels — not two designs.

### 4.5 ⌘K index plan

Indexed: projects (name, slug, labels), environments, services (name, type), settings sections (by title + synonyms), documentation titles ⓵, commands (§6.7 registry), notifications (by title, unread first). Ranking: current env → current project → current org → global; exact slug match always first. Result anatomy in §6.7.

### 4.6 Type-tab plan

N/A — the Shell owns no Kit-based service pages. (Recorded per DES-000: absence is a decision.)

---
## §5 — Journeys & Workflow Instantiation **[M]**

### 5.1 Journey participation

**J1 — First five minutes** *(Shell owns steps 2–7 of 8; PDS-ACC owns 1; PDS-001 co-owns 5–7).*

| Step | Surface | Lane: Console | Lane: CLI |
|---|---|---|---|
| 1 | Sign up / log in | (PDS-ACC) | `steloit login` (device-code flow) |
| 2 | Land in empty org | SH-03 first-run state | `steloit projects list` → empty-state output with next command |
| 3 | Start creation | "Create your first project" CTA → wizard | `steloit project create` (interactive) or flags |
| 4 | Name + select/describe services | SH-08/09/10 | prompts, or `--services postgres` / `--describe "…"` ⓵ |
| 5 | Configure with defaults | SH-11 (Postgres middle: PDS-001 §6.2) | prompts with `[default]` accept-on-enter |
| 6 | See estimate, confirm | SH-12 estimate panel | itemized estimate table → `Proceed? [y/N]` |
| 7 | Provisioning → ready | SH-13 async progress → celebration 1 | live progress lines → summary block |
| 8 | Connect from code | SH-19 connect block → celebration 2 on first successful connection ⓵ (detected via FND-004 credential first-use event) | `steloit env pull` writes `.env.steloit`; connect block printed |

Emotional temperature: anxiety peaks at step 6 (money) and step 8 (does it actually work?). Design spend: the estimate panel's honesty (§6.5) and the connect block's copy-paste-run certainty (§6.5, SH-19).

**J2 — Daily loop** *(Shell owns the frame; targets: context-oriented < 2s, health-read < 5s, into-a-service < 10s).* Open project (SH-04) → health strip + topology status rings answer "is it okay" without reading → recent events column answers "what changed" → one click into any service. Keyboard: `g o`, `g s`, `g t` (§6.11).

**J3 — Add a capability** *(Shell owns add-service and the external connect variant; binding creation between services is FND-004's flow rendered in topology).* Services list → `+ Add service` → wizard steps 2–5 in single-service mode (same screens, pre-scoped) → ready → connect. External-runtime variant (R2, delta 12): connect block offers three tabs — `steloit env pull` · platform sync (Vercel/Netlify export ⓵) · manual with rotation warning. The manual tab carries the persistent note that rotated credentials will not auto-update (copy in §9.2, C-41).

**J7 — Bring the team** *(steps 3–4 only: the invited teammate's first session).* Deep link honored post-auth (§6.4 rule 3); first-run banner variant for members joining an existing project ("You've joined **ecommerce**. Start at the overview →"); no wizard push.

**J9 — Retire safely** *(Shell owns entirely for projects/environments).* Project settings → Danger zone → Archive (reversible, T1+summary) or Destroy (T3, 7-day grace); environment destroy (T2; production blocked while project active). Flows in §6.9/§6.12; copy in §9.2; tiers in §13.

### 5.2 Workflow instantiation table

| Standard workflow (GOV-005 §5) | Shell instantiation |
|---|---|
| 1 Create & Provision | **Defines the concrete form** (P-2): wizard SH-08→SH-13; single-service mode for J3; template variant ⓵ entry stub (PDS-TPL) |
| 2 Connect (Bind) | Renders: topology `+ Bind` affordance and connect block (P-9); flow logic per FND-004; full binding UX matures with multi-service projects ⓵ |
| 3 Observe & Pivot | Slot only: nav item + overview's events column deep-link into PDS-005 surfaces |
| 4 Change Safely | Not instantiated in v0 shell (no promotable config at shell level until env-promotion ⓵/⓶); diff presenter consumed from DES-005 when it arrives |
| 5 Protect & Restore | Renders archived-project restore (SH-03 archived filter → Restore, T1); service-level restore is PDS-001/006 |
| 6 Scale | Not applicable at shell level (N/A recorded) |
| 7 Retire | **Defines the concrete form** for project/env: §6.12 |

---

## §6 — Interaction Model **[M]**

### 6.0 Notation & shared baseline

Wireflow notation per DES-000 §4.2 (boxes=screens, solid arrow=user action, dashed=system, ⚠=danger-tiered, ⌘=keyboard-reachable, `cli:` ribbon). Wireframes per DES-000 §4.3: grayscale, real content, pattern-cited.

**Baseline state table B-0** (applies to every screen unless a delta row overrides — DES-000 delta discipline):

| State | Entry | Display | Actions | Exit |
|---|---|---|---|---|
| loading | route enter, data pending | skeleton of known layout ≤ 200ms budget (G10); no spinners for < 400ms ops | none | data → ready; error → error |
| ready | data resolved | content | all | — |
| partial | some queries failed | content + inline section-level error blocks (G6 anatomy) | retry per section | retry → ready |
| error | page-level failure | G6 error frame: what/why/next/ref-ID | retry, ⌘K escape | retry |
| forbidden | 403 (FND-002 `E-PERM-403`) | "You don't have access" frame + role needed + request-access action → notifies org admin | request access, switch org | — |
| empty | zero objects | teaching empty state: what this is + primary action + docs link (G6) | primary action | object created → ready |
| offline | websocket lost | amber "reconnecting" pill in context bar; live regions freeze with "as of {time}" stamps (G6 honesty) | none (auto-retry) | reconnect → live |

### 6.1 Screen inventory (six-line declarations per DES-000 §4.4)

| ID | Screen | Purpose | Pattern | Journey | Primary object | States | CLI counterpart |
|---|---|---|---|---|---|---|---|
| SH-01 | App frame | Persistent chrome: nav + context bar + content slot | P-1, nav shell | all | context triple | B-0 + offline | context flags / `steloit use` |
| SH-02 | Projects home | Org's projects: find, assess, enter, create | card grid + table toggle | J1-2, J2 | Project | B-0, empty=first-run | `steloit projects list` |
| SH-03 | First-run state | Empty-org teaching state → wizard | G6 empty | J1-2 | Organization | single | `projects list` empty output |
| SH-04 | Project overview | Health, shape, changes in 10s | overview anatomy | J2 | Project(env lens) | B-0 + degraded-mix | `steloit status` |
| SH-05 | Services list | All services in env; add | table + filter bar | J2, J3 | Service (rendered) | B-0, empty teaches add | `steloit services list` |
| SH-06 | Topology | The project drawn: nodes+edges, full canvas | P-4 | J2, J3 | Service/Binding | B-0, empty, single-node | `steloit status --graph` |
| SH-07 | Environments mgmt | List/create/inspect envs | table + side panel | J3, J9 | Environment | B-0 | `steloit env list/create` |
| SH-08 | Wizard: Name & region | Project name, slug preview, prod region | P-2 step | J1-4 | Project(draft) | form states (G5) | `project create` prompt 1 |
| SH-09 | Wizard: Select services | Manual selection cards | P-2 step | J1-4 | Service(draft) | form | prompt 2 / `--services` |
| SH-10 | Wizard: Describe (AI) ⓵ | Describe app → reviewed recommendation | P-2 step + G9 proposal | J1-4 | Proposal artifact | idle/thinking/proposal/error | `--describe` ⓵ |
| SH-11 | Wizard: Configure | Per-service config w/ defaults | P-2 step; middles owned by service PDSs | J1-5 | Service(draft) | form | prompts 3..n |
| SH-12 | Wizard: Review & estimate | Itemized estimate + confirm | P-2 + P-3 | J1-6 | Estimate | ready/estimate-unavailable | estimate table + confirm |
| SH-13 | Wizard: Provisioning | Async progress → ready | P-8 | J1-7 | Operation | running/step-fail/done | progress lines |
| SH-19 | Connect | Credentials/config into code | P-9 | J1-8, J3 | Binding/credential | ready/rotating | `steloit env pull` |
| SH-20 | ⌘K palette | Search/commands/ask overlay | P-5 | all | mixed | idle/results/no-results/ask ⓵ | (CLI itself; see §7 ruling) |
| SH-21 | Notification inbox | Panel: triage events | P-6 | J2, J4 entry | Notification | B-0, empty="quiet" | `steloit notifications list/ack` |
| SH-14 | Org settings frame | General, members*, billing*⓵, policies⓵, audit⓵ (slots*) | P-7 | J7 | Organization | B-0 | `steloit org …` |
| SH-15 | Project settings frame | General, labels, integrations⓵, danger zone | P-7 | J9 | Project | B-0 | `steloit project …` |
| SH-16 | Env settings frame | General(region ro), secrets slot, danger zone | P-7 | J9 | Environment | B-0 | `steloit env …` |
| SH-17 | User settings | Profile, appearance, notification prefs | P-7 | — | User | B-0 | `steloit config …` |
| SH-22 | Archive project dialog | Reversible retire | G3-T1+summary ⚠ | J9 | Project | confirm/working | `project archive` |
| SH-23 | Destroy project dialog | Irreversible retire, grace | G3-T3 ⚠ | J9 | Project | 3-stage | `project delete` |
| SH-24 | Destroy environment dialog | Env retire | G3-T2 ⚠ (prod: blocked) | J9 | Environment | confirm/working | `env delete` |
| SH-25 | Context-error pages | 404-in-context / gone / conflict | G6 error frames | all | — | per-case | exit codes 4/5 |
| SH-26 | Environment create dialog | Name, region, (source: blank) | G5 form | J3 | Environment | form | `env create` |

*(26 declared screens; SH-18 intentionally unassigned — reserved for the ⓶ Deployments frame to keep future numbering stable.)*

### 6.2 Wireframe — SH-01 App frame (P-1)

```
┌──────────────────────────────────────────────────────────────────────────┐
│ ⬡ steloit │ Acme ▾ / ecommerce ▾ / production ▾ ①      ⌘K ② 🔔③ ● ④ 👤 │
├───────────┬──────────────────────────────────────────────────────────────┤
│ PROJECT   │                                                              │
│ ▸Overview │                                                              │
│  Services │                 [ content slot ]                             │
│  Topology │                                                              │
│  Observe⓵ │                                                              │
│  Settings │                                                              │
│ ───────── │                                                              │
│ ORG    ⌄  │                                                              │
│  Projects │                                                              │
│  Members  │                                                              │
├───────────┤                                                              │
│ ⓘ status  │⑤                                                             │
└──────────────────────────────────────────────────────────────────────────┘
① Context triple: three switcher buttons, slugs in mono; env button carries
   region tag ("iad") and, for production, the G8 ambient marker — a thin
   underline in `--production-accent` + word "production", never a red alarm.
② ⌘K affordance shows the literal shortcut; click = keyboard = same overlay.
③ Bell + unread count (max display "9+"); opens SH-21 panel anchored right.
④ Offline/reconnecting pill mounts here (B-0 offline).
⑤ Platform status line (INF-005 status page summary): dot + "All systems
   normal" / incident link. Nav collapses to icons at <1100px; ⌘\ toggles.
```

### 6.3 Wireframe + states — SH-02 Projects home / SH-03 first-run

```
┌ Projects — Acme ────────────────────────────────── [labels ▾][⊞|≣][+ New]┐
│ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐                      │
│ │ ecommerce    │ │ analytics    │ │ ▦ archived(1)│ ← archived behind    │
│ │ ●ready ①     │ │ ◐degraded    │ │   filter chip│   filter, restorable │
│ │ 3 services   │ │ 2 services   │ └──────────────┘                      │
│ │ $14.20 mtd ② │ │ $9.10 mtd    │                                       │
│ │ prod +1 env  │ │ prod         │                                       │
│ └──────────────┘ └──────────────┘                                       │
└──────────────────────────────────────────────────────────────────────────┘
① Card status = worst service state across default env (G2 colors/icons).
② Month-to-date actual (FND-010), not estimate — G4 running-cost placement.
Card click → project (last-used env, §6.4). Table view ≣ for orgs >12 projects.
```

SH-03 (empty): headline "Create your first project" · one paragraph teaching the model (copy C-01) · primary CTA → wizard · secondary "Explore a sample project" ⓵ (read-only demo project, org-scoped, dismissible) · docs link. No other chrome competes.

**State deltas (SH-02):** `empty` → SH-03 replaces grid. `partial` → cost chips show "—" with tooltip "billing data delayed" if FND-010 query fails; cards still render (health > money in degradation order).

### 6.4 Context model mechanics (delta 3; G1 instantiated)

1. **Persistence:** last-used env stored per (user, project); last-used project per (user, org); org from URL or last session.
2. **Deep links win:** a full URL always sets context exactly; persistence only fills gaps.
3. **Post-auth resume:** links followed while logged out re-resolve after PDS-ACC auth completes (J7-3).
4. **Environment switch = filter:** switcher swaps the `{env}` segment of the *current* route and re-renders in place; scroll position preserved where the page shape matches; 150ms crossfade (§11).
5. **Route-not-in-target:** if the current page's object doesn't exist in the target env (e.g., a service page for a service not present there), redirect to nearest ancestor (services list) + toast: copy C-12 ("**db-main** doesn't exist in **staging** — showing Services."). Never a dead end, never a silent jump to overview.
6. **Context conflict (SH-25):** URL org/project/env unknown or access-denied → dedicated frames: `E-CTX-404` "not found or you don't have access" (deliberately ambiguous per SEC-001 enumeration rules), with org-switcher and ⌘K as exits; `deleted-with-grace` shows restore path if permitted.
7. **Switcher anatomy (all three):** type-ahead list, current item pinned top, recent 3, then alphabetical; env switcher rows show region + production marker; footer action per switcher ("New project" / "New environment" / org: "Create organization" hidden ⓵ behind plan rules).

### 6.5 The wizard (P-2, P-3, P-8) — SH-08 → SH-13 → SH-19

**Wireflow W-1 (J1 core / J3 single-service mode):**

```
[SH-02/05 entry]──+ New──►[SH-08 Name&Region]──Continue──►[SH-09 Select]
   cli: steloit project create                    │   ▲
                                                  │   └─"Describe instead"⓵─►[SH-10 AI]──proposal accepted──┐
                                                  ▼                                                        ▼
                                        [SH-11 Configure]◄─────────────────────────────────────────────────┘
                                                  │ Continue
                                                  ▼
                                        [SH-12 Review&Estimate]──⚠Create (T-cost)──►[SH-13 Provisioning]
                                                  │◄──Back edits any step             │ (dashed: steps complete)
                                                  ▼                                   ▼
                                            [abandon→draft saved 24h]        [SH-19 Connect]──first use──►celebration②
   cli ribbon: create --name --region [--services|--describe] → estimate table → y/N → progress → connect block
```

**SH-08:** name field (live slug preview, mono, edit affordance; validation per FND-001 slug rules inline per G5); production region select (FND-009 list, default nearest, one-line explainer C-05 "All services in an environment share its region. You can add more environments later."). Single column, nothing else — first step must feel weightless.

**SH-09:** service cards (Postgres ⓿, Valkey/Storage/Queue ⓵ appear per version) — checkbox cards with one-line role descriptions (C-06..C-09) and from-price ("from $X/mo" per FND-010 floor price); "Describe your app instead ⓵" link switches to SH-10 without losing selections.

**SH-10 (AI path — §12 governs):** textarea "What are you building?" → submit → thinking state (honest: "Analyzing your description…", no fake progress) → **proposal artifact** rendered per PRD-012 contract: recommended service list, each with *reason* chip ("PostgreSQL — user data"), evidence line ("based on: 'SaaS', 'user accounts', 'file uploads'"), estimate delta, and per-item add/remove toggles. Footer: "You choose — nothing is created yet." (C-15). Accept → SH-11 with selections applied; edit → toggles are live; reject → back to SH-09 with nothing changed. AI attribution badge on the proposal card (G9).

**SH-11:** one collapsible section per selected service, all pre-filled with plan defaults, advanced collapsed (G5). Postgres section content = PDS-001 §6.2 (co-owned seam; this PDS fixes only the frame: section header, default-badge, "view as CLI" toggle on every section per G5's signature move).

**SH-12 (P-3 minted here):**

```
┌ Review — ecommerce (production · iad) ──────────────────────────────┐
│ PostgreSQL   starter-1  ①                          $12.00 /mo      │
│ Object Storage                              ~$2.00 /mo at 20GB ②   │
│ ─────────────────────────────────────────────────────────────────  │
│ Estimated monthly total                            $14.00 + usage  │
│ ⓘ Fixed prices exact; ~usage items estimated at typical use. ③     │
│ Free-plan credit applied first. Nothing bills until you confirm.   │
│                                   [Back]   [Create project  $14/mo]│
└─────────────────────────────────────────────────────────────────────┘
① config summary links back to its SH-11 section (Back preserves everything)
② usage-based dimensions always ~-prefixed with the assumption stated (G4)
③ estimate-unavailable state: if FND-010 estimate call fails, the confirm
  button is REPLACED by "Retry estimate" — creation without a shown cost is
  forbidden by G4; CLI identically refuses without --acknowledge-no-estimate.
```

**SH-13 (P-8 minted):** ordered step list (Create project → Create environment → Provision PostgreSQL → …) each with state glyph (pending/running/done/failed), elapsed time, and log-line disclosure; overall progress honest to FND-002 async-op statuses — no synthetic percentages. Per-step failure: step turns failed with error (G6 anatomy) + `Retry step` + "everything completed stays" reassurance (C-20); wizard never rolls back silently. Completion → celebration ① (single confetti-free moment: check animation + "**ecommerce** is live." C-21) → auto-advance to SH-19 after 1.5s or on click.

**SH-19 Connect (P-9, co-minted with PDS-001):** tabs `CLI` (default: `steloit env pull` copy block + what it writes) · `Platform sync ⓵` (Vercel/Netlify export cards) · `Manual` (per-service connection blocks — reveal-on-click credentials, copy buttons, and warning C-41: "Manually copied credentials won't update when rotated. Prefer `env pull` or sync."). Right rail: "waiting for first connection…" live indicator → flips to celebration ② ⓵ on FND-004 first-use event ("Connected. You're building. →" C-22, links to service page).

### 6.6 Topology (P-4) — SH-06 + overview embed

Rendering rules: nodes = services (type icon, name, G2 status ring); edges = bindings (arrow consumer→target; dashed while binding `provisioning`); layout deterministic (same project always draws the same — layered left-to-right by dependency direction, stable ordering by creation time) so the map becomes spatial memory, never a re-shuffling graph. Compact embed (overview): max 8 nodes then "+N more →" into full canvas. Full canvas: pan/zoom (trackpad + `+/-`), fit-on-load.

Interactions: node click → service page · node focus + `Enter` = click · edge click → binding side panel (scope, credentials age, created-by, `Unbind` ⚠T2) · canvas `+ Add service` (→ J3 wizard mode) and `+ Bind` ⓵ (source→target picker; disabled with teaching tooltip until ≥2 bindable services) · right-click/`.` menu on node = the service's lifecycle verbs (FND-003) it exposes. Empty state: ghost node + C-25 "Your project's shape appears here. Add a service to draw the first node." Single-node state: node + ghost hint toward Connect.

Non-visual traversal: §10.3.

### 6.7 ⌘K palette (P-5) — SH-20

Modes by prefix: *(none)* search · `>` commands · `?` ask ⓵ (policy-gated per FND-007 AI policy; hidden entirely when disabled — no teasing). Anatomy: input → grouped results (Jump to / Actions / Docs ⓵ / Ask ⓵) → footer keymap hints. Ranking per §4.5. Result rows: icon, title, context breadcrumb (mono slugs), right-aligned type tag; actions show their consequence scope ("Create service — in ecommerce/production"). Destructive verbs are **excluded** from the command registry (G3: never one keystroke; registry rule R-CMD-1). `?` mode renders answers as G9-attributed cards with citations into docs, and hands off to a full assistant surface only via PDS-012 (link stub ⓵). No-results state offers scope-widening ("Search all of Acme") and docs search ⓵.

Command registry v0 (complete): `New project · Add service · New environment · Switch environment → · Switch project → · Open settings (org/project/env/user) · Copy current URL · Toggle theme · Go to Overview/Services/Topology/Environments · Invite member (→PDS-007 slot) · View notifications · steloit CLI setup`.

### 6.8 Notification inbox (P-6) — SH-21

Categories (delta 5): `Alerts` (PDS-005 events) · `Backups` · `Billing & budget` · `Membership` · `Lifecycle` (provision/archive/destroy results) · `System` (platform incidents, maintenance). Grouping: by project, newest first, unread bold; category filter chips; bulk "mark read". Quiet-by-default thresholds: only `Alerts:firing`, `Backups:failed`, `Billing:budget-exceeded`, `Lifecycle:failed`, `System:incident` produce badge-count events; everything else accumulates silently (G6/GOV-002 §4.5 severity discipline — every default interruption above is justified as action-required). Row click → deep link to the object at the event's timestamp. Footer: "Notification settings →" (user prefs SH-17: per-category × per-channel matrix — in-app/email/Slack ⓵, project-level channel config lives in project settings ⓵ per GOV-002 §4.5 "configured once per project").

### 6.9 Settings frames (P-7) — SH-14/15/16/17

One frame pattern: left section rail, right content, every row = setting name + one-line consequence + control; searchable via ⌘K (sections indexed with synonyms). Placement obeys GOV-002 §4.6's test; the complete v0/v0.5 placement table:

| Setting | Scope | Section | Notes |
|---|---|---|---|
| Org name/slug/avatar | Org | General | slug rename: 30d window then locked (FND-001) |
| Members, teams, roles | Org | Members | slot → PDS-007 |
| Billing, plan, invoices ⓵ | Org | Billing | slot → PDS-009 |
| Policies ⓵ | Org | Policies | policy rows per FND-007; AI enablement toggle lives here |
| Audit log ⓵ | Org | Audit | event stream view (read-only table, FND-008) |
| Project name/slug | Project | General | rename keeps redirect 90d |
| Labels & folder | Project | General | |
| Git integration ⓶ | Project | Integrations | frame reserved |
| Notification channels ⓵ | Project | Notifications | Slack webhook etc. |
| Danger: archive/destroy | Project | Danger zone | §6.12 |
| Env name | Env | General | production rename blocked (C-33 explains) |
| Region | Env | General | read-only + explainer (FND-009) |
| Secrets | Env | Secrets | slot → PDS-008 |
| Env danger: destroy | Env | Danger zone | §6.12 |
| Profile, email | User | Profile | |
| Theme (system/dark/light) | User | Appearance | G11 |
| Notification prefs matrix | User | Notifications | §6.8 |
| CLI sessions/tokens view | User | Sessions | read-only list + revoke (revoke ⚠T1) |

### 6.10 First-run & celebrations

First-run = SH-03 + wizard with two additions: progress breadcrumb copy in teaching voice (C-02 series) and a dismissible "how Steloit is organized" inline diagram (org→project→env→service, 4 nodes, one sentence — the model taught once, visually, at the exact moment it's relevant). Celebrations: exactly the two moments in §1; each is a single check-draw animation + one sentence; both respect reduced-motion (§10/§11); neither ever repeats for that project.

### 6.11 Keyboard map (shell scope, within G10)

`⌘K` palette · `⌘\` nav collapse · `g o/s/t/e` go Overview/Services/Topology/Environments · `g ,` project settings · `[` `]` cycle environments (with C-12 rule on missing routes) · `n p` new project (from home) · `?` shortcut sheet · `Esc` closes topmost layer, never navigates. All destructive actions: unreachable in fewer than two deliberate interactions (G3), tab-order places Cancel before Confirm in every ⚠ dialog.

### 6.12 Retire flows (workflow 7 concrete) — wireflow W-2

```
[SH-15 Danger zone]──Archive──►[SH-22 ⚠T1: consequence summary]──Confirm──►(dashed: services suspend,
        │                        "stops services, keeps data, $0 compute;     snapshot; card → archived)
        │                         restore anytime" C-35                       toast + inbox Lifecycle event
        └────Destroy──►[SH-23 ⚠T3]: consequences list → type project slug →
                        [Delete in 7 days] ──►(dashed: state=deleting, banner
                        "Deletes {date}. [Restore]" on all project surfaces;
                        T3 delay = the grace period itself, per FND-001)
[SH-16 Danger zone]──Destroy env──►[SH-24 ⚠T2 typed-name]──►(dashed: 72h grace)
    production env: control disabled + C-38 "Production can't be deleted while
    the project exists. Destroy the project instead." (policy FND-007 default)
cli: project archive|delete (typed slug re-entry required in TTY; --yes honored
     only with --slug matching, never bare), env delete idem. Exit code 7 on
     confirmation mismatch.
```

### 6.13 Consolidated state tables (deltas over B-0)

**SH-04 Project overview**

| State | Entry | Display | Exit |
|---|---|---|---|
| ready | all sections resolve | health strip (worst-state summary sentence C-28) · compact topology · cost chip mtd · recent events (10) | — |
| degraded-mix | ≥1 service not `ready` | health strip names the culprits ("2 of 3 ready — **cache** degraded"), topology rings show it; page itself is calm (G8: signal, not alarm) | services recover |
| env-empty | env has 0 services | teaching state + Add service | service added |
| partial | events or cost query fails | affected column shows section error, rest intact | retry |

**SH-13 Provisioning (per FND-002 async op)**

| State | Entry | Display | Actions | Exit |
|---|---|---|---|---|
| running | op accepted | step list live; elapsed; cancel disabled after first irreversible step (marked) | view logs | all done → done; step error → step-failed |
| step-failed | step status=failed | failed glyph + G6 error + retry step; completed steps stay | retry, abandon-keep (explains what exists) | retry → running |
| done | op complete | celebration ① → advance | continue | → SH-19 |

**SH-10 AI describe** ⓵

| State | Entry | Display | Actions | Exit |
|---|---|---|---|---|
| idle | tab opened | textarea + examples of good descriptions (C-14) | submit | → thinking |
| thinking | submitted | honest wait line; cancel | cancel | proposal / error |
| proposal | artifact returned | full artifact per §12.2 | toggle items, accept, edit desc, reject | accept → SH-11 |
| error | provider failure | "Couldn't analyze that — pick services manually?" (C-16) + switch to SH-09 preserving text | retry, manual | — |
| disabled | FND-007 AI policy off | tab absent entirely (no teasing) | — | — |

**SH-20 ⌘K:** idle (recents shown) / results / no-results (scope-widen) / command-armed (action rows show target context before Enter) / ask-thinking ⓵ / ask-answer ⓵. **SH-21:** B-0 with empty = "All quiet. You'll hear about what matters." (C-30). **SH-06:** B-0 + empty/single-node per §6.6; `edge-provisioning` renders dashed. **Switchers:** loading (cached list instantly, refresh silently), long-list (>50 → server search). **SH-22/23/24:** confirm → working (button spinner, controls locked) → done (dashed exits per W-2); mismatch on typed slug = inline error, no shake animation (§11 restraint).

---
## §7 — Surface Parity Specification **[M]**

Every §6 capability, four surfaces. CLI cells give: command · key flags · human output shape · `--json` (always the FND-002 resource envelope) · exit codes (0 ok · 1 error · 3 not-found · 4 forbidden · 7 confirmation-mismatch, per EXP-003). Docs cells name the EXP-007 doc type owed.

| Capability | Console | CLI | API (FND-002) | Docs |
|---|---|---|---|---|
| Set/inspect context | Context bar (SH-01) | `steloit use <org>/<proj>[/<env>]` · `steloit context` prints triple; per-command `--org --project --env` override (precedence: flags > env vars `STELOIT_*` > `steloit use` > none→error E-CTX-000) | n/a (client concept; API paths are always explicit) | task: "Working with context" |
| List projects | SH-02 | `steloit projects list [--label k=v] [--archived]` → table name/status/services/mtd-cost | `GET /orgs/{org}/projects` | ref |
| Create project (manual) | Wizard SH-08→13 | `steloit project create [name] [--region iad] [--services postgres,storage]` → interactive for gaps → estimate table → `Proceed? [y/N]` → progress lines → connect block | `POST …/projects` then `POST …/services` + `GET /operations/{id}` | task: "Create a project" |
| Create project (describe) ⓵ | SH-10 | `… --describe "text"` → proposal rendered as annotated table (service·reason·est) → `[a]ccept all / toggle by number / [m]anual` | `POST …/recommendations` → proposal id → create referencing it | concept: "How recommendations work" |
| Configure step | SH-11 | prompts show `[default]`; every prompt's flag named in-line ("--size starter-1") | service create bodies | ref per service |
| Estimate & confirm | SH-12 (P-3) | itemized table, identical line items and ~ markers; refuses without estimate unless `--acknowledge-no-estimate` | `POST /estimates` (FND-010) | concept: "How estimates work" |
| Provisioning progress | SH-13 (P-8) | streamed step lines `[2/4] Provisioning PostgreSQL… done (34s)`; `--no-wait` prints op id; `steloit ops watch <id>` | `GET /operations/{id}` | ref |
| Connect / env pull | SH-19 | `steloit env pull [--format dotenv|json]` writes `.env.steloit` + prints injected names; `--print` to stdout | `GET …/envs/{env}/config` (FND-004 injection doc) | task: "Connect your app" |
| Services list | SH-05 | `steloit services list [--type data]` | `GET …/services` | ref |
| Topology | SH-06 | `steloit status --graph` (ASCII nodes/edges + states); `steloit bindings list` | `GET …/services?expand=bindings` | concept: "Topology" |
| Project health | SH-04 | `steloit status` → health sentence + per-service table + mtd cost | composite GETs | task |
| Environments | SH-07/26 | `steloit env list` · `env create <name> --region iad` | `POST …/envs` | task |
| Env switch | Switcher (filter rule §6.4) | `steloit use …/<env>` or `--env` | path segment | task |
| Search | ⌘K search | **Justified partial** (DX-lead sign-off DX-SO-1): list commands + shell/OS grep are the CLI's native search; a bespoke `steloit find` adds no capability. Deep-link parity via `steloit open <resource>` (prints/opens Console URL) | `GET /search?q=` ⓵ powers Console | ref |
| Commands (`>` mode) | ⌘K commands | **The CLI itself is this mode** (justified identity, DX-SO-2); registry R-CMD maps 1:1 to commands | — | — |
| Ask ⓵ | ⌘K `?` | `steloit ask "…"` (PDS-012 surface; stub here) | PRD-012 endpoint | concept |
| Notifications | SH-21 | `steloit notifications list [--unread] [--category alerts]` · `notifications ack <id|--all>` | `GET/PATCH /notifications` | ref |
| Settings read/write | SH-14–17 | `steloit org|project|env|config get/set <key> [value]` — every §6.9 row's key named in ref docs | `PATCH` resources | ref: "Settings keys" |
| Archive project | SH-22 ⚠ | `steloit project archive <slug>` → consequence summary → confirm | `POST …/archive` | task |
| Destroy project | SH-23 ⚠T3 | `steloit project delete <slug>` → consequences → **retype slug** (TTY) or `--yes --slug <slug>` → prints grace deadline + restore command | `DELETE …` (grace per FND-001) | task: "Deleting safely" |
| Restore (grace) | Banner action | `steloit project restore <slug>` | `POST …/restore` | task |
| Destroy env | SH-24 ⚠T2 | `steloit env delete <name>` idem; production → error E-ENV-PROT + C-38 text | `DELETE …/envs/{env}` | task |
| Theme/user prefs | SH-17 | `steloit config set theme dark` etc. | user prefs resource | ref |

Zero unjustified single-surface rows; two justified partials (search, commands) carry DX sign-offs recorded above.

---

## §8 — Component & Pattern Consumption **[M]**

**Consumed (DES-005/004, by registry name):** `nav-shell` · `table` (dense, keyboard) · `card` · `form/*` (G5 set: field, select, slug-input, section-collapse, view-as-cli toggle) · `dialog` + `dialog-danger/T1|T2|T3` · `status-badge` (G2) · `toast` · `tabs` · `sparkline` · `code-block` (copy, reveal variants) · `filter-chips` · `side-panel` · `skeleton/*` · `empty-frame` · `error-frame` · `time` (G7 formatter) · `cost-chip`.

**Minted here (per §3.3):** P-1…P-9, each entering the registry with this PDS as source and a DES-005 spec page authored by the Systems track from §6's definitions.

**Library requests (blocking ⛔ / non-blocking ○):**

| Req | Component | For | Flag |
|---|---|---|---|
| LR-1 | `switcher-menu` (type-ahead, pinned-current, recents) | context bar ×3 | ⛔ |
| LR-2 | `step-progress` (async op steps, per-step states) | SH-13, base of P-8 | ⛔ |
| LR-3 | `graph-canvas` (pan/zoom, deterministic layout, a11y traversal API) | P-4 | ⛔ |
| LR-4 | `proposal-card` (G9 artifact: reason/evidence/toggles/attribution) | SH-10 | ⛔ for ⓵, ○ for ⓿ |
| LR-5 | `inbox-list` (grouped, read-state, category chips) | P-6 | ⛔ |
| LR-6 | `celebration-check` (single-use, reduced-motion-aware) | §6.10 | ○ |
| LR-7 | `banner` (suspension/grace/incident variants) | §6.4, W-2 | ⛔ |

---

## §9 — Content Specification **[M]**

### 9.1 Terminology (GOV-003 submissions/confirmations)

Confirmed nouns used verbatim: Organization, Project, Environment, Service, Binding, Notification, Archive, Destroy (never "Remove" for irreversibles; "Remove" reserved for reversible dissociations). New submissions: **Connect** (the post-provision act; verb + screen name), **Describe** (the AI wizard path label), **Topology** (canonical name of the map — not "Graph", not "Canvas"). R1 confirmations rendered: CLI noun `steloit db`; slugs lowercase-kebab, mono everywhere.

### 9.2 Microcopy inventory (complete for v0/⓵ screens; IDs stable for Figma annotation)

| ID | Location | Copy |
|---|---|---|
| C-01 | SH-03 body | "A **project** holds everything one application needs — its services, environments, and settings. Create one and Steloit will help you provision what it needs." |
| C-02 | Wizard breadcrumb (first-run) | Step names: "Name it · Choose services · Configure · Review · Done" |
| C-03 | SH-08 name field help | "Lowercase letters, numbers, and dashes. This becomes your project's address." |
| C-04 | SH-08 slug preview | "app.steloit.com/acme/**{slug}**" |
| C-05 | SH-08 region help | "All services in an environment share its region. You can add more environments later." |
| C-06 | SH-09 PostgreSQL card | "PostgreSQL — your application's database. Users, records, relations." |
| C-07 ⓵ | Valkey card | "Valkey — in-memory speed. Sessions, caching, rate limits." |
| C-08 ⓵ | Storage card | "Object Storage — files and uploads. S3-compatible." |
| C-09 ⓵ | Queue card | "Queue — background work. Emails, jobs, events." |
| C-10 | SH-09 footer | "Only what you select gets created. Add or remove services anytime." |
| C-11 | Estimate ~ tooltip | "Usage-based. Estimated at typical starter usage; you pay for actual use." |
| C-12 | Env-switch toast | "**{service}** doesn't exist in **{env}** — showing Services." |
| C-13 ⓵ | SH-10 heading | "Describe what you're building" |
| C-14 ⓵ | SH-10 placeholder | "e.g. A SaaS tool with user accounts, file uploads, and a weekly email digest" |
| C-15 ⓵ | SH-10 footer | "You choose — nothing is created yet." |
| C-16 ⓵ | SH-10 error | "Couldn't analyze that. Pick services manually — your description is saved." |
| C-17 ⓵ | Proposal reason chips | pattern: "{Service} — {need}" e.g. "PostgreSQL — user data", "Storage — file uploads" |
| C-18 ⓵ | Proposal evidence line | "Based on: {quoted fragments from your description}" |
| C-19 | SH-12 assurance | "Nothing bills until you confirm." |
| C-20 | SH-13 step-fail | "This step failed — everything already created is safe. Retry when ready." |
| C-21 | Celebration ① | "**{project}** is live." |
| C-22 ⓵ | Celebration ② | "Connected. You're building. →" |
| C-23 | SH-13 irreversible marker | "From this step on, resources exist and may bill." |
| C-24 | SH-19 CLI tab | "Run this in your app's directory:" + block; "Writes **.env.steloit** with your connection settings. Add it to .gitignore." |
| C-25 | Topology empty | "Your project's shape appears here. Add a service to draw the first node." |
| C-26 | Topology single-node hint | "Connect your code → " (links SH-19) |
| C-27 | + Bind disabled tooltip ⓵ | "Binding connects two services. Add another service first." |
| C-28 | Overview health sentence | patterns: "All {n} services ready." / "{k} of {n} ready — **{name}** {state}." |
| C-29 | Overview events empty | "No recent activity in {env}." |
| C-30 | Inbox empty | "All quiet. You'll hear about what matters." |
| C-31 | Forbidden frame | "You don't have access to this. Your role: **{role}**. Ask an org admin, or request access." |
| C-32 | Offline pill | "Reconnecting… data as of {time}" |
| C-33 | Env rename blocked | "**production** keeps its name — tools and policies depend on it." |
| C-34 | Region read-only | "Region is set when an environment is created. Need another region? Create a new environment." |
| C-35 | SH-22 body | "Archiving stops all services and compute billing. Data is snapshotted and kept. You can restore **{project}** anytime." |
| C-36 | SH-23 body | "This deletes **{project}** and every environment, service, and byte of data in it. It stays restorable for **7 days**, then it's gone permanently." |
| C-37 | SH-23 input label | "Type **{slug}** to confirm" |
| C-38 | SH-24 production | "Production can't be deleted while the project exists. Destroy the project instead." |
| C-39 | Grace banner | "**{project}** deletes {date}. [Restore]" |
| C-40 | Suspension banner | "This organization is suspended ({reason-category}). Data is safe; actions are read-only. → Resolve" |
| C-41 | Manual connect warning | "Manually copied credentials won't update when rotated. Prefer `env pull` or platform sync." |
| C-42 | First-run model diagram caption | "One org. Projects for each app. Environments to keep prod safe. Services inside." |

### 9.3 Error catalog (Shell-owned; codes pinned to FND-002 r2 taxonomy)

| Code | Trigger | Console message | CLI (same words + code) | Next step offered |
|---|---|---|---|---|
| E-CTX-404 | unknown/unauthorized org, project, or env in URL | "**{slug}** wasn't found — it may not exist, or you may not have access." | idem, exit 3/4 | switch org · ⌘K · request access |
| E-CTX-410 | object in delete-grace | "**{slug}** is scheduled for deletion ({date})." | idem | Restore (if permitted) |
| E-CTX-000 | CLI: no context resolvable | "No context set. Run `steloit use <org>/<project>` or pass --project." | exit 2 | — |
| E-SLUG-422 | invalid name/slug | "Names use lowercase letters, numbers, dashes; start with a letter." | idem, exit 2 | inline fix |
| E-SLUG-409 | slug taken | "**{slug}** already exists in {org}." | idem | suggest `-2` variant |
| E-QUOTA-402 | plan limit (FND-011) | "Your **{plan}** plan allows {n} {resource}. " | idem, exit 5 | view plans (no dark pattern: equal-weight "manage existing" action) |
| E-EST-503 | estimate unavailable | "Can't fetch the estimate right now — creation is paused so you're never billed blind." | refuses; `--acknowledge-no-estimate` documented | retry |
| E-OP-STEP | provisioning step failure | C-20 + provider detail + ref-ID | step line + ref-ID | retry step |
| E-ENV-PROT | production destroy attempt | C-38 | exit 4 | destroy project path |
| E-CONF-7 | typed confirmation mismatch | "That doesn't match **{slug}**." | exit 7 | retype |
| E-AI-503 ⓵ | recommendation failure | C-16 | idem | manual path |

### 9.4 Docs obligations (EXP-007)

Concepts: *How Steloit is organized* · *How estimates work* · *How recommendations work* ⓵ · *Topology*. Tasks: *Create a project* · *Connect your app* · *Work with environments* · *Delete safely* · *Working with context (CLI)*. Reference: full settings-key table · notifications categories · CLI commands (generated).

---

## §10 — Accessibility Specification **[M]** *(beyond the G11 floor; each item testable at G-C)*

1. **Context announcement:** on any context change (switch or deep link), screen readers receive one polite announcement: "Now viewing {project}, {env}." — never three separate region updates.
2. **Switchers:** full combobox pattern (type-ahead, arrow nav, `aria-activedescendant`); production rows announce "production environment" (the G8 marker is never color-only — the word is always present).
3. **Topology non-visual traversal:** `graph-canvas` (LR-3) exposes a linear traversal: Tab enters map → arrow keys move node→node in layout order → Enter opens → `b` lists the focused node's bindings as a menu → Esc exits. A visually-hidden "list view" toggle renders the identical data as nested lists (services → their bindings) — same information, zero graph dependence.
4. **Wizard focus:** step transitions move focus to the step heading; SH-13's live progress is a single `aria-live=polite` region announcing step completions only (not elapsed-time ticks); step failure switches to `assertive` once.
5. **Estimate panel:** the total is programmatically associated with its caveat line (C-11/C-19) — cost is never announced without its qualifier.
6. **Danger dialogs:** `role=alertdialog`; consequences list read before input; typed-confirmation field labeled with C-37 including the literal slug; Cancel is first in tab order.
7. **Celebrations:** decorative animations `aria-hidden`; the sentence (C-21/22) is the announced content; reduced-motion replaces draw animation with instant state (§11).
8. **Status semantics:** every G2 badge = icon + word (never color alone); topology rings likewise carry `aria-label="{name}, {state}"`.
9. **⌘K:** listbox semantics, results-count announcements, mode changes announced ("Command mode").
10. **Density (G10) vs zoom:** all layouts functional at 200% zoom / 320px logical width; nav collapse behavior specified at §6.2 covers reflow.

---

## §11 — Motion Specification **[C — applies: shell owns three motions]**

Global system (DES-004 motion chapter) inherited for all transitions. Shell-specific, each purposeful, token-timed, reduced-motion-safe:

1. **Environment-switch crossfade (G1's feel):** 150ms `--motion-fast` crossfade of the content slot, context bar env chip slides 4px; communicates "same place, different lens." Reduced-motion: instant swap, chip state change only.
2. **Topology settle:** on load/added node, nodes ease into deterministic positions over 250ms once; never continuous physics (spatial memory rule §6.6). Reduced-motion: appear in place.
3. **Celebration check-draw:** 400ms single stroke, twice ever per project (§6.10). Reduced-motion: static check.
Explicitly rejected: shake-on-error (§6.13), skeleton shimmer beyond `--motion-subtle`, animated confetti (register: calm).

---

## §12 — AI Behavior Specification **[C — applies]**

### 12.1 Registered capabilities (PRD-012 registry — complete list for this PDS)

| Cap ID | Capability | Surface | Version |
|---|---|---|---|
| AIC-SHELL-1 | Provisioning recommendation (describe → proposal) | SH-10, wizard + CLI `--describe` | ⓵ |
| AIC-SHELL-2 | ⌘K `?` ask — *entry point only*; conversation surface is PDS-012's | SH-20 | ⓵ stub |

No other AI affordances exist in the Shell. Never-zones honored: no AI presence in settings frames, danger dialogs, IAM slots, or notification actions.

### 12.2 AIC-SHELL-1 — the proposal artifact rendered (PRD-012 contract fields → UI, R4 resolution)

| Contract field | Rendering |
|---|---|
| `recommendations[] {service, reason}` | service row: type icon, name, reason chip (C-17 pattern) |
| `evidence[] {fragment}` | evidence line under the list (C-18) quoting the user's own words — the AI shows its reading, verbatim, no paraphrase |
| `estimate` | delta rows feeding the standard P-3 panel (same component — AI costs are never displayed differently than human-chosen costs) |
| `confidence` | low-confidence renders as "Not sure about: {service} — {reason}?" row with default-off toggle; never a percentage (numbers imply precision the field doesn't have) |
| attribution | G9 badge "Suggested by Steloit Assistant" + "why?" popover explaining the capability in one paragraph |

Laws rendered: proposal is inert until the user acts (C-15); every item toggleable; accept routes into the normal wizard — the AI path and manual path converge at SH-11 and are indistinguishable downstream; the audit event (SEC-003) records `user X via assistant, proposal {id}` on creation. Failure/uncertainty: §6.13 SH-10 table. Policy: FND-007 AI toggle removes the path entirely (no upsell ghost).

### 12.3 AIC-SHELL-2

`?` prefix reveals ask mode only when policy-enabled; answers render as attributed cards with doc citations; any action suggestion is a deep link to the real (permissioned) surface — the ask mode never executes. Full conversation UX: PDS-012 (cross-ref, not designed here).

---

## §13 — Cost & Danger Map **[M]**

### 13.1 Cost map (G4 — every recurring/significant cost and where it renders)

| Action | Cost nature | Estimate shown | Running actual shown |
|---|---|---|---|
| Create project w/ services | recurring | SH-12 panel (itemized, ~ marked) — confirmation impossible without it (E-EST-503) | project card chip (SH-02), overview chip (SH-04) |
| Add service (J3) | recurring | same P-3 panel in single-service mode | service card/row chip |
| Create environment | recurring (its services, later) | SH-26 shows "$0 until you add services" (honest zero) | env rows in SH-07 |
| Archive project | *saves* money | SH-22 states "compute billing stops" (C-35) | card shows $0.00 mtd + archived |
| Restore archived | resumes billing | restore dialog restates prior monthly figure before confirm | resumes chips |
| Destroy project/env | ends billing after grace | SH-23/24 state final-snapshot retention per policy | — |

### 13.2 Danger map (G3 — every destructive/expensive action, tier, undo story, copy)

| Action | Tier | Undo/grace | Copy |
|---|---|---|---|
| Archive project | T1 + consequence summary | fully reversible (Restore) | C-35 |
| Destroy project | **T3**: consequences → typed slug → grace-as-delay | 7-day restore window; banner C-39 everywhere | C-36/37 |
| Destroy environment (non-prod) | T2 typed name | 72h grace | mirrors C-36 scaled |
| Destroy production env | **blocked** | — | C-38 |
| Unbind (edge) ⓵ | T2 | re-bind creates new credentials (stated) | panel copy |
| Revoke CLI session | T1 | re-login | — |
| Cancel provisioning (pre-irreversible marker) | T1 | nothing existed | C-23 marks the boundary |

Reviewed line-by-line against GOV-002 §5's law: archive (reversible) is one click; destroy (irreversible) is typed + 7 slow days; everything expensive shows its number first. ✔

---

## §14 — Instrumentation, Assumptions & Open Questions **[M]**

### 14.1 Instrumentation (DES-008 hooks; event names final)

J1 funnel: `shell.firstrun.viewed → wizard.step.viewed{step} → wizard.describe.submitted⓵/proposal.decided{accept|edit|reject}⓵ → wizard.estimate.viewed → wizard.confirmed → provision.completed{duration} → connect.viewed → connect.first_use{method, t_from_signup}` — `t_from_signup` *is* the five-minute metric. Plus: `context.env_switched{via}` · `palette.opened{mode}` / `palette.actioned` · `topology.node_opened` / `topology.a11y_list_used` · `inbox.opened` / `notification.actioned{category}` · `danger.flow{action, tier, completed|abandoned}` · wizard abandonment per step. Benchmarks: J1 ≤ 5min p50; J2 into-a-service ≤ 10s p75; env-switch render ≤ 200ms p95.

### 14.2 Assumptions register

| # | Assumption | Validation |
|---|---|---|
| A-1 | R1 resolutions (noun `db`, kebab slugs) hold through GOV-003 final | confirmed at G-A |
| A-2 | R2: `env pull` + sync + manual-with-warning covers ≥90% of v0 external-runtime connects | connect-method telemetry, 30d post-v0 |
| A-3 | R3: branch costs surface in PDS-001's tabs, not the Shell wizard (wizard creates no branches) | PDS-001 G-B cross-check |
| A-4 | R4 proposal contract fields (§12.2) are PRD-012-final | PRD-012 approval before ⓵ build |
| A-5 | Deterministic topology layout stays legible to ~12 nodes; beyond that needs clustering ⓶ | usability study at P2 (4-service projects) |
| A-6 | Two-mode ⌘K (search/commands merged ranking) beats three separate pickers | palette study, D2 |
| A-7 | Quiet-inbox thresholds (§6.8) won't under-notify | complaint-rate + missed-alert audit, v0.5 |

### 14.3 Open questions (each owned; register must be empty/accepted at G-C)

| # | Question | Owner | Deadline |
|---|---|---|---|
| Q-1 | Does `env pull`'s file (`.env.steloit`) need a lockfile-style checksum to detect manual edits before overwrite? | DX design + FND-004 author | before G-B |
| Q-2 | Sample project ⓵ (SH-03 secondary): real provisioned demo (costs us) vs static tour? | PM + BIZ-001 owner | before ⓵ build |
| Q-3 | Org switcher "Create organization": plan-gating rules unwritten (BIZ-003) | BIZ-003 owner | before ⓵ build |
| Q-4 | Suspension banner (C-40) reason categories: final taxonomy from SEC-007/BIZ-003 pending | SEC-007 author | before v0 GA copy freeze |

---

## Appendix A — Wireflow index
W-1 wizard/J1 core (§6.5) · W-2 retire flows (§6.12) · Environment-switch behavior (§6.4 rules 4–5, exercised in J2 map) · J1 end-to-end board lives in Figma `02-Journeys/J1` with this document's screen IDs as frame names.

## Appendix B — Gate checklists
G-A material: §§1–5 (this draft). G-B material: §§6–14 + LR-1..7 triage + PDS-001 seam review (SH-11 Postgres section, SH-19/P-9 co-mint). Figma G-C will be audited screen-by-screen against §6.1's inventory, §6.13's states, §9.2's copy IDs, §10's ten assertions.

*— End of PDS-SHELL v1.0 draft —*
