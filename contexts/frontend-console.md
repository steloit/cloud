---
id: frontend-console
owns: [apps/console/**]
see: [api-conventions, canon-testing, events-spine]
---

# Console integration

The console renders all 152 frames against MSW canon mode; epic E8 swaps surfaces to the real API
slice-by-slice behind a per-family flag. `apps/console/AGENTS.md` carries the stack conventions;
this pack carries the integration knowledge.

## Slice protocol (E8)

- One slice = one surface family (auth/org → create/services → deploy → observe → settings/billing
  → AI). A slice flips only when: generated client regenerated (`pnpm gen:api`), four-state
  verification green under fault injection, Playwright subset green, canon mode still boots.
- **Canon mode is permanent** (ADR-026): it's the demo world and the E2E harness. Real-API paths
  are added beside it, never instead of it.
- SSE contract (`docs/product/10-state-management/state.md`): SSE-primary for toasts/bell/status
  pills with poll fallback 2s→backoff 10s; provisioning entities poll independently and **survive
  drawer close** (U5 domains pattern). Optimistic updates ONLY for pin/dismiss/layout-drag —
  never for anything provisioning, billing, or destructive.
- Query keys mirror URLs and always include `env` for project-scoped data (env-as-filter).

## The audit grammar (keep green — the suites check these)

Four-state truth on every query-backed surface · gated verbs visible-but-disabled with reason (B6)
· h1 "Area · Thing" · one active rail/snav item · pills carry text · money via `fmtMoney` mono ·
Esc closes overlays non-destructively · violet = AI only · microcopy verbatim from frames.

## Mistake bank

- Error-as-empty (an error state rendered as an empty state) — the audits' top recurring defect.
- Removing "static" chrome that is actually frame-specced blocked states — grep the frame id
  in `docs/product/00-sources/` before deleting anything that looks decorative.
- Optimistic UI on money/provisioning/destructive actions (server truth only).
- Breaking canon mode while wiring a real slice (both must stay green).
- Hiding a gated verb instead of disabling-with-reason (B6 violation).
