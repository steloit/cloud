# ADR-0012 — The cell namespace derives from the environment ID, not `proj--env`

**Status:** Proposed (agent, 2026-07-26) — **needs founder ratification**
**Deciders:** Founder
**Relates to:** INF-001 D7 (Environment → namespace) · US-3.3 · ADR-0003

## Context

INF-001 §D7 and `contexts/provisioning.md` specify the environment's Kubernetes
namespace as **`proj--env`** (project name, two dashes, environment name).
US-3.3 implemented it and the architecture review found it **unsafe**:

`projects` is `UNIQUE (org_id, name)` — project names are unique **per org, not
globally**. Two different orgs each with a project `api` and an environment
`prod` therefore derive the SAME namespace and land in it on the same cell,
**sharing the tenant isolation boundary D7 exists to create**: the namespace's
default-deny NetworkPolicy, its ResourceQuota, and CNPG's generated
`<cluster>-app` credential Secrets. This is the *default* case for common project
names, not an edge case.

Two further defects follow from name-derivation:

- **Sanitization collisions inside one org**: `My App`, `my_app`, `my.app` all
  sanitize to `my-app`; and because single dashes survive, `a--b` + `c` is
  ambiguous with `a` + `b--c`.
- **Renames orphan running clusters**: a project rename changes the derived
  namespace, so a later delete is issued against a namespace that does not hold
  the cluster — it 404s, reports `gone`, and the real Postgres keeps running.

## Decision

Derive the namespace from the **environment's ID** — globally unique and
immutable — rather than from project/environment names:

```
namespace = sanitize(env_id)      e.g. env_9f3c… → env-9f3c…
```

D7's *contract* is unchanged: environment → exactly one namespace, 1:1, carrying
default-deny NetworkPolicies, ResourceQuota and LimitRange. Only the **name
derivation** changes, from a colliding function of mutable names to an injective
function of an immutable id.

## Consequences

- **Cross-tenant collision becomes impossible by construction**, not by
  convention or validation.
- **Renames are safe**: the namespace never moves, so no cluster is orphaned.
- **Cost: human readability.** `env-9f3c…` does not say "acme/prod" at a glance;
  an operator needs one lookup. Mitigations if this proves painful: label the
  namespace with org/project/env names (searchable, non-load-bearing), or append
  a readable prefix to the id.
- **`00-sources/INF-001` §D7 and `contexts/provisioning.md` now differ from the
  implementation** until this ADR is ratified and D7's wording updated. That
  update is a **human-only** edit (AGENTS.md hard rule); this ADR exists so the
  deviation is visible and decided rather than silently shipped.

## Alternatives considered

- **Keep `proj--env`, add a uniqueness check** — does not work: names are only
  unique per org, so the check would have to forbid a second org from using a
  common project name.
- **`org--proj--env`** — removes the cross-org collision but keeps the rename
  hazard and the sanitization ambiguity, and lengthens toward the 63-char limit.
- **Hash suffix on names** (`acme-prod-9f3c`) — readable AND unique; rejected for
  now only because it still moves on rename. A reasonable founder alternative if
  readability outweighs that.
