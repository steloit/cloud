---
id: US-3.3l
title: "cnpg.io/cluster is now a capability label, and nothing owns who may set it"
epic: E3
status: ready
phase: MVP
priority: medium
sprint: 4
issue: 0
labels: [Platform, Security]
module: M4 Provisioning
contexts: [provisioning]
files:
  - services/cell-agent/internal/driver/tenancy/**
  - services/api/internal/provisioning/**
  - tasks/e3-provisioning/US-3.3l.md
verify:
  - "a pod that is not operator-managed cannot obtain the CNPG allowances by carrying cnpg.io/cluster, asserted rather than argued"
  - "cd \"$(git rev-parse --show-toplevel)/services/cell-agent\" && go test -count=1 -race ./..."
owner: agent
---

## The shape US-3.3c created

`allow-cnpg-egress` selects `cnpg.io/cluster` (Exists) and grants that pod three
things default-deny withholds from everything else in the namespace: the
**metadata server** (169.254.169.254 — Workload Identity), the **kube-apiserver**,
and **public TCP/443**.

That makes `cnpg.io/cluster` a **capability label**. Any pod carrying it obtains
the CNPG allowances, and nothing at admission decides who may carry it.

## Not reachable today — and why that is not the same as safe

Checked, not assumed:
- nothing renders customer pods (only the CNPG driver is wired into
  `selectRenderer`; `valkey.Driver` exists but no code path constructs it),
- customers have no Kubernetes API access,
- every rendered object name passes through `dnsName`, so no label injection is
  possible from `svc.ID` or `Shape`.

So there is no path from a customer to that label right now. What changed is that
one exists to be found: **the first driver that renders a pod from a
customer-influenced value, or any future path granting pod-create inside
`env-*`, turns a label into self-service privilege escalation** — metadata server
included, which is what AC 9 exists to deny customer code.

The selector is still the right mechanism (US-3.3c measured that
`cnpg.io/podRole` misses the bootstrap Job and fences the cluster). The gap is
that nothing owns the label's *provenance*.

## Acceptance criteria

1. A decision is recorded on who may set `cnpg.io/cluster` inside an environment
   namespace — admission policy, a reserved-label refusal in the renderer, or an
   explicit "operator-managed namespaces only" statement with its reasoning.
2. Whatever is chosen is ASSERTED: a test shows a pod that is not
   operator-managed does not obtain the CNPG allowances.
3. If the answer is "unreachable, accepted", it is recorded where the next
   author will read it — the tenancy package doc — with the three checks above
   restated, so the acceptance is re-checkable rather than remembered.

## Found by

US-3.3c's architecture review (2026-08-25), which noted it as a follow-up
rather than a change to that PR: reachable today by nobody, and a shape worth
owning before something makes it reachable.
