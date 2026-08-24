# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root
- **`docs/adr/`** — read ADRs that touch the area you're about to work in

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## Layout

This is a **single-context** repo:

```
/
├── CONTEXT.md          ← created lazily by /domain-modeling
├── docs/adr/           ← engineering ADRs
├── apps/console/
├── services/
└── packages/
```

## This repo's existing authority (read before contradicting anything)

`AGENTS.md` (symlinked as `CLAUDE.md` / `GEMINI.md`) defines an authority order that outranks anything in this file:

- `docs/product/` — the design authority; `00-sources/` is the top of the ranking
- `docs/architecture.md` — Architecture v1, FROZEN (ADR-0001)
- `docs/product/18-philosophy/decisions.md` — product ADR log, human-decision-only
- `contexts/` — Context Packs; load the ones a task lists

Skills that name a domain concept should use the vocabulary from those docs.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal — either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-0007 (event-sourced orders) — but worth reopening because…_
