---
id: US-3.18
title: "Valkey is priced by memory and renders no resources — and a valkey create can never converge"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 3
estimate: 0.5ew
deps: []
issue: 0
labels: [Backend, Billing, Reliability]
module: M3 Provision
contexts: [provisioning, billing]
files:
  - services/cell-agent/internal/driver/valkey/**
  - services/api/internal/provisioning/**
  - tasks/e3-provisioning/US-3.18.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
owner: agent
---

## The same defect as US-3.16, already in committed code

`pricing.json` charges valkey `memory_cents_per_gb: 2200` against the shape's
`memory_mb`. `driver/valkey` renders a StatefulSet with **no `resources:` block** —
memory is read by nothing. It also hardcodes `replicas: 1` and ignores
`driver.Spec.Instances` entirely, so the count US-3.16 now computes is dropped by
the second product.

The package has **no test file at all**. Its only coverage asserts
`len(m) != 0 && m[0].Name == "svcx"`, which would pass against an empty StatefulSet.

## And a valkey service can never reach ready

`cmd/cell-agent/main.go` wires only `cnpg.New()`. Nothing gates creates: there is
no product allow-list in `createService`, so a `valkey` create is **priced,
accepted, and billed on ready** — while `CNPGRenderer.Converge` returns
`cnpg: not a postgres product` forever. Priced, accepted, unprovisionable.

That second half is the more urgent one: it is reachable today through the public
API.

## Acceptance criteria

1. A valkey create is either **refused at the API** with a `remediation` naming
   what is supported, or it converges. Not accepted-and-stranded. Decide which,
   and if refusal: the allow-list is derived from the wired drivers, not a second
   hand-maintained list that can disagree.
2. `TestValkeyProvisionsTheMemoryItCharges` — `{memory_mb: 1024}` vs
   `{memory_mb: 4096}` must differ in the container's `resources.requests.memory`,
   and equal the shape.
3. `TestValkeyRendersTheInstancesItWasGiven` — `Spec.Instances = 3` renders
   `replicas: 3`.
4. The seam guard generalises: `render.TestEveryPricedDimensionMovesItsOwnProvisionedField`
   covers postgres only. Extend the catalog-completeness check to **every product
   in `pricing.json`**, so `web`/`worker` are covered before their driver exists.
   They are the only products priced **per instance** (`instance_cents`), which
   makes them the highest-exposure case of this class when that driver lands.

## Notes

Found by the US-3.16 QA pass. The bug class is now named and guarded for postgres;
this is the same class in the product next door, and the guard should be what
prevents the third instance rather than another review catching it.
