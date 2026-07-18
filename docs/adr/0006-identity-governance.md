# ADR-0006 · Identity & Governance: one model, inherited by every product

**Status:** **Accepted** · founder-ratified 2026-07-18 · confirms the existing design (11-permissions/rbac.md, F10/F11, events spine, E2/E7, Architecture v1.2 identity module); adds one capability (CI/OIDC federation, V2). Product ADR log: ADR-036. Full analysis: `docs/plan/identity-governance-review.md`.
**Trigger (measured):** founder-requested adversarial review; two evidence sweeps (authorization models; auth build/buy + machine identity), 2026-07-18.

## Decision

Identity, Authorization, Policy, and Audit are **designed once and inherited by every managed product and Binding**, via the **governed-resource contract**: a product is *governed* iff it (1) is a resource in the org→project→env tree, (2) declares its `resource.action` permissions in the role matrix, (3) routes every mutation through the two-layer evaluator, and (4) emits lifecycle events to the append-only spine. The contract is a module boundary (Architecture v1.2 §15, depguard-enforced) — a new product inherits identity/authZ/policy/audit for free; it cannot opt out without bypassing the evaluator or the spine, which the boundary forbids.

**Confirmed (keep unchanged):**
- **AuthZ = RBAC + a narrowing-policy overlay** — role matrix as a *ceiling that only grants*, policy layer that *can only narrow*, composing as intersection, inherited down the tree. This is exactly the **AWS Organizations SCP + permission-boundary** model ("an SCP never grants… the effective permissions are the logical intersection"), reapplied to org→project→env. **Reject ReBAC (Zanzibar/OpenFGA/SpiceDB)** — a separate globally-consistent stateful service that solves arbitrary *sharing graphs* (Google Docs), not a *containment tree*; every verified peer (Vercel/Neon/Supabase) uses RBAC+scoping, no relationship engine.
- **AuthN = own the identity model + authZ; adopt libraries** (`go-webauthn`, argon2id, TOTP, recovery codes), **passkeys-first with fallback**; **reject Clerk/Auth0/Cognito/Firebase as the primary store** (SaaS in the trust path, per-MAU tax, owns your users). A dev-cloud's authZ *is* its product structure and cannot live in a vendor DB.
- **Audit = the events spine** (append-only, `via∈{user,assistant,system}`, `/audit` is a view). Immutable; corrections are new events.
- **Roles fixed** (owner/admin/developer/billing, data-driven matrix); **custom roles are new matrix columns, deferred to v3.**
- **Centralized in the `identity` module** (in-process interfaces), **not a separate authorization service.**

**Clarified:**
- **Policy = a typed-Go evaluator over the finite, platform-defined policy set** (spend caps, region allowlists, allowed providers, approval gates, quiet hours) — the "CAN vs SHOULD" split. **Reserve a policy language (Cedar/cedar-go) until customers must author custom policies (v3)** — `cedar-go` is v1.8 and still lacks the schema validator/formatter/templates; a DSL is premature for a finite set. **"Why was this denied" explainability is load-bearing from day one** (denials name their source — the E3 grammar).
- **Enterprise SSO/SCIM = WorkOS at v3** (federates an assertion *into* our model — does not own our users); self-hosted Dex/Keycloak only at scale (hundreds of connections). *Refines the prior "Dex for SSO" note.* Price SSO honestly (per-connection, not a 100× tier jump) — a trust brand visibly rejects the "SSO tax."

**Net-new (the one addition — add early):**
- **CI/OIDC workload-identity federation (V2).** Steloit becomes an OIDC relying party so a customer's GitHub Actions (later GitLab CI / cloud workload identity) federates directly into a **scoped, short-lived platform token — zero long-lived platform key stored in CI.** The OIDC `sub`/`environment` claim maps onto an env-scoped role in the two-layer evaluator. This is the 2025-26 default (GitHub/GCP/AWS/Azure all recommend it over stored keys), it is "infrastructure trust" made literal, it is a differentiator, and it is internally consistent with invariant 11 ("zero static keys"). Long-lived personal/org tokens remain for humans and legacy; federation is the *blessed machine path*.

## Context

Evidence (`identity-governance-review.md`): the two-layer model is validated by the AWS SCP precedent (the largest multi-tenant authZ system); ReBAC/Zanzibar is "likely overkill for single-application systems with simple RBAC" (AuthZed's own words) and adds a separate stateful service; peer dev-clouds build core identity and buy only the SSO edge (WorkOS's customer wall = Vercel/PlanetScale/Netlify/OpenAI); a policy DSL earns its cost only for customer-authored policies; CI/OIDC federation is the modern default for machine identity.

## Consequences

No architecture change (the identity module, two-layer evaluator, and events spine already exist in v1.2). No enum/product-surface change. No INF-001 amendment. One net-new V2 capability (CI/OIDC federation) and a few doc clarifications. The governed-resource contract makes "designed once, inherited by all" an enforced module boundary rather than an aspiration.

## Product ADR log entry (ADR-036 — applied 2026-07-18)

Applied to `docs/product/18-philosophy/decisions.md` as ADR-036 (see there). The governed-resource contract is the canonical identity abstraction; no managed product, Binding, or module may implement its own authN/authZ/policy/audit — all use the contract; any exception requires a new ADR.

## Ripple (on ratification)

ADR-0006 accepted; ADR-036 to the product log · `rbac.md` pack gains the governed-resource contract + SCP framing + typed-Go/Cedar-reserved + CI/OIDC note · roadmap places the V1/V2/V3 pieces · E7 note refined (WorkOS not Dex-first; passkeys-first) · one new V2 task (CI/OIDC federation) · mirror to handoff. No architecture/enum/INF-001/frozen change.
