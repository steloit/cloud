---
id: rbac
owns: [services/api/src/identity/**, docs/product/11-permissions/**]
see: [api-conventions, events-spine]
---

# Identity & Governance — the governed-resource contract

Authority: `docs/product/11-permissions/rbac.md` + `rbac-matrix.csv` (the matrix is DATA, loaded,
never hard-coded). Codified by **ADR-0006 / ADR-036**.

**The governed-resource contract (canonical — ADR-0006).** Identity, authZ, policy, and audit are
designed once and inherited by every managed product and Binding. A product/Binding is *governed* iff
it (1) is a resource in the org→project→env tree, (2) declares its `resource.action` permissions in the
matrix, (3) routes every mutation through the two-layer evaluator, and (4) emits lifecycle events to
the append-only spine. **No product/Binding/module may implement its own authN/authZ/policy/audit** —
all use the contract; any exception needs a new ADR. Enforced as a module boundary (depguard); the
`identity` module holds the evaluator as in-process middleware (centralized, not a separate service).

## The two-layer evaluator

Every mutating request evaluates:

```
allow = matrix[role][permission] == Y
        AND policies.evaluate(actor, permission, {org, project, env}) == permit
```

This is the **AWS Organizations SCP model**: the matrix grants a ceiling, the policy layer can only
*narrow*, composing as intersection, inherited down the tree. Recognized prior art — not novel or risky.
**RBAC + narrowing-policy, never ReBAC/Zanzibar** (that solves arbitrary *sharing graphs*; Steloit is a
*containment tree*). **Policy = a typed-Go evaluator over the finite platform set** (spend caps, region
allowlists, allowed providers, approval gates, quiet hours) — no DSL. Cedar/Rego is reserved for
*customer-authored* policies (v3); a language is premature for a finite set.

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
- **Machine identity (V2): CI/OIDC workload-identity federation is the blessed path** (ADR-0006) —
  Steloit is an OIDC relying party; a customer's GitHub Actions federates into a **scoped, short-lived
  token** (zero long-lived key stored in CI), the OIDC `sub`/`environment` claim mapping to an
  env-scoped role. Long-lived personal/org tokens remain for humans and legacy.
- AuthN: own the model, adopt libraries (`go-webauthn`/argon2id/TOTP), passkeys-first; **WorkOS for
  enterprise SSO/SCIM at v3** (federates into our model — never owns our users); never buy Clerk/Auth0
  as the primary store. "Why was this denied" explainability is load-bearing (denials name their source).

## Mistake bank

- Widening a matrix N via a policy (impossible by contract — property-test it).
- Checking permissions in middleware only for "important" routes — every mutation, both layers.
- Caching a token's grants past a role change (re-evaluate at use).
- Denials without the role/policy name in problem+json `detail` (breaks the E3 grammar).
- Skipping the audit event on denial (denials are audited, not just grants).
