---
id: US-3.3g
title: "A plan change does not reach the cell until each service is next touched"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
issue: 0
labels: [Platform, Backend]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/internal/provisioning/**
  - services/api/internal/identity/**
  - tasks/e3-provisioning/US-3.3g.md
verify:
  - "upgrading an org's plan updates the desired doc of every service in every environment it owns"
  - "cd \"$(git rev-parse --show-toplevel)/services/api\" && go test -race ./..."
owner: agent
---

## The gap

US-3.3e resolves the org's plan to a quota envelope and ships the VALUES in each
service's desired doc — the right boundary, since the cell-agent must not hold a
copy of `plans.json`.

But the desired doc is rebuilt when a **service** changes, not when an org's
**plan** changes. So an upgrade from `pro` to `business` does not reach the cell
until each service in each environment is next touched, and until then the
namespace keeps `pro`'s ResourceQuota. A customer who upgrades to get more
headroom does not get it, and nothing says so.

The direction matters: an upgrade under-provisions (annoying, invisible), a
**downgrade leaves the larger envelope in place** (we bill for the smaller plan
and enforce the bigger one).

## Why it was not fixed in US-3.3e

It is a control-plane concern — a plan-change hook that rewrites the desired docs
of every service the org owns — and it needs the transaction and event semantics
of the subscription path, not the rendering path. Recorded rather than hidden.

## Acceptance criteria

1. Changing `orgs.plan` (or the subscription that drives it) rewrites the desired
   doc of every service in every environment the org owns, in one transaction.
2. The generation is bumped so the reconciler actually re-polls them.
3. A test upgrades a plan and asserts the rendered ResourceQuota changes for a
   service that was NOT otherwise touched.
4. A test downgrades and asserts the envelope shrinks — the direction that
   currently leaves us enforcing more than we bill.
5. Mutation-verified on a GREEN-baseline harness.

## Read first

- `services/api/internal/provisioning/services.go` (`envelopeFor`, `desiredDoc`)
- `services/api/internal/identity/store/subscription.sql.go` (the plan master)
