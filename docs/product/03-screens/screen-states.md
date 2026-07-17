# Screens — purpose, entry/exit, actions, permissions, states

`screen-inventory.csv` lists all 152 frames (id, family, kind, purpose — extracted from the gallery labels, which are authoritative one-line purposes). This doc adds the per-screen contract by family; where a specific frame *is* a state, it's named.

## Family map
| Family | Area | Frames |
|---|---|---|
| A | Auth, invitations, onboarding | A1–A11 (A6 accept, A7 failure states, A5→A11→A8→A9 wizard, A10 welcome) |
| W | Home / project overview | W1–W5 (+W10 proposal grammar) |
| DB | Dashboards (under Home) | DB1–DB8 |
| SW | Switchers | SW1–SW3 |
| C | Creation canvas + templates gallery | C1–C12 |
| S / D / M | Product pages (per-instance) & deep tabs | S1–S4, D1–D23, M1–M8 |
| O / N | Observe suite, notifications | O1–O5, N1–N3 |
| DP | Deploy suite | DP1–DP3 |
| E | Environments | E1–E5 |
| G | Settings & policies | G1–G12 |
| X | AI Gateway + BYOC cells | X1–X3 |
| B | Billing & subscription | B1–B12 |
| P | Account settings | P1–P6 |
| T | Infra templates management | T1–T3 |
| U | Interaction tier exemplars | U1–U8 |
| AI | Assistant & AI surfaces | AI1–AI12 |

## Per-screen contract (applies to every page)
- **Entry points**: rail/sidebar item, crumb, ⌘K, cross-links printed on frames (e.g. G7→AI3, B4→B5, B2→B7, W2→T3, C5→T1).
- **Exit paths**: crumb (up/over), rail, Esc (overlays), explicit "Open →"/"↗" links.
- **Permissions**: see `11-permissions/rbac-matrix.csv`. UI never hides gated actions — it disables with the stated reason (gating rules, B6).
- **Empty states**: every list ships one — pattern: glyph + one-line meaning + primary CTA + CLI equivalent (e.g. "No templates yet · Save one from any project — `steloit template save …`"). Never a bare "no data".
- **Loading**: skeleton rows on `--surface2`; provisioning anything uses `--prov` pulse with step text; async flows say so ("you can close this — we keep checking", U5).
- **Error**: inline under the control that failed + `.banner warn` for page-level; every error names a next step (no dead ends — A7 is the reference).
- **Success**: toast for small actions; state-change rendered in place (pill flips, row appears); irreversible reveals get their own modal state (U7 token reveal).

## Frames that ARE states (don't re-derive)
- Empty/first-run: A10 (welcome), C1 (empty create), AI5 (coachmark).
- Warning/limit: B8 (80% egress), D9/W4 (192/200 connections), X2 (cell verifying).
- Failure/edge: A7 (invite expired/invalid/wrong-account), B10 (dunning day 8), DP3 (rollback), O3/N2 (incident evidence).
- Async mid-state: U5 (domain verifying), X2 (cell), DB2 fleet live.
- Destructive: U6 (typed confirm), T1 (delete modal).
