---
id: rbac
owns: [services/api/src/identity/**, docs/product/11-permissions/**]
see: [api-conventions, events-spine]
---

# Authorization — the two-layer contract

Authority: `docs/product/11-permissions/rbac.md` + `rbac-matrix.csv` (the matrix is DATA, loaded,
never hard-coded). Every mutating request evaluates:

```
allow = matrix[role][permission] == Y
        AND policies.evaluate(actor, permission, {org, project, env}) == permit
```

- Roles (org-level): owner · admin · developer · billing. Permissions are `resource.action`.
- **Y is a ceiling** (policies may narrow it). **N is final** (nothing can widen it — tighten-only).
- There is NO environment column in the matrix: environment-sensitive restrictions (prod promotion
  gates, region allowlists, compute ceilings) are **always policies, never role rows**.
- Policy inheritance: closest wins; org sets floors/ceilings.
- Denial grammar (E3 frames): matrix denial names the **missing role**; policy denial names the
  **policy**; both are audited as events; UI renders gated verbs visible-but-disabled (B6).
- Tokens: personal tokens act AS the user and re-evaluate against **current** roles at use time
  (demotion shrinks a live token). Org API keys carry explicit scopes. Reveal-once; hash stored.
- `ai.apply_proposal` is "per underlying action": applying an AI proposal re-runs BOTH layers for
  the change it contains — the assistant can never hold apply rights (Law 1).

## Mistake bank

- Widening a matrix N via a policy (impossible by contract — property-test it).
- Checking permissions in middleware only for "important" routes — every mutation, both layers.
- Caching a token's grants past a role change (re-evaluate at use).
- Denials without the role/policy name in problem+json `detail` (breaks the E3 grammar).
- Skipping the audit event on denial (denials are audited, not just grants).
