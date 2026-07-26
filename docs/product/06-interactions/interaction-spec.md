# Interaction specification

## The tier decision rule (verified across all 152 frames)
- **Modal (centered, 460px)** — one decision or ≤4 fields, page context not needed: invite (U1), token/key create → reveal (U7), delete template (T1), remove member, revoke key, rename, cancel-deploy. **Typed-confirm** escalation for data-destructive (U6: dependents named, final backup, button enabled on exact name match).
- **Drawer (right, 424px)** — complex forms needing previews/live checks/page context: binding (U2), lifecycle rule w/ dry-run (U3), schedule w/ next-runs + {{date}} payload (U4), custom domain async (U5), alert rule w/ backtest (U8), dashboard add-widget (DB7), add secret, edit shape (small), webhook.
- **Page** — multi-step/provisioning/asset creation: service canvas (C), policies (G9), template capture (T3), cell connect (X2), promote (DP), plan change (B11/12), scale tab (D22).
Inline create/edit forms are prohibited (the P5/G8/O5/D11/D15/D17 cleanup is the precedent). Overlays never remove the page; Esc/✕ closes non-destructively; every overlay action audited like its page equivalent.

## Global interactions
- **⌘K** command palette (jump anywhere); **⌘J** assistant drawer; **Esc** closes topmost overlay; **/** focuses page search where present.
- **⚑ pre-fill grammar**: any chart in Metrics/Logs/Billing offers "New alert rule" and "Add to dashboard" — both land in their drawer pre-filled.
- **Drag & drop**: dashboard edit only — handle `⋮⋮`, grid snap, layout saved on "Done".
- **Dropdowns/menus**: `.inp` select pattern (chevron), env pill ▾, row "…" menus for secondary actions; destructive items last, red, separated.
- **Hover**: cards/rows tint to surface2; disabled controls tooltip their reason; locked features tooltip the gating plan.
- **Search & filters**: chiprow filters (single or multi), search inputs debounce 250ms; filters are AND'd; dashboard filter row applies to every widget.
- **Pagination**: tables paginate at 50 (server-driven cursor); ledgers (audit, invoices, activity) infinite-scroll with sticky header.
- **Bulk actions**: selection checkboxes appear on hover in ledgers that support them (members, tokens); bulk bar slides in bottom; destructive bulk → typed count confirm.
- **Confirmations**: only for destructive/irreversible; everything else is undoable or stated ("effective next deploy").
- **Toasts**: bottom-right, 5s, action link when reversible.
