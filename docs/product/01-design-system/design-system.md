# LATTICE — the Steloit design system

Dark-first console. Source of truth for every value: `15-assets/tokens.css` (extracted verbatim from the gallery). Frames are 1440×900; the console is a desktop-first product.

## Tokens
- **Surfaces**: `--canvas` (app background) → `--surface` (cards, sheets) → `--surface2` (inputs, insets, hovers). Elevation is expressed by surface step + `--e1/--e2` shadows, never by lightening borders.
- **Ink ramp**: `--ink1` primary text · `--ink2` secondary · `--ink3` tertiary/metadata. Never use pure white.
- **Hairlines**: `--hair` for all 1px borders/dividers.
- **Semantic**: `--ok` running/healthy · `--warn` degraded/attention · `--err` failed/destructive · `--prov` provisioning (always animated) · `--steel` primary action & focus · `--assist` the AI layer (violet — AI content only, never selection).
- **Type**: Inter (UI) + JetBrains Mono (`.mono` — ids, money, code, metrics). Sizes in the gallery run 8–22px; body 11–12px, `h1` 19px, `hsub` follows at ink3.
- **Radius**: cards 12–14px, inputs/buttons 8–9px, pills fully rounded. **Spacing**: 4px base; page padding `.pgpad` = 18px 22px; card padding 12–16px; gaps 8–14px.
- **Motion**: 150–200ms ease for hovers/overlays; `--prov` states pulse; no decorative animation.

## Components (class → contract)
- **`.rail`** — fixed left icon rail. Items `.rit` (24px icon), active `.rit.on`, fleet badge `.rcnt` (only multi-instance products, e.g. db ×2), dashed `.rit.add` (create), gear last. Zones: Home · Observe · Deploy fixed; services zone; + and ⚙ fixed.
- **`.ctx` crumb** — org avatar `.cav` / `.csep` / project / environment pill `.envpill` ▾ (env-as-filter: changing it filters every project-scoped page). `.ctxsearch` = ⌘K.
- **`.snav` sidebar** — `.snhead` (glyph + title/subtitle `.t/.u`), items `.nit` (+`.on`), counts `.cnt`, group headers `.nsec`, footer `.nfoot` (pinned with `margin-top:auto`).
- **`.pghead`** — `h1` (19px; subpage grammar "Billing · X", "Organization · X") + `.hsub`; actions right after `.sp`. **No breadcrumb line inside pghead** — location is crumb + active nit.
- **`.card`** — the container. Dashed border = add-affordance or locked/gated feature.
- **`.tbl` in `.tblwrap`** — data tables; left-aligned; centered `th` only for matrix/comparison tables (M3, B5 precedent).
- **`.pill`** — status: `ok/warn/err/prov/st/mut/ai`. `.dot` = 8px status dot.
- **`.btn`** — `p` primary (steel) · `s` secondary · `gh` ghost · `dgr` danger. Disabled = `opacity:.55` + reason nearby.
- **`.banner`** — exactly two variants: default and `warn`. Full-width, above pghead.
- **`.chip/.chiprow`** — filters and segmented choices (`.on`).
- **`.inp` + `.flabel`** — inputs; `.foc` focus ring + caret span; `.ph` placeholder; `.mono` for technical values.
- **`.logwell`** — evidence/preview well (mono, `.t` for dimmed tokens). Used for: log excerpts, dry-runs, DNS checks, math-in-the-open, next-run previews, backtests.
- **`.tab/.tabrow`**, **`.steps`** (`.stepdot/.steplbl/.stepbar`; done=✓ `.on`, active=number `.on`), **`.glyph`** (icon chip), **`.uav/.cav`** avatars, **`.copybit`** (copy control), **`.kbd`**, **`.eyebrow`**, **`.spark`/inline `<svg><polyline>`** charts, **`.prop`** (AI proposal block), **`.toast`**.
- **Assistant button** — violet tinted in the top nav on all tier-1 pages (⌘J); **filled** only while its drawer is open.

## Overlay recipes (verbatim from U-frames)
- **Modal**: `position:absolute;inset:0;z-index:40` wrapper → scrim `rgba(6,9,12,.44)` → centered 460px card (`--surface`, 14px radius, `--e2`), title/body, actions bottom-right. Danger uses `.btn.dgr`; data-destructive escalates to **typed-confirm** (button disabled until name matches).
- **Drawer**: same scrim → right sheet 424px, `border-left --hair`, header (title/sub + ✕), scrolling body, sticky footer. AI drawer variant uses `--assist` accents.

## States (every component)
default · hover (surface2 tint) · active/selected (steel tint bg + steel text) · focus (steel ring) · disabled (.55 + stated reason) · loading (skeleton on surface2 / `--prov` pulse) · error (err border + inline message under field).

## Accessibility
- Contrast: ink1 on surface ≥ 7:1; ink3 is metadata-only, never sole carrier of meaning — pair color with text/icon (pills always carry words).
- Full keyboard: ⌘K palette, ⌘J assistant, Esc closes overlays (never destructive), visible focus ring (`--steel`), overlays trap focus and return it.
- All icons via `<use>` get `aria-hidden`; interactive elements need accessible names; tables use real `th`.
- Live regions for toasts/banners; `prefers-reduced-motion` disables the prov pulse.

## Responsive
Console is desktop-first (≥1280px canonical). 1024–1280: sidebar collapses to icons, cards wrap 2-up→1-up. <1024 (read-mostly): rail becomes bottom bar, tables gain horizontal scroll in `.tblwrap`, overlays go full-screen sheets. Marketing site (future) is mobile-first; console is not.
