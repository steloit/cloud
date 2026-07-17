# Steloit — Development Handoff Package

Everything required to build the Steloit developer-cloud console, for human engineers **and** AI coding agents. Three artifacts are the ground truth; everything else is derived from them and cross-referenced by frame id:

- `00-sources/GOV-002-product-architecture.md` — the **product architecture**: nine primitives, the hierarchy, product families and boundaries, lifecycle, the AI four laws (§7), the Cell/BYOC model, the v0→v4 rhythm, and the five tests. The design spec is grounded solely in this document.
- `00-sources/INF-001-infrastructure-constitution.md` — the **infrastructure constitution** (locked, 2026-07-09): what Steloit itself runs on and when it's allowed to spend. Governing principle: *cheap on capacity, never on shape*. Decisions D1–D11, day-one invariants, phase gates with budgets, verified costs, unit economics. Agents cite decision IDs; §8 is binding.
- `00-sources/Steloit-Console-Screens.html` — **152 validated frames** (1440×900) of every page, state, and overlay. Frame labels are authoritative one-line purposes; frame ids (W3, B10, U6, DB7, AI3…) are the cross-reference currency used throughout this package.
- `00-sources/Steloit-Console-Design-Spec.md` — the design spec: every decision, rule, and reversal, with reasoning.

## How to use this package (agents: read this)
1. Load `01-design-system` first — components are class contracts extracted from the gallery; never invent a component.
2. Resolve any screen question by opening its frame in `00-sources` (search the frame id).
3. Copy microcopy **verbatim** from frames — the words are spec.
4. API shapes come only from `08-api/openapi.yaml`; generate types, don't hand-write.
5. The invariants in `16-qa/qa.md` are the same audits the design passed — automate them.

## Index
| Dir | Contents |
|---|---|
| 00-sources | The four ground-truth artifacts (GOV-002 architecture · INF-001 infrastructure · frame gallery · design spec) |
| 01-design-system | design-system.md · tokens.json · icons/ (sprite.svg + inventory) |
| 02-information-architecture | ia.md — context model, rail, sidebars, URLs |
| 03-screens | screen-inventory.csv (all 152) · screen-states.md |
| 04-user-flows | flows.md — 27 flows end-to-end |
| 05-features | feature-specs.md — F1–F13 with business rules |
| 06-interactions | interaction-spec.md — the modal/drawer/page tier system |
| 07-forms | forms.md — fields, validation, payloads |
| 08-api | openapi.yaml — REST spec, conventions, schemas |
| 09-data-models | models.md — ERD (mermaid), tables, enums, indexes |
| 10-state-management | state.md |
| 11-permissions | rbac.md + rbac-matrix.csv |
| 12-ai | ai-implementation.md — four laws, proposal object, prompts, tools |
| 13-design-to-code | mapping.md — components/API/loading per screen family |
| 14-development | architecture.md — structure, conventions, error handling |
| 15-assets | tokens.css · sprite.svg (icons) |
| 16-qa | qa.md — canonical scenarios, consistency + regression checklists |
| 17-brand | brand.md (identity: "the Slip", lockups, color, type) · voice.md (writing guide) · messaging.md (positioning & proof points) · logo/ SVGs |
| 18-philosophy | product-philosophy.md (the constitution) · decisions.md (ADR log — check before relitigating anything) · glossary.md (the owned vocabulary) |
| 19-canon | canon.md + fixtures.json — the one demo world (API-response-shaped, arithmetic-verified); **no demo data exists outside canon** |
| 20-clients | cli.md (the `steloit` grammar & output conventions) · sdk.md (client design guide, generated-core + ergonomics) |
| 21-playbooks | new-product.md — how to ship a product without breaking the grammar |
| 22-agents | agent-guide.md — canonical onboarding for AI coding agents (CLAUDE.md is a pointer into it) |
| 23-emails | emails.md — transactional email inventory, subject grammar, skeleton |
| 24-rung0 | Rung 0 execution artifacts (design-partner pitch · interview script · interview logs) — consumes 17-brand + INF-001 §4 |
| 99-history | Superseded program archive (pre-LATTICE docs + Instrument prototype) — **never cite as authority**; see its README status map |

## Non-negotiable product rules (the short list)
Estimate before provision · env-as-filter · one arithmetic everywhere · plans gate capabilities, never safety · soft limits bill / hard limits fail loudly at 80%-warned · templates copy, never link; secrets never captured · tokens reveal once · interaction tiers (no inline forms) · the AI four laws · every failure state has a way forward.
