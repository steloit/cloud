# QA documentation

## Canonical test scenarios (from canon — deterministic fixtures)
1. Incident replay: deploy #142 → order.paid failures → DLQ 2 → insight/proposal prp_7c31a2 → fix ships #143 → replay → p95 431ms visible on O2, DB3, DB7 markers.
2. Quota warning: egress 87/100GB → banner+bell+email at 80%; math shows ~$1.62; no upsell when overage is cheaper.
3. Dunning: fail card → day 8 = banner, provisioning paused, retries listed; payment clears everything instantly; day-90 rule never earlier.
4. Downgrade block: Business→Pro with 12 members & 2 cells → 409 with both reasons, verbatim.
5. Template safety: save with secrets in source → contents contain zero secret material; excluded binding → required_input; delete used template → instantiations untouched.
6. Token reveal: create → plaintext exactly once; GET returns prefix+hash only; role shrink narrows token scope immediately.
7. AI disable: set policy disabled → all AI surfaces gone (AI1/2/4/5/6–9/10–12/W10), Create/Observe/Deploy/Settings byte-identical; re-enable restores threads.
8. Invite lifecycle: expire at 7d; renew notifies inviter; accept from wrong email blocked; already-used → "sign in" path.
9. Env-as-filter: switching env pill re-filters services, metrics, logs, dashboards(project-scoped) without route change beyond ?env=.
10. Typed confirm: delete db-main disabled until exact name; response names api+worker; final backup recorded.

## UI consistency checklist (mirror of the design audits — automate)
- exactly one active `.nit` per snav; one `.rit.on` per rail; banner classes ⊆ {default, warn}
- no inline create/edit forms outside page-tier screens (census rule)
- pghead: no breadcrumb line; h1 subpage grammar "Area · Thing"
- assist button filled ⇔ drawer open; violet = AI content only
- money arithmetic: org total == Σ projects + plan fee everywhere it appears
- every frame-id-style cross-link resolves; disabled controls carry a reason; empty states carry CTA + CLI
- pills always carry text (never color-only); Esc closes overlays non-destructively

## Regression checklist (per release)
auth/invite happy+edge · onboarding 4 steps · create service w/ estimate · bind/unbind rotate · deploy+rollback+markers · alert backtest · dashboard CRUD+drag+share+filters · template save/consume/delete · billing arithmetic & invoices · plan up/down/cancel/reactivate · dunning timeline · tokens/keys reveal-once · policies incl. AI3 · quiet hours routing-only · a11y: keyboard-only pass, focus trap, contrast.
