# Steloit — Complete Program Handoff

Everything approved to date, in one bundle: the eight authority documents and the
full-platform interactive prototype. All documents are **v1.0 approved** and immutable;
changes go through change requests, not edits.

---

## What's inside

```
steloit-handoff/
├── HANDOFF.md            ← you are here
├── docs/                 ← the eight authority documents (authority order below)
└── prototype/            ← the full-platform interactive prototype (open index.html)
```

## The document chain, in authority order

| # | File | What it decides |
|---|---|---|
| 1 | `GOV-002-product-architecture.md` | The product model: nine primitives, the Cell, service catalog + rationale, lifecycle, **AI layer & its four laws (§7)**, version rhythm v0→v3, **BYOC (§2.2, §9)**, ten-year posture. Everything downstream derives from this. |
| 2 | `GOV-003-documentation-master-plan.md` | The 61-document corpus: what gets written, when, by whom; the FND contract layer; what is deliberately deferred (e.g. INF-009 BYOC, full PRD-012 AI). |
| 3 | `GOV-005-product-design-strategy.md` | How design operates: global decisions G1–G11, journeys J1–J11, the design sequence, gates G-A→G-D. |
| 4 | `DES-000-pds-system.md` | What a Product Design Specification *is*: the 14-section anatomy, precedence rules (FND > PRD > DES > PDS). |
| 5 | `DES-009-pds-execution-plan.md` | The 15 PDSs and their waves P0→P4; resolved questions R1–R4 (incl. the AI evidence contract). |
| 6 | `PDS-SHELL-v1.0.md` | The Shell, fully specified: 26 screens, 9 patterns, URL grammar, ⌘K, danger tiers, 42 microcopy strings, error catalog, AI components AIC-SHELL-1/2. |
| 7 | `EXP-002-desktop-console-ux.md` | Desktop UX doctrine: seven laws, three altitudes, side panels, keyboard map, wireflows, multi-window behavior. |
| 8 | `DES-004-006-visual-spec.md` | The "Instrument" visual language: tokens, closed hue law (gold=production, violet=AI, rose=danger), type, motion, 19 component families, 26 screen specs. |

**Precedence when documents appear to conflict:** GOV-002 > GOV-003 > GOV-005 > DES-000/009 > PDS-SHELL > EXP-002 > visual spec. (In practice they don't conflict; each cites its parents.)

## The prototype

`prototype/` is the complete platform rendered clickable — all phases P0–P4: Shell,
PostgreSQL depth (branches/backups/connection/logs), Observe, IAM, Valkey/Storage/Queue,
Secrets, Billing, Templates, the Assistant, and the v2 Deployments preview with
copy-on-write DB branches. Pure HTML/CSS/vanilla JS.

Run it:

```
cd prototype && python3 -m http.server 4173   # → http://localhost:4173
```

(Plain file:// works too; serving enables the smooth environment-switch crossfade.)
See `prototype/README.md` for the coverage map, keyboard shortcuts, demo walkthroughs,
and the per-screen state controls (loading/empty/error/offline/light/reduced-motion).

## Reading paths

- **New engineer:** GOV-002 §1–5 → PDS-SHELL §1–6 → prototype alongside.
- **New designer:** GOV-005 → DES-000 → visual spec → EXP-002 → prototype.
- **Leadership / stakeholder:** GOV-002 §9 (version rhythm) → prototype walkthrough → GOV-003 §Deferred for what's intentionally unwritten.
- **"What did we decide about X?":** grep the docs folder — every decision is written down, and every deferral says where its answer will live.

## Open items (deliberately, by plan)

- **INF-009 (BYOC architecture)** — scoped (bootstrap, trust boundary, upgrades, support access, prerequisites) but deferred to the v2/v2.5 window; INF-001's Cell seam is its down-payment.
- **PRD-012 (AI, full)** — the four laws, G9, and the R4 evidence contract are binding now; the complete per-product capability registry and copilot/operator horizons author later.
- FND contract layer (12 specs) and remaining PDSs per DES-009's wave plan.
