---
id: US-3.9
title: "Opaque shape fields have no sub-schema, so an observation can become contract-binding"
epic: E3
status: ready
phase: MVP
priority: low
sprint: 5
estimate: 0.5ew
deps: [US-3.7]
issue: 0
labels: [Backend]
module: M4 Provisioning
contexts: [provisioning, api-conventions]
files:
  - services/api/internal/estimates/engine.go
  - services/api/internal/estimates/engine_test.go
  - tasks/e3-provisioning/US-3.9.md
verify:
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go build ./... && go vet ./... && go test ./..."
owner: agent
---

## The problem

US-3.7 made every declared shape field part of the contract identity. Two of
them — postgres `connections` and `pgmq` — are `opaque`: carried through
unexamined and compared structurally, because canon models them as objects.

But canon models `connections` as **observed runtime data**
(`{"used": 192, "max": 200}`), while the schema now treats the whole object as
**contracted configuration**. So `used` — an observation — sits inside the
contract identity.

Harmless today: no client sends `connections`, and estimate and create are
symmetric, so nothing false-refuses.

## The trap

It springs the moment anything on the observe path writes a live
`connections.used` back into `services.shape` — which is the natural place to
put it, precisely because canon's fixture shows it there. From then on:

- every template capture and every clone freezes an observation into a contract;
- the identity check starts refusing legitimate creates non-deterministically,
  because `used` differs between the estimate and the create.

## The fix — declared sub-schemas, not opaque

Stop `opaque` meaning "we don't know what is in here". Give these fields a
declared sub-schema and reject unrequestable sub-keys **on input**:

- `connections` accepts `{max}` (a real knob — postgres `max_connections`) and
  REJECTS `used`;
- `pgmq` accepts `{delivery, dlq}`, and someone decides whether `dlq_depth` is a
  requested bound or an observed depth.

This is not a contract change: the closed schema already rejects unknown
top-level keys, and this is the same rule one level down. Response shapes,
`openapi.yaml` and canon are untouched. The observation then cannot enter the
contract by construction, and the identity keeps covering every requested field.

Rejected alternatives: leaving it (accepts the trap); dropping `connections`
from the identity (reopens the substitution hole asymmetrically — `pgmq` stays,
`connections` does not); splitting observed-vs-requested in the shape schema (an
owner-level contract change to a frozen authority).

## Acceptance criteria

- [ ] `connections` and `pgmq` have declared sub-schemas; an unrequestable
  sub-key is refused at the API with a field-level validation error.
- [ ] The contract identity covers every requested sub-key.
- [ ] The `shapeSchema` comment no longer describes `connections` as canon's
  observation shape.
- [ ] Mutation-verified: a sub-key removed from the identity fails a test.

## Precondition

**Any observe-path work that writes into `services.shape` must land this first.**
That write is the event that turns this from tidy into a bug.

## Found by

US-3.7's architecture review (2026-07-26).

## Related

US-3.7 (introduced the opaque fields) · `docs/product/19-canon/fixtures.json`
