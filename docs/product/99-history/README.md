# 99-history — Program archive (superseded; nothing here is authoritative)

This directory preserves the **pre-LATTICE program bundle**: the documentation program and interactive prototype that preceded the current handoff package. It exists so decisions keep their reasoning and reversals stay on the record (the constitution's amendment discipline, applied retroactively). **No document here may be cited as authority for new work** — every live rule from this era either survives in `00-sources/GOV-002-product-architecture.md` (promoted out of this bundle to ground truth) or was deliberately superseded by the current design spec.

## Status map

| Document | What it was | Status today |
|---|---|---|
| `docs/BUNDLE-MANIFEST.md` | The bundle's own handoff manifest (was `HANDOFF.md`), incl. the eight-document authority order | Historical record of the old authority chain |
| **GOV-002** | Product Architecture Specification | **Not here — promoted to `00-sources/`.** Still fully binding; the design spec's sole cited grounding. |
| `docs/GOV-003-documentation-master-plan.md` | The 61-document corpus plan (FND/PRD/DES/EXP series, waves, owners) | Superseded by this package's structure. Its principles (contracts before products, delta documents, one owner/one status/one ID) survive in the package's authority rules and the constitution. Its "deliberately deferred" list (INF-009 BYOC, full PRD-012 AI) remains a useful record of intent. |
| `docs/GOV-005-product-design-strategy.md` | How the design phase operates: global decisions G1–G11, journeys J1–J11, gates G-A→G-D, pioneer/instantiate rule | Process superseded; the *decisions* fed the console spec (context model, env-as-filter, one-pioneer rule → "exactly one product pioneers a pattern"). Mine for ADR extraction. |
| `docs/DES-000-pds-system.md` | What a PDS is: 14-section anatomy, FND > PRD > DES > PDS precedence | Superseded — the current package replaced the PDS pipeline with the gallery-as-spec model (frames + design spec + derived docs). |
| `docs/DES-009-pds-execution-plan.md` | The 15 PDSs, waves P0–P4, resolved questions R1–R4 | Superseded as a plan. R1–R4 resolutions (incl. the AI evidence contract) survive in `12-ai/` and the design spec. |
| `docs/PDS-SHELL-v1.0.md` | The Shell fully specified: 26 screens, 9 patterns, URL grammar, ⌘K, danger tiers, 42 microcopy strings | **Superseded by the console design spec** (`00-sources/Steloit-Console-Design-Spec.md`): the PROJECT/ORGANIZATION sidebar model was redesigned into the rail + Dub-pattern shell; screens grew 26 → 152. Its intent language ("the Shell is the platform's body language") remains quotable history. |
| `docs/EXP-002-desktop-console-ux.md` | Desktop UX doctrine: seven laws, three altitudes, multi-window, keyboard map | Superseded as authority, but its seven laws visibly survive, evolved, in the current spec (URL-as-state, context worn not visited, ten-second truth, verbs over destinations, console teaches CLI, calm is a feature, never a dead end). Prime ADR-extraction material. |
| `docs/DES-004-006-visual-spec.md` | The **"Instrument"** visual language: periwinkle accent, IBM Plex Mono, gold=production hue law, 19 component families | **Superseded by LATTICE** (`01-design-system/`, `17-brand/`). Do not take any token, hue rule, or type choice from this document — several directly contradict current canon (gold means nothing in LATTICE; mono is JetBrains; the accent is steel). |
| `prototype/` | The full-platform interactive prototype (Instrument era, P0–P4 coverage). `cd prototype && python3 -m http.server 4173` | Superseded by the 152-frame gallery as visual/spec truth. Still useful as a *behavior* reference (clickable flows, per-screen state controls: loading/empty/error/offline/light/reduced-motion) — behaviors it demonstrates are suggestive, never normative. |

## Missing ancestors (known, still wanted)

- **GOV-001 — "Steloit — The Developer Cloud Platform" (the Vision document).** Cited by GOV-002 as its source of truth; not present in any recovered bundle. If found, it belongs in `00-sources/`.
- **GOV-004 — the document registry** referenced by GOV-005/DES-000. Likely absorbed by GOV-003's ID scheme; low priority.

## Why keep this at all

Three reasons: (1) GOV-005/EXP-002/DES-009 contain the *reasoning* behind decisions the current spec states as conclusions — the raw material for `18-philosophy/decisions.md`; (2) the Instrument→LATTICE transition is itself a decision worth being able to reconstruct; (3) the prototype demonstrates interaction behavior the static gallery cannot.
