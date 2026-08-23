---
id: US-3.3c
title: "The D7 NetworkPolicy allow-set: land it on a real cell, and prove a pod cannot cross an environment"
epic: E3
status: blocked
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
deps: [US-3.3f]
files:
  - services/cell-agent/internal/driver/tenancy/**
  - infra/k8s/policy/**
  - docs/plan/**
  - tasks/e3-provisioning/US-3.3c.md
verify:
  - "a pod in env A cannot open a connection to a pod in env B, on a live cell"
  - "tenancy.Render refuses a policy carrying endPort"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
owner: agent
---

## Why this exists as a file

**It was being cited as an owner before it was one.** ADR-0015 and US-3.3f
delegate four obligations to "US-3.3c", and US-3.3a's withdrawal of the D7
policies pointed here too — while no such task existed. One of those citations
asserts, in the present tense, that "US-3.3c carries an AC that `tenancy.Render`
must refuse a policy carrying `endPort`, so the constraint has an owner where
policies are written rather than living in prose." It had no owner and lived
exactly in the prose that sentence disclaims. Filed so the citations resolve.

Blocked on **US-3.3f** (enforcement) and on a cell existing: verified
2026-08-23, `gcloud container clusters list --project steloit-dev` returns `[]`.

## The four obligations, collected

1. **The allow-set itself.** US-3.3a shipped default-deny NetworkPolicies and
   withdrew them: nothing enforced them, AND the set as written denies what CNPG
   requires — the metadata server (Workload Identity), GCS (WAL archiving) and
   the apiserver (the in-pod instance manager). Turning enforcement on with that
   set fences the first Postgres pod before it reaches ready.
2. **`endPort` must be REFUSED, not written.** Dataplane V2 silently does not
   enforce port RANGES on affected versions — the same class of defect ADR-0015
   exists to close, a policy the API server accepts and does not apply. The
   refusal belongs in `tenancy.Render`, where policies are produced, so it cannot
   be forgotten by a caller.
3. **Customer code blocked from `169.254.169.254`, managed CNPG allowed.** GKE
   Sandbox's own documentation recommends NetworkPolicy as the control for
   blocking the metadata server from sandboxed pods, while CNPG *requires* it for
   Workload Identity. Different pools, different policies — this is a design
   point, not a single rule.
4. **NetworkLogging over gVisor pods is undocumented either way.** ADR-0015's
   first stated reason for choosing Dataplane V2 over Calico is denied-connection
   logging; whether it covers sandboxed pods must be confirmed on the first real
   cell rather than assumed.

## Acceptance criteria

1. A pod in env A cannot open a connection to a pod in env B. On a live cell,
   measured — not a rendered manifest. This is the assertion `terraform test`
   structurally cannot make.
2. CNPG reaches `ready` with the allow-set applied: metadata server, GCS and
   apiserver all reachable from the managed pool.
3. Customer-code pods CANNOT reach `169.254.169.254`; managed CNPG pods can.
4. `tenancy.Render` refuses a policy carrying `endPort`, with a test.
5. `infra/k8s/policy/network-logging.yaml` is APPLIED by something, and denied
   connections appear in logs. It is authored and wired to nothing today.
6. NetworkLogging's coverage of gVisor pods is confirmed and recorded either way.

## Read first

- `docs/adr/0015-cell-datapath-dataplane-v2.md` (accepted known issues)
- `services/cell-agent/internal/driver/tenancy/tenancy.go` (why the policies were
  withheld; what US-3.3e reinstated and what it did not)
- commit `7e94f26` — the withdrawn allow-set, and exactly what it denied
