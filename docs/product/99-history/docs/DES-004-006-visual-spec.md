# DES-004/006 — Steloit Console: Desktop Visual Design Specification

**Document class:** Design System + Screen Specification (fulfills DES-004 Visual Language, DES-005 Component Library screen-consumption view, DES-006 Screen Specs)
**Status:** v1.0 → implementation-ready; subordinate to PDS-SHELL v1.0 and EXP-002 v1.0 for all behavior
**Rule:** this document adds *pixels, tokens, and rendering* only. Any behavioral statement here that contradicts PDS-SHELL/EXP-002 is a defect in this document.
**Version marks:** ⓿ v0 · ⓵ v0.5 · ⓶ v2 frame.

---

# PART I — DESIGN LANGUAGE & TOKENS

## 1.1 Design language: "Instrument"

The Console looks like a precision instrument, not a marketing surface. Reference posture: Linear's density and restraint, Stripe's typographic authority, Vercel's dark confidence — but with one identity move of our own: **the topology map is the brand.** Everything else recedes so the map, status color, and monospace identifiers carry the personality.

Five visual principles (each enforceable in review):

1. **Dark graphite, one blue soul.** A blue-black graphite field with a single periwinkle accent. No gradients on chrome, no glassmorphism, no decorative illustration inside the app. Color is *information*: if a hue appears, it means something (status, production, AI, danger).
2. **Borders over shadows.** Depth on dark themes comes from luminance steps + 1px borders. Shadows exist only on true overlays (popover/dialog layers), never on in-flow cards.
3. **Mono is the voice of truth.** Every identifier, slug, number-that-matters, path, and command renders in IBM Plex Mono. Sans talks *about* the system; Mono *is* the system. Users learn to trust mono at a glance.
4. **13px density, 4px rhythm.** G10's density made concrete: 13px base UI type, 4px spatial grid, 32px row heights. Whitespace is spent on separating *concerns*, never on air inside them.
5. **Motion whispers.** 150ms is the house tempo. Nothing bounces, nothing springs. The only two flourishes in the product are the celebration check-draws (PDS-SHELL §6.10).

## 1.2 Color tokens

Token architecture: `primitive → semantic → component`. Components may reference **semantic tokens only** (lint-enforced). All values below are the dark theme (default, G11); §1.2.4 gives the light mapping.

### 1.2.1 Primitives (dark)

```
graphite/950 #0A0D13   graphite/900 #0D1017   graphite/850 #10141D
graphite/800 #131722   graphite/750 #171C29   graphite/700 #1A2030
graphite/600 #232B3D   graphite/500 #2E3950   graphite/400 #3D4966
ink/100 #E6EAF2  ink/200 #C3CAD9  ink/300 #9AA4B8  ink/400 #6B7488  ink/500 #4A5266
periwinkle/300 #93A0FF  periwinkle/400 #7C8CFF  periwinkle/500 #6A79E8  periwinkle/900 #1B2140
green/400 #3DDC97   blue/400 #5AB8FF   amber/400 #F5A623   rose/400 #F4587A
gold/400 #D9A54A    violet/400 #B08CFF  slate/400 #8A93A8
(status colors carry /900 tint variants at 12% alpha for fills)
```

### 1.2.2 Semantic tokens (complete set)

| Token | Dark value | Use |
|---|---|---|
| `--bg-app` | graphite/900 | app canvas |
| `--bg-inset` | graphite/950 | code blocks, wells, topology canvas |
| `--bg-raised` | graphite/800 | cards, table headers, nav |
| `--bg-overlay` | graphite/700 | popovers, dialogs, panels |
| `--bg-hover` | graphite/750 | row/item hover |
| `--bg-active` | periwinkle/900 | selected rows, active nav |
| `--border-subtle` | graphite/600 | default 1px lines |
| `--border-strong` | graphite/500 | inputs, emphasized separation |
| `--border-focus` | periwinkle/400 | focus rings (2px) |
| `--text-primary` | ink/100 | body, values |
| `--text-secondary` | ink/300 | labels, meta |
| `--text-tertiary` | ink/400 | hints, timestamps |
| `--text-disabled` | ink/500 | disabled |
| `--text-on-accent` | graphite/950 | text on filled accent |
| `--accent` / `-hover` / `-pressed` | periwinkle 400/300/500 | primary actions, links, focus |
| `--accent-subtle` | periwinkle/400 @12% | selected fills, active chips |
| `--status-ready` | green/400 | G2 `ready` |
| `--status-provisioning` | blue/400 | G2 `provisioning` (+ animated dash) |
| `--status-degraded` | amber/400 | G2 `degraded` |
| `--status-suspended` | slate/400 | G2 `suspended` |
| `--status-deleting` / `--danger` | rose/400 | G2 `deleting`, destructive |
| `--danger-hover` | #FF6B8F | destructive hover |
| `--production` | gold/400 | G8 marker only — never reused |
| `--ai` | violet/400 | G9 attribution only — never reused |
| `--cost` | ink/100 on `--bg-inset` | money renders neutral, never green/red (G4: cost is fact, not judgment) |
| `--shadow-overlay` | 0 8px 24px rgb(0 0 0 / 0.45) | overlay layers only |
| `--shadow-dialog` | 0 16px 48px rgb(0 0 0 / 0.55) | dialog/takeover |

Every status fill = its color @12% alpha bg + color text + 1px color @32% border. Contrast audit: all text tokens ≥ 4.5:1 on their sanctioned surfaces; status-on-tint pairs ≥ 4.5:1; `--text-tertiary` reserved for ≥12px only (G11/WCAG AA).

### 1.2.3 Hue law

Blue-periwinkle = interactive. Green/blue/amber/slate/rose = the five G2 states, verbatim mapping, nowhere else. Gold = production, nowhere else. Violet = AI attribution, nowhere else. Rose doubles as danger (deliberate: deleting *is* dangerous). No other hues exist in the product. Charts (§2.16) use a sequenced neutral-plus-accent ramp, not the status hues.

### 1.2.4 Light theme

Same semantic names, remapped: `--bg-app` #FAFBFD, `--bg-raised` #FFFFFF, `--bg-inset` #F1F3F8, borders #E3E7F0/#CBD2E1, text #171C29/#4A5266/#6B7488, accent periwinkle/500 (darker for contrast), status hues darkened one step (green #1FA968, amber #C77E10, rose #D93A63, gold #A87B2E, violet #7E5BD6), shadows lightened. Theme = `data-theme` attribute; components are token-pure so zero component-level overrides. System/dark/light per user setting (SH-17).

## 1.3 Typography

| Token | Family | Size/Line | Weight | Use |
|---|---|---|---|---|
| `--type-display` | IBM Plex Sans | 24/32 | 600 | wizard step titles, celebrations |
| `--type-title` | Sans | 16/24 | 600 | page titles, dialog titles |
| `--type-heading` | Sans | 14/20 | 600 | card/section headers |
| `--type-body` | Sans | 13/20 | 400 | **base UI** |
| `--type-body-strong` | Sans | 13/20 | 600 | emphasis, health sentence |
| `--type-small` | Sans | 12/16 | 400 | meta, help text |
| `--type-micro` | Sans | 11/14 | 500, +0.02em caps | column headers, badges, nav group labels |
| `--type-mono` | IBM Plex Mono | 13/20 | 400 | slugs, values, code |
| `--type-mono-small` | Mono | 12/16 | 400 | table identifiers, timestamps |
| `--type-mono-metric` | Mono | 20/24 | 500, tabular-nums | costs, counts |

Rules: numerals in tables/metrics always `font-variant-numeric: tabular-nums`; slugs always mono at surrounding size; no font sizes outside this scale (lint); max line length for prose blocks 68ch.

## 1.4 Spacing, radius, elevation, layout

- **Space scale:** `--s-05`=2 `--s-1`=4 `--s-15`=6 `--s-2`=8 `--s-3`=12 `--s-4`=16 `--s-5`=20 `--s-6`=24 `--s-8`=32 `--s-10`=40 `--s-12`=48 `--s-16`=64. Component-internal padding uses ≤ s-4; between-concern gaps use s-6/s-8.
- **Radius:** `--r-1`=4 (inputs, chips) `--r-2`=6 (buttons, cells) `--r-3`=8 (cards, dialogs) `--r-4`=10 (takeover panels) `--r-full` (badges, avatars).
- **Control heights:** input/button md=32, sm=26, lg=38 (wizard CTAs only). Table row=36 (comfortable) /32 (dense toggle ⓵). Context bar=48. Nav width=224 expanded /56 rail.
- **App grid (SH-01):** CSS grid `48px topbar / [224px|56px] nav / 1fr content`; content max-width 1200px centered for reading surfaces (settings, wizard), full-bleed for canvas surfaces (topology, tables).
- **Z-layers (locked to EXP-002 §9):** content 0 · sticky chrome 10 · side panel 20 · popover/switcher/context-menu 30 · palette 40 · dialog 50 · takeover 45 (below dialog: dialogs may open over wizard) · toast 60.

## 1.5 Iconography & illustration

Icon set: 16px grid, 1.5px stroke, round caps (Lucide-compatible custom set); service-type glyphs (postgres/valkey/storage/queue/compute) get filled 20px "chip" variants for cards and topology nodes. Status icons are fixed glyph+color pairs (●ready ◐degraded ◌provisioning-spinner ⏸suspended ⌫deleting) — the word always accompanies the glyph (G2/A-8). Empty-state art: single-weight line drawings in `--border-strong`, max 96px, no color, no mascots.

## 1.6 Motion tokens

`--motion-fast`=80ms (hover, focus) · `--motion-base`=150ms (crossfades, panels, house tempo) · `--motion-slow`=250ms (topology settle, dialog enter) · `--motion-celebrate`=400ms (check-draw only). Easing: `--ease-out`=cubic-bezier(.2,.8,.2,1) default; `--ease-in-out` for crossfades. `prefers-reduced-motion` collapses all to 0ms except opacity ≤80ms; celebration renders static check (PDS-SHELL §11).

## 1.7 Focus, selection, states (global)

Focus ring: 2px `--border-focus` outline, 2px offset, radius-following — identical on every focusable, never suppressed. Selection: `--bg-active` + 2px left accent bar on rows; `aria-selected` always paired. Disabled: 40% opacity + `not-allowed`, never removed from DOM/tab order without reason. Hover: `--bg-hover` ≤80ms. Every interactive element has all five states (rest/hover/active/focus/disabled) specified by token — no ad-hoc styling (lint).

---
# PART II — COMPONENT LIBRARY

Format per component: anatomy → sizes/variants → states → tokens → a11y → dev notes. All components consume semantic tokens only; behavior cites PDS-SHELL/EXP-002.

## 2.1 Buttons
Variants: **primary** (accent fill, `--text-on-accent`), **secondary** (transparent, `--border-strong`), **ghost** (transparent, no border), **danger** (rose fill; danger-outline for T-dialog secondary). Sizes sm/md/lg (§1.4). Anatomy: optional 16px leading icon · label · optional shortcut hint (`--type-micro`, tertiary). Cost-bearing CTAs render the figure inside the label ("Create project · $14/mo", figure in mono). States per §1.7 + `loading` (spinner replaces icon, width locked, label persists). A11y: loading sets `aria-busy`; icon-only buttons require `aria-label` + tooltip. Dev: single `<Button>` with `intent|size|loading`; never compose danger from primary via overrides.

## 2.2 Status badge (G2)
Anatomy: glyph · word, `--type-micro` caps, status tint pill (`--r-full`, 20px height). Variants: the five states only — the enum is closed; new states require FND-003 change. Provisioning spins glyph 1.2s linear (reduced-motion: static). A11y: `role=status` not required (static text suffices); glyph `aria-hidden`, word carries meaning. Dev: `<StatusBadge state>` typed to the FND-003 union.

## 2.3 Cost chip (G4)
Mono figure on `--bg-inset` pill, `~` prefix for usage-estimates with tooltip C-11, "—" + tooltip when billing data delayed (B-0 partial). Month-to-date chips label "mtd" in micro tertiary. Never colored by magnitude (§1.2 `--cost` rule). Dev: `<CostChip value period estimated?>`; formatting via shared money util (2dp, user locale).

## 2.4 Navigation (SH-01 chrome family)
**Top bar (48px):** logo mark 24px · context triple · spacer · ⌘K affordance (bordered pill showing "⌘K") · bell+badge · avatar 28px. **Context buttons:** ghost buttons, mono slugs, 4px chevron; env button appends region tag (`--type-micro` on inset pill) and production underline: 2px `--production` bar under the button + word "production" — gold appears nowhere else on chrome. **Nav rail:** group labels `--type-micro` caps tertiary; items 32px: 16px icon · label · optional count; active = `--bg-active` + accent left bar 2px + icon accent; icon-rail mode shows tooltip labels. **Status line (bottom):** 12px dot (`--status-ready`/degraded per INF-005) + micro text. A11y: nav is `<nav aria-label="Primary">`; active `aria-current=page`; rail collapse persists (N2). Dev: nav tree is config-driven from the §4.4 route table — no hardcoded items; version-gated items (Observe ⓵) feature-flagged.

## 2.5 Switcher menu (LR-1, P-1)
280px popover under its button: search input (autofocus) · pinned current (check) · "Recent" group (3) · alpha list · footer action. Rows 32px: icon/avatar · name · mono slug right-aligned tertiary; env rows add region + gold "production" word. Combobox pattern (`aria-activedescendant`), type-ahead filters ≤50ms local (§11 EXP-002), >50 items switches to server search with loading row. Org switch renders full navigation (W4: hard cut, no crossfade). Dev: one `<SwitcherMenu kind>`; data from context cache, refresh on open.

## 2.6 Tables
Anatomy: sticky header row (`--bg-raised`, micro caps headers) · rows 36px · cell types: primary (link, `--text-primary`), mono (ids/values), badge, chip, sparkline, actions (hover-reveal, P-HK), checkbox (P-SEL). Row states: hover, selected (accent bar), focused (ring inset). Sort: header click, arrow glyph, single-column v0. Bulk bar: bottom-fixed 48px raised bar, count + intersection verbs (EXP-002 P-SEL), slides in `--motion-base`. Column priority attribute drives responsive drops (§20). Zebra striping: none (borders separate; stripes add noise at 13px density). A11y: real `<table>`, `aria-sort`, row selection announced with count; `x`/`shift-click` per P-SEL. Dev: virtualize >100 rows; update pills (P-RT) render as header-attached "1 update" chip — rows never reorder live.

## 2.7 Cards — resource family
**Project card (SH-02):** 280×148, `--bg-raised`, `--r-3`, 1px border; hover raises border to strong + `--bg-hover` (no shadow, §1.1-2). Grid: name (heading) + folder path micro · status badge · meta line "N services · prod +k envs" (small, secondary) · footer: cost chip mtd + label chips (max 2 + "+n"). Archived variant: 60% content opacity + "Archived" slate badge + Restore ghost button on hover. Grace variant: rose left bar 2px + C-39 line. **Service card (wizard SH-09):** checkbox card 220×120: type glyph 20px · name heading · role line (C-06..09, small secondary, 2-line clamp) · "from $X/mo" mono small. Checked = accent border 1.5px + `--accent-subtle` wash + check top-right. Card click toggles; whole card is the label. A11y: cards are single anchors/labels — no nested interactive except explicit hover verbs (P-CTX via `.`). Dev: `<ResourceCard>` polymorphic on object contract; status/cost slots from §2.2/2.3.

## 2.8 Forms (G5 family)
Field anatomy: label (small, secondary) · control · help/error line (small; error rose + icon). Input 32px, `--bg-inset`, `--border-strong`, focus ring per §1.7; mono inputs for slugs. **Slug input:** live preview line beneath (C-04 pattern, mono, tertiary with accent slug); availability check inline (spinner→check/×) debounced 300ms. **Select:** popover listbox styled as §2.5 minus search under 12 options. **Section-collapse:** heading + "defaults" badge when untouched; advanced `<details>` chevron. **View-as-CLI toggle:** top-right of every form section (G5 signature): switch swaps section body to read-only code block of the equivalent command, copy button; state per-user. Validation: inline on blur, summary never used (errors focus first invalid, P-FORM); ⌘Enter submits. Dev: form schema drives both the controls and the CLI rendering from one definition (EXP-003 grammar) — divergence impossible by construction.

## 2.9 Dialogs (G3 family)
Base: 480px (`--r-3`, `--bg-overlay`, `--shadow-dialog`), title row · body · footer (Cancel left of confirm — tab order law). **T1:** body = consequence summary line. **T2:** + typed-name field (mono, paste blocked with inline note "type it out — deliberately"). **T3 (SH-23):** + consequences list (each row: rose dot + object count, from live query: "3 environments · 4 services · 21 GB data") + typed slug + confirm disabled until match; confirm label carries the timeline ("Delete in 7 days"). Danger dialogs: rose header icon, confirm = danger button; `role=alertdialog`, consequences read before input (A-6). Working state: controls lock, confirm spins. Mismatch: E-CONF-7 inline, no shake. Dev: `<DangerDialog tier>` generates from object contract (name, consequences query, grace copy) — screens supply data, never markup.

## 2.10 Right panels (side panel, EXP-002 §3.2)
440px right-anchored, full-height under top bar (z-20), `--bg-raised`, left border strong; header: object icon+name+status · "open as page" (MW5) + close. Body: definition-list rows (label secondary / value primary·mono). Slides in `--motion-base` from right (reduced-motion: fade). One panel max; opening a second replaces with crossfade. Esc closes, focus returns (A-12). Deep-linked via `?panel=binding:{id}`. Dev: panel content components shared with their popped-out page route (one component, two frames).

## 2.11 Split views
v0 has exactly one designed split: **settings frame** (rail 200px + content, §3.14) — and the topology+panel combination (canvas resizes to `calc(100% - 440px)` when panel open, map re-fits with `--motion-slow` ease, nodes never reshuffle). General side-by-side document splits: **not in v0** (multi-window is the split strategy, EXP-002 §14; recorded as a decision). Dev: panel-open triggers single ResizeObserver re-fit on the canvas.

## 2.12 Command palette (P-5)
640×max 480, top-anchored 15vh, z-40, `--bg-overlay` + `--shadow-dialog`, backdrop `rgb(0 0 0/.5)`. Input row 48px: mode glyph (🔍/❯/✦) · input (`--type-body`) · scope capsule (mono micro pill, EXP-002 §11). Results: group labels micro caps tertiary; rows 36px: icon · title · breadcrumb (mono micro) · right type tag; active row `--bg-active`. Command-armed second stage: breadcrumb chip "Switch environment ▸" left of input. Ask mode ⓵: answer card (§2.15 styling) inline below input. Footer: 28px keymap hints. Motion: opens scale .98→1 + fade `--motion-fast`. A11y: `role=combobox`+listbox, mode changes announced, results count announced on settle. Dev: local index in worker, ≤50ms budget; server groups stream labeled (§11); registry R-CMD typed, destructive verbs excluded at type level.

## 2.13 Context menus (P-CTX)
Min 180px, z-30, item rows 30px: icon · verb · shortcut hint; danger verbs grouped last after separator, rose text (still tier-gated on invoke — menu never confirms). Generated from object contract verbs + Copy link + Open in new tab. Opens at pointer or focused-element corner (`.` key). A11y: `role=menu`, typeahead, Esc returns focus. Dev: one generator; per-object curation forbidden (EXP-002 P-CTX).

## 2.14 Notifications suite (P-6)
**Inbox panel:** 400px popover under bell (z-30): header (title + mark-all ghost) · category chips row (§6.8 categories; active = accent-subtle) · grouped list by project (group header mono micro) · rows 56px: category icon (status-colored dot ring) · title (body, unread=strong + accent dot) · meta line (project/env mono micro · relative time G7) · hover reveals mark-read. Empty: C-30 with line-art bell. **Toast:** bottom-right stack (z-60), 360px, `--bg-overlay`, auto-dismiss 5s (8s with undo verb, P-UNDO), max 3 stacked then queue. **Banners (LR-7):** full-width under context bar: grace (rose left bar, C-39 + Restore button) · suspension (amber, C-40) · incident (blue, status link). Banners push content, never overlay. A11y: badge count in bell `aria-label`; toasts `role=status`; undo reachable by keyboard (toast focus hotkey F8-style via shortcut sheet). Dev: notification rows deep-link with (object, timestamp) per W3 delta.

## 2.15 AI components (G9, §12 PDS-SHELL)
**Attribution badge:** ✦ glyph + "Steloit Assistant" micro, `--ai` tint pill — the only violet in the product. **Proposal card (LR-4):** `--bg-raised`, 1px `--ai`@32% border (the one non-graphite border in the app): header (badge + "why?" popover) · recommendation rows (type glyph · name · reason chip C-17 · est delta mono · toggle switch) · low-confidence rows prefixed "Not sure:" with default-off toggle · evidence block (`--bg-inset`, quote glyph, C-18 verbatim fragments) · footer C-15 (small tertiary) + Accept primary / Adjust ghost. **Ask answer card ⓵:** same frame, body prose (68ch), citation chips → docs, action suggestions render as plain links (never buttons — ask never executes). Thinking state: single pulse line "Analyzing your description…" (no typing theater). A11y: toggles labeled with service+reason; card is one labeled region. Dev: renders PRD-012 contract fields 1:1 (§12.2 table); unknown fields ignored loudly in dev mode.

## 2.16 Graphs & charts
**Sparkline (tables/cards):** 72×20 SVG, 1.5px `--accent` line, no axes, no dots except last (2px), tooltip on hover with mono value+time; flat/insufficient data renders dashed baseline + "—". **Charts (Observe ⓵ consumes; defined here for system completeness):** line/area/bar on `--bg-inset` well; axis text mono-small tertiary; gridlines `--border-subtle` horizontal only; series ramp: accent → blue/400 → violet/400 → ink/300 (max 4 series; more = table, not chart); status hues in charts only when the metric *is* a status count. Crosshair: full-height rule + tooltip card listing all series (mono values, tabular). Empty/loading: skeleton wave / C-29-style line. A11y: every chart paired with data-table toggle (same query); summaries via `aria-describedby` sentence ("p95 latency 240ms, stable"). Dev: one chart kit (wrapping d3-scale), themes from tokens, no per-screen chart styling.

## 2.17 Topology components (P-4, LR-3)
**Canvas:** `--bg-inset` full-bleed, dot-grid texture (1px dots, `--border-subtle`, 24px pitch — the only texture in the app; orients pan without decoration). **Node:** 168×56 `--bg-raised` `--r-3`; 20px type glyph · name (body-strong) + status word (micro, status color) · status ring: 2px border in status color (ready renders border-subtle + small green dot — calm L6: healthy is quiet, only non-ready states color the whole ring). Selected: accent ring + panel opens. Ghost node ("your app" external runtime): dashed border, tertiary text. **Edge:** 1.5px `--border-strong` bezier, arrowhead 6px; hover/focus → accent; provisioning → `--status-provisioning` dashed animated (8px dash, 600ms cycle; reduced-motion static); edge hit-area 12px invisible stroke. **Toolbar (canvas top-right):** zoom −/＋/fit · list-view toggle · "+ Add service" primary sm · "+ Bind" ⓵ secondary sm (disabled state carries C-27 tooltip). **Embed variant (SH-04):** 100%×280, no toolbar, fit-locked, "+N more →" chip bottom-right. Motion: settle 250ms once (§1.6); pan/zoom direct-manipulation (no easing). A11y: full §10.3 traversal (Tab enter, arrows in layout order, Enter open, `b` bindings menu, Esc exit) + hidden list-view parity. Dev: layout = layered DAG by binding direction, orderings by created-at (deterministic, M3); positions cached per project; SVG with DOM nodes for a11y tree.

## 2.18 Wizard components (P-2, P-3, P-8)
**Takeover frame:** nav hidden, context bar persists (§9 layering), content column 640px centered, breadcrumb steps top (C-02 names; done=check, current=accent, future=tertiary; steps are links backward only). Esc-guarded (draft notice). **Estimate panel (P-3):** `--bg-raised` card: line rows (name+config summary link ← left, mono price right, tabular alignment) · usage rows `~`-prefixed · divider · total row (`--type-mono-metric`) + "+ usage" suffix · caveat lines C-11/C-19 (small tertiary) · estimate-fail state swaps confirm for Retry (E-EST-503; panel border goes amber). **Async progress (P-8, LR-2):** step rows 40px: state glyph (pending ring / running spinner / done check / failed rose ×) · step name · elapsed mono-small right · disclosure for log lines (mono-small, `--bg-inset`); irreversibility marker: thin amber rule + C-23 between the reversible and irreversible steps. **Celebration:** 64px circle, check-draw 400ms, sentence display-type below; fires once per project per moment (§6.10). Dev: wizard state machine server-draft-backed (W1 delta EXP-002 §8); estimate panel consumes FND-010 response verbatim — no client math ever.

## 2.19 Empty / loading / error frames (G6)
**Empty frame:** centered max 420px: line-art 96px · heading (title type) · body (small secondary, ≤2 lines) · primary action · doc link ghost. Three species' copy per EXP-002 §17. **Skeletons:** shapes mirror real layout (rows for tables, card grids for grids, map silhouette for topology); shimmer at `--motion-subtle` opacity pulse (reduced-motion: static); ≤200ms rule — never flash for fast loads (§18). **Error frames:** page-level: centered, rose icon ring, what/why/next stacked, ref-ID mono chip (copy affordance), Retry primary; section-level: inset card same anatomy sm; field-level: §2.8. Dev: one `<ErrorFrame scope>`; ref-ID from FND-002 envelope.

---
# PART III — SCREEN SPECIFICATIONS

Every PDS-SHELL §6.1 screen. Format: **Purpose · Layout · Hierarchy · Responsive · Interaction · Motion · A11y · Dev notes.** Shared frames specified once + deltas (settings ×4, danger dialogs ×3, wizard steps share the takeover frame §2.18). Responsive column assumes §20 EXP-002 breakpoints (1280/1100/960).

## 3.1 SH-01 — App frame
**Purpose:** persistent chrome; renders context (L2) and hosts everything. **Layout:** grid §1.4 (48px bar / 224|56px nav / content); content scrolls, chrome fixed. **Hierarchy:** TopBar(Logo · ContextTriple(§2.5×3) · ⌘K pill · Bell · Avatar) / NavRail(§2.4) / ContentSlot / StatusLine / BannerSlot (below bar, §2.14). **Responsive:** ≥1280 expanded; 1100–1280 auto icon-rail; <960 overlay nav (hamburger appears far-left). **Interaction:** all EXP-002 N-rules; offline pill mounts right of bell (C-32). **Motion:** nav collapse 150ms width; content-slot route swap = 100ms crossfade keeping chrome static (§18 perceived-speed law). **A11y:** landmark structure (banner/nav/main/status); context-change announcement single polite region (A-1/A-13). **Dev:** context from URL only (MW2); chrome never re-renders on route change (slot architecture); title element implements the tab-title status pattern (J2: "●|◐ {project} — Steloit").

## 3.2 SH-02 — Projects home
**Purpose:** fleet altitude — find/assess/enter/create (D3: 5s per project). **Layout:** 1200px max; header row (title · label filter · folder filter · view toggle ⊞/≣ · + New primary); card grid `repeat(auto-fill, minmax(280px,1fr))` gap s-4; folders render as micro-caps group headers. **Hierarchy:** PageHeader / FilterChips / CardGrid(ProjectCard §2.7) | Table(§2.6: name·status·services·envs·mtd·labels) / ArchivedFilterChip. **Responsive:** grid auto-fills; table drops labels→envs columns by priority. **Interaction:** card click enters (N5 section memory); ⌘click new tab (N4); `.` context menu (verbs: open, copy link, archive ⚠, labels); bulk select in table view (P-SEL: label edit, archive). **Motion:** none beyond hovers (grid never animates layout). **A11y:** grid = list semantics; card = single anchor (§2.7); filter state announced. **Dev:** ≣/⊞ preference per-user (W2); >12 projects defaults table (PDS-SHELL §6.3); cost chips independently queried (B-0 partial).

## 3.3 SH-03 — First-run
**Purpose:** install M1 and start J1. **Layout:** empty frame §2.19 center-page + the org→project→env→service diagram (four 1.5px line-art nodes, C-42 caption) above heading C-01. **Hierarchy:** ModelDiagram / Heading / Body / PrimaryCTA("Create your first project") / GhostCTA(sample ⓵) / DocLink. **Interaction:** CTA → `/new`; diagram is decorative (aria-hidden, caption carries meaning). **A11y:** focus lands on CTA on mount. **Dev:** shown when projects=0 ∧ not-dismissed; member-join variant swaps to joined-banner (J7).

## 3.4 SH-04 — Project overview
**Purpose:** the 10-second truth (L3, D1). **Layout:** 1200px; top: HealthStrip full-width; main grid `1fr 340px` gap s-6: left = TopologyEmbed (280px §2.17) then below it nothing (map is the hero); right column = CostCard + RecentEvents. **Hierarchy:** HealthStrip(status glyph · C-28 sentence body-strong · env/region mono micro right) / TopologyEmbed / CostCard(mtd metric `--type-mono-metric` · "estimate basis →" link) / EventList(10 rows: dot · text · time G7, each deep-linking with timestamp per W3). **Responsive:** <1100 stacks right column below map. **Interaction:** sentence culprit names are links to their service; embed nodes clickable; "+N more →" → SH-06. **Motion:** ring state changes crossfade 150ms; list follows P-RT (update pill, no live reorder). **A11y:** health sentence is the page's `aria-describedby`; embed exposes traversal or list-view equally. **Dev:** health = worst-state fold (DB.worstState logic); empty-env state replaces grid with teach+Add (§6.13).

## 3.5 SH-05 — Services list
**Purpose:** all services in env; J3 entry. **Layout:** full-bleed table page; header: title · filter bar `Data | Compute ⓶(disabled+tooltip) | All` (segmented control, §4.4 delta ruling) · + Add service primary. **Hierarchy:** Table(§2.6): name(mono link)·type(glyph+word)·status badge·sparkline(owning-PDS metric)·mtd cost·created(G7). **Responsive:** drops sparkline→created by priority. **Interaction:** row → service page; `.` menu = FND-003 verbs; arrival with `?missing=` shows C-12 toast (context rule 5). **A11y:** §2.6; filter segmented = radiogroup. **Dev:** sparkline queries lazy + independently failable (partial state).

## 3.6 SH-06 — Topology (full canvas)
**Purpose:** M3's home; the brand surface. **Layout:** full-bleed canvas under chrome; toolbar floating top-right; legend chip bottom-left (5 status colors + words, dismissible, per-user). **Hierarchy:** Canvas(§2.17: Nodes · Edges · GhostNode) / Toolbar / SidePanel slot (node/binding §2.10). **Responsive:** <960 defaults list view (§20). **Interaction:** §6.6 complete (click/Enter open panel not page — panel's "open as page" goes to service; canvas empty-click closes panel); drag node→node ⓵ opens bind panel pre-filled (P-DD1); wheel/trackpad zoom, space-drag pan, +/−/fit keys. **Motion:** settle-once 250ms; panel-open re-fit `--motion-slow` (§2.11); provisioning edge dash. **A11y:** §10.3 traversal verbatim; toolbar reachable before canvas in tab order; zoom controls are real buttons. **Dev:** deterministic layout cache keyed (project, env, node-set-hash); re-layout only on topology change, never on data refresh.

## 3.7 SH-07 — Environments management
**Purpose:** list/create/inspect envs (env-independent route). **Layout:** 1200px table page; + New environment primary → SH-26. **Hierarchy:** Table: name(mono, production gets gold word tag)·region·services count·mtd·created; row → env settings (SH-16); side panel on row hover verb "details" optional v0-cut. **Interaction:** production row pins first. **Dev:** this page renders *without* env segment (§4.3) — context bar env button shows "—" state here (spec: button label "environments", disabled).

## 3.8 SH-26 — Environment create dialog
**Purpose:** J3 env creation. **Layout:** dialog 480 (§2.9 base): name field (slug input §2.8) · region select (FND-009 list, flag-free text rows: "iad — US East") · info line "$0 until you add services" (§13.1 honest zero). **Interaction:** create → row appears (toast + P-UNDO? no — creation isn't undo-toast class; plain success toast) → offer "Switch to it" action in toast. **A11y:** §2.9. **Dev:** name uniqueness inline-checked; region immutable warning in help text (C-34 phrasing).

## 3.9 SH-08 — Wizard: Name & region
**Purpose:** J1-4 start; weightless first step. **Layout:** takeover §2.18; single column: project name field + live slug preview (C-03/04) · region select + C-05 help · Continue primary lg right-aligned. **Interaction:** Enter advances when valid; slug editable via preview's edit affordance. **Motion:** step transitions slide 8px + fade 150ms directional. **A11y:** focus → name on mount; step change announces (A-4). **Dev:** draft persisted on field blur (W1 delta).

## 3.10 SH-09 — Wizard: Select services
**Purpose:** manual selection. **Layout:** service-card grid 2×2 (§2.7 checkbox cards) · "Describe instead ⓵" ghost link top-right · footer C-10 (small tertiary) · Continue. **Interaction:** cards toggle (space/enter/click); ≥1 required to continue; selections survive tab-switch to Describe (§6.5). **A11y:** group labeled "Services"; cards = checkboxes. **Dev:** card set version-gated (⓿ postgres only renders solo card centered).

## 3.11 SH-10 — Wizard: Describe (AI) ⓵
**Purpose:** AIC-SHELL-1 surface. **Layout:** textarea (5 rows, placeholder C-14) + submit; on proposal: ProposalCard §2.15 replaces input area (edit-description link reopens). **Interaction/States:** §6.13 table verbatim (idle/thinking/proposal/error/disabled-invisible). **Motion:** thinking pulse only; proposal card fades in 150ms (no reveal theater). **A11y:** §2.15; thinking state announced once politely. **Dev:** renders PRD-012 contract; C-16 error preserves text (never lose user writing).

## 3.12 SH-11 — Wizard: Configure
**Purpose:** per-service config, defaults-first (G5). **Layout:** one §2.8 section-collapse per selected service, ordered by selection; each header: type glyph · name · "defaults" badge · view-as-CLI toggle. Postgres section body = PDS-001 §6.2 (seam; frame only here). **Interaction:** all sections valid-by-default → Continue always enabled unless user invalidates. **Dev:** section schemas from owning PDS registries; unknown service types impossible by construction.

## 3.13 SH-12 — Wizard: Review & estimate
**Purpose:** M4's altar — the money gate. **Layout:** EstimatePanel §2.18 center · Back ghost + confirm primary lg (cost in label). **Interaction:** line-row config links jump back to their SH-11 section (state preserved); estimate-fail = Retry replaces confirm (E-EST-503, amber border). **A11y:** total associated with caveats (A-5). **Dev:** re-estimates on entry every time (config may have changed); CLI parity: identical line items (P-3 contract).

## 3.14 SH-13 — Wizard: Provisioning
**Purpose:** honest async (P-8). **Layout:** step list §2.18 · irreversibility rule + C-23 · overall elapsed top-right. **Interaction:** cancel enabled only above the amber rule; step-fail → retry-step + C-20 (§6.13 table); done → celebration ① → auto-advance 1.5s. **Motion:** step glyph transitions 150ms; celebration §2.18. **A11y:** §10-4 (polite step completions, assertive on fail). **Dev:** server op resume on reload (W1); logs lazy-fetch on disclosure.

## 3.15 SH-19 — Connect
**Purpose:** J1-8; the promise kept. **Layout:** takeover final step: title "Connect your app" · Tabs(§ CLI default / Platform sync ⓵ / Manual): CLI = code-block `steloit env pull` + C-24; sync = provider cards; manual = per-service connect blocks (reveal-on-click credential rows §P-CP + C-41 warning banner-inline). Right rail 280px: "waiting for first connection…" live row → celebration ② (C-22). **Interaction:** copy affordances everywhere; "Skip for now" ghost → overview. **A11y:** credential reveal announces; celebration sentence is the announcement (A-7). **Dev:** first-use event via FND-004 subscription; celebration once-ever flag server-side; audit event on reveal (P-CP).

## 3.16 SH-20 — Command palette
Complete spec at §2.12; screen-level notes: available on every route incl. takeovers (z-40 > 45? — correction: palette z must exceed takeover: **palette 48**, dialog 50; locked); scope capsule reflects wizard context inside takeover. **Dev:** index worker warm on app load; frecency store per-user per-org.

## 3.17 SH-21 — Notification inbox
Complete spec at §2.14 (panel). Screen-level: bell anchor, panel opens 150ms slide-fade; category chips persist per-user; footer → SH-17 notifications. **Dev:** cross-tab badge via broadcast channel (MW3, ≤2s).

## 3.18 SH-14/15/16/17 — Settings frames (shared P-7 spec + deltas)
**Purpose:** the four scopes' settings (placement table §6.9). **Layout (shared):** 1100px max; left rail 200px (section links, active accent bar) · content column ≤640px; rows: `grid 1fr auto` — name+consequence (body + small secondary) left, control right; section dividers s-8. Danger zone: bottom section, rose micro-caps header, rows carry consequence copy + danger-outline buttons. **Responsive:** <960 rail becomes top tabs. **Interaction:** dirty-guard per P-FORM; each row's setting key shown in help via view-as-CLI affordance (row-level, small). **A11y:** rail = nav landmark "Settings sections"; each row label→control associated. **Dev:** rows generated from settings schema (§7 parity keys) — the schema drives Console rows, CLI keys, and docs table from one source.
**Deltas:** **SH-14 org:** General (name/slug 30d note/avatar) · Members slot · Billing slot ⓵ · Policies ⓵ (incl. AI toggle → §2.15 surfaces vanish) · Audit ⓵ (read-only table §2.6). **SH-15 project:** General (rename + 90d redirect note) · Labels (chip editor) · Notifications ⓵ (channels) · Danger: Archive (T1 C-35) + Destroy (T3 → SH-23) + grace state replaces zone with C-39 banner + Restore. **SH-16 env:** General (name — production shows C-33 locked row; region read-only C-34) · Secrets slot → PDS-008 · Danger: destroy T2 (production: disabled row + C-38). **SH-17 user:** Profile · Appearance (theme radio system/dark/light — applies live on selection, no save) · Notifications matrix (category × channel checkboxes grid) · Sessions (table + revoke T1).

## 3.19 SH-22/23/24 — Danger dialog family
Complete spec §2.9; instance data: SH-22 T1 body C-35, confirm "Archive project" (secondary-weight danger? no — danger fill; archive is safe but consequential: **decision: T1 uses primary accent, not rose** — rose reserved for irreversibles; recorded). SH-23 T3: consequences query rows, C-36/37, confirm rose "Delete in 7 days". SH-24 T2: typed name, 72h grace copy; production variant never opens (row disabled upstream, C-38).

## 3.20 SH-25 — Context error pages
**Purpose:** L7 dead-end elimination. **Layout:** ErrorFrame page-level §2.19; variants: E-CTX-404 (heading "Not found" + ambiguous body per SEC-001 + org switcher inline + ⌘K hint) · E-CTX-410 (C-39 phrasing + Restore if permitted) · forbidden (C-31 + request-access primary → notifies admin, PDS-007 seam). **Dev:** these render inside chrome when org resolves, full-page when it doesn't (two frames, one component).

---

# PART IV — IMPLEMENTATION & QA

**4.1 Token export.** Single source `tokens.json` → CSS custom properties (`:root` dark, `[data-theme=light]` overrides) + TS constants; Figma variables mirror the same names 1:1 (Foundations file). Lint: raw hex/px in component code = CI failure; semantic-only enforcement (§1.2).

**4.2 Component build order (unblocks screens fastest):** tokens → Button/Badge/Chip/forms → Table/Card → nav+switchers (SH-01/02 shippable) → dialogs/panels/toasts → wizard kit (P-2/3/8) → palette → topology (longest pole; LR-3 spike first) → AI kit ⓵.

**4.3 QA checklist per screen (gate G-D):** all §6.13 states reachable with mock flags · keyboard-complete per §19 map · focus-restoration verified per layer (A-12) · reduced-motion pass · 200% zoom pass · light theme pass · copy IDs (C-xx) rendered verbatim · token lint clean · tab-title pattern (J2) · offline freeze behavior.

**4.4 Recorded decisions this document adds (delta register):** dot-grid canvas texture (only texture) · healthy-node quiet-ring rule (§2.17) · cost-is-neutral color law (§1.2) · T1-archive uses accent not rose (§3.19) · palette z above takeover (§3.16) · charts never use status hues except status-count metrics (§2.16) · no zebra striping (§2.6) · light-theme status darkening map (§1.2.4). Each is visual-layer-only; none alters approved behavior.

*— End of DES-004/006 v1.0 —*
