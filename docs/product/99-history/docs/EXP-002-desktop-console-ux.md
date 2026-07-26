# EXP-002 — Steloit Console: Desktop UX Exploration & Product Experience

**Document class:** Experience Exploration (EXP)
**Status:** Draft v1.0 → input to Figma phase; subordinate to PDS-SHELL for anything both specify
**Pinned authorities:** GOV-001 · GOV-002 · GOV-005 (G1–G11, J1–J11, W1–W7) · DES-000 · DES-009 · PDS-SHELL v1.0
**Rule of precedence:** where this document and PDS-SHELL both speak, PDS-SHELL wins (it is the gated spec; this is the connective tissue around and beyond it). Where this document speaks alone — mental models, multi-window, tab semantics, discoverability, productivity ergonomics, responsive strategy — it is the authority of record until promoted into DES/PDS documents.
**Non-goals honored:** no visuals, no color, no type, no high-fidelity anything. This is the UX of the desktop Console as *behavior*.

---

## 1 — UX Philosophy

Seven laws, each derived from an approved authority, each falsifiable in review:

**L1 — The URL is the application state.** (FND-002, PDS-SHELL §4.3.) Every screen, every filter worth sharing, every selected object is addressable. Copy the address bar at any moment and a teammate lands *exactly* where you are, permission-checked. Consequences: browser Back always works and never loses data; refresh is always safe; "send me a link" is the platform's native collaboration primitive — before any realtime feature exists, deep links *are* multiplayer.

**L2 — Context is worn, not visited.** (G1.) Org/project/environment is a lens you look *through*, rendered persistently in the context bar, not a place you go. Nothing in the Console ever asks "which environment?" mid-task — the answer is always already on screen. The test: a screenshot of any Console screen, cropped to any quadrant containing the top bar, answers where/what/which-lens.

**L3 — Ten-second truth.** (J2.) The daily loop — "is my stuff okay, what changed" — must complete in ten seconds from muscle memory: open project, read health sentence, scan topology rings, leave. Every element competing with that read on the overview must justify itself against it.

**L4 — Verbs over destinations.** (G10.) The fastest path to any action is naming it (⌘K), not finding it. Navigation exists for orientation and browsing; the palette exists for intent. Power users should be able to live a full session touching the pointer only for the topology map.

**L5 — The Console teaches the CLI.** (G5.) Every form can be viewed as the CLI command it will run; every wizard completion offers its CLI equivalent; every list's filter state maps to flags. The Console is the CLI's tutorial and the CLI is the Console's script — one grammar, two costumes (EXP-003).

**L6 — Calm is a feature.** (G2, G8, DES-001 register.) Status is stated, not performed. Production is marked, not alarmed. Red is spent only on states demanding action. An incident-free Console session should feel like a quiet, competent tool — the drama budget is reserved for the two celebrations and genuine failures.

**L7 — Never a dead end.** (G6.) Every empty state teaches and offers the first verb. Every error states what/why/next and carries a reference ID. Every 404 offers the switchers and the palette. Every destructive path states its undo story before asking for conviction.

---

## 2 — Mental Models

The Console's job is to install four models in the user's head, in order, mostly without words:

**M1 — "A project is my app's home."** Installed by: first-run diagram (C-42), the wizard's shape (name → services → live), the overview's composition. The project is where a developer's sense of "my stuff" attaches; org is ambient (identity/money), environment is a lens (M2), services are contents.

**M2 — "Environments are parallel universes of the same project."** Installed by: the env switcher's filter behavior — same page, same scroll position, different data, 150ms crossfade. The user learns by *feeling* it: switching never navigates, it re-lenses. Reinforced by the missing-route rule (C-12): the universe metaphor is honest about non-parallel objects instead of pretending.

**M3 — "The topology is my project's true shape."** Installed by: deterministic layout (the map never reshuffles → spatial memory forms), the overview embedding it as hero, and every service/binding being reachable through it. Goal state: users *think in the map* — "the thing to the right of the database" becomes a valid mental address.

**M4 — "Steloit tells me the price before I say yes, and slows me down before I break things."** Installed by: the estimate panel gate (E-EST-503 — the platform literally refuses to proceed blind), ~-marked usage honesty, and the graduated danger tiers. This is the trust model; it converts to willingness-to-add-services (BIZ metric) more than any feature does.

Anti-models to actively prevent: *"the dashboard is where I monitor"* (no — Observe is; the overview is orientation), *"archive = delete"* (copy C-35 and the reversibility asymmetry prevent it), *"AI did something to my account"* (inert-proposal law, §12 PDS-SHELL).

---

## 3 — Information Architecture

Inherited whole from PDS-SHELL §4 (URL grammar, nav tree, Data-item resolution, switcher anatomy). This section adds the *desktop-specific* IA layers PDS-SHELL left implicit:

### 3.1 The three altitudes

The Console has exactly three working altitudes; every screen belongs to one, and the visual/interaction weight of chrome shrinks as altitude drops:

| Altitude | Question answered | Screens | Chrome behavior |
|---|---|---|---|
| **Fleet** (org) | "What do I have?" | Projects home, org settings, inbox | Full nav, context bar shows org only |
| **Project** (default) | "Is it okay? What changed? What's next?" | Overview, services, topology, environments, project/env settings | Full context triple, project nav group active |
| **Object** (service/binding) | "Operate this thing." | Service pages (owning PDSs), side panels | Context triple persists; local tab bar takes over secondary nav |

Rule: no screen may mix altitudes' primary content (a service page never lists other projects; the fleet view never renders service internals). Cross-altitude jumps happen through the context bar, palette, or explicit links — never through ambient content drift.

### 3.2 Primary vs secondary object surfaces

Primary objects (project, environment, service) get destinations. Secondary objects (bindings, notifications, labels, operations) get **side panels** — the desktop's key IA instrument: a right-anchored panel (400–480px logical) that opens over the current screen without navigation, is deep-linkable via `?panel=` (L1), stacks one level max, and closes with Esc. Panels keep the user's place; destinations change it. The decision test for any new object: "does a user ever need to *stay* here for minutes?" → destination; "seconds"? → panel.

### 3.3 Time as an IA dimension (G7)

Recent-events columns, notification rows, and operation logs share one timeline grammar: relative time under 24h ("14m ago"), absolute after, always with full timestamp on hover, always in the viewer's timezone with UTC on hover. "What changed" queries (J2/J4) resolve against this single grammar everywhere — time never needs re-learning per screen.

---

## 4 — Navigation

PDS-SHELL §4.4's tree stands. Desktop behavioral layer:

**N1 — Two-rail model.** Left nav = orientation (where can I be); context bar = position (where am I). They never duplicate: the nav contains no switchers, the bar contains no sections.

**N2 — Nav states.** Expanded (default ≥1100px) · icon rail (user-collapsed via ⌘\, persisted) · overlay (<960px). Collapse is a *user preference*, not a responsive accident — the Console remembers it per user.

**N3 — Back-button contract.** Browser Back traverses *navigations* only: route changes, altitude changes. It does not reopen closed panels, does not undo env switches made <2s ago in rapid `[`/`]` cycling (rapid cycles coalesce into one history entry), never resubmits forms, and never loses wizard state (wizard steps are history entries; Back = the wizard's own Back).

**N4 — Link discipline.** Plain click = navigate. ⌘/middle-click = new tab, *always honored* — every navigable element is a real anchor, no div-with-onClick fakes. Drag a nav item, project card, or service node to the tab bar/another window = the browser's native link drag (real hrefs make multi-window free; §14).

**N5 — Section memory.** Entering a project returns you to your last section *in that project* (overview by default). Entering a service returns to its last-used tab (owning PDS's tabs). Memory is per-user, per-object, expires never; deep links always override (L1 > memory).

---

## 5 — Workspace Hierarchy & Model

"Workspace" is deliberately not a Steloit noun (GOV-002 rejected it as a primitive). At UX level, the *effective workspace* is the (project, environment) pair — the unit a developer inhabits for a working session. Model consequences:

**W1 — Session shape.** A typical session = 1 org · 1–2 projects · 1 env each · N services. The Console optimizes for *depth within one pair* and *cheap glances at a second* (multi-window, §14) — not for fleet-wide dashboards, which is Observe's job ⓵ at org altitude.

**W2 — Personal, not shared, ephemera.** Filter states, collapsed sections, table column widths, nav collapse, last-used tabs: all per-user, all local-first with server sync, none ever visible to teammates. Shared state lives only in real objects (labels, saved views ⓶). This keeps L1 honest: a shared URL reproduces *content position*, not the sender's personal chrome arrangement.

**W3 — Folders and labels (FND-001) at fleet altitude only.** Folders group the projects home; labels filter it and the inbox. Neither ever appears inside project altitude — grouping is a fleet concern.

**W4 — Multi-org reality.** Consultants and agencies hold 3–10 orgs. Org switching is rarer than env switching but higher-stakes (different bill, different identity). Hence: org switcher is leftmost (stable anchor), org changes get a full navigation (never a filter-crossfade — universes with different owners deserve a hard cut), and the palette scopes to current org by default with explicit "search all orgs" escape.

---

## 6 — Dashboard Philosophy & Hierarchy

The Console has exactly one screen called a dashboard by users — the project overview — and its philosophy is **orientation, not observation**:

**D1 — The overview answers three questions in reading order:** Is it okay? (health sentence C-28 + topology rings) → What is it? (the map) → What changed? (events column). Cost chip rides along (G4 running-actual placement). Nothing else earns overview real estate in v0; every future candidate widget must displace nothing and answer one of the three questions better.

**D2 — No configurable dashboards in v0/v1.** Configurability is where dashboard philosophies go to die — it exports the design problem to users. The overview is opinionated and identical everywhere; team-specific monitoring composition belongs to Observe ⓵ (PDS-005), where it's a real workflow with real personas.

**D3 — Hierarchy of glanceability.** Fleet altitude: project cards = one-line health + cost (5s per project). Project altitude: overview = 10s truth. Object altitude: service pages open on their owning PDS's "first tab that answers 'is it okay'" (Kit rule). Each altitude's landing surface is a *summary of the one below*, and every summary element deep-links to its detail — the Console is glanceable top-down and drillable everywhere.

**D4 — Degradation renders proportionally.** One degraded service among three = calm amber presence in sentence + ring (L6). It does not repaint the page, does not banner, does not pulse. Full-env outage = the same grammar at higher count. The *user's* adrenaline is not the Console's to spend; J4's escalation lives in notifications and Observe.

---
## 7 — User Journeys (desktop lens)

The eleven canonical journeys (DES-003) stand. This section re-cuts the shell-relevant ones through desktop-specific behavior; wireflows for each in §20.

| Journey | Desktop-specific commitments |
|---|---|
| **J1 First five minutes** | Single tab, zero forced context decisions (org auto-created by PDS-ACC, production env implicit); wizard is keyboard-completable end-to-end (Tab/Enter/type only); connect block's copy buttons announce copied state; celebration ② arrives even if the user has tabbed away — title-bar dot + inbox event, never a blocking modal on return |
| **J2 Daily loop** | Muscle-memory path: pinned browser tab → `g o` → read → `g s`/node click if needed. Target unchanged (≤10s p75); desktop adds: health readable at icon-rail nav collapse, and from a background tab via title pattern ("● ecommerce — Steloit" flips to "◐ …" on degradation — the tab title is a status surface) |
| **J3 Add capability** | Entry from three places (services list, topology canvas, palette) into the same single-service wizard mode; on completion, topology settle animation spatially introduces the new node relative to existing ones — the map teaches what just changed |
| **J4 Something's wrong** | Entry: inbox row or tab-title flip → overview (which names the culprit, C-28) → node click → service page (owning PDS). Shell's contract: ≤3 interactions from notification to the failing object's detail, context auto-set by the deep link |
| **J7 Bring the team** | Invite (PDS-007 slot) → teammate's deep link honored post-auth → joined-project banner. Desktop: the *sender's* flow is copy-link-from-anywhere (L1) — every screen is an onboarding surface |
| **J9 Retire safely** | Danger zone reachable only through settings (never palette, R-CMD-1); grace banners render across all the project's surfaces and in the fleet card — the countdown is impossible to not-know |
| **J11 Ship a change** ⓶ | Reserved: deployments frame (SH-18); desktop commitments deferred to v2 PDS |

Persona pressure-test (DES-001 personas): *Solo builder* lives J1→J2 in one pinned tab — served by L3/L4. *Platform-minded team lead* lives J2 across 4 projects + J7 — served by fleet altitude, multi-window (§14), link discipline. *Skeptical senior* audits M4 — served by estimate gates, view-as-CLI, audit slots.

## 8 — Workflows (desktop instantiation deltas)

Standard workflows W1–W7 (GOV-005 §5) as instantiated by PDS-SHELL stand. Desktop deltas only:

- **W1 Create & Provision:** wizard holds a draft (24h) keyed to user+org — closing the tab mid-wizard and reopening `/new` resumes with a "Resume draft? started 2h ago / Start fresh" choice. Provisioning (SH-13) survives tab closure: the operation is server-side (FND-002); returning renders current step; completion notifies (inbox + title dot).
- **W2 Connect:** binding creation ⓵ is drag-capable on the topology (drag node→node = open bind panel pre-filled, pointer affordance only, keyboard path via `b` menu remains primary) — drag is sugar, never the only path (§10, P-DD1).
- **W3 Observe & Pivot:** shell contract = every event row anywhere is a deep link carrying (object, timestamp) so Observe ⓵ opens pre-scoped to the moment.
- **W7 Retire:** typed-confirmation fields never accept paste of the slug (deliberateness is the point); CLI parity via exit-code-7 retype rule.

## 9 — Screen Hierarchy

PDS-SHELL §6.1's 26-screen inventory is authoritative. Desktop grouping by chrome shell:

```
Shell chrome (always): SH-01 frame ▸ context bar ▸ nav ▸ status line
├─ Fleet altitude:    SH-02/03 projects home · SH-14 org settings · SH-21 inbox (panel)
├─ Project altitude:  SH-04 overview · SH-05 services · SH-06 topology · SH-07 envs
│                     SH-15/16 project/env settings
│  └─ Object altitude: services/{svc}/* (PDS-001+) · side panels (bindings, ops, events)
├─ Overlay layer:     SH-20 palette · switcher popovers · dialogs SH-22/23/24/26 · shortcut sheet
└─ Takeover layer:    wizard SH-08→13, SH-19 (nav hidden, context bar persists, Esc-guarded)
```

Layering rules: max one takeover, one panel, one popover, one dialog concurrently; Esc pops the topmost only (PDS-SHELL §6.11); focus is trapped per-layer and restored on close to the invoking element — always.

## 10 — Desktop Interaction Patterns

The pattern grammar all screens draw from (beyond DES-005 components):

**P-SEL Selection & bulk.** Tables support single-click row-open (primary cell) and checkbox multi-select; `x` toggles selection on the focused row, `shift-click` ranges. Bulk bar slides in at bottom with the *count and the verbs that survive intersection* of selected objects' capabilities. Destructive bulk verbs inherit the highest tier among members (G3). v0 scope: projects home (label edits, archive) and inbox (mark read).

**P-DD1 Drag as sugar.** Permitted: project cards → folders (fleet), node→node bind ⓵, file drops where owning PDSs accept them. Law: every drag has a keyboard/palette equivalent; drop targets announce on drag-start; no drag-only capabilities, ever.

**P-CTX Context menus.** Right-click (and `.` on focused object) opens the object's verb menu = exactly its FND-003 lifecycle verbs + "Copy link" + "Open in new tab". Menus are generated from the object contract, not hand-curated per screen — one object, same verbs everywhere it appears (card, row, node, panel).

**P-HK Hover economics.** Hover reveals *secondary* affordances (copy-link icons, row verbs) but never *information* required for decisions (G10 density means truth is printed, not hidden behind hover). Tooltips carry supplements (full timestamps, key hints), max 1 sentence, 300ms delay, instant for keyboard focus.

**P-CP Copy behaviors.** Every identifier (slug, connection host, credential, ref-ID) renders with a copy affordance; copied state confirms inline (checkmark swap, 1.2s) + SR announcement. Copying a credential logs an audit event (SEC-003) — stated in the reveal UI, not hidden.

**P-UNDO.** Reversible actions (archive, mark-read, label edits) confirm via toast *with undo verb* (8s window) instead of pre-confirming — G3's inverse: make reversible things frictionless and catchable. Irreversibles never use toast-undo (they use tiers).

**P-FORM.** All G5 rules; desktop adds: ⌘Enter submits any focused form; dirty-state guard on navigation (browser-native beforeunload only for takeovers; in-app nav uses a lightweight "Discard changes?" dialog); errors focus the first invalid field.

**P-RT Real-time discipline.** Live surfaces (status rings, health sentence, step progress, inbox badge) update via push; *lists never reorder under the cursor* — new/changed rows enter with a subtle "1 update" pill the user clicks to merge (calm > live). Offline: B-0 rule (freeze + stamp).

## 11 — Command Palette UX (P-5 deepened)

PDS-SHELL §6.7 defines modes, ranking, registry. Desktop behavioral layer:

- **One palette, three prefixes** — never separate pickers; mode is a state of the same muscle memory. Backspace on empty prefix returns to search mode.
- **Argument flow:** commands taking arguments (Switch environment →) enter a second stage: the palette narrows to the argument list, breadcrumb shows `Switch environment ▸`, Esc backs one stage. Max two stages ever; anything deeper is a form, not a command.
- **Context capsule:** the input row's right edge shows the scope capsule (`ecommerce/production`); actions display their consequence scope inline (PDS-SHELL rule). `Tab` on a Jump-result pins it as scope for a follow-up query ("search *within* this project") — the palette's only power-user secret, taught in the shortcut sheet.
- **Recents & frecency:** empty-query state = 5 frecency-ranked jumps + 3 recent commands; frecency is per-user per-org.
- **Latency budget:** results ≤50ms from local index (projects/envs/services/settings/commands cached), server-backed groups (docs ⓵, all-orgs) stream in labeled sections ≤400ms — the palette never blocks on the network for local intent.
- **No results is a teacher:** offers scope-widen, docs search ⓵, and "Ask Steloit ?" ⓵ handoff with the query preserved.

## 12 — Search UX

Search = the palette's default mode (no separate search page in v0 — one retrieval surface, L4). Semantics: prefix/fuzzy over names+slugs+labels; slug exact-match always rank 1; grouped by object type in altitude order; results show status inline (a degraded service is findable *as* degraded). Scope defaults current-org, capsule-pinnable (§11), all-orgs escape explicit. Non-goals v0: full-text over logs/events (Observe ⓵ owns), doc search ⓵ (streams into palette when EXP-007 ships its index). Metrics: search→open ≤3 keystrokes p50 for recents; zero-results rate <8%.

## 13 — AI UX

Scope discipline: exactly AIC-SHELL-1 (describe→proposal) and AIC-SHELL-2 (ask entry) exist (PDS-SHELL §12). Desktop UX posture for both, and for every future capability by precedent:

- **AI is a colleague who drafts, never an agent who acts.** Proposals are inert artifacts; acceptance routes through the same forms/estimates/tiers as manual work. There is no "AI did X" moment in the Console — only "you accepted X".
- **Evidence before eloquence.** The proposal quotes the user verbatim (C-18); ask-answers cite docs. No unattributed confidence, no percentages, no anthropomorphic delay theater ("thinking…" is honest wait copy, not personality).
- **Spatial containment.** AI renders only inside G9 artifact cards with the attribution badge — never inline-mixed with system truth (status, cost, audit are never AI-touched surfaces). Never-zones re-affirmed: settings, danger, IAM, notifications.
- **Dismissal is free and remembered.** Rejecting a proposal or closing ask leaves no residue, no re-prompting, no "are you sure"; the Describe tab doesn't re-open itself.
- **Policy-off is invisible.** FND-007 disables → surfaces vanish entirely (no locked-feature upsell inside work surfaces).

## 14 — Multi-Window Workflows & Context Preservation

The Console is a browser app; multi-window is inherited power, deliberately amplified:

**MW1 — Everything is a real link (N4)** → any card/row/node/nav item opens in new tab/window natively. Common patterns designed-for: overview + service page side-by-side (J4), staging + production of the same route in two windows (comparison), wizard in one window while reading docs in another.

**MW2 — Windows are independent lenses.** Context (org/project/env) is *per-tab* (URL-derived, L1), never synced across tabs — two tabs on different envs never fight. Personal ephemera (W2) sync lazily; read-states and drafts reconcile last-write-wins with server authority.

**MW3 — Cross-window coherence signals.** Actions completing in tab A that affect tab B's content surface in B as P-RT update pills (never auto-refresh under the user). Inbox badge and title-dot are consistent across tabs within ~2s.

**MW4 — Session restoration.** Browser restart → tabs restore by URL alone (L1 sufficiency test: *no* Console state may exist that a URL + per-user memory can't reconstruct). The wizard draft (W1 delta) survives restarts server-side.

**MW5 — Popout panels.** Any side panel (binding, operation log) offers "open as page" (its `?panel=` URL promoted to a standalone route rendering the panel content full-bleed) — the mechanism that makes panels multi-window-capable without designing two components.

## 15 — Notifications UX

PDS-SHELL §6.8 stands (categories, quiet thresholds, routing). Desktop layer: badge count is cross-tab consistent (MW3); title-dot only for badge-worthy events on *this tab's* project (tab title = per-project status surface, J2); browser-native notifications are opt-in per category ⓵, default off, and deep-link on click; inbox rows support `x` select + bulk mark-read (P-SEL); every row's timestamp obeys §3.3. Anti-goals: no notification center "features" (snooze, pinning) in v0 — triage → deep link → act is the whole loop.

## 16 — Error UX · 17 — Empty States · 18 — Loading Strategy

**Errors (G6 + PDS-SHELL §9.3 catalog):** one anatomy everywhere — *what happened / why (if known) / next step / ref-ID (copyable)*. Scope containment: field < section < page < app; an error never escalates beyond its blast radius (a failed cost query dims the chip, not the page — B-0 partial). App-level (rare): full frame with status-page link. Tone: plain, unapologetic, no "Oops". Ref-IDs resolve in support tooling (INF-005).

**Empty states:** three species, one template (what this is / why it's empty / first verb / doc link): *first-use* teaches the model (SH-03, C-25), *filtered-empty* names the filter and offers clearing ("No projects labeled `team:ml`. [Clear filter]"), *quiet-empty* reassures (C-30). Empties never render skeletons first (loading≠empty; resolve, then declare).

**Loading (G10 budgets):** skeleton of known layout ≤200ms for route loads; sub-400ms ops render nothing (no flash); optimistic UI only for P-UNDO-class reversibles (label edit paints instantly, reconciles); long ops are honest server-side operations (P-8) with per-step truth, never fake progress bars; palette budget §11. Perceived-speed law: *navigation must never feel slower than the palette* — route transitions keep chrome static and swap only the content slot.

## 19 — Accessibility & Keyboard (desktop consolidation)

PDS-SHELL §10's ten assertions are the floor. Desktop additions: **A11** full app usable at 200% zoom with nav collapsed (already asserted) *plus* all P-DD1 drags keyboard-equivalent (asserted per-drag); **A12** focus is never lost — every close/submit/navigation names its focus target in spec (layer rules §9); **A13** all live updates route through exactly two live regions (polite: context/status; assertive: failures) — never per-widget regions that chorus.

**Complete v0 keyboard map** (shell scope; owning PDSs extend within G10's reserved namespaces):

| Keys | Action | | Keys | Action |
|---|---|---|---|---|
| ⌘K | Palette | | g o/s/t/e | Overview/Services/Topology/Envs |
| ⌘\ | Collapse nav | | g , / g p | Project settings / Projects home |
| [ / ] | Cycle environments | | n p | New project (fleet) |
| . | Verb menu on focused object | | x | Select row |
| b | Bindings menu (topology node) | | +/- | Topology zoom |
| ⌘Enter | Submit form | | ? | Shortcut sheet |
| Esc | Close topmost layer | | ⌘/middle-click | Open in new tab |

Reserved for owning PDSs: single-letter verbs on service pages. Forbidden: shortcuts on destructive verbs (R-CMD-1 extended to keys).

## 20 — Responsive Strategy

Desktop-first, honest about it: the Console's floor is **960px comfortable / 320px functional** (A11 reflow, not a mobile product). Breakpoint behavior: ≥1280 full density; 1100–1280 nav auto-icon-rail (user preference overrides); 960–1100 panels overlay instead of squeeze; <960 nav becomes overlay + topology defaults to list view (the a11y list view doing double duty) + tables drop tertiary columns by declared priority. Wizard and settings frames are single-column already (no reflow risk). Touch is *supported, not designed-for* in v0 (hit targets ≥40px logical via density tokens; no hover-only affordances per P-HK). Mobile companion experience: explicitly deferred (would be a read-and-acknowledge product, not this Console shrunk — future EXP).

---

## 21 — Annotated Wireflows: Primary Journeys

Notation: `[SH-xx Screen]` · `──verb──►` user action · `╌╌►` system · ⚠ tiered danger · ⌘ keyboard path exists · `cli:` parity ribbon.

### WF-A · J1 First five minutes (happy path + the two failure detours that matter)

```
[Auth done (PDS-ACC)]╌╌►[SH-03 First-run]──"Create your first project"──►[SH-08 Name&Region]
                                                    ⌘ full wizard is Tab/Enter-only
[SH-08]──Continue──►[SH-09 Select services]──(⓵ "Describe instead")──►[SH-10 AI Describe]
   │                        │ ◄────reject/error C-16 (text preserved)────┘
   │                        ▼ accept╌╌selections applied
   └──Back──┐        [SH-11 Configure (defaults visible, advanced folded)]
            ▼               │ Continue        ⌘⏎
       (draft kept    [SH-12 Review & Estimate]────"Create · $14/mo"──►[SH-13 Provisioning]
        24h, resume         │ ▲                                            │╌╌steps done
        offered)            │ └─estimate-fail: confirm REPLACED by      [celebration ①]
                            │   [Retry estimate] (E-EST-503 gate)           │╌╌1.5s
                            ▼                                               ▼
                     step-fail detour: C-20, retry step,            [SH-19 Connect]
                     nothing rolls back                    tabs: env pull ▸ sync ⓵ ▸ manual(C-41)
                                                                    │╌╌first credential use ⓵
cli: steloit project create → prompts → estimate → y → progress → [celebration ② C-22]
     connect block printed                                          └─►[service page (PDS-001)]
Annotations: ① anxiety peak = SH-12; the gate that refuses blind billing IS the trust design.
② tab closed mid-provision: op continues server-side; return renders current step; inbox event on done.
```

### WF-B · J2 Daily loop (target ≤10s)

```
[pinned tab: "● ecommerce — Steloit"]──open/g o──►[SH-04 Overview]
   title dot ◐ = pre-read signal          │ read C-28 sentence + rings (0 clicks)
                                          ├─all good──► leave. (done, ~5s)
                                          ├─"cache degraded"──node click/⌘K "cache"──►[service page]
                                          └─"what changed?"──event row──►[Observe ⓵, pre-scoped]
Annotation: every element on SH-04 exists to serve one of these three exits — D1 enforced.
```

### WF-C · J3 Add a capability (entry ×3 → one flow)

```
[SH-05 "+ Add service"] ┐
[SH-06 canvas "+" ]     ├──►[SH-09 single-service mode]──►[SH-11]──►[SH-12 est.]──►[SH-13]
[⌘K "> Add service"]    ┘        (pre-scoped to project/env)                          │
                                                                                      ▼
                     [SH-06 topology: new node settles into place ╌╌ spatial teach]──►[SH-19 connect]
cli: steloit services add postgres --size … → estimate → y
```

### WF-D · J4 Something's wrong (shell's ≤3-interaction contract)

```
[inbox row: "Alert · cache degraded"]──click──►[SH-04, context auto-set]╌╌C-28 names culprit
        (or tab-title flip ◐)                        │ node click (interaction 2)
                                                     ▼
                                            [service page @ diagnosing tab (PDS-001/005)]
Annotation: shell owns delivery + orientation; diagnosis belongs to owning PDS. Deep link carries
(object, timestamp) so the landing view is already at the moment of the event (W3 delta).
```

### WF-E · J9 Retire safely (both branches)

```
[SH-15 Danger zone]──Archive──►[SH-22 ⚠T1 + C-35 summary]──Confirm──╌╌►suspend, snapshot,
      │                                                              card→archived, toast+event
      └───Destroy──►[SH-23 ⚠T3]──consequences C-36──type slug (paste blocked)──[Delete in 7 days]
                                        │ mismatch: E-CONF-7 inline
                                        ▼
                        ╌╌ state=deleting · C-39 grace banner on every project surface +
                           fleet card · [Restore] one click · day-before reminder (Lifecycle)
cli: steloit project delete ecommerce → consequences → retype slug → exit 0, prints restore cmd
```

### WF-F · J7 Bring the team (receiver side)

```
[teammate opens deep link]──►[auth (PDS-ACC)]╌╌►[exact linked screen, context set]
                                                  │ first visit to this project
                                                  ▼
                                        [joined-banner: "You've joined ecommerce → overview"]
Annotation: no tour, no wizard push — the link's destination IS the onboarding (L1 as culture).
```

---

## 22 — Promotion Plan & Exit Criteria

This document's novel sections promote as follows at Figma-phase start: §§10–11, 14 patterns → DES-005 pattern specs (P-SEL, P-DD1, P-CTX, P-CP, P-UNDO, P-RT, MW-5 popout); §19 map → DES-002 G10 appendix; §3.2 panel rules + §9 layering → DES-005 layout chapter; §20 breakpoints → DES-004 grid tokens. Exit criteria: every §21 wireflow reproduced as a Figma journey board with PDS-SHELL screen IDs; the three-altitude rule and layer rules applied as Figma page lint; no open contradictions with PDS-SHELL (none known at v1.0).

*— End of EXP-002 v1.0 —*
