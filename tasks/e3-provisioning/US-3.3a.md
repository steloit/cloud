---
id: US-3.3a
title: "Nothing creates the env namespace (nor its D7 default-deny policies)"
epic: E3
status: ready
phase: MVP
priority: high
sprint: 4
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/api/internal/provisioning/**
  - services/cell-agent/internal/**
  - infra/modules/**
verify:
  - "creating an environment yields a namespace with default-deny NetworkPolicy, ResourceQuota and LimitRange"
  - "a service in a brand-new environment converges without a hand-created namespace"
owner: agent
---

## Goal

An environment's Kubernetes namespace — and the isolation D7 requires — must be
created by the system, not by a human running a runbook.

## Why — found in US-3.3's review

US-3.3 renders into `desired.namespace`, but **nothing creates that namespace**:
not the control plane, not the agent, not terraform (`module.cnpg` creates only
`control_plane`). The live e2e worked because the runbook did
`kubectl create ns` in preflight. On a real cell a genuinely new project/env
would 404 on first apply — and, until US-3.3's ErrNotConverged fix, would have
retried forever silently.

D7 also requires the namespace to carry **default-deny NetworkPolicies, a
ResourceQuota and a LimitRange** — the tenant isolation boundary. None exist.
US-3.3 fixed the namespace *collision* (env-id derived) but not its *creation*
or *policing*, so isolation is nominal until this lands.

## Acceptance criteria

- [ ] Creating an environment creates its namespace (control-plane-side at env
  creation, or agent-side as part of converge — decide and record why).
- [ ] The namespace carries default-deny NetworkPolicy, ResourceQuota, LimitRange
  (D7). Egress/ingress defaults stated, not implied.
- [ ] A service in a brand-new environment converges with no hand-created
  namespace; proven without the runbook's preflight.
- [ ] Deleting an environment removes the namespace (and nothing else's).

## Related

US-3.3 (found it) · INF-001 D7 · `infra/modules/cnpg`
