# AI agent development guide

The canonical onboarding for **any** autonomous coding agent (Claude Code, Cursor, Codex, or future tools) working in this package. Harness-specific files (`CLAUDE.md`, `.cursorrules`, …) are thin pointers into this document — one room, many doors. Humans: this works for you too; the constitution's reading paths are gentler.

## 0. What you are working in

This is **not source code** — it is the complete design & specification package for building the Steloit developer-cloud console (React + TS + Vite SPA over the REST API in `08-api/`). There is no build/lint/test here; every file is spec. "Build screen X" means *generating application code from these specs*, not editing an app in this repo.

## 1. Authority order (memorize this; conflicts are findings, not judgment calls)

1. **`00-sources/GOV-002-product-architecture.md`** — the product model. Resolve any "GOV-002 §N" citation here.
2. **`00-sources/Steloit-Console-Screens.html`** — 152 frames; frame ids (`W3`, `U6`, `AI3`…) are the cross-reference currency. A claim about a screen resolves by searching its frame id. Frame labels and microcopy are authoritative.
3. **`00-sources/Steloit-Console-Design-Spec.md`** — every decision with reasoning. Where it refines GOV-002's IA sketches (e.g. the shell), the spec wins — it is the later, binding refinement.
4. **The derived docs (01–22)** — each owns exactly one concern (README's index says which). They never contradict 00-sources; if one appears to, that's a defect to surface.
5. **`18-philosophy/product-philosophy.md`** governs *decisions not yet made*; **`decisions.md`** (ADRs) records what's settled — **check it before proposing any structural change**; you will otherwise confidently re-propose something already rejected (the create-dialog, the AI FAB, the Workspace primitive…).
6. **`99-history/` is never authority.** Superseded program docs and the pre-LATTICE prototype. Do not take tokens, patterns, or shell structure from it.

Single-owner rule for concerns: components → `01-design-system` · API shapes → `08-api/openapi.yaml` (generate types, never hand-write) · microcopy → the frames, verbatim · permissions → `11-permissions` (two-layer: matrix ceiling, then policy) · invariants → `16-qa` (automate them) · demo data → `19-canon` **only** · terminology → `18-philosophy/glossary.md`.

## 2. Workflow for any task

1. **Orient:** read the relevant frame(s) in 00-sources (search the frame id) → the owning derived doc → the design-spec section for the *why*.
2. **Check the ADR log** if your task touches structure, navigation, pricing display, AI behavior, or anything that feels like a "better idea."
3. **Build from owners:** components by design-system class contract (PascalCase component = class name: `Pill` → `.pill`); types generated from openapi.yaml; copy pasted from frames; fixtures imported from canon.
4. **Verify against 16-qa:** the consistency checklist is the definition of "aligned"; the ten canonical scenarios are the acceptance tests; the arithmetic invariants must stay green (import them from `19-canon/fixtures.json`, never retype).
5. **Report honestly:** what you followed, what you couldn't resolve, exactly which frames/docs you cited.

## 3. Hard rules (violating any of these is a defect, not a style choice)

- **Never invent a component.** A screen that seems to need a new one → it's a design-system change *first*, then use it.
- **Never hand-write API types**; never add an endpoint the spec lacks (spec change first).
- **No inline create/edit forms** — modal / drawer / page per `06-interactions`; the tier is a decided rule.
- **Microcopy verbatim** — the words are spec ("shown once", "no silent limbo"). New words follow `17-brand/voice.md` and register new nouns in the glossary first.
- **The AI four laws bind the code you generate:** no auto-apply path, proposals render evidence → reasoning → change → impact with human-only Apply, retrieval scoped to viewer RBAC, every AI surface must disappear cleanly under the `ai-assistant` policy.
- **Money is integer cents** end-to-end, rendered by `fmtMoney`, mono. Status uses the six-mark vocabulary (`ready`, not `running`).
- **No demo data outside canon.** Errors are problem+json with `remediation` always present; every failure surface names a next step.
- **One room:** never create a second surface/implementation for a job that has an owner; entry points may multiply, canonical surfaces may not.

## 4. When to stop and ask (instead of resolving silently)

- Two authorities genuinely conflict (frame vs spec vs doc) — surface it with both citations; conflicts are findings.
- A task requires a component, endpoint, policy, or term that no owner defines — propose the owner-level change; don't improvise locally.
- Your change would stress the grammar (page anatomy, tiers, status language, arithmetic) — escalate as a grammar question (playbook step 6); never resolve it with a bespoke screen.
- You're about to touch `00-sources/`, the constitution, or the ADR log — these change by human decision only.

## 5. The review checklist you run on your own output

From the constitution (§7): what does the user *know* after this? · what did we stop hiding? · which question does this surface own, and does anyone own it already? · does it pass all eight grammar elements? · what do the failure/empty/limit states say? · does it work with AI disabled?
