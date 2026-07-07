# Development documentation

## Architecture
SPA console (React+TS+Vite) → REST API (`08-api/openapi.yaml`) → services (control plane) + per-product data planes; event stream (SSE) for realtime; CLI shares the API. AuthN: session + tokens; AuthZ: RBAC middleware from `11-permissions`.

## Frontend structure
```
src/
  app/            # router, providers, layout shells (rail, ctx, snav)
  design-system/  # tokens.css import, primitives (Button, Pill, Card, Table, Modal, Drawer, Logwell, Spark…)
  features/
    org/ projects/ services/ observe/ deploy/ dashboards/ billing/
    templates/ policies/ cells/ assistant/ account/ onboarding/
  api/            # generated client from openapi.yaml + query hooks
  lib/            # ctx (org/project/env), rbac, shortcuts (⌘K/⌘J), fmt (money/mono)
```
One feature = routes + components + hooks; cross-feature imports only via design-system, api, lib.

## Backend structure
```
services/ api-gateway | control-plane (orgs, projects, services, bindings, templates, policies)
          billing (meters, quotas, dunning) | observe (ingest, query, alerts)
          assistant (threads, insights, proposals — read-only tools) | provisioner (per-product drivers)
shared/   types (generated from openapi), events schema, rbac
```

## Conventions
- Naming: ids `xxx_` prefixed; components PascalCase = design-system class name (Pill→.pill); files kebab-case; API snake_case JSON.
- TypeScript strict; types generated from OpenAPI — never hand-written for API shapes.
- Money: integer cents server-side; render via `fmtMoney` (mono font).
- Copy: reuse gallery microcopy verbatim — it is spec ("shown once", "no silent limbo", "calculators, not sales").

## Error handling
problem+json everywhere; client maps `status` → inline field error (422) | banner (409/402 with remediation) | toast (5xx w/ retry) | 429 honors Retry-After. Every error surface names a next step (A7 is the bar). Sentry-style capture with event id shown in mono.
