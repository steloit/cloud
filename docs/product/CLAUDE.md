# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repository is

This is **not source code** — it is the design & specification handoff package for building the Steloit developer-cloud console (a React+TS+Vite SPA over a REST API). There is no build, lint, or test command; every file is Markdown, CSV, JSON, YAML, SVG, or a single HTML gallery. The deliverable this package describes has not been implemented here.

When asked to "build" or "implement" a screen/feature, you are generating application code *from* these specs, not editing an existing app in this repo.

## Ground truth and the cross-reference system

Two artifacts in `00-sources/` are authoritative; everything else is derived from them:

- `Steloit-Console-Screens.html` — 152 validated frames (1440×900) of every page, state, and overlay. **Frame ids** (`W3`, `B10`, `U6`, `DB7`, `AI3`, `C1`, `D22`…) are the cross-reference currency used across the entire package. Frame labels are authoritative one-line purposes.
- `Steloit-Console-Design-Spec.md` — every design decision, rule, and reversal, with reasoning.

To resolve any question about a screen, **search its frame id** in `00-sources/`. Docs elsewhere key their specifics to these ids (e.g. `mapping.md` and `screen-inventory.csv` are indexed by frame family).

Reading order for a task (per README): `01-design-system` → the relevant frame in `00-sources` → the domain doc (`05-features`, `07-forms`, `08-api`, etc.).

## Authority rules — where each kind of decision is fixed

Do not invent; each concern has exactly one source of truth:

- **Components** — `01-design-system/design-system.md` defines class contracts (e.g. `Pill` → `.pill`) extracted from the gallery. A screen never invents a component; if one seems missing, it is a design-system change *first*, then use it.
- **Microcopy** — copy the words from the frame **verbatim**. The words are spec ("shown once", "no silent limbo", "calculators, not sales").
- **API shapes** — only `08-api/openapi.yaml`. Generate TypeScript types from it; never hand-write API types. JSON is snake_case; ids are `xxx_`-prefixed.
- **Invariants / QA** — `16-qa/qa.md` holds the audits the design already passed; treat them as tests to automate, not suggestions.
- **Permissions** — `11-permissions/` (rbac.md + rbac-matrix.csv) is the AuthZ source; retrieval and AI scope are bounded by the *viewer's* RBAC.

## Cross-cutting product invariants (short list)

These recur throughout and constrain most screens: estimate before provision · env-as-filter · one arithmetic everywhere · plans gate capabilities never safety · soft limits bill / hard limits fail loudly, 80%-warned · templates copy never link, secrets never captured · tokens reveal once · no inline create/edit forms (see tier system) · the AI four laws · every failure state names a way forward (frame `A7` is the bar).

## Interaction tier system (`06-interactions/interaction-spec.md`)

Overlay choice is a decided rule, not a judgment call — **inline create/edit forms are prohibited**:
- **Modal** (centered, 460px) — one decision or ≤4 fields; typed-confirm escalation for data-destructive.
- **Drawer** (right, 424px) — complex forms needing previews/live checks/page context.
- **Page** — multi-step / provisioning / asset creation.

Global: `⌘K` command palette · `⌘J` assistant drawer · `Esc` closes topmost overlay · `/` focuses page search.

## AI is a proposal engine, not an actor (`12-ai/ai-implementation.md`)

Four non-negotiable laws: suggest never act (no auto-apply path exists in the API) · every claim cites evidence · retrieval scoped to viewer RBAC · platform is whole without AI (`ai-assistant` org policy toggles it, deletes nothing). Assistant tools are read-only + draft; applying a proposal is a normal human-session API call. Render the proposal object in order: `evidence[]` → `reasoning` → `proposed_change` → `impact` → human-only Apply → audit event.

## Conventions when generating code (`14-development/architecture.md`)

- Frontend: one feature = routes + components + hooks; cross-feature imports only via `design-system`, `api`, `lib`. Components are PascalCase matching the design-system class name.
- Files kebab-case; TypeScript strict.
- Money: integer cents server-side, rendered via `fmtMoney` in mono font.
- Errors: problem+json everywhere; client maps `status` → inline field error (422) / banner (409/402 with remediation) / toast (5xx w/ retry) / 429 honors Retry-After. Every error surface names a next step.
