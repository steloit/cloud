# UI Design Audit — steloit-console

> **Progress log**
> **Pass 3 — done (2026-07-11, uncommitted).** Four-state grammar adopted across all ~30 remaining query-backed surfaces (settings: members/api-keys/policies×2/templates/cells/audit · account: tokens · billing: overview/invoices/usage/quotas/payment · observe: O1–O6 + U8 error branches · notifications filter-empty + detail placeholder · assistant insights · deploy: DP1–DP3 · dashboards: DB1/DB2/DB3/DB5/DB7 · W3 · M3 · estimate flows W2/C1/new-env). Pattern everywhere: SkeletonRows/Skeleton pending → ApiFailureCard (real endpoint + refetch) on error → EmptyState/EmptyRow (wired CTA + CLI) when empty → populated byte-identical. Lying numbers fixed en route: "12/20 seats" fallback, O5's hardcoded "Firing · 1" (now derived), O2 header stats over loading charts, billing overview's fmtMoney(0), DB5's phantom checkout-health row, W3's "No services yet" asserted while pending. Filter-empties got working Clear affordances (O4, N2, audit, templates). Estimate failures are now loud (warn Banner + retry; confirm reasons reflect failure) and create.tsx's failed estimate no longer re-fires infinitely. Verified: biome/tsc/vitest/build green; 29/29 headless checks including injected-400 fault tests proving six ApiFailureCard branches render (no phantom canon content beside them) and a delayed-response test catching the skeletons mid-load; canon smoke intact across 21 pages.
> **Pass 2 — done (2026-07-11, uncommitted).** The P0 truth bugs. (a) Instance scoping: all 12 frame-fixed depth tabs (sql-editor/tables/backups/branches/insights · data-browser/cli · messages/objects/shell/deployments/scaling) now gate canon content on the canon instance (db-main/cache/jobs/assets/api/worker); every other instance keeps the page chrome but gets honest scoped states via EmptyState — canon rendering verified byte-identical. (b) Four-state grammar on the four data-driven tabs (bindings/schedules/lifecycle/domains): SkeletonRows while pending, ApiFailureCard + refetch on error, EmptyState with a real wired CTA when empty; display-enrichment maps re-keyed from names to fixture ids so created rows can't inherit canon history. (c) Error-as-empty fixed: W1 failed-projects and template-detail failures now render ApiFailureCard, never "no projects"/"wasn't found". (d) The org shell renders skeleton/failure/designed-not-found instead of a blank screen; M2 postgres-zero gets an in-table empty row. (e) Two latent data-lie bugs found en route and fixed: 16 tab routes called `useServices(env)` without `resolveEnvKey` (internal-tools tabs resolved ecommerce's services), and three MSW list handlers (domains/lifecycle-rules/schedules) ignored `:service`, serving canon rows to every instance — now scoped to their canon owners (D21 api · D15 assets · D17/S5 jobs+worker). Verified: biome/tsc/vitest/build green; 18/18 headless checks (canon intact for db-main/cache/jobs/assets/api + scoped truth for db-reports/tools-db/files + org not-found); zero page errors.
> **Pass 1 — done (2026-07-11, uncommitted).** Foundations shipped: `--scrim` token + `Modal`/`Drawer`/`Scrim`/`useOverlay` primitives (Esc at document level, Tab focus-trap, focus restore) with all 11 hand-rolled overlays migrated (invite, token-reveal reveal-once kept non-dismissible, new-dashboard, delete-service 480, palette 640-on-Scrim, alert-rule, domain, lifecycle, schedule, create-binding, add-widget); assistant panel 396→424 + Esc (documented no-scrim exception); bell/coachmark/welcome popover recipe (Esc, no trap); `Skeleton`/`SkeletonRows`/`SkeletonLines` + `EmptyState`/`EmptyRow` primitives built (adoption = Pass 3); LATTICE type ramp `text-10…text-14` (1031 arbitrary sizes collapsed, sub-10px floor enforced, ≥15px display one-offs remain by design); `MetricChart` size tiers sm/md/lg (24 callers); icon sweep ✓/✗/✕/↶ → sprite + `Dot` tones `none`/`ai` + 9 raw `.dot` spans (kept as data: plans matrix, CLI/log output, embedded-copy glyphs; ▾/▲/⚠/★ deferred — no sprite equivalents / Pass-6 entanglement); plus opportunistic fixes: palette dead hints (`tab ask`, stale "Phase 3" assistant row now opens the drawer), welcome-widget toaster collision (bottom-16), domain-drawer "Recheck now" dead control now disabled-with-reason, reset-password marks now state-honest, `settings.cells` `#0B0E11` → `text-canvas`, bell-panel kbd hints via `Kbd`. Verified: biome/tsc/vitest/build green, 27/27 headless Playwright checks, zero page errors.

**Date:** 2026-07-11 · **Baseline:** main @ 2405b27 · **Scope:** design completeness & consistency only (not functionality). All 152 gallery frames exist; this audit is about the *quality and completeness of the design surface* — states, overlays, affordances, and system conformance.

**Method:** five parallel audits — (1) shell/navigation/global surfaces, (2) Home/Dashboards/Observe/Deploy/Create, (3) service overviews + depth tabs + gateway, (4) governance/billing/account/auth/AI plane, (5) a mechanical design-system conformance sweep across all 160 TSX files. Every finding carries a file:line.

**Priority key**
- **P0** — a hole a user hits immediately (blank UI, wrong data, error shown as empty)
- **P1** — a missing overlay/screen/state or a clear pattern break
- **P2** — polish, drift, tokenization

**Headline numbers**
- Error-state coverage: **2 `isError` branches against 36 `useQuery` calls (~6%)** — `ApiFailureCard` exists but is wired in exactly one place
- Loading skeletons: **7 `animate-pulse` uses total**; only W1 has a real skeleton; **no skeleton primitive exists**
- **~25 overlays from the spec's overlay census are missing** (rename, confirms, typed-confirms, flow drawers)
- **~110 raw unicode glyphs (✓ ✗ ▾ ⚠ ↶ ●) across 30+ files** stand in for sprite icons — including inside `design-system/steps.tsx`
- **22 distinct font sizes** (8.5px→64px) with ~610 half-pixel usages; 41 uses below the 10px floor
- Scrim `rgba(6,9,12,.44)` copy-pasted in **11 files**; no `--scrim` token; **no shared Modal/Drawer primitive**; **no focus trap anywhere**; Esc handling absent or bound only to the focused input in every overlay

---

## Part A — Systemic issues (fix once, benefits everywhere)

These are the root causes behind dozens of per-page findings. Fixing them first makes Part B mostly mechanical.

### A1. Overlay infrastructure — **P0 foundation**
- [ ] No shared `Modal` / `Drawer` primitive — every overlay reimplements scrim + layout. Build one pair (modal 460 / typed-confirm 480 / drawer 424) and migrate.
- [ ] **Esc-to-close** is broken everywhere: bound only to the autofocused input (`command-palette.tsx:271`, `delete-service-modal.tsx:104`, `invite-modal.tsx:65`, `bindings-tab.tsx:316`) or absent entirely (`alert-rule-drawer`, `lifecycle-drawer`, `schedule-drawer`, `domain-drawer`, `bell-panel`, `coachmark`, `welcome-widget`, `assistant-drawer`). Handle at the container.
- [ ] **No focus trap** in any custom overlay (only Radix menus trap). One systemic fix in the shared primitive.
- [ ] Promote the scrim to a token (`--scrim`), replacing 11 copy-pasted `bg-[rgba(6,9,12,0.44)]` literals.
- [ ] Width drift: `assistant-drawer.tsx:220` is **396px** (drawer standard is 424); `bell-panel.tsx:155` is 404.

### A2. The four-state grammar — **P0 foundation**
- [ ] Build a `Skeleton` primitive (rows on surface2) — none exists in the codebase.
- [ ] Build a shared `EmptyState` (glyph + one-line meaning + primary CTA + CLI copybit) — currently hand-rolled correctly only on W1/W3/C5 and bell-panel.
- [ ] Wire `ApiFailureCard` (already exists in `features/errors/failure-states.tsx`) into every query-backed surface — today it is used once (`observe.metrics.tsx:75`).
- [ ] Fix the "**error masquerades as empty**" bug class: failed `useProjects` renders the zero-projects empty state (`_app.$org.index.tsx:142`); failed template query renders "wasn't found" (`template.$tpl.tsx:199`).

### A3. Icon system — **P1**
- [ ] Replace raw glyphs with sprite icons: ✓ (50 uses / 20 files → `s-check`), ✗✕ (12 / 9 files → `s-x`), ▾ (26 → `s-chevd`), ⚠ (3), ↶ (1 → `s-undo`), ● (5 → `<Dot>`). Start with `design-system/steps.tsx:17`, which bakes ✓ into a shared component.
- [ ] Extend `DotTone` with the missing `none`/`ai` tones — their absence forces inline `var(--hair)`/`var(--assist)` hacks in `bell-panel.tsx:29` and `notifications.tsx:42`.
- [ ] Replace 9 raw `.dot` spans with `<Dot>` (`failure-states.tsx:103`, `provisioning.tsx:32`, `overviews.tsx:591`, `billing.payment.tsx:502`, `settings.cells.tsx`, bell/notifications).

### A4. Type & spacing scale — **P1**
- [ ] Define a discrete type ramp (tokens/utilities) and collapse the 22 ad-hoc `text-[Npx]` sizes. Kill the 41 sub-10px uses (`text-[8.5px]`/`[9px]`/`[9.5px]`, concentrated in the assistant surfaces, `settings.cells`, `create`, `new-env`).
- [ ] Chart heights: callers pass 64/72/80/110/120/130/150/170 with no tiers — define 2–3 chart height tokens (see D14, O/DB items).

### A5. Confirmation idiom — **P1**
Four different idioms guard destructive actions: two-click armed buttons (revoke binding, remove member, revoke token, rollback row), typed-confirm modal (delete service only), inline reason input (insight dismiss), and **no confirm at all** (header "↶ Roll back to #141" at `deployments.tsx:127` fires immediately — on the same page where the row rollback is two-click armed). Pick the census rule: modal confirm for destructive, typed-confirm 480 for irreversible-with-name, and apply it uniformly.

### A6. Dead affordances ("looks clickable, does nothing") — **P1**
A recurring class, worst offender the **▾ chevron chip that has no menu** (~20 sites, itemized per module below). Rule: a chip that reads as a control must act, or must be restyled as static text / disabled-with-reason.

### A7. Stale "Phase N" reasons that contradict shipped features — **P1**
- [ ] M3 "+ New environment" disabled "C8 lands in Phase 5" — **C8 exists and works** (`environments.tsx:111`)
- [ ] O2 "⚑ Alert on this query" disabled "U8 lands in Phase 3" — **U8 drawer is built and live on O5** (`observe.metrics.tsx:109`)
- [ ] Palette "Ask the assistant" toasts "drawer lands in Phase 3" — **drawer is mounted** (`command-palette.tsx:219`)
- [ ] Observe snav "Dashboards" disabled "Phase 3" while org Dashboards is fully live (`snav-observe.tsx:59`)
- [ ] Vocabulary drift: "Phase 3 / Phase 4 / Phase 5 / a later phase / lands with per-product telemetry / spec change first (finding)" — same concept, six phrasings; also "Approval threads" is Phase 4 in `policies.index.tsx:73` and Phase 5 in `bell-panel.tsx:57`.

### A8. h1 grammar "Area · Thing" — **P1**
Five whole page families skip the prefix that Billing/AI Gateway/Organization/Project/Deploy correctly enforce: **Account** (Profile, Sessions, Security & MFA, Personal tokens, Notifications), **Assistant** (Ask, Activity, Insights, Capabilities), **Observe** subpages, **Dashboards**, and **all non-gateway service tabs** (bare "Metrics", "SQL Editor", "Backups"… — note "Metrics" now collides between service-metrics and observe-metrics). A third convention uses the raw service name (`branches.tsx:64`, `ai.tsx:412`, `insights.tsx:143`). Standardize.

### A9. Scope leaks — hardcoded canon strings in chrome — **P1**
- [ ] `snav-dashboards.tsx:27` — "Acme · all projects" regardless of org
- [ ] `snav-assistant.tsx:29` — "ecommerce · production" regardless of scope
- [ ] `assistant-drawer.tsx:130,254,286` — deep links hardcode ecommerce/production
- [ ] `rail.tsx:180` — gateway entry gated on literal `project === "ecommerce"`
- [ ] `ctx.tsx:86` — org-switcher subtitle "Business · 4 projects · $482/mo" for every org
- [ ] Settings/billing pages hardcode Acme/ecommerce strings (see G/B sections) — switching orgs shows Acme's identity

### A10. Light-mode breaks & untokenized colors — **P2**
- [ ] `settings.cells.tsx:183-184` — `text-[#0B0E11]` ×2 (true light-mode breaks)
- [ ] `lattice.css:662` — `.rcnt.warn` uses `var(--canvas)` as ink on amber → light-on-amber in light mode
- [ ] 8 non-token `white` uses (toggle knobs, avatar initials, `!text-white` in `ctx.tsx:198`)
- [ ] Org-avatar gradient `#E36C4B→#B34A2E` duplicated inline in 4 files — extract to a token/class
- [ ] ~10 `style={{background: var(--token)}}` + 5 `fontFamily:"Inter"` inline overrides → utilities (`.chip` needs a non-mono variant)

### A11. Component adoption — **P2**
- [ ] `overviews.tsx` hand-rolls `<button className="btn p|s">` 7× (lines 278–692); `ctx.tsx:198` once → use `<Btn>`
- [ ] No Tooltip component — every hint is native `title=` (inaccessible to keyboard); kbd hints rendered three ways (`<Kbd>`, plain text in `bell-panel.tsx:188`, dead hints: palette advertises `tab` and `?` with no handlers)
- [ ] 5 template-literal className concats → `cn()` (`invite.$inviteId:39`, `billing.invoices:81,85`, `billing.quotas:101`, `$org.index:49`)
- [ ] DP2 gates table is the only `.tbl` **not** wrapped in `.tblwrap` (`deploy.$dep.tsx:158`)

---

## Part B — Module checklist

### 1 · Shell & navigation

**P1**
- [ ] Rail active-state substring bugs: `/settings` branch first means a service's Settings tab lights the **gear**, and `/deploy*` matches a service's Deployments tab lighting the **Deploy** icon (`_app.$org.tsx:42-54`) — violates "exactly one `.rit.on`"
- [ ] `!org` renders a **blank screen** during org load (and forever on a bad slug) — no skeleton, no NotFound (`_app.$org.tsx:38`); rail product icons pop in after services load with no placeholder (`:32-36`)
- [ ] Snav "back chevron" in settings/account headers is decorative, not a control (`snav-settings.tsx:43`, `snav-account.tsx:17`)
- [ ] Assistant button only renders on service pages but ⌘J opens the drawer everywhere; coachmark points at a button that may not exist (`ctx.tsx:194` vs `_app.tsx:20`)
- [ ] Bottom-right mount collision: Toaster (bottom-right) + WelcomeWidget (`fixed right-4 bottom-4 z-40`) + AssistantDrawer (z-40) — overlapping, undefined stacking

**P2**
- [ ] Rail "+" allows create-service with zero projects (dead-end); gear deep-links to `settings/audit` instead of a settings home; rail avatar is inert `<span>` while the ctx avatar is a menu
- [ ] "Keyboard shortcuts" menu item opens the palette, advertises an unbound `?` shortcut (`ctx.tsx:269`)
- [ ] Account plane top bar drops search/bell/assistant/theme — chrome grammar diverges (`_app.account.tsx:25`)
- [ ] Org avatar 24px `rounded-md` in snav vs 18px `rounded-[5px]` in crumb; menu classes duplicated between `ctx.tsx` and `snav-product.tsx`
- [ ] Only two ctx search placeholders for three scopes; nit badge color hardcoded inline (`snav.tsx:41`)

### 2 · Auth, onboarding, invites (A)

**P1**
- [ ] Dead SSO affordances: "Continue with GitHub/Google", "Use SSO", "Sign up with GitHub", "Connect GitHub", invite "Not you? Switch account" — all render interactive, none act (`login.tsx:44-92`, `signup.tsx:34`, `onboarding.connect.tsx:42`, `invite.$inviteId.tsx:187`). Disable-with-reason or wire.

**P2**
- [ ] `login.tsx:80` — email-format error prints under the **password** field
- [ ] `signup.tsx:24` — one error string funnels name/email/password errors under password; no per-field errors; no email-verification step by design
- [ ] `onboarding.team.tsx:28` — invalid email silently dropped, no field error; pending rows show "{role} ▾" as text
- [ ] `reset-password.tsx:79` — requirement rows always print ✓ (only color changes); "expires in 27:12" is static
- [ ] Bright spot to copy: `invite.$inviteId.tsx` has full four-state coverage — the reference implementation

### 3 · Home & projects (W1–W3)

**P0**
- [ ] W1: failed `useProjects` shows the **zero-projects empty state** (`_app.$org.index.tsx:142`) — add an error branch

**P1**
- [ ] W3: vitals ("214/s", "812 ms"…), "Needs attention · 4", and event spine are fully hardcoded with no loading/empty/error variants (`_app.$org.$project.index.tsx:297-402`); failed `useServices` shows the populated shell with an empty topology
- [ ] W2: "Add another service" has no onClick; region envpill renders "⌄" but is not a control; estimate failure silently shows **$0/mo** (`new-project.tsx:200,136,85`)

**P2**
- [ ] W1 header shows literal "Loading…" while the body shows skeletons — two loading treatments on one screen
- [ ] W2 success = toast + navigate; no designed success surface

### 4 · Create canvas & templates (C, T2)

**P1**
- [ ] AI1 multi-service create is a dead end — "Review & create N services →" permanently disabled (`create.tsx:542`)
- [ ] Estimate mutation has no error branch: confirm button sticks on "Waiting for the estimate" forever (`create.tsx:286`)
- [ ] C5: 5 of 6 gallery previews disabled; template detail has a real layout only for `store` — a one-exemplar screen; empty state is a bare card (no glyph/CTA/CLI) (`new-project_.templates.tsx:214,181`)
- [ ] T2 (`template.$tpl.tsx`): "Rename · Delete…" is **plain text**, not controls (`:339`); no rename modal, no delete confirm; two divergent layouts under one route (h1 "store" vs "Templates · {name}"); org-template load is text "Loading…", errors masquerade as "wasn't found" (`:199`)

**P2**
- [ ] Dead chips styled as controls: "+ add rule", "Autoscale 1–3 · CPU target 70%", "+18 allowlisted" (`type-blocks.tsx:723,631,413`)
- [ ] `create.tsx:270` silently targets `projects.data?.[0]`; zero-project case builds a route with `project: undefined`
- [ ] `type-blocks.tsx` ships its own `usd()` helper + 28 hardcoded money strings — route through `fmtMoney`
- [ ] C8 (`new-env.tsx`): estimate fully hardcoded ($46/$0/$21/$17/$8/~$180) unlike W2/create which call the live estimate; `var(--steel-tint)` inline 4×; `listCells` has no loading/error
- [ ] T3 (`templates_.new.tsx:121`): visibility is `<div className="inp">Organization ▾</div>` — a fake select

### 5 · Service depth tabs (D-series) — the largest block

**P0 — instance-scoping (the "db-reports shows db-main's data" class)**
- [ ] `sql-editor.tsx:22` — results hardcoded to db-main's orders for **any** postgres instance
- [ ] `tables.tsx:29` — db-main rows for any instance
- [ ] `backups.tsx:17` — db-main snapshots for any instance (a 6-minute-old db shows 3 days of backups)
- [ ] Same class (P1): `branches.tsx:25` (4 branches incl. degraded main for every instance), `data-browser.tsx:29`/`cli.tsx:63` (valkey), `messages/objects/shell/deployments/scaling` all frame-fixed per product. This breaks SW3's own footer promise "Metrics stays Metrics, scoped to the other instance" (`snav-product.tsx:294`)
- [ ] The four live tabs (bindings/schedules/lifecycle/domains) scope by id but key display enrichment on **names** (`schedules.tsx:20`, `lifecycle.tsx:21`, `domains.tsx:25`) — differently-named instances lose all enrichment

**P0 — empty states a freshly created service hits immediately**
- [ ] Bindings: zero-consumer service → empty `<tbody>`, nothing else (`bindings-tab.tsx:127`)
- [ ] Schedules: new queue/worker → empty table (`schedules.tsx:94`)
- [ ] Lifecycle: new bucket → empty table (`lifecycle.tsx:87`)
- [ ] (P1) Domains: no true "no custom domains yet — add one" state (`domains.tsx:84`); none of the four live queries handles `isPending` or `isError`

**P1 — missing overlays (census)**
- [ ] Rotate-binding confirm (`bindings-tab.tsx:144`) · Revoke-binding confirm (currently two-click, `:84`) · Revoke gateway key (`gateway-tabs.tsx:363`)
- [ ] Cancel-deploy confirm (deployments) · Rename modals (service/domain/schedule/branch — none exist anywhere)
- [ ] Edit-shape drawer (postgres `:524`, valkey `:218` — inline chips today) · Webhook config drawer (`alert-rule-drawer.tsx:176` — chip toggle only)
- [ ] Edit TTL (D3, `data-browser.tsx:162`) · Redrive/Discard confirms (D8, `messages.tsx:162`) · Purge typed-confirm (D18, `queue.tsx:83` — copy promises "type the queue name", no modal) · FLUSHALL typed-confirm (D14, `valkey.tsx:248`) · Restore-to-branch flow (D5, `backups.tsx:84`)
- [ ] Add-secret drawer (P2, `web.tsx:76`)
- [ ] Note: the one typed-confirm that exists (`delete-service-modal`, correct 480px) is reachable **only from postgres settings** — every other product's delete is disabled

**P1 — missing detail views**
- [ ] D4 caption promises "click a row for the full plan, sampled parameters, and history" — only the regressed row is clickable, no plan/params/history panel (`insights.tsx:110`)
- [ ] Per-message detail is one hardcoded payload for `msg_9f221`; rows not clickable (`messages.tsx:125`)
- [ ] Branch "Open" buttons have **no handler** (`branches.tsx:93`) — W5's promise leads nowhere
- [ ] (P2) Per-deployment detail (D19 rows), per-domain detail ("Make primary" is hardcoded `disabled "already primary"` on **every** row incl. non-primary, `domains.tsx:97`)

**P2 — consistency**
- [ ] Wrong-product fallback renders h1+hsub on some tabs, bare Card on others — one pattern
- [ ] `domain-drawer.tsx:126` "Recheck now": styled `opacity-55` but **not disabled, no reason, no onClick** — a dead control masquerading as disabled
- [ ] `ai.tsx:354` uses `opacity-50` vs the standard 55; danger-zone borders drift (`border-err` vs `/40` vs `/45`); toggle knobs `bg-white` ×3; live-row/regressed-row highlights use three different mechanisms (`deployments.tsx:150`, `insights.tsx:114`)

### 6 · AI Gateway (X1)

**P2**
- [ ] Gateway model/route tables have no per-row detail (`gateway-tabs.tsx:97,161`)
- [ ] Usage "Meter" cell plain text where siblings are mono (`gateway-tabs.tsx:210`)
- [ ] SW3: products at n=1 (incl. gateway) can never reach the `/instances/$product` landing from the snav (`snav-product.tsx:218`); SW3 dropdown hardcodes db-main/db-reports strings by name (`:263`)
- [ ] (Note) Gateway tabs are the **only** service tabs following "Area · Thing" — they become the template for A8

### 7 · Observe (O1–O6)

**P1**
- [ ] O2: 4 of 5 category tabs (Databases/Queue/Network/Cost) are one-liners — four missing screens (`observe.metrics.tsx:58`)
- [ ] O5: History and Silences tabs are one-liners — two missing screens; "Firing · 1" count is hardcoded while the table filters live data (`observe.alerts.tsx:104,78`)
- [ ] O3: search input is `defaultValue`-only — typing doesn't filter; facets captioned "always clickable truths" are non-interactive divs; advertised `j/k/e/c` keys mostly unbound (`observe.logs.tsx:215,242,258`)
- [ ] O6: only `tr_8814` has a waterfall; other trace rows are static divs — no selection, no detail; search doesn't filter (`observe.traces.tsx:25,63`)
- [ ] O1/O3/O4/O5: no loading/empty/error on scoreboard, logwell, events, rules table; O4 filter-miss renders a blank card with only footer text

**P2**
- [ ] ObserveChrome time chips + "compare/split-by/Saved:" chips are display-only ▾ controls; "live" pill static
- [ ] O2 is the app's best error-state example but covers only the p95 chart, not the 3 small ones
- [ ] U8 drawer: `create`/`backtest` have no error branch (backtest failure reads as "no firings")
- [ ] O4 "select any two events" copy describes an interaction that doesn't exist (`observe.events.tsx:168`)

### 8 · Deploy (DP1–DP3)

**P1**
- [ ] DP1: only #143 has a detail; #140 "Why?" is a `title=` tooltip — no rollout-log/abort view; promotion lane + staging rows hardcoded; no loading/"no deployments yet"/error (`deploy.index.tsx:93`)
- [ ] Header "↶ Roll back to #141" fires with **no confirmation** while the row version is two-click armed — same page, contradictory guarding (`deployments.tsx:127` / A5)
- [ ] DP3: "policy ▾" chip dead; preview "Open" has no onClick; rows static, no per-preview detail (`deploy.previews.tsx:30,55`)

**P2**
- [ ] DP1 env column hardcodes `production` for every row (`:170`); rollback has no success feedback
- [ ] DP2: non-`dep_143` fallback is an ad-hoc card, not the empty grammar; gates table missing `.tblwrap` (`deploy.$dep.tsx:38,158`)

### 9 · Dashboards (DB1–DB8)

**P1**
- [ ] DB5: 6 of 7 dashboards have disabled Open/Duplicate/Delete — no detail views exist for them (`dashboards.mine.tsx:152`)
- [ ] DB6: entire layouts gallery is static; all 5 "Use layout" disabled — the instantiation screen doesn't exist (`dashboards.layouts.tsx:59`)
- [ ] DB2: fleet rows for staging/events-db have disabled "Open →" — dead rows; connections chart lacks the threshold line its own caption references, and deploy markers (`dashboards.postgres-health.tsx:46,135`)
- [ ] DB3: page claims "deploys drawn on every chart" but 3 of 5 charts pass no markers (`dashboards.infrastructure.tsx:28,64-76`)
- [ ] DB1: 3 of 6 prebuilt cards disabled with the reason **only in `title=`**, not stated inline (`dashboards.index.tsx:77`)
- [ ] DB7: no skeleton/error — grid renders empty on pending/error (`dashboards.$dashId.tsx:344`)
- [ ] DB8: "other project ▾" chip only toggles; created dashboard isn't shown after create; no `onError`

**P2**
- [ ] DB7 edit mode: drag handle decorative, widget `×` disabled, "Save layout" echo-only; non-canon dashId gets an ad-hoc not-found card; `StatWidget` returns literal "2" for non-spend widgets (`:117`)
- [ ] DB3 "Saturation" captioned "valkey memory 95%" but plots db-main connections (`:69`)
- [ ] DB1 pinned captions hardcoded while sparklines are live — numbers and charts can disagree; DB4 money strings hardcoded/non-mono; sparkline sizes drift within one page (96×24 vs 200×36)

### 10 · Settings & governance (G, T1/T3)

**P1**
- [ ] G5 org-name and G1 project-name are live-looking `<Inp defaultValue>` with **no save button and no rename modal** — editable-looking, fully inert (`settings.general.tsx:36`, `$project.settings.general.tsx:36`)
- [ ] Remove-member is two-click inline, not the census's confirm modal (`settings.members.tsx:79,219`)
- [ ] Policy Review/Deny approval thread disabled — no approval-thread overlay (`settings.policies.index.tsx:73`)
- [ ] Four-state coverage: members, api-keys, policies, templates tables — none of loading/empty/error (`members.tsx:178`, `api-keys.tsx:144`, `policies.index.tsx:93`, `templates.tsx:134`)

**P2**
- [ ] Change-role menu mutates immediately with no consequence surface; pending-invite Revoke has no confirm (`members.tsx:196,242`)
- [ ] Delete-org/project have no typed-confirm modal built (the pattern exists in `delete-service-modal` but is unused here); transfer flows absent (`settings.general.tsx:58-74`)
- [ ] Audit filter envpills ▾ inert (`audit.tsx:88`); audit has skeleton but no empty/error
- [ ] `settings.cells.tsx:183` — the two hardcoded `#0B0E11` inks (light-mode break); cells grid has no states
- [ ] AI-policy enforcement control paints **every** selected segment green incl. "Disabled" (`policies.ai-assistant.tsx:109`)
- [ ] Scope leaks: "Acme"/"acme/store"/"asha is owner" hardcoded (`settings.general.tsx:31-53`, `settings.git.tsx:31,62`); project policies filter hardcodes `prj_ecommerce`

### 11 · Billing (B)

**P1**
- [ ] Invoice lines: footer promises "every line expands to the usage rows behind it" — **no expand interaction exists**; "disputes: open from the line" — no affordance (`billing.invoices.tsx:127,163`)
- [ ] Set-budget modal missing (`billing.overview.tsx:213`); replace-card/add-backup flows missing (`billing.payment.tsx:141`)
- [ ] Plans: no upgrade action for Free→Pro or Pro→Business; "current" column hardcoded to Business; the only working change is trial→Pro with Pro/$29/Borealis hardcoded (`billing.plans.tsx:65`, `confirm.tsx:31`)
- [ ] Four-state coverage: invoices, usage meters, quotas — none (`invoices.tsx:69`, `usage.tsx:213`, `quotas.tsx:90`)

**P2**
- [ ] Overview renders `fmtMoney(0)` while loading (reads as "$0.00"); payment methods/subscription have no loading/error
- [ ] Usage project chips + env pill static; literal `$99.00` / `$72.42` strings (`overview.tsx:332`, `usage.tsx:262`); a `$` in a Pghead title ("…Pro, $29/mo")

### 12 · Account (P)

**P1**
- [ ] Change-password modal missing (`account.security.tsx:52`); revoke-token is two-click, not a confirm (`account.tokens.tsx:76`)
- [ ] Tokens table: no loading/empty/error (`tokens.tsx:151`)

**P2**
- [ ] MFA/passkey setup, regenerate-recovery-codes confirm, remove-factor, session revoke, delete-account — all disabled with no flows built (acceptable if that's the intent, but they're census items)
- [ ] P2 profile fields `readOnly` while G1/G5 are editable-inert — three identity screens, two broken contracts (unify with A: rename modals)
- [ ] P6 quiet-hours inputs `defaultValue` with no save affordance (`account.notifications.tsx:117`)
- [ ] Account h1s drop "Account ·" (A8); account org table hardcodes Acme only

### 13 · AI plane (AI) & notifications (N)

**P2**
- [ ] `assistant.ask.tsx:180` — conversation rail (search + thread rows) entirely non-functional while presenting as interactive
- [ ] Drawer chips "change ▾" / "1 of 3 open ▾" inert (`assistant-drawer.tsx:257,271`)
- [ ] Insight snooze fires immediately — no duration picker; insights list has no skeleton and a bare-text empty state (`assistant.insights.tsx:153,356`)
- [ ] Four-laws doctrine authored three different ways (`svc…ai.tsx:369`, `policies.index.tsx:112`, `policies.ai-assistant.tsx:24`) — extract one component
- [ ] Sub-10px text concentration is here (A4); 5 inline `fontFamily:"Inter"` chip overrides
- [ ] Notifications: filter-miss renders blank list + blank detail pane with no empty state (`notifications.tsx:308`); "Project: all ▾ / Env: all ▾" inert
- [ ] Bright spots: drawer↔full-page composer parity is correct; evidence-pill grammar consistent; bell-panel empty state is the reference EmptyState

### 14 · Environments & instances (M2/M3)

**P1**
- [ ] M3 "+ New environment" disabled for a shipped screen (A7); "Add to this environment" also disabled
- [ ] M2 empty handling differs by product: illustrative products always show 2 fake rows even with 0 real instances; postgres with 0 → **blank table, no message**; only illustrative-less products get a one-liner (`instances.$product.tsx:405`)

**P2**
- [ ] M3 matrix presence + notes hardcoded; no states on environments/services queries
- [ ] M2 illustrative money hardcoded; non-product param falls to bare "Instances" h1

---

## Part C — Suggested execution order

**Pass 1 — Foundations (fix once):** A1 shared Modal/Drawer with Esc + focus trap + scrim token · A2 Skeleton + EmptyState primitives, wire ApiFailureCard · A3 icon glyph sweep + DotTone gaps · A4 type ramp + chart height tiers. *Everything after this is mostly mechanical adoption.*

**Pass 2 — P0 truth bugs:** error-masquerades-as-empty (W1, T2) · instance scoping of postgres/valkey depth tabs · empty states on bindings/schedules/lifecycle/domains + M2 postgres-zero · blank org shell.

**Pass 3 — Four-state adoption:** roll skeleton/empty/error across the ~30 query-backed surfaces itemized above (G/B/P tables, Observe, Dashboards, Deploy, W3).

**Pass 4 — Overlay census (~25 overlays):** rename modals (org/project/service/template/domain/schedule) · confirm modals replacing two-click arms (remove member, revoke token/binding/key, redrive/discard, cancel deploy) · typed-confirms (purge, FLUSHALL, delete org/project reusing delete-service-modal) · flow drawers (edit-shape, webhook, add-secret, edit TTL, restore-to-branch, set-budget, replace-card, change-password, snooze picker, invoice line expand).

**Pass 5 — Missing screens & details:** O2 category tabs ×4 · O5 History/Silences · DB6 layout instantiation · DB5 dashboard details · trace/message/deployment/branch/query detail views · template detail beyond `store` · plan upgrade paths.

**Pass 6 — Consistency polish:** A5 one confirmation idiom · A6 dead-chip sweep (act or de-affordance) · A7 stale-reason sweep + one vocabulary · A8 h1 grammar (gateway tabs as template) · A9 scope-leak strings · A10/A11 tokenization and component adoption.
