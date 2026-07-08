# Steloit — Brand Identity System (v1.0 · Jul 2026)

The visual identity, derived entirely from decisions already made in this package (LATTICE design system, product invariants, marketing direction). Where this file and the product spec conflict, **the spec wins**. Full rationale, exploration, and rendered guidelines: the brand book artifact ("Steloit — Brand Identity & Logo System").

**Brand thesis: Steloit shows its work.** Territory = proof (visible structure, arithmetic, state) — not speed, not magic. Personality: the candid engineer — candid · composed · exact · structural.

## The mark — "the Slip" (selected Jul 2026, after 6 exploration rounds)

A machined steel coin: twin kerfs on the 60° lattice axis free the core, and the core slides a half-step home while the caps hold. A stable platform; a deployment slotting in. Register: solid, rounded, Linear/Neon/Supabase-class — but cut and displaced, never striped.

- **Primary cut (Slip, ≥24px):** 32-unit grid. Coin r `11.5` at `C(16,16)`. Kerf centerlines at `±3.2u` normal offset on the 60° axis, kerf width `2.0u` (core = 4.4u band). Shear `s = 1.8u` along the cut, up-right. Caps never move.
- **Small cut (16–24px, favicon):** the identical Slip with kerfs widened `2.0u → 2.6u` (offsets ±3.2 and shear unchanged) so the cuts stay crisp. Same mark, optically compensated — never a different design.
- **Below 16px:** don't render; use the wordmark or nothing.
- **Optical:** the shear shifts silhouette mass up-right ≈ (0.2,−0.35)u — nudge the coin 0.3u down-left when centered in circles/squircles. Kerf faces stay square; never round the cut, never outline the coin.
- **Motion:** the only sanctioned logo animation — caps fade (300ms), core slides home along the cut (700ms, house curve). Doubles as the loading mark.
- Superseded: "the Fold" (angular S, v1.0) — kept in the brand book as exploration history.

## Lockups & usage

| Context | Version |
|---|---|
| Web header, docs, email | Horizontal lockup (`logo/steloit-lockup-horizontal.svg`) |
| Square placements | Vertical lockup (mark above wordmark; compose per rules below) |
| Avatars / app icons / favicon | Symbol only (small cut <24px) |
| Running text, legal | Wordmark only |
| 1-color print, engraving | Monochrome — node inherits the ink |

- **Alignment:** coin center sits on the wordmark cap midline; coin diameter = 1.30× cap height (reference-measured against Linear/Upstash navs); optical gap (coin edge → first letter) = 0.46× cap height — measured off Upstash/Linear lockups; CSS/box gaps must be derived from this visual number, not equal to it.
- **Clear space:** ¼ of the coin diameter, all sides, all versions. **Min sizes:** coin 16px / 4mm (small cut); lockup 88px / 22mm.
- **App icon:** 1024 master, full-bleed steel gradient `#4D7CFE → #3B63E8 → #2F53D6` (vertical), white coin at 60% (with the 0.3u optical nudge); platform applies its own mask. No text ever.
- **Avatar:** steel gradient circle, white coin at 62%.

**Misuse (all forbidden):** rotate (the cut sits on the 60° axis) · gradient fills on the flat mark (gradients live on tiles only) · shadows/glows · outline redraws (the coin is solid; the kerf is the void) · semantic-color recolors (green = `ready`, not brand) · stretching · containers (the hexagon belongs to the Project glyph) · violet (violet is the AI layer's).

## Color

Canon tokens (`15-assets/tokens.css`) elevated to brand level. Two planes: **graphite** (product) · **daylight** (marketing). Steel ramp cores: `450 #4D7CFE` on graphite, `500 #3B63E8` on daylight; `600 #2F53D6` hover; `50/100` tints; `700 #2643AC` text-on-tint.

Rules: ~90% neutrals, ≤8% steel, ≤2% semantic · one steel focal point per composition · steel = action/focus/brand, never status · semantic colors are load-bearing (nothing decorative may be green/amber/red/prov-blue) · violet `#9D8CFF / #6D5AE6` = AI only, never selection · the avatar/app-icon gradient is the only licensed gradient · dark and light are equals.

Measured contrast (key pairs): ink1/surface 15.02:1 · ink2/surface 7.33:1 · steel450/graphite-canvas 5.19:1 · steel500/white 5.08:1 (= white/steel500) · white/steel450 3.73:1 → button labels ≥12.5px semibold on graphite. ink3 is metadata-only (3.15:1). err on graphite 4.9:1 → always paired with a word (pills carry words).

## Typography

- **Satoshi 500/700** — display, headings, wordmark. Never below 15px.
- **Inter** — UI and body (console canon; 13px base in product).
- **JetBrains Mono** — *if it can be pasted into a terminal, it is mono* — plus every number, price, id, eyebrow. Tabular figures wherever digits align; money via `fmtMoney`, integer cents.
- Marketing scale: display clamp(38–62)/1.03 −2.5% · h1 40 · h2 31 · h3 19 · body 15–16.5/1.6 (max 68ch) · eyebrow mono 11/+16% uppercase.
- Email/offline fallback Helvetica + Courier New; wordmark never renders in fallback — use the SVG.

## Iconography

"Wiring diagrams, not pictures." 20px grid · 1.5px stroke (scales, never re-weighted) · butt caps, mitered joins · angles 0/90/60/45° only · nodes = filled circles ≤3px (the only fills besides status) · `currentColor` always · one metaphor per glyph, owned forever (primitive glyphs per spec §1.4; sprite is source of truth) · animation = 200ms draw-in on creation only.

## Motion

Motion is state; nothing moves for fun. One easing: `cubic-bezier(.2,.7,.3,1)`.

| token | value | use |
|---|---|---|
| micro | 120ms | hover, focus |
| base | 150–200ms | overlays, page transitions (fade + 2px rise) |
| draw | 600–700ms | logo draw-on, first-render edges, frame reveals |
| pulse | 1.6s loop | provisioning **only** — nothing else loops |

Logo animation = the caps fade in, the core slides home (the only sanctioned logo animation; doubles as loading). AI proposals never animate in (canon). `prefers-reduced-motion`: everything resolves to end state; pulse → static dot + the word.

## Assets & imagery

Everything recombines four elements: the coin, the tittle node, the **lattice field** (dot grid, ≤1 edge path, one full-opacity steel node, ≤1 node/400px²), and the **receipt**. Supergraphic = the mark cropped past the frame *with its construction grid showing*. Illustration language = the shipped lattice benchmark: one iso ground plane, node-and-edge grammar, one steel accent (may pulse), lowercase mono caption, composed scenes (no lone floating elements). Photography: real product UI, documentary team shots; never stock/orbs/circuit-boards/clouds/padlocks; demo data only from canon fixtures ($208 `store-baseline` etc.). OG 1200×630: graphite, lattice field, one sentence, mono URL row. Sticker slogans are product invariants verbatim ("no silent limbo", "estimate first").

Print: steel ≈ `C78 M60 Y0 K0` (nearest PMS 2726 C); graphite = rich black `C60 M50 Y40 K100`; matte stocks; single-color jobs go monochrome.

## Files

```
17-brand/logo/
  steloit-mark.svg               # the Slip, currentColor
  steloit-mark-steel.svg         # the Slip, steel, auto light/dark
  steloit-mark-small.svg         # small cut (<24px), currentColor
  steloit-favicon.svg            # small cut, steel, theme-adaptive
  steloit-wordmark.svg           # outlined Satoshi + steel tittle
  steloit-lockup-horizontal.svg  # Slip + wordmark, theme-adaptive
  steloit-app-icon.svg           # 1024 master, gradient tile + white Slip
```
