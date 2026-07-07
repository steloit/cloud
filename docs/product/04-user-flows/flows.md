# User flows — end to end (frame refs in parentheses)

1. **Sign up / sign in**: A1/A2 → org create (A5) or invite landing. SSO for Business+ orgs (B6 enabled-state).
2. **Invitation**: email link → A6 (role explained, sent-to-you check) → accept → member (audited) | decline (notifies) | failure states A7 (expired→request-new notifies inviter; invalid→"sign in first"; wrong account→switch).
3. **Onboarding**: A5 Organization (name, home region) → A11 Invite team (email+role, bulk, seat-aware, skippable) → A8 First project (blank | template w/ price) → A9 Connect (CLI verify · Git · none) → A10 welcome widget.
4. **Create service**: rail + → C1 canvas (grid | describe-to-provision AI1 | template) → configure w/ live estimate → provision (`--prov`) → service page. Nothing bills before the estimate is accepted.
5. **Create project**: Home → New project (W2) → same canvas grammar; "Save as template instead" → T3.
6. **Environments**: E1–E5; env is created per project, selected via crumb pill; per-env region override.
7. **Bindings**: service page → Create binding (U2 drawer): target, scope ro/rw, env-var preview; effective next deploy; edge appears pending on topology (W3).
8. **Deploy/promote**: Deploy suite DP1–DP3; promotion with diff, rollout, rollback; deploy markers propagate to charts (#143 canon).
9. **Domains**: service → Add custom domain (U5 drawer): CNAME+TXT → async verify (close-safe) → cert auto-issues → bell.
10. **Scaling**: D22 scale tab (page-tier form, reason required for temp overrides).
11. **Backups/PITR**: product pages; PITR window by plan (Pro 7d, Business 30d); final backup before deletion (U6).
12. **Observe**: O1 health → O2 metrics (⚑ → alert rule U8 / dashboard widget) → O3 logs/traces → O5 alerts; N-series bell → inbox.
13. **Alert rule**: O5 → drawer U8: query (pre-filled from ⚑), condition/window, routes, 7-day backtest → create → fires through Observe.
14. **Dashboards**: Home → DB1 overview → open pre-built (DB2/3/4, fork-to-customize) | create (DB8 modal: name, scope org/project, visibility, start-from) → edit (DB7 drag ⋮⋮, add-widget drawer, ⚑ lands pre-filled) → share (DB5).
15. **Templates (infra)**: W2 "Save as template" → T3 capture (services subset, secrets never, excluded-binding→required input, estimate) → manage T1/T2 → consume in C1/C5/A8.
16. **Billing read**: B1 overview (one arithmetic everywhere) → B2 usage / B7 quotas → B3 invoices → B4 payment & plan.
17. **Quota warning**: 80% → banner+bell+email (B8) → math in the open → recommendation (calculator, "do nothing" valid).
18. **Plan change**: B4 → B5 compare → upgrade immediate+prorated | downgrade at anchor, blocked-with-reasons if over limits.
19. **Trial → paid**: B9 contract-up-front → B11 confirm ($29, anchor) | lapse to Free (nothing deleted).
20. **Failed payment**: charge fails → grace timeline B10 (day 0/7/21/90, position marked) → update card → instant clear.
21. **Cancel/reactivate**: B12 — what stops vs keeps running+billing; Resume/Restart one click.
22. **Tokens & keys**: P5/G8 → Create (modal) → U7 reveal-once (hash stored) → revoke anytime.
23. **Policies**: G7 list → G9–G11 create (page tier) → enforcement; AI policy AI3 (Enabled/Opt-in/Disabled, exact hide list, no data deleted).
24. **AI flows**: entry AI4 drawer/⌘J or AI2 workspace → ask w/ cited evidence → insight (AI10 inbox, AI6–8 panels) → follow-up (AI9) → proposal (W10 grammar: evidence→reasoning→change→impact) → human applies → audited ("as you, via assistant"). Disable: AI3 removes all surfaces; platform unaffected (Law 4).
25. **Notifications & quiet hours**: N1–N3, P6 — quiet hours affect routing, never recording.
26. **BYOC cells**: X2 connect (page) → verify → cell appears in region selector; Business+ (control-plane fee/cell).
27. **Error recovery**: every failure carries a path (A7, B10, U5 recheck, DP3 rollback, 429+Retry-After on hard limits).
