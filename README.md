# Steloit Console

The Steloit developer-cloud console — a React + TypeScript + Vite SPA generated from the
design & spec handoff package (`steloit-handoff`). The gallery frames are the spec; microcopy
is copied verbatim; API types are generated from `08-api/openapi.yaml`; all demo data comes
from `19-canon/fixtures.json`.

## Stack

Vite · React 19 · TanStack Router (file-based, `?env=` is a first-class filter) · TanStack
Query · Zustand (thin global store: theme, palette, assistant, session) · Tailwind v4 over the
LATTICE tokens (`src/styles/tokens.css`, dark-first, `.light` class swap) · Hey API generated
client · MSW canon mode · Biome · Vitest.

## Scripts

| Script           | What it does                                            |
| ---------------- | ------------------------------------------------------- |
| `pnpm dev`       | Dev server in canon mode (MSW serves the canon world)   |
| `pnpm gen:api`   | Regenerate the client from `src/lib/api/openapi.yaml`   |
| `pnpm typecheck` | `tsr generate && tsc --noEmit`                          |
| `pnpm lint`      | Biome                                                   |
| `pnpm test`      | Vitest — includes the 16-qa canon arithmetic invariants |
| `pnpm build`     | Typecheck + production build                            |

## Canon mode

`VITE_API_MODE=canon` (the default) boots MSW and serves `19-canon/fixtures.json` through the
real API contract — list envelopes are `{data, next_cursor}`, errors are problem+json with a
required `remediation`. Set `VITE_API_MODE=live` and `VITE_API_URL` to talk to a real API.
Sign in with any email (canon session defaults to `priya@acme.dev`), then explore
`acme/ecommerce` — the canonical incident world ($208 project, deploy #142 → p95 812 ms →
proposal `prp_7c31a2` → #143).

## Source-of-truth layout

- `src/design-system/` — components named for their class contracts (`Pill` → `.pill`)
- `src/app/` — shell (rail · ctx · snav variants) and the ⌘K palette
- `src/features/` — one folder per domain (org, projects, services, invites, auth, onboarding)
- `src/lib/api/` — generated client + problem+json normalization; never hand-write API types
- `src/lib/canon/` — fixtures.json (verbatim copy) + frozen canon clock
- `src/routes/` — TanStack file routes; env is always `?env=`, never a path segment
