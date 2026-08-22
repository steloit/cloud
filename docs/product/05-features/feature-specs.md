# Feature specifications

Format per feature: goal · stories · functional requirements (FR) · business rules (BR) · validation · permissions · edge cases · acceptance criteria (AC). Permissions reference the RBAC matrix.

## F1 Organizations & membership
Goal: identity/billing/policy boundary. Stories: owner creates org; admin invites; member joins via A6.
FR: create (name→permanent slug, home region), invite (email+role, bulk, 7-day expiry), roles Owner/Admin/Developer/Billing, audit every membership change. BR: seats per plan (Free 3 / Pro 5 / Business 20, then $7/seat prorated); slug immutable; last Owner cannot leave/demote. Validation: email RFC + dedupe against members+pending; slug `[a-z0-9-]{3,32}`. Edge: invite to existing member → noop w/ message; wrong-account acceptance blocked (A7 card 3). AC: accept grants access instantly, logged; decline notifies inviter and invalidates the link.

## F2 Projects, environments, services
FR: project CRUD; env per project w/ region override; services created only through the canvas with a live estimate; env-as-filter everywhere. BR: nothing provisions or bills before estimate acceptance; scale-to-zero where the product supports it; deleting anything takes a final backup first (U6). Edge: create in a project at plan project-limit → gate with reason. AC: estimate shown == first invoice line grammar.

## F3 Bindings
FR: create from a service (drawer U2): target service, scope `read-only|read-write`; credentials minted at bind, rotated at unbind; injected env vars deterministic (`<TARGET>_URL`). BR: bindings are wiring — $0; effective next deploy; ro enforced by the datastore. Edge: bind to already-bound target → disabled option; delete a bound service → typed-confirm names dependents (U6). AC: topology edge appears `pending` immediately.

## F4 Deployments & promotion
FR: deploy per env; promotion with diff; rollback one-click; deploy markers emitted to metrics/dashboards. BR: previews are plan-quota'd (hard limit → queue with reason). AC: marker (#id) visible on every chart of the affected env.

## F5 Domains & TLS
FR: add via drawer (U5): CNAME+TXT, 60s rechecks, async-safe, cert auto-issue + bell. BR: TLS never optional and never plan-gated. Edge: DNS >48h → guidance, never expiry of the request. AC: closing the drawer doesn't cancel verification.

## F6 Observe & alerts
FR: metrics/logs/traces/events; alert rules via drawer (U8): query, condition, window, routes (bell/email/webhook), 7-day backtest. BR: quiet hours (P6) affect routing only; alerts respect env filter. AC: backtest reproduces historical firings (431ms canon).

## F7 Dashboards
FR: org section under Home (single door); pre-built = generated views (fleet PostgreSQL, Valkey, Infrastructure, Cost & Usage, Deployments; the pre-A5 AI Gateway dashboard is S9-pending — re-cast as AI-Binding usage or drop) — never files, fork-to-customize; custom dashboards: scope (org-wide|project) ⊥ visibility (personal|org|restricted); widgets across planes (metrics/logs/cost/deploys/AI/alerts); drag layout; ⚑ add-to-dashboard pre-fills the widget drawer; filters project/env/region/product apply to all widgets. BR: project-scoped = born filtered + project permissions; org-wide renders only viewer-accessible projects and says so; dashboards read, never provision. AC: editing a shared dashboard is live for all viewers and audited.

## F8 Infra templates
FR: save-as-template (T3): service subset, name, visibility; manage (T1/T2): edit shapes, duplicate, delete, refresh-from-source; consume in create flows w/ estimate. BR: template = frozen copy, never a live link (delete/edit never affects instantiations); **secrets/data/keys never captured** — placeholders re-mint per consumer; binding to an excluded service becomes a required input. AC: instantiation estimate shown at save, in list, and at consume.

## F9 Subscription & billing (hybrid model)
BR (canonical): subscription = platform capabilities; pay-as-you-go = infrastructure; overage = beyond included quotas. **ADR-041 (2026-07-18):** presentation is product-first — lines labeled by outcome via the `intent` tag, expanding exactly to cost components and meters (generalized exact-sum; usage ranges bounded by the cap); Jobs (incl. scheduling), basic Search, and Vector are **included Database capabilities within plan allowances** (included, not unlimited; future tiers stay features of the Database product, never separate products) — never separately metered; Pro $29 canonical (INF-001 §6's $19/"Team" corrects via S9). Tiers: Free $0 / Pro $29 / Business $99 / Enterprise custom (matrix in B5; SSO=Business+, audit export=Enterprise, BYOC cells=Business+ w/ per-cell control-plane fee, dedicated cells=Enterprise). Quotas: soft (egress $0.09/GB, seats $7, builds $0.01/min, events $1.20/M, AI requests $2/1k) keep working and bill; hard (previews queue w/ reason; API 429+Retry-After) fail loudly, warned at 80% (banner+bell+email w/ math). Upgrades immediate+prorated; downgrades at anchor, blocked with reasons when over limits; cancellation: plan ≠ resources (services keep running and metering); dunning: day 0 fail → retries →7 provisioning paused →21 suspend state-kept →90 only-then deletion w/ final notice. Never gated: TLS, backups, MFA, policies, alerts, dunning protections, self-deletion. **The cap (flagship, elevated 2026-07-18 per wedge review):** an org may set a hard monthly spend bound (B1 budget); the platform *enforces* it — at the bound, new provisioning pauses and soft-overage stops accruing (running services are never deleted; state kept; one click resumes on raise/renew) — never alerts-only. The bound composes with the estimate gate: an estimate that would cross the cap is refused at accept time with the math shown. AC: one arithmetic everywhere ($383 + $99 = $482 grammar); recommendations are calculators ("do nothing" valid); the cap is real (crossing it is impossible by construction, not by vigilance).

## F10 Tokens & API keys
FR: personal tokens (act as the user, shrink with roles) and org keys; create via modal; **reveal once** (U7), hash stored; scopes ro/full; expiry; revoke. AC: secret never retrievable after the modal.

## F11 Policies & governance
FR: org+project policies (G-series), segmented enforcement, per-project overrides, audit trail with event ids. Includes `ai-assistant` policy (AI3).

## F12 AI assistant (four laws — see 12-ai/)
Suggest-never-act (proposals only, human applies) · explainable with cited evidence · analyze broadly, act within the viewer's permissions · platform whole without AI, org-disableable (AI3 hides AI1/2/4/5/6–9/W10/AI10–12; never touches Create/Observe/Deploy/Settings; nothing deleted; instant re-enable).

## F13 BYOC cells
FR: connect flow (X2/X3), health verify, appears in region selector; Business+; per-cell control-plane fee; Enterprise adds dedicated/private.

## F14 Masking-by-policy (elevated to F-level 2026-07-18 per wedge review; founder-approved · investment level reaffirmed 2026-08-22 · `docs/plan/positioning-v2.md`)
BR (canonical): any environment born from production-shaped data (preview branches, restore-to-branch, migration-test envs) carries a **masking policy applied at creation** — masked columns are transformed before the environment is reachable; the PR/console line states it (`masked · policy`). FR: org/project-level masking policies (column rules: redact/hash/synthesize; sensible defaults for common PII shapes); policy versioned like all policies (F11); v0 = static ruleset at branch creation; V1 = policy-driven + DDL-aware sync feeds for long-lived envs. Never gated by plan (masking is safety). AC: a masked environment provably contains zero unmasked values for policy-covered columns (verifiable check, not assertion); no branch of production exists without a named policy applied; masking failures block environment creation loudly (fail closed, remediation shown). Rationale: safety whenever a preview carries real data — SaaS teams legally cannot use prod-shaped previews without it; implementation rides F2/F4 branching mechanics (which remain implementation, never positioning). **Positioning note (`docs/plan/positioning-v2.md`):** previews-with-masked-data demoted from wedge to secondary capability; the *functional* requirements above are unchanged and the investment level is unchanged — the fence is that masking is never described as the product's differentiator, and never as a testing or QA capability.
