---
id: US-3.3j
title: "allow-cnpg-operator-ingress admits the whole cnpg-system namespace, and narrowing it needs a live cell"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
estimate: 0.25ew
deps: []
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/tenancy/**
  - tasks/e3-provisioning/US-3.3j.md
verify:
  - "the operator ingress peer names the operator's pod label, and a managed postgres still reaches ready on a live cell with enforcement on"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
owner: agent
---

## The gap

`allow-cnpg-operator-ingress` admits ingress on port 8000 from
`namespaceSelector: kubernetes.io/metadata.name: cnpg-system` with **no
podSelector** — every pod in that namespace, not just the operator.

US-3.3c's own structural test (AC 3) forbids exactly this shape and **caught
it**; it passes today only because that one policy is a NAMED exception in
`TestEveryAllowPeerIsStructurallyNarrow`. The exception is argued in the test,
not hidden.

## Why it was not just fixed

The narrowing could not be **verified**. The live cell had already been destroyed
when the test found it, and the operator's pod label was therefore unconfirmed —
`app.kubernetes.io/name: cloudnative-pg` is the chart's documented label, but
shipping a selector on an unverified label fences the operator off the instance
manager and every managed Postgres stops reaching ready.

An unverified tightening of a security control is worse than a named, owned
exception: it looks stronger and can take the product down. US-3.3c's whole
premise is that a policy which *looks* right and is not enforced (or fences what
it must not) is the failure mode to avoid.

## Blast radius today

`cnpg-system` is created by our own Helm release (`module.cnpg`) and contains
only the CNPG operator, so "every pod in that namespace" is "the operator we
installed". Real, bounded, and smaller than it reads — but wider than the rule
this policy set enforces everywhere else.

## Acceptance criteria

1. The peer carries a podSelector naming the operator, in the SAME peer as the
   namespaceSelector (an AND, not an OR).
2. The label is READ FROM A LIVE OPERATOR, not from documentation — record the
   `kubectl get pods -n cnpg-system --show-labels` output in the PR.
3. A managed Postgres reaches `Cluster in healthy state` on a live cell with
   enforcement on, AFTER the narrowing. This is the assertion that the change is
   safe; without it this task has not been done.
4. The named exception is removed from `TestEveryAllowPeerIsStructurallyNarrow`,
   so the general rule covers every policy again.

## Read first

- `services/cell-agent/internal/driver/tenancy/tenancy.go` — the policy set
- `docs/dev/us33c-live-evidence.md` — how the live verification was run and torn down
