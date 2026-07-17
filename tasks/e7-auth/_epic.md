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

MFA (TOTP+WebAuthn), recovery codes, session list/revoke, password-reset emails, org API keys with scopes, invite emails + renew/decline (S3 fixes), leave-org/account-delete grace windows. Closes ADR-draft 7.4 so the console's A/P planes integrate without the seam. (implementation-plan §5 E7)

> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.
