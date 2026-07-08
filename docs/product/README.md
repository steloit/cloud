# Steloit — Development Handoff Package

Everything required to build the Steloit developer-cloud console, for human engineers **and** AI coding agents. Two artifacts are the ground truth; everything else is derived from them and cross-referenced by frame id:

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
| 00-sources | The two ground-truth artifacts |
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
| 17-brand | brand.md — identity system ("the Slip" mark, lockups, color, type) + logo/ SVGs |
| 18-philosophy | product-philosophy.md — the constitution: philosophy, promise, grammar, question ownership, review checklist, the "nevers" |

## Non-negotiable product rules (the short list)
Estimate before provision · env-as-filter · one arithmetic everywhere · plans gate capabilities, never safety · soft limits bill / hard limits fail loudly at 80%-warned · templates copy, never link; secrets never captured · tokens reveal once · interaction tiers (no inline forms) · the AI four laws · every failure state has a way forward.
