---
id: US-3.8
title: "An instance override changes real capacity with no reprice and no budget check"
epic: E3
status: in-progress
phase: MVP
priority: high
sprint: 4
estimate: 0.5ew
deps: []
issue: 0
labels: [Backend, Billing]
module: M4 Provisioning
contexts: [provisioning, api-conventions]
files:
  - services/api/internal/provisioning/services.go
  - services/api/internal/identity/services_integration_test.go
  - tasks/e3-provisioning/US-3.8.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## The defect

`UpdateService` sets `params.Override` OUTSIDE the `if shape != nil` reprice
branch, so an override is applied without repricing and without the hard spend
cap that every other capacity change must pass. The cell renders exactly that
count (`cell-agent/internal/render/cnpg_renderer.go` reads the override).

Reproduced during US-3.7's review, against real Postgres:

```
create postgres           → monthly_estimate_cents 1900
PATCH {"override":{"instances":9,"reason":"load test"}} → 200
                          → monthly_estimate_cents STILL 1900
                          → desired doc carries instances: 9
```

Nine instances provisioned, one instance billed.

## Why it matters

It is the same law US-3.7 just strengthened, pointing at the update path: what
runs must be what was priced. A shape change reprices and re-checks the cap
(services.go, the scale-up branch); an override does neither, so the cap is
bypassable by anyone who can PATCH a service — the exact bypass the scale-up
comment says it exists to prevent.

Whether an override SHOULD reprice is the design question. Two readings:

1. **An override is capacity** — reprice it like a shape change and enforce the
   cap. Simplest, and consistent with "metering follows what runs".
2. **An override is an operator escape hatch** — deliberately un-priced, for
   incident response. Then it needs an authorization boundary (who may set one),
   an audit edge, and a bound, none of which exist today.

Decide with the founder if it is not obvious; do not silently pick (1).

## Acceptance criteria

- [ ] An override that raises capacity either reprices and passes the spend cap,
  or is refused for callers without an explicit permission — per the decision.
- [ ] The billed amount and the rendered capacity agree after any override.
- [ ] Mutation-verified: reverting the guard reproduces the 9-instances-1-billed
  result above.
- [ ] The audit spine records the override with its reason (it already carries
  `reason`; check it reaches the event).

## Found by

US-3.7's architecture review (2026-07-26), reproduced live.

## Related

US-3.7 (the same law on the create path) · T11.6 (the hard spend cap) · D10
