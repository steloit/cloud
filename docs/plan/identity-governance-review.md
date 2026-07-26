# Identity & Governance Review — Adversarial First-Principles Pass

**Status:** Proposed for founder decision · 2026-07-18 · Same governance as ADR-0003/0004/0005: analysis; nothing touching frozen surfaces or human-only docs changes until ratified.
**Method:** ADR-0003-style attack on Identity, AuthZ, Policy, and Audit. Two evidence sweeps (authorization models; auth build/buy + machine identity) + the strategy corpus + the existing design (11-permissions/rbac.md, F10/F11, the events spine, E2/E7, Architecture v1.2's identity module).
**Verdict up front:** the subsystem is **already largely designed and already correct** — and the core question is already answered *yes by construction*. This is a **confirm-and-codify** review (like BYOC), not a redesign. The evidence *validated* the two-layer model against the strongest possible precedent (AWS Organizations SCPs) and *rejected* the tempting complexity (Zanzibar, a policy DSL, buying auth). It surfaced exactly **one net-new capability worth adding early — CI/OIDC workload-identity federation** — plus a few clarifications.

---

## 0 · The core question — yes, by construction

**"Can Identity, Authorization, Policy, and Audit be designed once so every future managed product automatically inherits them?"** Yes — and three decisions already frozen into Architecture v1.2 make it true:

1. **The `identity` module holds the two-layer evaluator as middleware** every mutating endpoint passes through (Architecture v1.2 §15). Authorization is centralized in-process, not per-product.
2. **The events spine is the single append-only pipeline** every product emits to; `/audit` and `/events` are views over it (GOV-002 primitive 9). Audit is centralized.
3. **The org→project→env→service tree is the tenancy** every product plugs into (GOV-002 §1.2). Ownership, isolation, and scoping are inherited, not re-invented.

The explicit contract this review names (§10): **a product or Binding is *governed* iff it is a resource in the org→project→env tree, declares its `resource.action` permissions in the matrix, evaluates every mutation through the two-layer evaluator, and emits lifecycle events to the spine.** Implement that interface and a new product inherits identity, authZ, policy, and audit for free. Postgres, Web, Workers, Valkey already do; so will every future product and Binding. **If not** — the only way a product could *fail* to inherit is by bypassing the evaluator or the spine, which the module boundaries (depguard, Architecture v1.2 §15) forbid.

---

## 1 · Authentication — build the model, adopt libraries, buy only the SSO edge

**Decision (confirms E2/E7):** **own the identity model + authZ; adopt *libraries* (not services) for authN primitives; adopt a *broker* for enterprise SSO — post-PMF only.** A dev-cloud's authorization *is* its product structure (org→project→env→resource governs billing, quotas, deploys) — it cannot live in a vendor's database.

The evidence is decisive: **WorkOS's own customer wall is the dev-cloud peer set** — Vercel, PlanetScale, Netlify, Cursor, OpenAI — every one of them *built* core identity and *bought only* the SSO/SCIM edge; Supabase runs its own auth server. Buying Clerk/Auth0/Cognito/Firebase as the *primary* user store is a category error for a product whose value proposition is infrastructure trust: they own your user records and meter you per-MAU precisely where you grow.

| Concern | Choice | Why |
|---|---|---|
| Password hashing | **argon2id** (m=19MiB, t=2, p=1 — OWASP) | Library, no service; D5 "never invent crypto" |
| Passwordless | **passkeys-first (`go-webauthn`) + email magic-link fallback** | Right for a dev audience (they carry password managers); no shared secret to phish; fallback for recovery/compliance |
| MFA | **TOTP + WebAuthn + recovery codes** | Self-implemented over libraries; never plan-gated (F9) |
| Enterprise SSO/SCIM | **WorkOS as a broker** (federates an assertion *into* our model — does not own our users), v3; reconsider self-hosted Dex/Keycloak only at hundreds of connections | The one acceptable "buy"; building SAML yourself is a signature-verification minefield |
| Reject | Clerk, Auth0, Cognito, Firebase as primary store | SaaS in the trust path; per-MAU tax; owns identity |

**Sequencing:** V1 email+password+TOTP+passkeys (the auth section, S1). V2 passkeys hardening, org API keys, **CI/OIDC federation (§3)**. V3 SSO/SCIM/SAML/JIT (WorkOS). *(Refines the prior "Dex for SSO" note: WorkOS is the pragmatic first move; Dex/Keycloak is the scale option.)*

## 2 · Identity model — the org owns everything; users are members

Confirms GOV-002. Entities: **User → Organization → Project → Environment → Service/Resource**, plus Membership (user↔org with a role), Invite, Token, API Key. **The organization owns every resource** — never a user directly (a user spans orgs; ownership must survive a user leaving). Answers to the review's questions:
- **Users own resources directly?** No. Resources roll up to the org (ADR-002: "the Project is the atom"). A user's leaving never orphans a resource.
- **Invitations:** email + role, 7-day expiry, dedupe against members+pending, wrong-account acceptance blocked, accept/decline audited (F1). Already specified.
- **Ownership transfer:** org/project `:transfer` endpoints (spec-change §2b) — org-level, audited, last-owner-protected.
- **Deleted users:** removing a member revokes sessions/tokens and *flags* (never silently deletes) owned resources for reassignment; the last owner cannot leave (DB trigger + 409). Account self-delete is grace-windowed.

## 3 · Service identity — OIDC federation is the modern default (the one net-new)

**The single genuine addition this review recommends, and the evidence says do it *early*, not late.** Machines should not carry long-lived stored secrets. The 2025-26 default across GitHub/GCP/AWS/Azure is **OIDC workload-identity federation**: a CI job requests a short-lived JWT (`sub=repo:org/repo:environment:prod`), the relying party trusts the issuer + gates on claims, and mints a credential valid only for that job. Google's own docs say long-lived SA keys "present a security risk."

**Recommendation:** Steloit becomes an **OIDC relying party** so a customer's GitHub Actions (later GitLab CI / cloud workload identity) federates directly into a **scoped, short-lived platform token — with zero long-lived platform key stored in their CI.** This maps cleanly onto the tree: the OIDC `sub`/`environment` claim → an env-scoped role in the two-layer evaluator. Long-lived personal/org tokens still exist for humans and legacy, but **federation is the documented, blessed path for machines** — which is "infrastructure trust" made literal, and a differentiator (it's also internally consistent with invariant 11's "zero static keys" for our *own* CI). **V2.**

## 4 · API keys — the hygiene is already specified; add IP allowlists at enterprise

Confirms F10 + the token schema. Personal tokens (act-as-user, shrink with roles, re-evaluated at use time) and org API keys (explicit scopes). Every key: **reveal-once (hash stored), scopes, expiry, revoke, `last_used_at`, audited on create/use/revoke.** Additions: **IP allowlists** are a v3-enterprise hygiene add (not V1); prefer **CI/OIDC federation (§3) over long-lived keys** as the recommended machine path. No redesign.

## 5 · Authorization — RBAC + a narrowing policy layer (this IS the AWS SCP model)

**The founder said "attack RBAC." Attacked — and RBAC + a policy overlay wins decisively for Steloit.** The dividing line the evidence draws is **containment tree vs sharing graph**:
- Steloit is a **tree**: org→project→env→service, roles inherit down, policies narrow per-environment. A bounded structure with a small closed set of roles.
- ReBAC engines (Zanzibar / OpenFGA / SpiceDB) exist to solve the **graph**: arbitrary user↔resource sharing edges authored at runtime (Google Docs, GitHub collab, Notion). OpenFGA's *own* teaching example is a Google-Drive clone. **That is a product feature (sharing), not an infra-hierarchy requirement.**
- Adopting a Zanzibar engine means running **a separate globally-consistent stateful service** with "zookie" consistency tokens on the critical path of *every* check — AuthZed itself calls it "likely overkill for single-application systems with simple RBAC, small teams." Exactly the premature complexity the frozen architecture avoids.
- **Every verified peer uses RBAC + scoping, no relationship engine:** Vercel (team-role + project-role + permission groups), Neon (org Admin/Member + project collaborators), Supabase (Postgres RLS).

**The validation that settles it:** Steloit's existing two-layer model — **role matrix as a ceiling that only grants, policy layer that can only narrow, composing as intersection, inherited down the tree** — is *exactly* AWS Organizations **SCPs + permission boundaries**: "an SCP never grants permissions… the effective permissions are the logical intersection." This is not novel or risky; it is the model the largest multi-tenant system on earth runs, reapplied to org→project→env. **Keep it unchanged.**

**One model serves every product** because permissions are `resource.action` strings in a data-driven matrix and the evaluator is product-agnostic middleware — a new product adds rows, not a new authorization system.

## 6 · Roles — fixed set now, composable permissions, custom roles at v3

Confirms the design. **Roles:** owner · admin · developer · billing (the `rbac-matrix.csv`). **Permissions are predefined and composable** (`resource.action`, matrix cells Y/N). **Custom roles are *not* necessary for V1–V2** — the policy layer already provides the fine-grained, environment-sensitive narrowing that custom roles would otherwise be invoked for (prod-promotion gates, region allowlists, compute ceilings are *policies*, never role rows). **Custom roles are a v3 enterprise feature** (GOV-002 v3), and even then they are new *matrix columns*, not a new model. A `security` role and a read-only `viewer` are the two most likely v3 additions; add them as columns when a customer needs them, not speculatively.

## 7 · Policy engine — "CAN vs SHOULD," a typed-Go evaluator; reserve Cedar

The founder's framing — permissions answer "**can** this user do this?", policies answer "**should** this operation be allowed?" — *is* the two-layer model, and naming it that way is clarifying. Layer 1 (matrix) = CAN; layer 2 (policy) = SHOULD (spend caps, region restrictions, allowed AI providers, deployment approvals, quiet hours, quotas, maintenance windows).

**Implementation decision: a hand-rolled *typed-Go evaluator over the finite, platform-defined policy set* — not a policy DSL.** The evidence: a policy *language* (Cedar/Rego/Casbin) earns its complexity only when policies are **open-ended or customer-authored**; Steloit's policy space is **finite and platform-defined**, so a typed evaluator is simpler, faster, fully type-checked, testable, and adds no DSL/sandbox/second service. Reinforcing facts: OPA/Rego's home turf is infra-config policy and it adds a second language + decision point; **`cedar-go` is v1.8 but still lacks the schema validator, formatter, and policy templates** (those are Rust-only). **Reserve Cedar (cedar-go) for the moment customers must author their own custom policies (v3 enterprise governance)** — that is the point the set stops being finite. Until then, typed Go, consistent with the frozen stack's boring-tech philosophy.

**One design requirement the evidence adds:** deny-only/intersection layers are hard to debug ("why can't I do X?" when a silent upstream narrowed it). **Build "why was this denied" explainability into the narrowing layer from day one** — which the rbac spec already mandates (denials name their source; the E3 grammar: who you are, what's required, who can grant it, denial audited). Confirm and keep it load-bearing.

## 8 · Audit — the events spine; enterprise export/streaming at v3

Confirms GOV-002 primitive 9. **One append-only pipeline** feeds audit + observability + notifications; `/audit` is a view. Every important action (login, deploy, provision, delete, restore, invite, permission change, policy violation, billing change, spend-cap hit, key creation, role assignment, org change) is an event with `via ∈ {user, assistant, system}` and an event id. **Immutable** (append-only at the DB layer; corrections are new events). **Retention:** days at alpha (INF-001), tiered up at revenue; **export (CSV/JSON) and SIEM streaming** are v3-enterprise (WorkOS audit-log streaming or native webhooks). Compliance (SOC 2 evidence) rides the same ledger. No new subsystem.

## 9 · Enterprise identity — v3, enterprise-only, priced honestly

SAML · OIDC · SCIM/directory-sync · group→role mapping · JIT provisioning · domain verification · seat management. **All v3, enterprise-tier** (consistent with the BYOC review and GOV-002 v3), added in the empirical order: SSO → domain-verify + JIT → SCIM (deprovisioning) → audit streaming. Delivered via **WorkOS** (federates into our model). **Reputational stance for a trust-brand:** price SSO reasonably (per-connection, not a 100× "enterprise tier" jump) and never bundle it with unrelated fluff — the "SSO tax" is a trust-eroding pattern a certainty-branded company should visibly reject.

## 10 · Architecture — centralized in the identity module; the governed-resource contract

**Identity & Governance is a *platform capability*, centralized in the `identity` module of the monolith (Architecture v1.2 §15) — not a separate service.** Authorization evaluation, policy evaluation, and audit emission are **in-process interfaces** every module calls; there is no network hop, no separate authz database, no consistency-token plumbing. This is the correct centralization for a modular monolith (a separate authz *service* would be the Zanzibar operational tax with none of the graph benefit).

**Every product consumes it by implementing the governed-resource contract (§0):** be a resource in the tree · declare `resource.action` permissions · route mutations through `authorize(actor, permission, {org, project, env})` (the two-layer evaluator) · emit lifecycle events to the spine. Bindings (storage/AI) are governed identically — an external Binding is still a resource in the tree with permissions and audit. **Designed once; inherited by all** — including future products and Bindings — because the contract is the module boundary, enforced by depguard.

## 11 · Database design (production-ready, sqlc/pgx, from models.md)

Confirms models.md; the tables already exist. Canonical set (integer-cents/`*_at`/prefixed-ids conventions, ADR-025):
- **users**(id `usr_`, email, …) · **orgs**(id `org_`, slug UNIQUE immutable, plan) · **members**(org_id, user_id, role, PK(org_id,user_id), ≥1-owner trigger) · **invites**(id, org_id, email, role, status, expires_at, UNIQUE(org_id,email) WHERE pending)
- **projects**(org_id, UNIQUE(org_id,name)) · **environments**(project_id, region, policy_flags[]) · **services**(env_id, product∈{postgres,valkey,web,worker}, …)
- **tokens**(id `tok_`/`key_`, user_id nullable, org_id nullable, kind∈{personal,org_key}, scope, hash, prefix, expires_at, last_used_at) — personal + org keys share the table; **plaintext never stored** · **secrets**(versioned, KMS-envelope, secret_ref)
- **policies**(id `pol_`, org_id, project_id nullable, env nullable, key, enforcement, config jsonb, version, last_change_event) · **bindings**(+target_type/provider/secret_ref, ADR-0004)
- **events**(id `evt_`, org_id, actor, via∈{user,assistant,system}, action, subject, at, detail) — append-only, idx(org_id, at desc); serves `/events` + `/audit`
- **Service accounts / machine identity:** modeled as **org API keys** + (V2) **federated OIDC identities** (issuer + subject-claim mapping → env-scoped role) — no separate "robot user" table needed; a machine is a token or a federated subject, not a User.

## 12 · API design — REST + SSE, contract-first (confirms Architecture v1.2 §9)

REST over `/v1` (the auth section added per S1, contract-first); no GraphQL/gRPC. Auth endpoints: sessions (sign-in/out, MFA, passkeys, recovery), `/me/tokens`, `/orgs/{org}/api-keys`, `/orgs/{org}/members` + invites, `/orgs/{org}/policies` (+`?dry_run=true`), `/orgs/{org}/audit`, `/orgs/{org}/oidc-federation` (V2), `/orgs/{org}/sso` (V3). **Events (audit) stream over SSE** (`x-streamable`) — no new protocol. Every endpoint enforces the two-layer evaluator; denials are problem+json naming the role/policy (the E3 grammar).

## 13 · Security review — the design's threat model, mostly already answered

| Threat | Defense (mostly already in the design) |
|---|---|
| Session fixation | Rotate session id on privilege change/login; server-side sessions |
| CSRF | SameSite cookies + token on state-changing requests; SSE is GET |
| JWT misuse | Sessions are server-side (opaque), not client JWTs; federated OIDC JWTs are validated (issuer + claims + audience) at the boundary and exchanged for our own scoped token |
| Token leakage | Reveal-once + hash-stored; short-lived federated tokens over long-lived keys (§3); `last_used_at` anomaly surface |
| Privilege escalation | Two-layer evaluator (N is final, policy can only narrow); tokens re-evaluate against *current* roles at use time |
| Confused deputy | **The AWS/Databricks external-ID pattern** for any cross-account/federation trust; OIDC claim gating (`sub`/`environment`); the AI plane's no-apply-path (Law 1) |
| Tenant isolation | org→project→env namespaces, default-deny NetworkPolicies, gVisor (D7); every query org-scoped; foreign-org id → 404 (not 403 — no existence leak) |
| Replay | Short token lifetimes; nonce on federation; idempotency keys (S7) |
| Secret rotation | Secrets versioned, KMS-envelope; bindings rotate on unbind; federation removes the rotation problem for CI |
| Least privilege | Scoped tokens; env-scoped federation; policy narrowing; workload identity (invariant 11) |
| Supply chain | Signed images + provenance (invariant 11); gitleaks/govulncheck (Architecture v1.2 §17) |
| Cross-account trust | Deferred with BYOC to v3 (ADR-0005); when it comes, least-privilege tag-scoped role + external-ID |

The design is defensively strong because authZ is centralized (one evaluator to audit), audit is immutable (one spine), and the machine-identity path moves from stored secrets to short-lived federation.

## 14 · Business review

- **Onboarding:** owned auth + passkeys-first = fast, no third-party redirect; org/project model is the product from minute one.
- **Enterprise sales:** SSO/SCIM (WorkOS) is the standard unlock at v3; audit + policy + spend-cap governance is the security-review answer; priced honestly (anti-SSO-tax) it *builds* the trust brand.
- **Compliance:** the events spine is the audit evidence; SOC 2 at v3.
- **Support:** centralized authZ + explainable denials ("why was this denied") cut the #1 support category (access confusion).
- **Pricing:** identity is never a per-MAU cost we pay (owned); SSO is a per-connection enterprise line (v3).
- **Self-hosting / BYOC:** the governed-resource contract + owned identity travel to a BYOC cell unchanged (the control plane — identity included — stays at Steloit; only the data plane relocates, ADR-0005). Owning identity is what *makes* BYOC's "same everywhere" true.
- **DX:** federation for CI, passkeys for humans, scoped tokens, and "why was this denied" explainability are all developer-loved.

## 15 · Recommendation — ✅ Confirm and codify (smallest change set)

**The existing Identity & Governance design is correct. Keep it. Codify it as ADR-0006 (the governed-resource contract as the canonical answer to "designed once, inherited by all"), with four sharpenings and one net-new capability:**

1. **AuthZ = RBAC + narrowing-policy (the AWS SCP model). Reject Zanzibar/ReBAC** as premature (a separate stateful service solving a graph problem Steloit doesn't have). *(Confirm.)*
2. **Policy = typed-Go evaluator over the finite platform set; reserve Cedar (cedar-go) for v3 customer-authored policies.** Keep "why was this denied" explainability load-bearing. *(Clarify.)*
3. **AuthN = own the model + libraries (`go-webauthn`, argon2id, TOTP), passkeys-first; WorkOS for SSO at v3** (federates into our model), Dex/Keycloak only at scale; never buy Clerk/Auth0/Cognito as the primary store. *(Confirm E2/E7; refine "Dex-first" → "WorkOS-first.")*
4. **Roles fixed now, custom roles v3; audit export/streaming v3; enterprise SSO/SCIM v3, priced honestly.** *(Confirm; anti-SSO-tax stance.)*
5. **NET-NEW: CI/OIDC workload-identity federation (V2)** — Steloit as an OIDC relying party so machines federate into short-lived scoped tokens with zero stored keys, mapped onto the tree via OIDC claims. The one genuine addition; a differentiator; add early.

**Rejected alternatives (on the record):** Zanzibar/OpenFGA/SpiceDB (graph engine for a tree problem; separate stateful service); Cedar/OPA/Rego *now* (DSL for a finite policy set); Clerk/Auth0/Cognito/Firebase as primary store (SaaS in the trust path, per-MAU tax, owns your users); a separate authorization *service* (the Zanzibar operational tax without the benefit).

## 16 · Ripple (small — confirm + one capability + clarifications)

**Migration effort: ~zero code (nothing built); ~2 hrs doc-work.**

- **ADR-0006** (new, `docs/adr/`): the Identity & Governance model — the governed-resource contract; RBAC+narrowing-policy (SCP-validated); typed-Go policy + Cedar-reserved; owned auth + WorkOS-SSO-v3; CI/OIDC federation (V2); centralized-in-the-identity-module. Product ADR log pointer (ADR-036).
- **Context pack `rbac.md`:** add the governed-resource contract, the AWS-SCP framing, typed-Go/Cedar-reserved, and the CI/OIDC-federation note; keep the "why was this denied" requirement explicit.
- **Roadmap:** place the pieces — V1 owned auth (S1) + passkeys + TOTP; **V2 CI/OIDC federation** + org API keys; V3 SSO/SCIM (WorkOS) + custom roles + audit export/streaming + IP allowlists.
- **E7 note:** refine "Dex for SSO" → "WorkOS for SSO (v3), Dex/Keycloak at scale"; passkeys-first.
- **New task:** CI/OIDC federation (V2) under E7/identity; light.
- **No architecture change** (identity module already in v1.2), **no enum/product-surface change**, **no INF-001 amendment** (identity was never under-specified there), **no frozen-surface change.**

**What does NOT change:** the two-layer evaluator, the roles, the policy primitive, the events spine, owned auth, the wedge, the product surface, Architecture v1.2. This review confirms the design and adds one capability the evidence says to add early.
