---
id: US-3.3i
title: "The storage envelope cannot be enforced until the API can predict the PVC the cell will create"
epic: E3
status: blocked
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform, Backend]
module: M4 Provisioning
contexts: [provisioning, billing]
deps: [T3.4c]
files:
  - services/api/internal/provisioning/**
  - services/api/internal/estimates/**
  - services/cell-agent/internal/driver/tenancy/**
  - services/cell-agent/internal/driver/cnpg/**
  - tasks/e3-provisioning/US-3.3i.md
verify:
  - "a create that would exceed the environment's storage envelope is refused at the API with remediation"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -count=1 -race ./internal/provisioning/ ./internal/estimates/"
owner: agent
---

## Why the storage half of the envelope is withheld

US-3.3e renders the founder's per-environment envelope as a ResourceQuota on
`requests.cpu` and `requests.memory`. **`requests.storage` is deliberately NOT
rendered**, and this task owns turning it on.

A ResourceQuota is enforced by the API server at admission. If the cell can
refuse a PVC, the control plane must be able to refuse the ORDER — otherwise the
API prices a shape, returns 201, starts billing it, and the service sits in
`provisioning` forever with no writeback and no alert. Measured on `task/US-3.3e`
before the withdrawal:

- a **free** org (10 GiB envelope) creating a `standard` postgres → **201**, and
  the cell renders a 32Gi PVC that admission rejects;
- the same org creating a **second `dev`** → **201**, 10Gi + 10Gi against 10Gi.

Free's envelope is exactly one `dev` PVC. That is a consequence of the founder's
ruled numbers, recorded in `docs/founder-config.md` §5 so it can be revisited —
but selling what cannot be built is not acceptable at any numbers.

## Why it cannot simply be gated today

**The API does not know what PVC the cell will create.** Measured on that branch:
`estimates.Resolve` returns `storage_gb: 0` for `dev`, `standard` AND
`performance`, while `cnpg.storageForShape` renders `10Gi`, `32Gi` and `128Gi`
from a size table the DRIVER owns. There is no number in the control plane to
compare against the envelope.

Copying that table into `services/api` would put a data-plane sizing rule in the
control plane — the boundary the two-plane split exists to keep (ADR-0001 D6),
and the same duplication class that produced `shape["storage"]` vs `storage_gb`
in the first place.

## What has to be true first

1. **T3.4c** — `storage_gb` actually sizes the PVC, so the priced number and the
   provisioned number are one number. (`deps`.)
2. **The minimum is catalog-owned.** Even with T3.4c, `dev` resolves to
   `storage_gb: 0` while the driver applies `minVolumeGB = 10`. That floor must
   come from the catalog both planes read, not from a constant in the driver —
   T3.4c's own mistake-bank entry says exactly this ("bind the driver to the
   catalog with a test that READS `pricing.json`").

## Acceptance criteria

1. The rendered PVC size is derivable in the control plane from the catalog, with
   a test that fails if the driver and the catalog disagree for ANY size.
2. `requests.storage` is rendered again, and `quota_test.go`'s pinned absence is
   replaced by a pinned presence.
3. A create whose PVCs would exceed the environment's envelope is REFUSED at the
   API layer — before the one-shot estimate is burned, like the spend cap — with
   problem+json carrying a `remediation` that names the arithmetic.
4. The same gate on the resize path, excluding the service's own current usage so
   a resize that fits is not refused.
5. A test asserts the two cases measured above: a free org cannot order a
   `standard`, and cannot order a second `dev`.
6. Mutation-verified on a GREEN-baseline harness, including the gate ignored at
   each call site.

## Found by

US-3.3e's architecture review, 2026-08-24, which reproduced both 201s live.
