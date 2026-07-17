# Information architecture

## Context model
**Organization → Project → Environment.** Org = identity/billing/policy boundary. Project = the application. Environment = a filter, not a place: the crumb's env pill filters every project-scoped surface in place (env-as-filter law). Switchers: org via crumb avatar (SW1), project via crumb, env via pill ▾.

## The rail (left, fixed)
Admission rule: **weight × frequency**; items are doorways, not containers.
1. **Home** (`s-hex`) — org-wide landing: projects, costs, activity; hosts the **Dashboards** section (single-door decision: dashboards are reached from Home only).
2. **Observe** (`s-pulse`) — telemetry suite: Health · Metrics · Logs · Traces · Alerts (+ events).
3. **Deploy** (`s-deploy`) — promotion, rollouts, previews (DP-series).
4. **Services zone** — per-product entries in current project context. Two shapes: *fleet* (badge ×n, e.g. db ×2) vs *capability* (no badge, e.g. AI Gateway).
5. **+** (dashed) — creation canvas (C1).
6. **⚙** — settings plane.
Never on the rail: Alerts, AI Insights, Dashboards, Templates, Cells (all reached through their owners).

## Sidebars (`.snav`) by area
- **Home**: All projects · **Dashboards** (Overview / Pre-built: PostgreSQL Health · Valkey Performance · AI Gateway · Infrastructure · Cost & Usage · Deployments / My · Shared · Templates) · Projects group · New project.
- **Settings**: Project group (General · Members & roles · Git integration · Policies) · Organization group (General · Members · Audit log · Policies · API keys · Cells · Templates) · Billing group (Overview · Usage · Invoices · Payment & plan). Account (P-series): Profile · Security · Personal tokens · Notifications · Quiet hours.
- **Assistant** (⌘J / top-nav button): Ask · Insights · Activity · Capabilities; footer names the four laws + the disable policy (AI3).

## URL structure
```
/login /signup /invite/:inviteId
/onboarding/{org|team|project|connect}
/:org                       → Home
/:org/dashboards[/:dashId]
/:org/settings/{general|members|audit|policies|api-keys|cells|templates}
/:org/billing/{overview|usage|quotas|invoices|payment|plans}
/:org/:project?env=:env     → project overview (env is a query param everywhere: env-as-filter)
/:org/:project/observe/{health|metrics|logs|traces|alerts}?env=…
/:org/:project/deploy?env=…
/:org/:project/svc/:service/{overview|…tabs}?env=…
/:org/:project/settings/{…project group}
/:org/create                → creation canvas
/account/{profile|security|tokens|notifications}
/assistant/{ask|insights|activity|capabilities}
```
Deep links preserve env; switching env rewrites the query param, never the path. Overlays (modals/drawers) are route-less UI state, except the Assistant drawer which may carry `#assistant`.

## Breadcrumb behavior
Crumb always shows Org / Project / env-pill in project context; org-level pages (Home, Dashboards, org settings, Billing) show Org only. Crumb elements are switchers, not links to "index pages".
