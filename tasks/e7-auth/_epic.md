---
id: E7
title: E7 · Auth hardening & org surface completion
epic: E7
status: stub
phase: V1
priority: high
sprint: 7
estimate: 4ew
deps: [E2]
issue: 8
labels: [Backend, Security]
module: M2 Identity
contexts: []
owner: founders
---

## Scope

MFA (TOTP+WebAuthn, **passkeys-first with fallback** — ADR-0006), recovery codes, session list/revoke, password-reset emails, org API keys with scopes, invite emails + renew/decline (S3 fixes), leave-org/account-delete grace windows. Closes ADR-draft 7.4 so the console's A/P planes integrate without the seam. Own the model, adopt libraries (`go-webauthn`/argon2id/TOTP) — never buy Clerk/Auth0 as the primary store. **Enterprise SSO/SCIM = WorkOS at v3** (federates into our model), Dex/Keycloak only at scale — *not* Dex-first (ADR-0006 refines the earlier note). **CI/OIDC workload-identity federation is a V2 addition (E7-1).** (implementation-plan §5 E7)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
