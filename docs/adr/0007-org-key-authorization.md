# ADR-0007 — Organization API keys authorize against an explicit permission subset of the one canonical matrix

**Status:** Accepted (founder-ratified 2026-07-19)
**Deciders:** Founder
**Relates to:** ADR-0006 / ADR-036 (governed-resource contract), T2.7 (org keys authenticate but had no authZ path — recorded finding), T7.4 (this task)

## Context

Org API keys (G8: "the org's robots") authenticate through the same `tokens` table
and bearer path as personal tokens, but they have **no membership row** — so the
two-layer evaluator, which resolves permissions through `membership → role → matrix →
policy`, denied every org-key request past authentication. T2.7 recorded this as a
finding requiring an owner ruling: *how does a non-member principal acquire
permissions?* The design spec (G8) mandates "least-privilege scopes" but did not define
the scope vocabulary or the mapping.

The temptation is a parallel scope language (OAuth-style `deploy:write` strings, or a
bespoke org-key role). That would split the platform's permission model in two.

## Decision

**Org API keys authorize against an explicit subset of permission strings drawn
directly from the canonical permission matrix (`rbac/matrix.csv`). No new scope language
or parallel authorization model is introduced.**

- An org key carries an explicit list of permission strings (e.g.
  `["deploy.promote", "observe.read"]`); each must be a real matrix permission.
- **Least-privilege is the default:** a key grants only what its list names — there is
  no implicit "full" for org keys. An empty list can do nothing.
- During authorization the principal type branches:
  - **User principals** resolve as today: `membership → role → matrix ceiling → policy`.
  - **Org-key principals** authorize **directly against the key's granted subset** in
    the key's org scope — no membership, no role.
- **Policies may still narrow** an org key's effective permissions (the tighten-only
  layer applies to both principal types), but a key can **never** grant a permission
  outside its explicit list. The subset is a ceiling exactly as the matrix Y-cell is a
  ceiling for a role.

## Consequence

One canonical permission language across users, service accounts, automation, and
future integrations — multiple principal types, no duplicated authorization systems.
The `tokens` table gains a `permissions` column; `TokenInput` gains an optional
`permissions` array (personal tokens ignore it; org keys require it). Delegated
permissions (AI Law 1) and the "not registered" deny-by-default behavior are unchanged —
an org key requesting an unknown or delegated permission is denied the same way a role
is.

**Engineering principle:** one permission model · multiple principal types · no
duplicated authorization systems.
